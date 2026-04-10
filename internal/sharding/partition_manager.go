package sharding

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"maps"
	"math/rand"
	"slices"
	"sync"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots         int
	clusterLen        int
	cluster           *replication.Cluster
	pTable            map[int][]string
	hasher            hash.Hash64
	perNodeSlots      map[string]int
	mutex             sync.RWMutex
	replicationFactor int
	optimalPartitions int
	chunksCh          chan []byte
	quitCh            chan struct{}
}

const minClusterLen int = 4

var ErrLackOfNodes = errors.New("unable to assign partition due to lack of nodes")

func NewPartitionTable(slots, rp int, cluster *replication.Cluster) *PartitionTable {
	return &PartitionTable{
		hashSlots:         slots,
		pTable:            make(map[int][]string),
		cluster:           cluster,
		hasher:            fnv.New64(),
		replicationFactor: rp,
	}
}

// only the coordinator can call this method
func (p *PartitionTable) AssignPartitions() error {
	if p.cluster.Len() < minClusterLen {
		return ErrLackOfNodes
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	var errs = p.hashSlots

	for slot := range p.hashSlots {
		partitionIndex := slot % p.cluster.Len()
		attachedNode := p.cluster.GetNodeFromLocation(partitionIndex)
		if attachedNode != "" && !p.nodeAlreadyPresent(p.pTable[partitionIndex], attachedNode) {
			p.pTable[partitionIndex] = append(p.pTable[partitionIndex], attachedNode)
			rfNodes := p.completeNodesWithRF(attachedNode)
			if rfNodes != nil {
				copy(p.pTable[partitionIndex], rfNodes)
			}
		}
		errs--
	}

	totalAssignments := len(p.pTable) * p.replicationFactor
	p.optimalPartitions = totalAssignments / p.cluster.Len()

	if errs > 0 {
		return fmt.Errorf("assigned only %d partitions", errs)
	}
	return nil
}

func (p *PartitionTable) nodeAlreadyPresent(nodes []string, targetNode string) bool {
	return slices.Contains(nodes, targetNode)
}

func (p *PartitionTable) completeNodesWithRF(attachedNode string) []string {
	if p.replicationFactor > 0 {
		var (
			nodes         = make([]string, 0)
			sprintCounter int
		)

		for {
			nodeID := rand.Intn(p.cluster.Len())
			selectedNode := p.cluster.GetNodeFromLocation(nodeID)
			if selectedNode == attachedNode || slices.Contains(nodes, selectedNode) {
				continue
			}

			sprintCounter += 1
			nodes = append(nodes, selectedNode)
			if sprintCounter >= p.replicationFactor {
				break
			}
		}
		return nodes
	}
	return nil
}

func (p *PartitionTable) ReadPartitionTable() map[int][]string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.pTable
}

func (p *PartitionTable) MergePartitions(table map[int][]string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// in case of the first ever request
	if len(p.pTable) == 0 {
		p.pTable = table
		return
	}

	maps.Copy(p.pTable, table)
}

func (p *PartitionTable) GetPartition(key []byte) int {
	p.hasher.Write(key)
	hash := p.hasher.Sum64()
	partition := int(hash) % p.hashSlots
	return partition
}

func (p *PartitionTable) FindNodes(pId int) []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.pTable[pId]
}

func (p *PartitionTable) FindNodePartitions() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.perNodeSlots = make(map[string]int)

	for _, nodes := range p.pTable {
		for _, node := range nodes {
			p.perNodeSlots[node]++
		}
	}
}

func (p *PartitionTable) GetPerNodePartitions() map[string]int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	mapCopy := make(map[string]int, len(p.perNodeSlots))
	for k, v := range p.perNodeSlots {
		mapCopy[k] = v
	}

	return mapCopy
}

type diff struct {
	nodeAddr       string
	partitionsList []int
	isHighest      bool
	distance       int
}

// getElemInCircularOrder handle lowDiffs as a circular buffer
func getElemInCircularOrder(lowDiffs []diff, index *int) diff {
	if *index == len(lowDiffs)-1 {
		*index = 0
		return lowDiffs[*index]
	}
	*index += 1
	return lowDiffs[*index]
}

// find the nodes with lowest partitions than the average
// find the nodes with the highest partiions than the average
func (p *PartitionTable) RebalancePartitions() {
	p.FindNodePartitions()
	nodePerPartitions := p.GetPerNodePartitions()
	average := p.optimalPartitions

	low := make(chan string)
	lowList := make([]diff, 0)
	high := make(chan string)
	highList := make([]diff, 0)

	go p.filterNodes(nodePerPartitions, low, high, average)

FIND_DELTAS:
	for {
		select {
		case node, ok := <-low:
			if !ok {
				break FIND_DELTAS
			}

			delta := nodePerPartitions[node] - average
			d := diff{
				nodeAddr:  node,
				isHighest: false,
				distance:  delta,
			}
			d.findPartitionsByNodes(p.ReadPartitionTable())
			lowList = append(lowList, d)
		case node, ok := <-high:
			if !ok {
				break FIND_DELTAS
			}

			delta := nodePerPartitions[node] - average
			d := diff{
				nodeAddr:  node,
				isHighest: true,
				distance:  delta,
			}
			d.findPartitionsByNodes(p.ReadPartitionTable())
			highList = append(highList, d)
		}
	}

	p.doBalance(lowList, highList)

	p.chunksCh = make(chan []byte)
	p.quitCh = make(chan struct{})

	go p.movePartitionData()
	go p.fragmentPTable()
}

func (p *PartitionTable) doBalance(lowList, highList []diff) {
	if len(lowList) == 0 || len(highList) == 0 {
		return
	}

	buffPosition := 0
	for _, highElem := range highList {
		for highElem.distance > 0 {
			pId := highElem.partitionsList[highElem.distance]

			nodes := p.pTable[pId]
			// TODO-> add tombostones to avoid aggressive delete operation
			idx := slices.Index(nodes, highElem.nodeAddr)
			if idx >= 0 {
				nodes = slices.Delete(nodes, idx, idx+1)
			}

			lowElem := getElemInCircularOrder(lowList, &buffPosition)
			nodes = append(nodes, lowElem.nodeAddr)
			p.pTable[pId] = nodes

			highElem.distance -= 1
		}
	}
}

func (p *PartitionTable) filterNodes(
	nodePerPartitions map[string]int,
	lowest, highest chan string,
	average int,
) {
	defer close(lowest)
	defer close(highest)

	for node, partitions := range nodePerPartitions {
		if partitions > average {
			highest <- node
		} else if partitions < average {
			lowest <- node
		}
	}
}

func (p *PartitionTable) movePartitionData() {
	defer close(p.chunksCh)
	defer close(p.quitCh)

	aliveNodes := p.cluster.GetAllNodes()
	for {
		select {
		case chunk, ok := <-p.chunksCh:
			if !ok {
				break
			}

			go replication.BroadcastMessage(chunk, aliveNodes)
		case <-p.quitCh:
			return
		}
	}
}

func (p *PartitionTable) fragmentPTable() {
	chunkSize := len(p.pTable) / 4
	if chunkSize == 0 {
		chunkSize = 20
	}

	batch := make(map[int][]string)

	takeBatch := func() {
		reqSingleChunk := model.TCPRequest{}
		reqSingleChunk.RequestType = model.ShardingReq
		reqSingleChunk.StoreRouter = model.ShardingSet
		reqSingleChunk.PTable = batch

		data, _ := json.Marshal(reqSingleChunk)
		p.chunksCh <- data

	}

	for pId, nodes := range p.pTable {
		batch[pId] = nodes
		if len(batch) == chunkSize {
			takeBatch()
			batch = make(map[int][]string)
		}
	}

	if len(batch) > 0 {
		takeBatch()
	}

	p.quitCh <- struct{}{}
}

func (d *diff) findPartitionsByNodes(ptableCopy map[int][]string) {
	for pId, nodes := range ptableCopy {
		if found := slices.Contains(nodes, d.nodeAddr); found {
			d.partitionsList = append(d.partitionsList, pId)
		}
	}
}
