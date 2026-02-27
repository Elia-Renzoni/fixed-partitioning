package replication_test

import (
	"testing"

	"github.com/fixed-partitioning/internal/replication"
)

func TestCluster(t *testing.T) {
	nodes := []string{
		"172.89.34.23:6060",
		"172.123.32.216:5050",
		"172.44.11.22:4040",
	}

	cluster := replication.NewCluster()
	for _, node := range nodes {
		if err := cluster.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}

	if cluster.GetNodeFromLocation(0) != "172.89.34.23:6060" {
		t.Fatal(0)
	}

	if cluster.GetNodeFromLocation(1) != "172.123.32.216:5050" {
		t.Fatal(1)
	}

	if cluster.GetNodeFromLocation(2) != "172.44.11.22:4040" {
		t.Fatal(2)
	}
}
