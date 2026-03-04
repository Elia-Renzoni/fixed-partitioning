package sharding_test

import (
	"fmt"
	"testing"

	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/sharding"
)

func TestPartitionTable(t *testing.T) {
	members := replication.NewCluster()

	// forcing error generation
	ptable := sharding.NewPartitionTable(500, members)
	if err := ptable.AssignPartitions(); err == nil {
		t.Fatal()
	}

	for i := range 7 {
		address := fmt.Sprintf("127.0.0.1:606%d", i)
		members.AddNode(address)
	}
	ptable = sharding.NewPartitionTable(500, members)
	err := ptable.AssignPartitions()
	if err != nil {
		t.Fatal(err)
	}

	for p, nodes := range ptable.ReadPartitionTable() {
		t.Logf("partition: %d - nodes: %s", p, nodes)
	}
}
