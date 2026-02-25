package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"

	"github.com/fixed-partitioning/internal/model"
	"github.com/google/uuid"
)

type Server struct {
	nodeID          uuid.UUID
	completeAddress *net.TCPAddr
	listener        *net.TCPListener
	globalErr       error
	connPool        chan *net.TCPConn
	poolCapacity    atomic.Int64
}

func NewServer(host, port string) (*Server, error) {
	n, _ := strconv.Atoi(port)
	if host == "" && n < 0 || n > 65_535 {
		return nil, errors.New("invalid network params")
	}

	address, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}

	uuid.SetNodeID([]byte(host))
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	return &Server{
		nodeID:          id,
		completeAddress: address,
		connPool:        make(chan *net.TCPConn, 100),
	}, nil
}

func (s *Server) DoListen() {
	s.listener, s.globalErr = net.ListenTCP("tcp", s.completeAddress)
	defer func() {
		s.listener.Close()
		log.Println("server stopped due to a sever network error")
	}()

	s.poolCapacity.Store(100)
	for {
		conn, err := s.listener.AcceptTCP()
		if err != nil {
			break
		}

		s.connPool <- conn
		if s.poolCapacity.Load() > 0 {
			s.poolCapacity.Add(-1)
			go s.handleConnection()
		}
	}
}

func (s *Server) handleConnection() {
	defer func() {
		s.poolCapacity.Add(1)
	}()

	for conn := range s.connPool {
		buffer := make([]byte, 2048)
		var (
			bytesRed int
			readErr  error
		)
		for {
			bytesRed, readErr = conn.Read(buffer)
			if readErr != nil {
				if readErr != io.EOF {
					log.Println("[ERR]: something went wrong while reading data")
				}
				break
			}
		}

		req := &model.TCPRequest{}
		data := buffer[:bytesRed]
		err := json.Unmarshal(data, req)
		if err != nil {
			log.Println("[ERR]: something went wrong while unmarhsaling data")
			break
		}

		res := &model.TCPResponse{}
		switch req.RequestType {
		case model.ClientReq:
			s.handleClientReq(req, res)
		case model.ReplicationReq:
			s.handleReplicationReq(req, res)
		case model.ShardingReq:
			s.handleShardingReq(req, res)
		case model.JoinReq:
			s.handleJoinReq(req, res)
		default:
			res.Message = "invalid request type"
		}
		data, _ = json.Marshal(res)
		conn.Write(data)

		conn.Close()
	}
}

func (s *Server) handleClientReq(req *model.TCPRequest, res *model.TCPResponse) {
}

func (s *Server) handleJoinReq(req *model.TCPRequest, res *model.TCPResponse) {
}

func (s *Server) handleReplicationReq(req *model.TCPRequest, res *model.TCPResponse) {
}

func (s *Server) handleShardingReq(req *model.TCPRequest, res *model.TCPResponse) {
}
