package bplustree

/*
Page layout per node (all fields fixed-width, sized by order):

   [0]                              1 B   isLeaf
   [1:5]                            4 B   keyCount (int32 LE)
   [5 : 5+8*(order-1)]              keys  (int64 LE each, order-1 slots)
   [5+8*(order-1) : +4*order]       children (int32 LE each, order slots)
   [+4*order : +4]                  next  (int32 LE, leaf linked list)
   [+4 : +64*(order-1)]             values (64-byte slots, leaves only)

 Total per page: 76*order - 63 bytes.  Max order with 4096-byte pages = 54.

 Page 0 is reserved for the header:
   [0:4]   magic   (0xB712EE01)
   [4:8]   order   (int32 LE)
   [8:12]  rootID  (int32 LE)
   [12:16] totalPages (int32 LE)
*/

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	pageSizeInBytes      = 4096
	magic                = uint32(0xB712EE01)
	valWidthInBytes      = 64
	isLeafWidthInBytes   = 1
	keyCountWidthInBytes = 4
	keyWidthInBytes      = 8
	int32WidthInBytes    = 4
)

var _ Store = (*PagedStore)(nil)

type PagedStore struct {
	f          *os.File
	order      int
	rootID     int32
	totalPages int32
}

// OpenPagedStore opens or creates a B+ tree file.
// order is used only when creating a new file; existing files use the stored order.
func OpenPagedStore(path string, order int) (*PagedStore, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	s := &PagedStore{f: f}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.Size() == 0 {
		if err := validateOrder(order); err != nil {
			f.Close()
			return nil, err
		}
		s.order = order
		s.rootID = noPage
		s.totalPages = 1 // page 0 = header
		if err := s.writeHeader(); err != nil {
			f.Close()
			return nil, err
		}
	} else {
		if err := s.readHeader(); err != nil {
			f.Close()
			return nil, err
		}
	}
	return s, nil
}

func validateOrder(order int) error {
	size := nodeSize(order)
	if size > pageSizeInBytes {
		return fmt.Errorf("order %d needs %d bytes per node, exceeds page size %d", order, size, pageSizeInBytes)
	}
	return nil
}

func nodeSize(order int) int {
	return isLeafWidthInBytes + keyCountWidthInBytes + keyWidthInBytes*(order-1) + int32WidthInBytes*order + int32WidthInBytes + valWidthInBytes*(order-1)
}

func (s *PagedStore) Order() int    { return s.order }
func (s *PagedStore) RootID() int32 { return s.rootID }
func (s *PagedStore) Close() error  { return s.f.Close() }

func (s *PagedStore) SetRootID(id int32) error {
	s.rootID = id
	return s.writeHeader()
}

func (s *PagedStore) AllocNode() (*Node, error) {
	id := s.totalPages
	s.totalPages++
	n := &Node{id: id, next: noPage}
	if err := s.writeHeader(); err != nil {
		return nil, err
	}
	if err := s.SaveNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *PagedStore) LoadNode(id int32) (*Node, error) {
	buf := make([]byte, pageSizeInBytes)
	if _, err := s.f.ReadAt(buf, s.offset(id)); err != nil {
		return nil, fmt.Errorf("read node %d: %w", id, err)
	}

	return s.decode(id, buf), nil
}

func (s *PagedStore) SaveNode(node *Node) error {
	buf := s.encode(node)
	if _, err := s.f.WriteAt(buf, s.offset(node.id)); err != nil {
		return fmt.Errorf("write node %d: %w", node.id, err)
	}

	return nil
}

func (s *PagedStore) offset(id int32) int64 { return int64(id) * int64(pageSizeInBytes) }

func (s *PagedStore) readHeader() error {
	buf := make([]byte, pageSizeInBytes)
	if _, err := s.f.ReadAt(buf, 0); err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	if m := binary.LittleEndian.Uint32(buf[0:4]); m != magic {
		return fmt.Errorf("bad magic: %#x", m)
	}
	s.order = int(int32(binary.LittleEndian.Uint32(buf[4:8])))
	s.rootID = int32(binary.LittleEndian.Uint32(buf[8:12]))
	s.totalPages = int32(binary.LittleEndian.Uint32(buf[12:16]))

	return nil
}

func (s *PagedStore) writeHeader() error {
	buf := make([]byte, pageSizeInBytes)

	binary.LittleEndian.PutUint32(buf[0:4], magic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(s.order))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(s.rootID))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(s.totalPages))

	if _, err := s.f.WriteAt(buf, 0); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	return nil
}

func (s *PagedStore) encode(n *Node) []byte {
	buf := make([]byte, pageSizeInBytes)

	if n.isLeaf {
		buf[0] = 1
	}

	binary.LittleEndian.PutUint32(buf[1:5], uint32(len(n.keys)))

	off := isLeafWidthInBytes + keyCountWidthInBytes
	for i, k := range n.keys {
		binary.LittleEndian.PutUint64(buf[off+i*keyWidthInBytes:], uint64(k))
	}
	off += keyWidthInBytes * (s.order - 1)

	// children slots (internal) - unused slots filled with noPage
	for i := 0; i < s.order; i++ {
		if i < len(n.children) {
			binary.LittleEndian.PutUint32(buf[off+i*int32WidthInBytes:], uint32(n.children[i]))
		} else {
			binary.LittleEndian.PutUint32(buf[off+i*int32WidthInBytes:], 0xFFFFFFFF)
		}
	}
	off += int32WidthInBytes * s.order

	// next (leaf linked list)
	binary.LittleEndian.PutUint32(buf[off:], uint32(n.next))
	off += int32WidthInBytes

	// value slots (leaf)
	for i, v := range n.values {
		b := []byte(v)
		if len(b) > valWidthInBytes {
			b = b[:valWidthInBytes]
		}
		copy(buf[off+i*valWidthInBytes:], b)
	}

	return buf
}

func (s *PagedStore) decode(id int32, buf []byte) *Node {
	n := &Node{id: id, next: noPage}
	n.isLeaf = buf[0] == 1
	keyCount := int(binary.LittleEndian.Uint32(buf[1:5]))

	off := isLeafWidthInBytes + keyCountWidthInBytes
	n.keys = make([]int64, keyCount)
	for i := range n.keys {
		n.keys[i] = int64(binary.LittleEndian.Uint64(buf[off+i*keyWidthInBytes:]))
	}
	off += keyWidthInBytes * (s.order - 1)

	if n.isLeaf {
		off += int32WidthInBytes * s.order // skip children section
		n.next = int32(binary.LittleEndian.Uint32(buf[off:]))
		off += int32WidthInBytes
		n.values = make([]string, keyCount)
		for i := range n.values {
			slot := buf[off+i*valWidthInBytes : off+i*valWidthInBytes+valWidthInBytes]
			end := valWidthInBytes
			for end > 0 && slot[end-1] == 0 {
				end--
			}
			n.values[i] = string(slot[:end])
		}
	} else {
		n.children = make([]int32, keyCount+1)
		for i := range n.children {
			n.children[i] = int32(binary.LittleEndian.Uint32(buf[off+i*int32WidthInBytes:]))
		}
	}

	return n
}
