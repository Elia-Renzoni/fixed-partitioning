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
	key      []byte
	value    []byte
	weight   int
	nextNode *skipListNode
}

func NewMemTable() MemTable {
	return MemTable{}
}

func (m *MemTable) AddDocument() {
}

func (m *MemTable) DeleteDocument() {
}

func (m *MemTable) FetchDocument() {
}
