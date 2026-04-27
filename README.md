# fixed-partitioning

A small experimental Go project exploring the **Fixed Partitions** sharding pattern (fixed logical partitions) and a partition assignment table (partitions → nodes).

The core idea is described in Martin Fowler’s article:  
https://martinfowler.com/articles/patterns-of-distributed-systems/fixed-partitions.html

## Why “Fixed Partitions”

Hashing a key directly to a node (e.g. `hash(key) % numNodes`) distributes well, but when `numNodes` changes most keys are remapped and you end up having to move a lot of data.

With **fixed partitions**, the cluster has a fixed number of logical partitions (often called “slots”):

- `partition = hash(key) % hashSlots` stays stable even if nodes join/leave
- a separate **partition table** maps partitions → nodes (and can be updated as the cluster changes)

In this repo, the implementation lives in `internal/sharding/partition_manager.go`.

## What’s in the repository

- `internal/sharding`: `PartitionTable` implementation + tests

## Data model

The table is represented as:

```go
map[int][]string // partitionID -> list of nodes (replicas included)
```

and is built/updated by methods such as:

- `NewPartitionTable(slots, replicationFactor, cluster)`
- `AssignPartitions()` (coordinator-only, to initialize the table)
- `GetPartition(key)` (computes the partition for a key)
- `FindNodes(partitionID)` (returns the nodes assigned)
- `MergePartitions(table)` (partial partitions merge/replace)

Note: `AssignPartitions()` requires at least `MinClusterLen` nodes (currently `4`), otherwise it returns `ErrLackOfNodes`.

## Rebalancing (prototype)

`RebalancePartitions()` tries to even out how many total assignments each node has (including replicas) by rewriting the table.

Current limitations:

- it does not migrate data (it only changes the table)
- it does not distinguish primaries vs replicas (it balances “appearances” in the `[]string`)
- it includes “chunk” fragmentation and a “forward” routine that, for now, prints to stdout

## Development

### Prerequisites
- Go (see `go.mod`)

### Test
```bash
make test
```

or:

```bash
go test ./...
```

### Build
```bash
make build
```

## Status

This is a learning/experimental repo: APIs and behavior may change, and some aspects (replication factor, rebalancing, chunk “forwarding”) are intentionally prototypical.
