package server

import (
	"errors"
	"log"
	"net"
	"strconv"
	"sync/atomic"

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

	}
}
