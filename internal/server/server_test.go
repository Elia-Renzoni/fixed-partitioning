package server_test

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/fixed-partitioning/internal/model"
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

func prepareClientRequest(storeRouter string) ([]byte, error) {
	req := model.TCPRequest{}
	req.Key = []byte("foo")
	if storeRouter == model.ClientAdd {
		req.Value = []byte("bar")
	}
	req.RequestType = "client"
	req.StoreRouter = storeRouter

	return json.Marshal(req)
}

func prepareJoinRequest(addr string) ([]byte, error) {
	req := model.TCPRequest{}
	req.RequestType = "join"
	req.NodeAddress = addr
	return json.Marshal(req)
}

func makeTCPRequest(data []byte, dataLen int, address string) (model.TCPResponse, error) {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return model.TCPResponse{}, err
	}
	defer conn.Close()

	conn.Write(data)

	buf := make([]byte, 2048)
	n, _ := conn.Read(buf)
	res := &model.TCPResponse{}
	err = json.Unmarshal(buf[:n], res)
	if err != nil {
		return model.TCPResponse{}, err
	}

	return *res, nil
}
