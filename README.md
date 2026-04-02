# fixed-partitioning

An experimental Go project exploring the **Fixed Partitions** sharding pattern for a small, TCP-based key/value store.

The core idea comes from Martin Fowler’s write-up on Fixed Partitions:
https://martinfowler.com/articles/patterns-of-distributed-systems/fixed-partitions.html

## Why “Fixed Partitions”

Hashing a key directly to a node (e.g. `hash(key) % numNodes`) gives a uniform distribution, but when `numNodes` changes most keys remap and you need to move a lot of data.

With **fixed partitions**, the cluster is created with a fixed number of logical partitions (sometimes called “slots”):

- `partition = hash(key) % hash_slots` stays stable even if nodes are added/removed
- a separate **partition table** maps partitions → nodes (and can be updated as the cluster changes)

In this repo, `hash_slots` is configured in `etc/config.yml`, and partition selection is implemented in `internal/sharding/partition_manager.go`.

## What’s in here

- TCP server that accepts JSON requests (`internal/server`)
- In-memory document store implemented as a concurrent skip-list memtable (`internal/store`)
- Cluster membership list + simple broadcast replication helpers (`internal/replication`)
- Partition table abstraction for routing keys to partitions and (eventually) partitions to nodes (`internal/sharding`)
- YAML configuration loader (`internal/options`)

## Rebalancing algorithm (prototype)

The current rebalancing logic lives in `internal/sharding/partition_manager.go` (`(*PartitionTable).RebalancePartitions` and `doBalance`).

At a high level, it tries to *even out how many partition assignments* each node has, by rewriting the partition table (metadata only). It does **not** migrate data between nodes yet.

### Data model it rebalances

`PartitionTable.pTable` is a `map[int][]string` where:

- the map key is a partition identifier
- the slice is the list of node addresses that should receive replication traffic for that partition

`FindNodePartitions()` computes a `perNodeSlots` map by counting how many times each node address appears across all `pTable` values (so replicas count too).

### Target (“average”) load

During `AssignPartitions()`, the code computes a target count per node:

- `totalAssignments = len(pTable) * replicationFactor`
- `optimalPartitions = totalAssignments / cluster.Len()`

`RebalancePartitions()` uses `optimalPartitions` as the “average” number of assignments each node should have.

### How `RebalancePartitions()` redistributes assignments

1. Count current assignments per node (`FindNodePartitions`).
2. Build two separate lists of nodes:
   - **underfull**: nodes with `count(node) < average`
   - **overfull**: nodes with `count(node) > average`
3. For each node in the **overfull** list:
   - compute `delta = count(node) - average` (how many assignments it must shed)
   - collect `partitionsList`: all partition IDs in which that node appears (`findPartitionsByNodes`)
4. For each **overfull** node, repeat `delta` times:
   - choose one partition from its `partitionsList`
   - remove the overfull node from that partition’s node list
   - add one node from the **underfull** list, selected in round-robin order

The **underfull** list is treated as a circular buffer: each time you take a node to receive a reassigned partition, you advance an index, wrapping around to the beginning when you reach the end.

### Current limitations / caveats

- It mutates the in-memory table only; there is no “move the data” step.
- It is not currently invoked by the server join/leave workflow (only exercised in `internal/sharding/partition_manager_test.go`).
- It does a single pass and doesn’t recompute deltas after each move.
- It doesn’t distinguish primaries vs replicas; it only balances “appearances in `[]string`”.
- The target “average” is derived from `replicationFactor` and the current table size, so it should be treated as an experimental heuristic.

## Request/response model (high level)

The server speaks a small JSON protocol (see `internal/model/data.go`):

- `type: "client"`: set/get/delete by key
- `type: "join"`: join a node to the cluster (sent to a coordinator)
- `type: "replication"`: internal replication fan-out
- `type: "sharding"`: partition-table get/set (used to distribute routing metadata)

## Running

### Local (single node)

1. Edit `etc/config.yml` to use a local address, for example:

```yaml
coordinator: "127.0.0.1:7000"
server_address: "127.0.0.1:7000"
hash_slots: 128
replication_factor: 3
```

2. Start the node:

```bash
go run ./cmd
```

### Local (tests)

Run the unit/integration tests:

```bash
make test
```

### Containerized (Podman)

There is a `Containerfile` and a helper script (`run_node.sh`) that starts a container on a Podman network and rewrites `etc/config.yml` with a generated IP.

Example:

```bash
podman build -t localhost/project -f Containerfile .
./run_node.sh
```

## Configuration

`etc/config.yml` contains:

- `coordinator`: address of the coordinator node (used as the join target)
- `server_address`: address this node should listen on
- `hash_slots`: number of logical partitions (fixed)
- `replication_factor`: how many additional replicas to target

The config loader looks for a YAML file under `./etc` (or under `/tmp/etc`).

## Status / caveats

This is a learning/experiment repo, not a production system. Expect missing pieces (e.g. durable storage, robust membership, consistent partition assignment, failure handling). The rebalancing logic described above is a prototype and isn’t yet wired into the coordinator workflow that assigns/distributes the partition table.
