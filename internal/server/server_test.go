package server_test

import (
	"fmt"
	"testing"

	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/server"
	"github.com/fixed-partitioning/internal/sharding"
)

func TestServer(t *testing.T) {
	members, err := createCluster()
	if err != nil {
		t.Fatal(err)
	}
	var pt *sharding.PartitionTable
	pt, err = createPartitionTable(members)
	if err != nil {
		t.Fatal(err)
	}
	var s *server.Server
	s, err = server.NewServer("127.0.0.1", "9090", members, pt)
	if err != nil {
		t.Fatal(err)
	}

	go s.DoListen()
	// TODO-> complete the test
}

func createCluster() (*replication.Cluster, error) {
	const clusterLen = 6
	var c = replication.NewCluster()
	for i := range clusterLen {
		err := c.AddNode(fmt.Sprintf("127.0.0.1:505%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func createPartitionTable(c *replication.Cluster) (*sharding.PartitionTable, error) {
	pt := sharding.NewPartitionTable(200, 3, c)
	err := pt.AssignPartitions()
	return pt, err
}
