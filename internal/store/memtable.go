package store

import (
	"bytes"
	"errors"
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
	selfLevel  int
	next       *skipListNode
	nodeLevels []*skipListNode
}

func newSkipListNode(key, value []byte) *skipListNode {
	return &skipListNode{
		key:        key,
		value:      value,
		nodeLevels: make([]*skipListNode, 0, maxLevels),
	}
}

func (s *skipListNode) createLevels() {
	for i := range maxLevels {
		s.selfCopy(i)
	}
}

func (s *skipListNode) selfCopy(latestLevel int) {
	var l int
	if latestLevel < len(s.nodeLevels) {
		l = latestLevel + 1
	}
	s.nodeLevels = append(s.nodeLevels, &skipListNode{
		selfLevel:  l,
		nodeLevels: s.nodeLevels,
	})
}

func NewMemTable() MemTable {
	return MemTable{}
}

// [1]-----------[3]----[4]
// [1]----[2]           [4]
// [1]----[2]----[3]----[4]
func (m *MemTable) AddDocument(documentKey, documentBody []byte) error {
	if len(documentKey) == 0 || len(documentBody) == 0 {
		return errors.New("empty data")
	}
	node := newSkipListNode(documentKey, documentBody)
	node.createLevels()

	if m.head == nil {
		m.head = node
	}

	return nil
}

func (m *MemTable) DeleteDocument(documentKey []byte) error {
	return nil
}

func (m *MemTable) FetchDocument(documentKey []byte) error {
	return nil
}

func (m *MemTable) find(documentKey []byte) *skipListNode {
	node := m.head
	for level := maxLevels; level > 0; level-- {
		nextNode := node.nodeLevels[level].next
		for nextNode != nil {
			if bytes.Compare(documentKey, nextNode.key) <= 0 {
				break
			}
			node = nextNode
			nextNode = nextNode.nodeLevels[level].next
		}
	}

	if node != nil && bytes.Equal(documentKey, node.key) {
		return node
	}
	return nil
}
