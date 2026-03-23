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

This is a learning/experiment repo, not a production system. Expect missing pieces (e.g. durable storage, robust membership, rebalancing, consistent partition assignment, failure handling). In particular, the “coordinator assigns and distributes the partition table” workflow is still in progress.
