package bplustree

import "sort"

type Node struct {
	keys     []int
	values   []string // leaves only
	children []*Node  // internal nodes only
	isLeaf   bool
	next     *Node // leaf linked list
}

type BPlusTree struct {
	root  *Node
	order int // max number of children; max keys per node
}

func New(order int) *BPlusTree {
	if order < 3 {
		order = 3
	}
	return &BPlusTree{order: order}
}

func (t *BPlusTree) Search(key int) (string, bool) {
	if t.root == nil {
		return "", false
	}
	node := t.root
	for !node.isLeaf {
		i := 0
		for i < len(node.keys) && key >= node.keys[i] {
			i++
		}
		node = node.children[i]
	}
	i := sort.SearchInts(node.keys, key) // binary search
	if i < len(node.keys) && node.keys[i] == key {
		return node.values[i], true
	}
	return "", false
}

func (t *BPlusTree) Insert(key int, value string) {
	if t.root == nil {
		t.root = &Node{isLeaf: true}
	}
	splitKey, splitNode := t.insert(t.root, key, value)
	if splitNode != nil {
		t.root = &Node{
			keys:     []int{splitKey},
			children: []*Node{t.root, splitNode},
		}
	}
}

func (t *BPlusTree) insert(node *Node, key int, value string) (int, *Node) {
	if node.isLeaf {
		i := sort.SearchInts(node.keys, key)
		if i < len(node.keys) && node.keys[i] == key {
			node.values[i] = value
			return 0, nil
		}
		node.keys = append(node.keys, 0)
		copy(node.keys[i+1:], node.keys[i:])
		node.keys[i] = key
		node.values = append(node.values, "")
		copy(node.values[i+1:], node.values[i:])
		node.values[i] = value
		if len(node.keys) > t.order-1 {
			return t.splitLeaf(node)
		}
		return 0, nil
	}

	i := 0
	for i < len(node.keys) && key >= node.keys[i] {
		i++
	}
	splitKey, splitNode := t.insert(node.children[i], key, value)
	if splitNode == nil {
		return 0, nil
	}
	node.keys = append(node.keys, 0)
	copy(node.keys[i+1:], node.keys[i:])
	node.keys[i] = splitKey
	node.children = append(node.children, nil)
	copy(node.children[i+2:], node.children[i+1:])
	node.children[i+1] = splitNode
	if len(node.keys) > t.order-1 {
		return t.splitInternal(node)
	}
	return 0, nil
}

func (t *BPlusTree) splitLeaf(node *Node) (int, *Node) {
	mid := len(node.keys) / 2
	right := &Node{
		isLeaf: true,
		keys:   append([]int{}, node.keys[mid:]...),
		values: append([]string{}, node.values[mid:]...),
		next:   node.next,
	}
	node.keys = node.keys[:mid]
	node.values = node.values[:mid]
	node.next = right
	return right.keys[0], right
}

// splitInternal moves the middle key up; left keeps keys[:mid], right gets keys[mid+1:].
func (t *BPlusTree) splitInternal(node *Node) (int, *Node) {
	mid := len(node.keys) / 2
	splitKey := node.keys[mid]
	right := &Node{
		keys:     append([]int{}, node.keys[mid+1:]...),
		children: append([]*Node{}, node.children[mid+1:]...),
	}
	node.keys = node.keys[:mid]
	node.children = node.children[:mid+1]
	return splitKey, right
}
