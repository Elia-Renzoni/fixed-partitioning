package server_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/server"
	"github.com/fixed-partitioning/internal/sharding"
)

const (
	clusterLen = 6
	key        = "foo"
	value      = "bar"
)

func TestServer(t *testing.T) {
	members := replication.NewCluster()
	var pt *sharding.PartitionTable
	pt, err := createPartitionTable(members)
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
	for index := range clusterLen {
		address := fmt.Sprintf("127.0.0.1:505%d", index)
		t.Log(address)
		reqData, err := prepareJoinRequest(address)
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, reqData)
	}

	for _, dataToSend := range reqs {
		// assuming that 127.0.0.1:5050 is the coordinator node
		res, err := makeTCPRequest(dataToSend, "127.0.0.1:5050")
		if err != nil {
			t.Fatal(err)
		}

		if res.Message == "" {
			t.Fatal("invalid response message when performing node JOIN")
		} else {
			t.Logf("response: %s", res.Message)
		}

		if res.Warning != "" {
			t.Fatal(res.Warning)
		}
	}

	// test client request
	var data []byte
	data, err = prepareClientRequest(model.ClientAdd)
	if err != nil {
		t.Fatal(err)
	}

	// send a tcp request to the coordinator
	var res model.TCPResponse
	res, err = makeTCPRequest(data, "127.0.0.1:5050")
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != "document succesfully added" {
		t.Fatal(res.Message)
	}

	if res.Warning != "" {
		t.Fatal(res.Warning)
	}

	// send a get request to the coordinator
	data, err = prepareClientRequest(model.ClientFetch)
	if err != nil {
		t.Fatal(err)
	}

	res, err = makeTCPRequest(data, "127.0.0.1:5050")
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != value {
		t.Fatal(res.Message)
	}

	if res.Warning != "" {
		t.Fatal(res.Warning)
	}

	// send a set request to the coordinator
	data, err = prepareClientRequest(model.ClientDelete)
	if err != nil {
		t.Fatal(err)
	}

	res, err = makeTCPRequest(data, "127.0.0.1:5050")
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != "document succesfully removed" {
		t.Fatal(res.Message)
	}

	if res.Warning != "" {
		t.Fatal(res.Warning)
	}
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
	return pt, nil
}

func prepareClientRequest(storeRouter string) ([]byte, error) {
	req := model.TCPRequest{}
	req.Key = []byte(key)
	if storeRouter == model.ClientAdd {
		req.Value = []byte(value)
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

	data, readErr := io.ReadAll(conn)
	if readErr != nil {
		return model.TCPResponse{}, readErr
	}
	if len(data) == 0 {
		return model.TCPResponse{}, fmt.Errorf("empty response from server")
	}

	res := &model.TCPResponse{}
	err = json.Unmarshal(data, res)
	if err != nil {
		return model.TCPResponse{}, err
	}

	return *res, nil
}
