package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/sharding"
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
	cluster         *replication.Cluster
	partitions      *sharding.PartitionTable
}

func NewServer(host, port string, members *replication.Cluster, pTable *sharding.PartitionTable) (*Server, error) {
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
		cluster:         members,
		partitions:      pTable,
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
		var (
			bytesRed int
			readErr  error
		)

		bufferPool := bytes.Buffer{}
		buffer := make([]byte, 2048)
		for {
			bytesRed, readErr = conn.Read(buffer)
			if bytesRed > 0 {
				bufferPool.Write(buffer[:bytesRed])
			}

			if readErr != nil {
				if readErr == io.EOF {
					break
				}
				return
			}
		}

		req := &model.TCPRequest{}
		data := bufferPool.Bytes()
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

		if req.RequestType == "replication" {
			return
		}

		if s.cluster.Len() == 0 {
			return
		}

		partition := s.partitions.GetPartition(req.Key)
		nodes := s.partitions.FindNodes(partition)

		newRequest := &model.TCPRequest{
			RequestType: "replication",
			StoreRouter: req.StoreRouter,
			Key:         req.Key,
			Value:       req.Value,
		}

		data, _ := json.Marshal(newRequest)
		acks := replication.BroadcastMessage(data, nodes)
		if int(acks) < len(nodes) {
			res.Warning = fmt.Sprintf("replication warning, message has arrived only to %d replicas", int(acks))
		}
	}
}

func (s *Server) handleJoinReq(ctx model.ConnContext) {
	var (
		req = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
	default:
		if req.NodeAddress == "" {
			res.Message = "invalid address"
		} else {
			if err := s.cluster.AddNode(req.NodeAddress); err != nil {
				res.Message = err.Error()
				return
			}
			res.Message = "node succesfully joined"

			if req.RequestType == "replication" {
				return
			}

			if s.cluster.Len() == 0 {
				return
			}

			replicationReq := model.TCPRequest{
				RequestType: "replication",
				StoreRouter: "cluster",
				NodeAddress: req.NodeAddress,
			}

			data, _ := json.Marshal(replicationReq)
			nodes := s.cluster.GetAllNodes()
			acks := replication.BroadcastMessage(data, nodes)
			if int(acks) < len(nodes) {
				res.Warning = fmt.Sprintf("replication warning, message has arrived only to %d replicas", int(acks))
			}
		}
	}
}

func (s *Server) handleReplicationReq(ctx model.ConnContext) {
	var (
		req = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
	default:
		// handle request
		switch req.StoreRouter {
		case model.ClientAdd, model.ClientDelete:
			s.handleClientReq(ctx)
		case model.JoinReq:
			s.handleJoinReq(ctx)
		case model.ShardingReq:
			s.handleShardingReq(ctx)
		default:
			res.Message = "invalid store router type"
		}
	}
}

func (s *Server) handleShardingReq(ctx model.ConnContext) {
	var (
		req = ctx.TCPRequest
		res = ctx.TCPResponse
	)

	select {
	case <-ctx.Ctx.Done():
		res.Message = ctx.Ctx.Err().Error()
	default:
		// handle request
		switch req.StoreRouter {
		case model.ShardingGet:
			table := s.partitions.ReadPartitionTable()
			res.Message = table
		case model.ShardingSet:
			s.partitions.MergePartitions(req.PTable)
			res.Message = "partition table merged succesfully"
		}
	}
}
