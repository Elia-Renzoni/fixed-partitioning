package sharding

import (
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"slices"
	"sync"

	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots    int
	clusterLen   int
	cluster      *replication.Cluster
	pTable       map[int][]string
	hasher       hash.Hash64
	averageSlots int
	mutex        sync.RWMutex
}

const minClusterLen int = 4

func NewPartitionTable(slots int, cluster *replication.Cluster) *PartitionTable {
	return &PartitionTable{
		hashSlots: slots,
		pTable:    make(map[int][]string),
		cluster:   cluster,
		hasher:    fnv.New64(),
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

func (p *PartitionTable) ReadPartitionTable() map[int][]string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.pTable
}

type delta struct {
	nodeAddress string

	// diff contains the effective delta between
	// the ideal partitions per node and the real
	// partition per node.
	// example: Ideal -> 10 per node
	//          Real ->  15 per node
	//          Diff ->  -5
	// example: Ideal -> 10 per node
	//          Real -> 3 per node
	//          Diff -> 7
	diff int
}

func (p *PartitionTable) BalancePartitions() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var unbalancedNodes = make([]delta, 0)

	// search the unbalanced nodes
	for _, pNodes := range p.pTable {
		for _, node := range pNodes {
			counter := 0
			for _, assignedNodes := range p.pTable {
				if slices.Contains(assignedNodes, node) {
					counter += 1
				}
			}

			if (counter - 1) != p.averageSlots {
				unbalancedNodes = append(unbalancedNodes, delta{
					nodeAddress: node,
					diff:        p.averageSlots - counter,
				})
			}
		}
	}

	if !(len(unbalancedNodes) > 0) {
		return
	}

	// TODO-> implements the balance logic
}

func (p *PartitionTable) MergePartitions(table map[int][]string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// in case of the first ever request
	if len(p.pTable) == 0 {
		p.pTable = table
		return
	}

	for partitionId, assignedNodes := range p.pTable {
		nodes, ok := table[partitionId]
		// in case of a new entry or update
		if !ok || slices.Equal(nodes, assignedNodes) {
			p.pTable[partitionId] = nodes
		}
	}
}
