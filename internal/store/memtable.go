package store

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"sync"
)

const maxLevels int = 16

// MemTable for a Document database
// based on a concurrent Skip List
// backed by a 2PL algorithm for lock grabbing
type MemTable struct {
	mutex     sync.RWMutex
	tableSize int
	head      *skipListNode
}

type skipListNode struct {
	key        []byte
	value      []byte
	nodeHeight int
	nodeLevels []*skipListNode
}

func newSkipListNode(key, value []byte, height int) *skipListNode {
	return &skipListNode{
		key:        key,
		value:      value,
		nodeHeight: height,
		nodeLevels: make([]*skipListNode, 0, height),
	}
}

func NewMemTable() MemTable {
	return MemTable{}
}

// [1]----[2]----[3]----[4]
func (m *MemTable) AddDocument(documentKey, documentBody []byte) error {
	if len(documentKey) == 0 || len(documentBody) == 0 {
		return errors.New("empty data")
	}

	var (
		node      = m.head
		nodeLevel = m.calculateMaxNodeHeight()
	)

	for level := nodeLevel; level >= 0; level++ {
		for node.nodeLevels[level] != nil {
			if bytes.Compare(documentKey, node.key) <= 0 {
				break
			}
			node = node.nodeLevels[level]
		}
		newNode := newSkipListNode(documentKey, documentBody, nodeLevel)
		newNode.nodeLevels[level] = node.nodeLevels[level]
		node.nodeLevels[level] = newNode
	}

	return nil
}

func (m *MemTable) DeleteDocument(documentKey []byte) error {
	return nil
}

func (m *MemTable) FetchDocument(documentKey []byte) error {
	return nil
}

func (m *MemTable) find(documentKey []byte) (*skipListNode, [maxLevels]*skipListNode) {
	node := m.head
	var tower [maxLevels]*skipListNode
	for level := maxLevels; level > 0; level-- {
		nextNode := node.nodeLevels[level]
		for nextNode != nil {
			if bytes.Compare(documentKey, nextNode.key) <= 0 {
				break
			}
			node = nextNode
			nextNode = nextNode.nodeLevels[level]
		}
		tower[level] = node
	}

	if node != nil && bytes.Equal(documentKey, node.key) {
		return node, tower
	}
	return nil, tower
}

func (m *MemTable) calculateMaxNodeHeight() (height int) {
	var (
		probability float64 = 0.5
	)

	height = 1
	for height < maxLevels && rand.Float64() < probability {
		height += 1
	}
	return
}
