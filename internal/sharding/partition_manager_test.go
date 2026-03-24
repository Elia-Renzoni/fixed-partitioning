package sharding_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/sharding"
)

func TestPartitionTable(t *testing.T) {
	members := replication.NewCluster()

	// forcing error generation
	ptable := sharding.NewPartitionTable(500, 0, members)
	if err := ptable.AssignPartitions(); err == nil {
		t.Fatal()
	}

	dec := 6
	helper := 0
	for i := range 30 {
		helper += 1
		if i == 10 || i == 20 {
			dec += 1
			helper = 0
		}
		address := fmt.Sprintf("127.0.0.1:60%d%d", dec, helper)
		members.AddNode(address)
	}
	ptable = sharding.NewPartitionTable(500, 3, members)
	err := ptable.AssignPartitions()
	if err != nil {
		t.Fatal(err)
	}

	for p, nodes := range ptable.ReadPartitionTable() {
		t.Logf("partition: %d - nodes: %s", p, nodes)
	}

	ptable.FindNodePartitions()
	average := ptable.GetPerNodePartitions()

	t.Log("------------ average partitions ----------------")
	for node, p := range average {
		t.Logf("node: %s - partitions: %d", node, p)
	}

	newSlots := map[int][]string{
		500: []string{"127.0.0.1:6060"},
	}

	ptable.MergePartitions(newSlots)

	table := ptable.ReadPartitionTable()
	nodes := table[500]
	if !slices.Contains(nodes, "127.0.0.1:6060") {
		t.Fatalf("got: %s", nodes)
	}

	// test partition shuffle operation
	// for each new cluster node the assigned partitions
	// must decrease.
	members.AddNode("127.0.0.1:1002")
	members.AddNode("127.0.0.1:1003")
	err = ptable.AssignPartitions()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("-----------------------------------------")
	for p, nodes := range ptable.ReadPartitionTable() {
		t.Logf("partition: %d - nodes: %s", p, nodes)
	}

	t.Log("------------ average partitions ----------------")
	for node, p := range average {
		t.Logf("node: %s - partitions: %d", node, p)
	}
}
