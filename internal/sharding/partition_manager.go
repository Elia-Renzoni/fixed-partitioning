package sharding

import (
	"fmt"
	"hash"
	"hash/fnv"
	"slices"

	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots    int
	clusterLen   int
	cluster      *replication.Cluster
	pTable       map[int][]string
	hasher       hash.Hash64
	averageSlots int
}

func NewPartitionTable(slots int, cluster *replication.Cluster) *PartitionTable {
	return &PartitionTable{
		hashSlots: slots,
		pTable:    make(map[int][]string),
		cluster:   cluster,
		hasher:    fnv.New64(),
	}
}

// only the coordinator can call this method
func (p *PartitionTable) AssignPartitions() []error {
	var errs []error = make([]error, 0)

	for slot := range p.hashSlots {
		partitionIndex := slot % p.cluster.Len()
		attachedNode := p.cluster.GetNodeFromLocation(partitionIndex)
		if attachedNode != "" {
			p.pTable[partitionIndex] = append(p.pTable[partitionIndex], attachedNode)
		} else {
			errs = append(errs, fmt.Errorf("failed matching for %d partiotion", slot))
		}
	}

	// compute average slots
	p.averageSlots = p.hashSlots / p.cluster.Len()
	return errs
}

func (p *PartitionTable) PullAssinedNodes(partitionId int) []string {
	nodes, ok := p.pTable[partitionId]
	if ok {
		return nil
	}
	return nodes
}

func (p *PartitionTable) GenerateKeyHash(key []byte) int {
	p.hasher.Write(key)
	return int(p.hasher.Sum64())
}

func (p *PartitionTable) GetPartitionIDFrom(hash int) int {
	return p.hashSlots % hash
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
