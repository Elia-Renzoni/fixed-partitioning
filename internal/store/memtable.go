package store

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"sync"
)

const maxLevels int = 16

// MemTable for a Document database with
// concurrent Skip List
type MemTable struct {
	mutex     sync.RWMutex
	tableSize int
	head      *skipListNode
	level     int
}

type skipListNode struct {
	key        []byte
	value      []byte
	nodeLevels []*skipListNode
}

func newSkipListNode(key, value []byte, height int) *skipListNode {
	return &skipListNode{
		key:        key,
		value:      value,
		nodeLevels: make([]*skipListNode, height),
	}
}

func NewMemTable() MemTable {

	head := newSkipListNode(nil, nil, maxLevels)

	return MemTable{
		head:  head,
		level: 1,
	}
}

func (m *MemTable) find(documentKey []byte) (*skipListNode, [maxLevels]*skipListNode) {
	var update [maxLevels]*skipListNode

	node := m.head
	for level := m.level - 1; level >= 0; level-- {
		for node.nodeLevels[level] != nil &&
			bytes.Compare(node.nodeLevels[level].key, documentKey) < 0 {

			node = node.nodeLevels[level]
		}
		update[level] = node
	}

	node = node.nodeLevels[0]
	if node != nil && bytes.Equal(node.key, documentKey) {
		return node, update
	}

	return nil, update
}

func (m *MemTable) AddDocument(documentKey, documentBody []byte) error {
	if len(documentKey) == 0 || len(documentBody) == 0 {
		return errors.New("empty data")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	found, update := m.find(documentKey)

	// update existing
	if found != nil {
		found.value = documentBody
		return nil
	}

	nodeLevel := m.calculateMaxNodeHeight()

	if nodeLevel > m.level {
		for i := m.level; i < nodeLevel; i++ {
			update[i] = m.head
		}
		m.level = nodeLevel
	}

	newNode := newSkipListNode(
		append([]byte{}, documentKey...),
		append([]byte{}, documentBody...),
		nodeLevel,
	)

	for i := range nodeLevel {
		newNode.nodeLevels[i] = update[i].nodeLevels[i]
		update[i].nodeLevels[i] = newNode
	}
	m.tableSize++

	return nil
}

func (m *MemTable) DeleteDocument(documentKey []byte) error {
	if len(documentKey) == 0 {
		return errors.New("empty key")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	found, update := m.find(documentKey)

	if found == nil {
		return errors.New("document not found")
	}

	for i := 0; i < len(found.nodeLevels); i++ {
		if update[i].nodeLevels[i] == found {
			update[i].nodeLevels[i] = found.nodeLevels[i]
		}
	}

	// shrink
	for m.level > 1 && m.head.nodeLevels[m.level-1] == nil {
		m.level--
	}

	m.tableSize--
	return nil
}

func (m *MemTable) FetchDocument(documentKey []byte) (error, []byte) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	node := m.head

	for level := m.level - 1; level >= 0; level-- {

		for node.nodeLevels[level] != nil &&
			bytes.Compare(node.nodeLevels[level].key, documentKey) < 0 {

			node = node.nodeLevels[level]
		}
	}

	node = node.nodeLevels[0]

	if node != nil && bytes.Equal(node.key, documentKey) {
		return nil, node.value
	}

	return errors.New("the searched document is not present"), nil
}

func (m *MemTable) calculateMaxNodeHeight() (height int) {
	const probability = 0.5

	height = 1

	for height < maxLevels && rand.Float64() < probability {
		height++
	}

	return
}
