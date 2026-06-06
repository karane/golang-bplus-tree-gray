package bplustree

const noPage = int32(-1)

type Node struct {
	id       int32
	isLeaf   bool
	keys     []int64
	values   []string // leaves only
	children []int32  // internal nodes only
	next     int32    // leaves only; noPage if none
}
