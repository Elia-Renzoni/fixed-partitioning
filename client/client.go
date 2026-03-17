package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/fixed-partitioning/internal/model"
)

type clientConf struct {
}

// pickNodeFromPTable returns the first element of the Partition Table
func pickNodeFromPTable(nodes []string) string {
	return ""
}

func exampleDocument() []byte {
	return []byte(`{
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
}

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

func sendGetPTableRequest(coordAddress string) (map[int][]string, error) {
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

	// Response message decodes into map[string]any; re-decode into a concrete map.
	raw, err := json.Marshal(res.Message)
	if err != nil {
		return nil, err
	}

	var tableJSON map[string][]string
	if err := json.Unmarshal(raw, &tableJSON); err != nil {
		return nil, err
	}

	table := make(map[int][]string, len(tableJSON))
	for k, v := range tableJSON {
		idx, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("invalid partition key %q: %w", k, err)
		}
		table[idx] = v
	}
	return table, nil
}

func main() {
}
