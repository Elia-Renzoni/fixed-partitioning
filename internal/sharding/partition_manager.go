package sharding

import (
	"fmt"

	"github.com/fixed-partitioning/internal/replication"
)

type PartitionTable struct {
	hashSlots  int
	clusterLen int
	cluster    *replication.Cluster
	pTable     map[int][]string
}

func NewPartitionTable(slots int, cluster *replication.Cluster) *PartitionTable {
	return &PartitionTable{
		hashSlots: slots,
		pTable:    make(map[int][]string),
		cluster:   cluster,
	}
}

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
