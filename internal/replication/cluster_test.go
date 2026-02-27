package replication_test

import (
	"testing"

	"github.com/fixed-partitioning/internal/replication"
)

func TestCluster(t *testing.T) {
	nodes := []string{
		"172.89.34.23:6060",
		"172.123.32.216:5050",
		"172.44.11.22.1:4040",
	}

	cluster := replication.NewCluster()
	for _, node := range nodes {
		if err := cluster.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
}
