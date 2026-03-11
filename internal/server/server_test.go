package server_test

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
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
	var listeners []*server.Server
	listeners, err = createMultipleListeners(members, pt)
	if err != nil {
		t.Fatal(err)
	}

	for _, listener := range listeners {
		go listener.DoListen()
	}

	time.Sleep(2 * time.Second)

	// test join request
	reqs := make([][]byte, 0)
	for _, node := range members.GetAllNodes() {
		reqData, err := prepareJoinRequest(node)
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, reqData)
	}

	wg := &sync.WaitGroup{}
	for _, dataToSend := range reqs {
		wg.Go(func() {
			// assuming that 127.0.0.1:5050 is the coordinator node
			res, err := makeTCPRequest(dataToSend, "127.0.0.1:5050")
			if err != nil {
				t.Fatal(err)
			}

			if res.Message == "" {
				t.Fatal("invalid response message when performing node JOIN")
			} else {
				t.Log(res.Message)
			}
		})
	}

	wg.Wait()
}

const clusterLen = 6

func createCluster() (*replication.Cluster, error) {
	var c = replication.NewCluster()
	for i := range clusterLen {
		err := c.AddNode(fmt.Sprintf("127.0.0.1:505%d", i))
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func createMultipleListeners(
	members *replication.Cluster,
	pTable *sharding.PartitionTable,
) ([]*server.Server, error) {
	servers := make([]*server.Server, 0)
	for i := range clusterLen {
		port := fmt.Sprintf("505%d", i)
		s, err := server.NewServer("127.0.0.1", port, members, pTable)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
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

func makeTCPRequest(data []byte, address string) (model.TCPResponse, error) {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return model.TCPResponse{}, err
	}
	defer conn.Close()

	conn.Write(data)
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// Signal end of request so the server doesn't block waiting for EOF.
		_ = tcpConn.CloseWrite()
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	buf := make([]byte, 2048)
	n, _ := conn.Read(buf)
	res := &model.TCPResponse{}
	err = json.Unmarshal(buf[:n], res)
	if err != nil {
		return model.TCPResponse{}, err
	}

	return *res, nil
}
