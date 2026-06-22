package bplustree

const maxKeys = 5 
type node struct {
	leaf     bool
	keys     []int
	vals     []any    // only used in leaves
	children []*node  // only used in internals
	next     *node    // linked list for leaf scan
}

func makeLeaf() *node {
	return &node{leaf: true}
}

func makeInternal() *node {
	return &node{leaf: false}
}

type BPlusTree struct {
	root *node
	sz   int
}

func New() *BPlusTree {
	return &BPlusTree{root: makeLeaf()}
}

func (t *BPlusTree) Size() int {
	return t.sz
}

func (t *BPlusTree) Search(key int) (any, bool) {
	lf := t.leafFor(key)
	for i, k := range lf.keys {
		if k == key {
			return lf.vals[i], true
		}
	}
	return nil, false
}

func (t *BPlusTree) leafFor(key int) *node {
	n := t.root
	for !n.leaf {
		pos := upperBound(n.keys, key)
		n = n.children[pos]
	}
	return n
}

func (t *BPlusTree) Insert(key int, value any) {
	upKey, newRight := t.doInsert(t.root, key, value)
	if newRight == nil {
		return
	}
	nr := makeInternal()
	nr.keys = []int{upKey}
	nr.children = []*node{t.root, newRight}
	t.root = nr
}

func (t *BPlusTree) doInsert(n *node, key int, val any) (int, *node) {
	if n.leaf {
		return t.insertLeaf(n, key, val)
	}

	pos := upperBound(n.keys, key)
	splitKey, splitChild := t.doInsert(n.children[pos], key, val)
	if splitChild == nil {
		return 0, nil
	}

	n.keys = sliceInsert(n.keys, pos, splitKey)
	n.children = nodeSliceInsert(n.children, pos+1, splitChild)

	if len(n.keys) < maxKeys {
		return 0, nil
	}
	return t.splitInternal(n)
}

func (t *BPlusTree) insertLeaf(lf *node, key int, val any) (int, *node) {
	pos := lowerBound(lf.keys, key)

	if pos < len(lf.keys) && lf.keys[pos] == key {
		lf.vals[pos] = val
		return 0, nil
	}

	t.sz++
	lf.keys = sliceInsert(lf.keys, pos, key)
	lf.vals = anySliceInsert(lf.vals, pos, val)

	if len(lf.keys) < maxKeys {
		return 0, nil
	}
	return t.splitLeaf(lf)
}

func (t *BPlusTree) splitLeaf(lf *node) (int, *node) {
	mid := 2 // order - 1
	right := makeLeaf()

	right.keys = append(right.keys, lf.keys[mid:]...)
	right.vals = append(right.vals, lf.vals[mid:]...)

	right.next = lf.next
	lf.next = right

	lf.keys = lf.keys[:mid]
	lf.vals = lf.vals[:mid]

	return right.keys[0], right
}

func (t *BPlusTree) splitInternal(n *node) (int, *node) {
	mid := 2
	promoted := n.keys[mid]

	right := makeInternal()
	right.keys = append(right.keys, n.keys[mid+1:]...)
	right.children = append(right.children, n.children[mid+1:]...)

	n.keys = n.keys[:mid]
	n.children = n.children[:mid+1]

	return promoted, right
}

func (t *BPlusTree) Delete(key int) bool {
	ok := t.doDelete(t.root, key, nil, 0)
	if !ok {
		return false
	}
	t.sz--

	if !t.root.leaf && len(t.root.keys) == 0 {
		t.root = t.root.children[0]
	}
	return true
}

func (t *BPlusTree) doDelete(n *node, key int, parent *node, childIdx int) bool {
	if n.leaf {
		return t.removeFromLeaf(n, key, parent, childIdx)
	}

	i := upperBound(n.keys, key)
	if !t.doDelete(n.children[i], key, n, i) {
		return false
	}


	child := n.children[i]
	minKeys := 2 // order - 1
	if len(child.keys) >= minKeys {
		return true
	}

	if i > 0 && len(n.children[i-1].keys) >= 3 { // order
		t.rotateRight(n, i)
	} else if i < len(n.children)-1 && len(n.children[i+1].keys) >= 3 {
		t.rotateLeft(n, i)
	} else if i > 0 {
		t.mergeChildren(n, i-1)
	} else {
		t.mergeChildren(n, i)
	}
	return true
}

func (t *BPlusTree) removeFromLeaf(lf *node, key int, parent *node, childIdx int) bool {
	pos := lowerBound(lf.keys, key)
	if pos >= len(lf.keys) || lf.keys[pos] != key {
		return false
	}

	lf.keys = sliceRemove(lf.keys, pos)
	lf.vals = anySliceRemove(lf.vals, pos)

	if parent != nil && pos == 0 && len(lf.keys) > 0 && childIdx > 0 {
		parent.keys[childIdx-1] = lf.keys[0]
	}
	return true
}

func (t *BPlusTree) rotateRight(parent *node, i int) {
	child := parent.children[i]
	left := parent.children[i-1]

	if child.leaf {

		stolen := left.keys[len(left.keys)-1]
		stolenVal := left.vals[len(left.vals)-1]
		child.keys = append([]int{stolen}, child.keys...)
		child.vals = append([]any{stolenVal}, child.vals...)
		left.keys = left.keys[:len(left.keys)-1]
		left.vals = left.vals[:len(left.vals)-1]
		parent.keys[i-1] = child.keys[0]
	} else {
		child.keys = append([]int{parent.keys[i-1]}, child.keys...)
		child.children = append([]*node{left.children[len(left.children)-1]}, child.children...)
		parent.keys[i-1] = left.keys[len(left.keys)-1]
		left.keys = left.keys[:len(left.keys)-1]
		left.children = left.children[:len(left.children)-1]
	}
}

func (t *BPlusTree) rotateLeft(parent *node, i int) {
	child := parent.children[i]
	right := parent.children[i+1]

	if child.leaf {
		child.keys = append(child.keys, right.keys[0])
		child.vals = append(child.vals, right.vals[0])
		right.keys = right.keys[1:]
		right.vals = right.vals[1:]
		parent.keys[i] = right.keys[0]
	} else {
		child.keys = append(child.keys, parent.keys[i])
		child.children = append(child.children, right.children[0])
		parent.keys[i] = right.keys[0]
		right.keys = right.keys[1:]
		right.children = right.children[1:]
	}
}

func (t *BPlusTree) mergeChildren(parent *node, i int) {
	left := parent.children[i]
	right := parent.children[i+1]

	if left.leaf {
		left.keys = append(left.keys, right.keys...)
		left.vals = append(left.vals, right.vals...)
		left.next = right.next
	} else {
		left.keys = append(left.keys, parent.keys[i])
		left.keys = append(left.keys, right.keys...)
		left.children = append(left.children, right.children...)
	}

	parent.keys = sliceRemove(parent.keys, i)
	parent.children = nodeSliceRemove(parent.children, i+1)
}
func (t *BPlusTree) Range(start, end int) [][2]any {
	var out [][2]any

	lf := t.leafFor(start)
	for lf != nil {
		for i, k := range lf.keys {
			if k > end {
				return out
			}
			if k >= start {
				out = append(out, [2]any{k, lf.vals[i]})
			}
		}
		lf = lf.next
	}
	return out
}


func lowerBound(keys []int, target int) int {
	lo, hi := 0, len(keys)
	for lo < hi {
		m := lo + (hi-lo)/2
		if keys[m] < target {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

func upperBound(keys []int, target int) int {
	lo, hi := 0, len(keys)
	for lo < hi {
		m := lo + (hi-lo)/2
		if keys[m] <= target {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

func sliceInsert(s []int, i, v int) []int {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func sliceRemove(s []int, i int) []int {
	return append(s[:i], s[i+1:]...)
}

func anySliceInsert(s []any, i int, v any) []any {
	s = append(s, nil)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func anySliceRemove(s []any, i int) []any {
	return append(s[:i], s[i+1:]...)
}

func nodeSliceInsert(s []*node, i int, v *node) []*node {
	s = append(s, nil)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func nodeSliceRemove(s []*node, i int) []*node {
	return append(s[:i], s[i+1:]...)
}
