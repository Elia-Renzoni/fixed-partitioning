package replication_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/replication"
)

func TestBroadcast(t *testing.T) {
	pool := make([]serverFunc, 10)
	cluster := replication.NewCluster()

	// init pool
	for index := range pool {
		pool[index] = server
	}

	var (
		node    serverFunc
		address string
	)
	for num := range 10 {
		address = fmt.Sprintf("127.0.0.1:505%d", num)
		cluster.AddNode(address)
		node = pool[num]
		go node(address)
	}

	time.Sleep(2 * time.Second)

	req := &model.TCPRequest{}
	req.RequestType = "test"
	data, _ := json.Marshal(req)

	acks := replication.BroadcastMessage(data, cluster.GetAllNodes())
	if int(acks) != len(cluster.GetAllNodes()) {
		t.Fatal("invalid ack counter")
	}
}
