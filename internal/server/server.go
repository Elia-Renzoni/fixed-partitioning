package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/store"
	"github.com/google/uuid"
)

type Server struct {
	nodeID          uuid.UUID
	completeAddress *net.TCPAddr
	listener        *net.TCPListener
	globalErr       error
	connPool        chan *net.TCPConn
	poolCapacity    atomic.Int64
	memTable        store.MemTable
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
		memTable:        store.NewMemTable(),
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

		s.doConfigure(conn)

		s.connPool <- conn
		if s.poolCapacity.Load() > 0 {
			s.poolCapacity.Add(-1)
			go s.handleConnection()
		}
	}
}

func (s *Server) doConfigure(conn *net.TCPConn) {
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.SetKeepAlive(true)
	conn.SetLinger(-1)
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

		serverCtx := model.NewConnContext()
		res := serverCtx.TCPResponse
		switch req.RequestType {
		case model.ClientReq:
			s.handleClientReq(serverCtx)
		case model.ReplicationReq:
			s.handleReplicationReq(serverCtx)
		case model.ShardingReq:
			s.handleShardingReq(serverCtx)
		case model.JoinReq:
			s.handleJoinReq(serverCtx)
		default:
			res.Message = "invalid request type"
		}
		data, _ = json.Marshal(res)
		conn.Write(data)

		conn.Close()
		serverCtx.AbortJob()
	}
}

func (s *Server) handleClientReq(ctx model.ConnContext) {
	var (
		req               = ctx.TCPRequest
		res               = ctx.TCPResponse
		err               error
		retreivedDocument []byte
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
		return
	default:
		switch req.StoreRouter {
		case model.ClientAdd:
			err = s.memTable.AddDocument(req.Key, req.Value)
			res.Message = "document succesfully added"
		case model.ClientFetch:
			err, retreivedDocument = s.memTable.FetchDocument(req.Key)
			res.Message = string(retreivedDocument)
		case model.ClientDelete:
			err = s.memTable.DeleteDocument(req.Key)
			res.Message = "document succesfully removed"
		default:
			res.Message = "invalid router type"
			return
		}

		if err != nil {
			res.Message = err.Error()
			return
		}
	}
}

func (s *Server) handleJoinReq(ctx model.ConnContext) {
	var (
		_   = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
		return
	default:
		// handle request
	}
}

func (s *Server) handleReplicationReq(ctx model.ConnContext) {
	var (
		_   = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
		return
	default:
		// handle request
	}
}

func (s *Server) handleShardingReq(ctx model.ConnContext) {
	var (
		_   = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
		return
	default:
		// handle request
	}
}
