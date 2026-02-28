package replication_test

import (
	"fmt"
	"testing"

	"github.com/fixed-partitioning/internal/replication"
)

func TestBroadcast(t *testing.T) {
	pool := make([]serverFunc, 10)
	cluster := replication.NewCluster()

	// init pool
	for index := range pool {
		pool[index] = server
	}

	for num := range 10 {
		address := fmt.Sprintf("127.0.0.1:505%d", num)
		cluster.AddNode(address)
	}

	br := replication.NewReplicator(cluster)
	br.BroadcastMessage()
}
