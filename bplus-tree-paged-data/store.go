package bplustree

import "fmt"

// Store abstracts node persistence and record storage.
type Store interface {
	Order() int
	AllocNode() (*Node, error)
	LoadNode(id int32) (*Node, error)
	SaveNode(*Node) error
	RootID() int32
	SetRootID(int32) error
	AppendRecord(value string) (RecordPtr, error)
	ReadRecord(ptr RecordPtr) (string, error)
	Close() error
}

var _ Store = (*MemStore)(nil)

// MemStore keeps all nodes and records in memory. No persistence across process restarts.
type MemStore struct {
	nodes   map[int32]*Node
	records []string
	nextID  int32
	rootID  int32
	order   int
}

func NewMemStore(order int) *MemStore {
	return &MemStore{
		nodes:  make(map[int32]*Node),
		rootID: noPage,
		order:  order,
	}
}

func (s *MemStore) Order() int    { return s.order }
func (s *MemStore) RootID() int32 { return s.rootID }
func (s *MemStore) Close() error  { return nil }

func (s *MemStore) SetRootID(id int32) error {
	s.rootID = id

	return nil
}

func (s *MemStore) AllocNode() (*Node, error) {
	id := s.nextID
	s.nextID++
	n := &Node{id: id, next: noPage}
	s.nodes[id] = n

	return n, nil
}

func (s *MemStore) LoadNode(id int32) (*Node, error) {
	n, ok := s.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %d not found", id)
	}

	return n, nil
}

func (s *MemStore) SaveNode(n *Node) error {
	s.nodes[n.id] = n

	return nil
}

// AppendRecord appends the value to the in-memory records slice and returns its pointer.
// PageID is always 0; Slot is the index in the slice.
func (s *MemStore) AppendRecord(value string) (RecordPtr, error) {
	slot := int32(len(s.records))
	s.records = append(s.records, value)

	return RecordPtr{PageID: 0, Slot: slot}, nil
}

func (s *MemStore) ReadRecord(ptr RecordPtr) (string, error) {
	if int(ptr.Slot) >= len(s.records) {
		return "", fmt.Errorf("slot %d out of range", ptr.Slot)
	}

	return s.records[ptr.Slot], nil
}
