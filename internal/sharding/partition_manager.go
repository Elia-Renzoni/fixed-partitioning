package sharding

import (
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"maps"
	"math/rand"
	"slices"
	"sync"
	"time"

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
}

const minClusterLen int = 4

func NewPartitionTable(slots, rp int, cluster *replication.Cluster) *PartitionTable {
	return &PartitionTable{
		hashSlots:         slots,
		pTable:            make(map[int][]string),
		cluster:           cluster,
		hasher:            fnv.New64(),
		replicationFactor: rp,
		perNodeSlots:      make(map[string]int),
	}
}

// only the coordinator can call this method
func (p *PartitionTable) AssignPartitions() error {
	if p.cluster.Len() < minClusterLen {
		return errors.New("unable to assign partition due to lack of nodes")
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

	p.optimalPartitions = p.hashSlots / p.cluster.Len()

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

	for _, nodes := range p.pTable {
		for _, node := range nodes {
			p.perNodeSlots[node]++
		}
	}
}

func (p *PartitionTable) GetPerNodePartitions() map[string]int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.perNodeSlots
}

type diff struct {
	nodeAddr       string
	partitionsList []int
	isHighest      bool
	distance       int
}

type deltaList []diff

func (d deltaList) insertOrdered(entry diff, latestTrue *int) {
	if len(d) == 0 {
		d = append(d, entry)
		if entry.isHighest {
			*latestTrue = 0
		}
		return
	}

	if !entry.isHighest && *latestTrue >= 0 {
		// poll the latest true.
		// append the polled element
		// replace the old element with entry
		node := d[*latestTrue]
		d = append(d, node)
		d[*latestTrue] = entry
		*latestTrue += 1
		return
	}

	d = append(d, entry)
}

func (d deltaList) splitList(pivot int) (deltaList, deltaList) {
	lowestList := d[:pivot-1]
	highestList := d[pivot:]
	return lowestList, highestList
}

func (d deltaList) hasNext(index int) bool {
	return index >= len(d)-1
}

func (d deltaList) next(index int) diff {
	return d[index]
}

// find the nodes with lowest partitions than the average
// find the nodes with the highest partiions than the average
func (p *PartitionTable) RebalancePartitions() {
	p.FindNodePartitions()
	nodePerPartitions := p.GetPerNodePartitions()
	average := p.optimalPartitions

	low := make(chan string)
	high := make(chan string)

	go p.filterNodes(nodePerPartitions, low, high, average)

	diffList := make(deltaList, 0)
	pivot := -1

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
			diffList.insertOrdered(d, &pivot)
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
			diffList.insertOrdered(d, &pivot)
		}
	}

	go p.doBalance(diffList, pivot)
}

func (p *PartitionTable) doBalance(diffs deltaList, pivot int) {
	lowestList, highestList := diffs.splitList(pivot)
	i := 0
	for highestList.hasNext(i) {
		highElem := highestList.next(i)
		d := highElem.distance
		for d >= 0 {
			partitionId := highElem.partitionsList[d]
			nodes := p.pTable[partitionId]

			// TODO-> add tombostones to avoid aggressive delete operation
			index, _ := slices.BinarySearch(nodes, highElem.nodeAddr)
			slices.Delete(nodes, index, index)

			// pick a radom node from lowestList
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			lowElem := lowestList.next(r.Intn(len(lowestList)))
			nodes = append(nodes, lowElem.nodeAddr)
			d--
		}
		i++
	}
}

func (p *PartitionTable) filterNodes(
	nodePerPartitions map[string]int,
	lowest, highest chan string,
	average int,
) {
	for node, partitions := range nodePerPartitions {
		if partitions > average {
			highest <- node
		} else if partitions < average {
			lowest <- node
		}
	}

	close(lowest)
	close(highest)
}

func (d *diff) findPartitionsByNodes(ptableCopy map[int][]string) {
	for pId, nodes := range ptableCopy {
		if _, found := slices.BinarySearch(nodes, d.nodeAddr); found {
			d.partitionsList = append(d.partitionsList, pId)
		}
	}
}
