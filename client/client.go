package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"time"

	"github.com/fixed-partitioning/internal/model"
)

// pickNodeFromPTable returns the first element of the Partition Table
func pickNodeFromPTable(nodes []string) string {
	if len(nodes) == 0 {
		return ""
	}

	var (
		isGood       bool = false
		selectedNode string
		times        int
	)

	const threshold int = 10
	for !isGood && times < threshold {
		selectedNode = nodes[rand.IntN(len(nodes))]
		if selectedNode == "" {
			times += 1
			continue
		}

		isGood = true
	}

	return selectedNode
}

var exampleDocument = []byte(`{
	           "_id": "65f1a2bc9d1e4f0012345678",
               "name": "Alice Rossi",
               "email": "alice.rossi@example.com",
               "age": 29,
               "active": true,
               "roles": ["user", "editor"],
               "profile": {
                    "bio": "Software engineer",
                    "location": "Pesaro, IT"
                },
                "createdAt": "2026-02-27T10:00:00Z"
	            }`)

func sendClientReq(destinationAddress string, req model.TCPRequest) (model.TCPResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return model.TCPResponse{}, err
	}

	conn, err := net.DialTimeout("tcp", destinationAddress, 2*time.Second)
	if err != nil {
		return model.TCPResponse{}, err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, err = conn.Write(data)
	if err != nil {
		return model.TCPResponse{}, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}

	var res model.TCPResponse
	resBytes, err := io.ReadAll(conn)
	if err != nil {
		return model.TCPResponse{}, err
	}
	if len(resBytes) == 0 {
		return model.TCPResponse{}, fmt.Errorf("empty response from %s", destinationAddress)
	}
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return model.TCPResponse{}, err
	}
	return res, nil
}

func sendSetKVRequest(nodeAddress string, key, value []byte) error {
	req := model.TCPRequest{
		RequestType: model.ClientReq,
		StoreRouter: model.ClientAdd,
		Key:         key,
		Value:       value,
	}

	res, err := sendClientReq(nodeAddress, req)
	if err != nil {
		return err
	}
	if res.Warning != "" {
		return fmt.Errorf("warning from server: %s", res.Warning)
	}

	if msg, ok := res.Message.(string); !ok || msg != "document succesfully added" {
		return fmt.Errorf("unexpected response: %v", res.Message)
	}
	return nil
}

func sendGetPTableRequest(coordAddress string) ([]string, error) {
	req := model.TCPRequest{
		RequestType: model.ShardingReq,
		StoreRouter: model.ShardingGet,
	}

	res, err := sendClientReq(coordAddress, req)
	if err != nil {
		return nil, err
	}
	if res.Warning != "" {
		return nil, fmt.Errorf("warning from server: %s", res.Warning)
	}

	if res.Message == nil {
		return nil, errors.New("empty partition table response")
	}

	msg, ok := res.Message.([]string)
	if !ok {
		return nil, errors.New("empty nodes")
	}

	return msg, nil
}

func main() {
	addr := flag.String("coord", "127.0.0.1:5050", "coordinator address")
	flag.Parse()

	pTableNodes, err := sendGetPTableRequest(*addr)
	if err != nil {
		panic(err)
	}

	node := pickNodeFromPTable(pTableNodes)
	if node == "" {
		panic("epmty node for the requested partition")
	}

	if err := sendSetKVRequest(node, []byte("foo"), exampleDocument); err != nil {
		panic(err)
	}
}
