package replication

import (
	"errors"
	"net"
	"sync"
)

type cluster struct {
	mutex   sync.RWMutex
	members []string
}

func newCluster() *cluster {
	return &cluster{
		members: make([]string, 0),
	}
}

func (c *cluster) addNode(joinedAddress string) error {
	_, err := net.ResolveTCPAddr("tcp", joinedAddress)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, node := range c.members {
		if node == joinedAddress {
			return errors.New("node already present")
		}
	}

	c.members = append(c.members, joinedAddress)
	return nil
}

func (c *cluster) getAllNodes() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.members
}
