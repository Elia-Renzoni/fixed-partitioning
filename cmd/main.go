package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/fixed-partitioning/internal/model"
	"github.com/fixed-partitioning/internal/options"
	"github.com/fixed-partitioning/internal/replication"
	"github.com/fixed-partitioning/internal/server"
	"github.com/fixed-partitioning/internal/sharding"
)

func main() {
	opt, err := options.ParseConf()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	cluster := replication.NewCluster()
	if opt.GetServerAddress() != "" {
		if err := cluster.AddNode(opt.GetServerAddress()); err != nil {
			log.Printf("warning: failed to add local node to cluster: %v", err)
		}
	}

	pTable := sharding.NewPartitionTable(opt.GetHashSlots(), opt.GetReplicationFactor(), cluster)

	host, port, err := net.SplitHostPort(opt.GetServerAddress())
	if err != nil {
		log.Fatalf("invalid server_address %q: %v", opt.GetServerAddress(), err)
	}

	srv, err := server.NewServer(host, port, cluster, pTable)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if opt.GetCoordinatorAddress() != "" && opt.GetCoordinatorAddress() != opt.GetServerAddress() {
		if err := sendJoin(opt.GetCoordinatorAddress(), opt.GetServerAddress()); err != nil {
			log.Printf("warning: join request failed: %v", err)
		}
	} else {
		go applyShardingStrategy(pTable)
	}

	log.Printf("listening on %s", opt.GetServerAddress())
	srv.DoListen()
}

func sendJoin(coordinator, self string) error {
	req := model.TCPRequest{
		RequestType: model.JoinReq,
		NodeAddress: self,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resBytes := replication.Send(coordinator, data)
	if resBytes == nil {
		return fmt.Errorf("no response from coordinator %s", coordinator)
	}

	var res model.TCPResponse
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return err
	}
	if res.Warning != "" {
		return fmt.Errorf("got warning %s", res.Warning)
	}
	return nil
}

func applyShardingStrategy(pTable *sharding.PartitionTable) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for tick := range ticker.C {
		log.Println("probe at", tick)
		err := pTable.AssignPartitions()
		if err == nil {
			break
		}

		if !errors.Is(err, sharding.ErrLackOfNodes) {
			panic(err)
		}
	}

	log.Println("partitions succesfully applied between nodes")
}
