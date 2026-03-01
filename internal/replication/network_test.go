package replication_test

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/replication"
)

type serverFunc func(addr string)

var server = func(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	defer listener.Close()

	var conn net.Conn
	conn, err = listener.Accept()
	if err != nil {
		return
	}

	defer conn.Close()

	buf := make([]byte, 2048)
	var n int

	n, err = conn.Read(buf)
	if err != nil {
		return
	}

	req := model.TCPRequest{}
	err = json.Unmarshal(buf[:n], &req)
	if err != nil {
		return
	}

	res := &model.TCPResponse{}
	res.Message = "test-pong"
	data, err := json.Marshal(res)
	if err != nil {
		return
	}
	conn.Write(data)

}

func TestSend(t *testing.T) {
	const address = "127.0.0.1:7676"
	go server(address)

	time.Sleep(2 * time.Second)

	req := &model.TCPRequest{}
	req.RequestType = "test"
	data, _ := json.Marshal(req)
	buf := replication.Send(address, data)

	res := &model.TCPResponse{}
	json.Unmarshal(buf[:], res)

	if res.Message != "test-pong" {
		t.Fatalf("unknown message, received: %s", res.Message)
	}
}
