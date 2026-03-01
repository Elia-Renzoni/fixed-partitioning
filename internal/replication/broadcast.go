package replication

import (
	"sync"
	"sync/atomic"
)

func BroadcastMessage(message []byte, nodes []string) int32 {
	var (
		wg         = &sync.WaitGroup{}
		ackCounter atomic.Int32
	)

	for _, node := range nodes {
		wg.Go(func() {
			response := Send(node, message)
			if response == nil {
				return
			}

			ackCounter.Add(1)
		})
	}

	wg.Wait()

	return ackCounter.Load()
}
