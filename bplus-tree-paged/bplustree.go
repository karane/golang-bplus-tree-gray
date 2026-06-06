package bplustree

import "sort"

type BPlusTree struct {
	store Store
}

func Open(store Store) *BPlusTree {
	return &BPlusTree{store: store}
}

func (t *BPlusTree) Close() error {
	return t.store.Close()
}

func (t *BPlusTree) Search(key int64) (string, bool, error) {
	if t.store.RootID() == noPage {
		return "", false, nil
	}
	nodeID := t.store.RootID()
	for {
		node, err := t.store.LoadNode(nodeID)
		if err != nil {
			return "", false, err
		}
		if node.isLeaf {
			i := sort.Search(len(node.keys), func(i int) bool { return node.keys[i] >= key })
			if i < len(node.keys) && node.keys[i] == key {
				return node.values[i], true, nil
			}
			return "", false, nil
		}
		i := 0
		for i < len(node.keys) && key >= node.keys[i] {
			i++
		}
		nodeID = node.children[i]
	}
}

func (t *BPlusTree) Insert(key int64, value string) error {
	if t.store.RootID() == noPage {
		root, err := t.store.AllocNode()
		if err != nil {
			return err
		}
		root.isLeaf = true
		if err := t.store.SaveNode(root); err != nil {
			return err
		}
		if err := t.store.SetRootID(root.id); err != nil {
			return err
		}
	}
	splitKey, splitID, err := t.insert(t.store.RootID(), key, value)
	if err != nil {
		return err
	}
	if splitID == noPage {
		return nil
	}
	newRoot, err := t.store.AllocNode()
	if err != nil {
		return err
	}
	newRoot.isLeaf = false
	newRoot.keys = []int64{splitKey}
	newRoot.children = []int32{t.store.RootID(), splitID}
	if err := t.store.SaveNode(newRoot); err != nil {
		return err
	}
	return t.store.SetRootID(newRoot.id)
}

func (t *BPlusTree) insert(nodeID int32, key int64, value string) (int64, int32, error) {
	node, err := t.store.LoadNode(nodeID)
	if err != nil {
		return 0, noPage, err
	}

	if node.isLeaf {
		i := sort.Search(len(node.keys), func(i int) bool { return node.keys[i] >= key })
		if i < len(node.keys) && node.keys[i] == key {
			node.values[i] = value
			return 0, noPage, t.store.SaveNode(node)
		}
		
		node.keys = append(node.keys, 0)
		copy(node.keys[i+1:], node.keys[i:]) //copy(dst, src)
		node.keys[i] = key
		node.values = append(node.values, "")
		copy(node.values[i+1:], node.values[i:])
		node.values[i] = value
		if len(node.keys) > t.store.Order()-1 {
			return t.splitLeaf(node)
		}
		return 0, noPage, t.store.SaveNode(node)
	}

	i := 0
	for i < len(node.keys) && key >= node.keys[i] {
		i++
	}
	splitKey, splitID, err := t.insert(node.children[i], key, value)
	if err != nil {
		return 0, noPage, err
	}
	if splitID == noPage {
		return 0, noPage, nil
	}
	node.keys = append(node.keys, 0)
	copy(node.keys[i+1:], node.keys[i:])
	node.keys[i] = splitKey
	node.children = append(node.children, 0)
	copy(node.children[i+2:], node.children[i+1:])
	node.children[i+1] = splitID
	if len(node.keys) > t.store.Order()-1 {
		return t.splitInternal(node)
	}
	return 0, noPage, t.store.SaveNode(node)
}

func (t *BPlusTree) splitLeaf(node *Node) (int64, int32, error) {
	mid := len(node.keys) / 2
	right, err := t.store.AllocNode()
	if err != nil {
		return 0, noPage, err
	}
	right.isLeaf = true
	right.keys = append([]int64{}, node.keys[mid:]...)
	right.values = append([]string{}, node.values[mid:]...)
	right.next = node.next
	node.keys = node.keys[:mid]
	node.values = node.values[:mid]
	node.next = right.id
	if err := t.store.SaveNode(right); err != nil {
		return 0, noPage, err
	}
	if err := t.store.SaveNode(node); err != nil {
		return 0, noPage, err
	}
	return right.keys[0], right.id, nil
}

// splitInternal moves the middle key up; left keeps keys[:mid], right gets keys[mid+1:].
func (t *BPlusTree) splitInternal(node *Node) (int64, int32, error) {
	mid := len(node.keys) / 2
	splitKey := node.keys[mid]
	right, err := t.store.AllocNode()
	if err != nil {
		return 0, noPage, err
	}

	right.isLeaf = false
	right.keys = append([]int64{}, node.keys[mid+1:]...)
	right.children = append([]int32{}, node.children[mid+1:]...)
	node.keys = node.keys[:mid]
	node.children = node.children[:mid+1]

	if err := t.store.SaveNode(right); err != nil {
		return 0, noPage, err
	}

	if err := t.store.SaveNode(node); err != nil {
		return 0, noPage, err
	}

	return splitKey, right.id, nil
}
