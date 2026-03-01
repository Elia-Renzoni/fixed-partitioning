package sharding

import (
	"fmt"
	"hash"
	"hash/fnv"
	"slices"

	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots  int
	clusterLen int
	cluster    *replication.Cluster
	pTable     map[int][]string
	hasher     hash.Hash64
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
