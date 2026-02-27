package model

import (
	"context"
	"net"
	"time"
)

const (
	ClientReq      = "client"
	JoinReq        = "join"
	ReplicationReq = "replication"
	ShardingReq    = "sharding"

	ClientAdd    = "client-add"
	ClientFetch  = "client-fetch"
	ClientDelete = "client-delete"
)

type TCPRequest struct {
	RequestType string   `json:"type"`
	StoreRouter string   `json:"route,omitempty"`
	Key         []byte   `json:"key,omitempty"`
	Value       []byte   `json:"value,omitempty"`
	NodeAddress net.Addr `json:"addr,omitempty"`
}

type TCPResponse struct {
	Message string `json:"text"`
}

type ConnContext struct {
	*TCPRequest
	*TCPResponse
	Ctx      context.Context
	AbortJob context.CancelFunc
}

func NewConnContext() ConnContext {
	ctx := ConnContext{
		TCPRequest:  &TCPRequest{},
		TCPResponse: &TCPResponse{},
	}

	ctx.Ctx, ctx.AbortJob = context.WithTimeout(
		context.Background(),
		time.Duration(3*time.Second),
	)

	return ctx
}
