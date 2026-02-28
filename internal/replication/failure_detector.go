package replication

import (
	"encoding/json"
	"time"

	"github.com/fixed-partitioning/internal/model"
)

type detector struct {
	scheduleTime  time.Duration
	members       *Cluster
	failedMembers []string
}

func NewDetector(scheduler int, cluster *Cluster) *detector {
	return &detector{
		scheduleTime:  time.Duration(time.Duration(scheduler) * time.Second),
		members:       cluster,
		failedMembers: make([]string, 0),
	}
}

func (d *detector) StartDetector() {
	go func() {
		tkr := time.NewTicker(d.scheduleTime)
		defer tkr.Stop()

		for range tkr.C {
			for _, node := range d.members.GetAllNodes() {
				// send message
				response := send(node, d.generateMessage())
				if response != nil {
					continue
				}

				d.failedMembers = append(d.failedMembers, node)
			}
		}
	}()
}

func (d *detector) generateMessage() []byte {
	req := model.TCPRequest{
		RequestType: "ping",
		StoreRouter: "ping",
	}

	b, _ := json.Marshal(req)
	return b
}

// TODO
func (d *detector) isPong(response []byte) bool {
	return false
}
