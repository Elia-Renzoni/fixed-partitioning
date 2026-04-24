package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

	srv, err := server.NewServer(opt.GetServerAddress(), cluster, pTable)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if opt.GetCoordinatorAddress() != "" && opt.GetCoordinatorAddress() != opt.GetServerAddress() {
		if err := sendJoin(opt.GetCoordinatorAddress(), opt.GetServerAddress(), cluster, pTable); err != nil {
			log.Printf("warning: join request failed: %v", err)
		}
	} else {
		go applyShardingStrategy(pTable, cluster)
	}

	log.Printf("listening on %s", opt.GetServerAddress())
	srv.DoListen()
}

func sendJoin(coordinator, self string, cluster *replication.Cluster, pTable *sharding.PartitionTable) error {
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

	var res struct {
		Message model.JoinInfo `json:"res"`
		Warning string         `json:"warn"`
	}
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return err
	}
	if res.Warning != "" {
		return fmt.Errorf("got warning %s", res.Warning)
	}

	for _, node := range res.Message.Nodes {
		if node == "" {
			continue
		}
		if err := cluster.AddNode(node); err != nil {
			// Ignore duplicates; coordinator may include this node address too.
			if !errors.Is(err, replication.ErrNodeAlreadyPresent) {
				return err
			}
		}
	}

	if len(res.Message.PTable) > 0 {
		pTable.MergePartitions(res.Message.PTable)
		return nil
	}

	// Best-effort: if the coordinator already has a partition table, fetch it
	// directly instead of relying on an inbound push.
	shardReq := model.TCPRequest{
		RequestType: model.ShardingReq,
		StoreRouter: model.ShardingGet,
	}
	shardData, _ := json.Marshal(shardReq)
	shardResBytes := replication.Send(coordinator, shardData)
	if shardResBytes == nil {
		return nil
	}

	var shardRes struct {
		Message model.Shard `json:"res"`
		Warning string     `json:"warn"`
	}
	if err := json.Unmarshal(shardResBytes, &shardRes); err != nil {
		return nil
	}
	if len(shardRes.Message.PTable) > 0 {
		pTable.MergePartitions(shardRes.Message.PTable)
	}
	return nil
}

func applyShardingStrategy(pTable *sharding.PartitionTable, c *replication.Cluster) {
	ticker := time.NewTicker(3 * time.Second)
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

	table := pTable.ReadPartitionTable()
	spreadPartitionTable(table, c)
	log.Println("partitions succesfully applied between nodes")
}

func spreadPartitionTable(table map[int][]string, c *replication.Cluster) {
	req := model.TCPRequest{
		RequestType: "sharding",
		StoreRouter: "sh-set",
		PTable:      table,
	}

	data, _ := json.Marshal(req)
	replication.BroadcastMessage(data, c.GetAllNodes())
}
