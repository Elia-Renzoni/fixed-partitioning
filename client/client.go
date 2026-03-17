package main

import "github.com/fixed-partitioning/internal/model"

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
}

func main() {
}
