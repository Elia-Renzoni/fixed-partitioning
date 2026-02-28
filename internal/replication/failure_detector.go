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
	tkr           *time.Ticker
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
		d.tkr = time.NewTicker(d.scheduleTime)
		defer d.tkr.Stop()

		for range d.tkr.C {
			for _, node := range d.members.GetAllNodes() {
				// send message
				response := Send(node, d.generateMessage())
				if response != nil && d.isPong(response) {
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

func (d *detector) isPong(response []byte) bool {
	var res model.TCPResponse
	const PONG string = "pong"

	err := json.Unmarshal(response, &res)
	if err != nil {
		return false
	}

	return res.Message == PONG
}
