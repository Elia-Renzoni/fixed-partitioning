package model

import "net"

const (
	ClientReq      = "client"
	JoinReq        = "join"
	ReplicationReq = "replication"
	ShardingReq    = "sharding"
)

type TCPRequest struct {
	RequestType string   `json:"type"`
	Key         []byte   `json:"key,omitempty"`
	Value       []byte   `json:"value,omitempty"`
	NodeAddress net.Addr `json:"addr,omitempty"`
}

type TCPResponse struct {
	Message string `json:"text"`
}
