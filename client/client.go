package main

import (
	"encoding/json"
	"net"
	"time"

	"github.com/dolthub/vitess/go/vt/log"
	"github.com/fixed-partitioning/internal/model"
	"github.com/influxdata/influxdb/pkg/testing/assert"
)

type clientConf struct {
}

// pickNodeFromPTable returns the first element of the Partition Table
func pickNodeFromPTable(nodes []string) string {
	return ""
}

func getClientRequest() model.TCPRequest {
	req := model.TCPRequest{}
	req.Key = []byte("foo")
	req.Value = []byte(`{
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

	return req
}

func sendGetPTableRequest(coordAddress string) {
}

func sendClientReq(nodeAddress string) {
	client, err := net.DialTimeout("tcp", nodeAddress, time.Duration(1*time.Second))
	if err != nil {
		log.Error(err)
		return
	}
	defer client.Close()

	req := getClientRequest()
	data, _ := json.Marshal(req)
	client.Write(data)

	buffer := make([]byte, 2048)
	n, _ := client.Read(buffer)

	res := &model.TCPResponse{}
	json.Unmarshal(buffer[:n], res)

	assert.Equal(res.Message, "document succesfully added")
}

func main() {
}
