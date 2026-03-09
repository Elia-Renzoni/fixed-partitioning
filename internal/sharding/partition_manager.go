package sharding

import (
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots         int
	clusterLen        int
	cluster           *replication.Cluster
	pTable            map[int][]string
	hasher            hash.Hash64
	averageSlots      int
	mutex             sync.RWMutex
	replicationFactor int
}

const minClusterLen int = 4

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

	// compute average slots
	p.averageSlots = p.hashSlots / p.cluster.Len()

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
			nodeID := rand.IntN(p.cluster.Len())
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
