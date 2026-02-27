package replication

import (
	"errors"
	"net"
	"sync"
)

type Cluster struct {
	mutex   sync.RWMutex
	members []string
}

func NewCluster() *Cluster {
	return &Cluster{
		members: make([]string, 0),
	}
}

func (c *Cluster) AddNode(joinedAddress string) error {
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

func (c *Cluster) GetAllNodes() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.members
}

func (c *Cluster) Len() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.members)
}

func (c *Cluster) GetNodeFromLocation(index int) string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if index <= len(c.members) {
		return c.members[index]
	}

	return ""
}
