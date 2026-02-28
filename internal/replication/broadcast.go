package replication

import "sync"

type Replicator struct {
	cluster *Cluster
}

func NewReplicator(cluster *Cluster) *Replicator {
	return &Replicator{
		cluster: cluster,
	}
}

func (r *Replicator) BroadcastMessage(message []byte) {
	wg := &sync.WaitGroup{}
	for _, node := range r.cluster.GetAllNodes() {
		wg.Go(func() {
			// TODO-> handle return buffer for more reliability
			Send(node, message)
		})
	}

	wg.Wait()
}
