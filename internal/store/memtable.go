package store

import "sync"

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
	nodeLevels []*skipListNode
	nextNode   *skipListNode
}

func newSkipListNode(key, value []byte) *skipListNode {
	return &skipListNode{
		key:        key,
		value:      value,
		nodeLevels: make([]*skipListNode, 0, 16),
	}
}

func (s *skipListNode) selfCopy(latestLevel int) {
	var l int
	if latestLevel < len(s.nodeLevels) {
		l = latestLevel + 1
	}
	s.nodeLevels = append(s.nodeLevels, &skipListNode{
		key:        s.key,
		value:      s.value,
		selfLevel:  l,
		nodeLevels: s.nodeLevels,
	})
}

func NewMemTable() MemTable {
	return MemTable{}
}

func (m *MemTable) AddDocument(documentKey, documentBody []byte) error {
	return nil
}

func (m *MemTable) DeleteDocument(documentKey []byte) error {
	return nil
}

func (m *MemTable) FetchDocument(documentKey []byte) error {
	return nil
}
