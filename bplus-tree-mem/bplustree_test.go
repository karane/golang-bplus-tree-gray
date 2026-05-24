package bplustree

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchEmpty(t *testing.T) {
	tree := New(3)
	_, found := tree.Search(1)
	assert.False(t, found)
}

func TestInsertAndSearchSingle(t *testing.T) {
	tree := New(3)
	tree.Insert(42, "forty-two")
	v, found := tree.Search(42)
	require.True(t, found)
	assert.Equal(t, "forty-two", v)
}

func TestSearchMissingKey(t *testing.T) {
	tree := New(3)
	tree.Insert(1, "one")
	tree.Insert(3, "three")
	_, found := tree.Search(2)
	assert.False(t, found)
}

func TestDuplicateKeyOverwrites(t *testing.T) {
	tree := New(3)
	tree.Insert(1, "first")
	tree.Insert(1, "second")
	v, found := tree.Search(1)
	require.True(t, found)
	assert.Equal(t, "second", v)
}

func TestLeafSplit(t *testing.T) {
	tree := New(3)
	tree.Insert(1, "one")
	tree.Insert(2, "two")
	tree.Insert(3, "three")
	for k := 1; k <= 3; k++ {
		_, found := tree.Search(k)
		assert.True(t, found, "key %d not found after leaf split", k)
	}
}

func TestInsertOrderedAllSearchable(t *testing.T) {
	tree := New(3)
	for k := 1; k <= 10; k++ {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}
	for k := 1; k <= 10; k++ {
		v, found := tree.Search(k)
		require.True(t, found, "key %d not found", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

func TestInsertReverseOrderAllSearchable(t *testing.T) {
	tree := New(3)
	for k := 10; k >= 1; k-- {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}
	for k := 1; k <= 10; k++ {
		v, found := tree.Search(k)
		require.True(t, found, "key %d not found", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

func TestInternalNodeSplit(t *testing.T) {
	tree := New(3)
	for k := 1; k <= 7; k++ {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}
	for k := 1; k <= 7; k++ {
		v, found := tree.Search(k)
		require.True(t, found, "key %d not found after internal split", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

func TestLargeInsertAllSearchable(t *testing.T) {
	tree := New(5)
	for k := 1; k <= 100; k++ {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}
	for k := 1; k <= 100; k++ {
		v, found := tree.Search(k)
		require.True(t, found, "key %d not found", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

// --- structural tests ---

// Single insert: root is a leaf holding exactly one key/value, no children.
func TestStructureSingleInsert(t *testing.T) {
	tree := New(3)
	tree.Insert(10, "ten")

	root := tree.root
	require.NotNil(t, root)
	assert.True(t, root.isLeaf)
	assert.Equal(t, []int{10}, root.keys)
	assert.Equal(t, []string{"ten"}, root.values)
	assert.Empty(t, root.children)
	assert.Nil(t, root.next)
}

// Two inserts, no split yet: root remains a leaf with both keys sorted.
func TestStructureTwoInserts(t *testing.T) {
	tree := New(3)
	tree.Insert(20, "twenty")
	tree.Insert(10, "ten")

	root := tree.root
	require.NotNil(t, root)
	assert.True(t, root.isLeaf)
	assert.Equal(t, []int{10, 20}, root.keys)
	assert.Equal(t, []string{"ten", "twenty"}, root.values)
	assert.Empty(t, root.children)
	assert.Nil(t, root.next)
}

// Third insert with order=3 triggers a leaf split.
// Expected tree:
//
//	internal root  keys=[2]
//	  left leaf    keys=[1]  values=["one"]
//	  right leaf   keys=[2,3] values=["two","three"]
//	  left.next -> right leaf
func TestStructureLeafSplit(t *testing.T) {
	tree := New(3)
	tree.Insert(1, "one")
	tree.Insert(2, "two")
	tree.Insert(3, "three")

	root := tree.root
	require.NotNil(t, root)
	assert.False(t, root.isLeaf, "root should be internal after split")
	assert.Equal(t, []int{2}, root.keys)
	require.Len(t, root.children, 2)

	left := root.children[0]
	right := root.children[1]

	assert.True(t, left.isLeaf)
	assert.Equal(t, []int{1}, left.keys)
	assert.Equal(t, []string{"one"}, left.values)

	assert.True(t, right.isLeaf)
	assert.Equal(t, []int{2, 3}, right.keys)
	assert.Equal(t, []string{"two", "three"}, right.values)

	assert.Same(t, right, left.next, "left.next should point to right leaf")
	assert.Nil(t, right.next, "rightmost leaf next should be nil")
}

// Four inserts with order=3: two leaf splits, root stays internal with 3 children.
// Expected:
//
//	internal root  keys=[2,3]
//	  leaf  keys=[1]
//	  leaf  keys=[2]
//	  leaf  keys=[3,4]
func TestStructureTwoLeafSplits(t *testing.T) {
	tree := New(3)
	for k := 1; k <= 4; k++ {
		tree.Insert(k, fmt.Sprintf("v%d", k))
	}

	root := tree.root
	require.NotNil(t, root)
	assert.False(t, root.isLeaf)
	assert.Equal(t, []int{2, 3}, root.keys)
	require.Len(t, root.children, 3, "three leaves expected")

	assert.Equal(t, []int{1}, root.children[0].keys)
	assert.Equal(t, []int{2}, root.children[1].keys)
	assert.Equal(t, []int{3, 4}, root.children[2].keys)

	// Linked list: left --> mid --> right
	assert.Same(t, root.children[1], root.children[0].next)
	assert.Same(t, root.children[2], root.children[1].next)
	assert.Nil(t, root.children[2].next)
}

// Seven inserts with order=3: verify the leaf linked list covers all keys
// in sorted order (tests that next pointers survive internal splits too).
func TestStructureLeafLinkedList(t *testing.T) {
	tree := New(3)
	for k := 7; k >= 1; k-- {
		tree.Insert(k, fmt.Sprintf("v%d", k))
	}

	// Walk down to the leftmost leaf.
	node := tree.root
	for !node.isLeaf {
		node = node.children[0]
	}

	var got []int
	for node != nil {
		got = append(got, node.keys...)
		node = node.next
	}
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7}, got)
}

func TestHigherOrderTree(t *testing.T) {
	tree := New(10)
	for k := 50; k >= 1; k-- {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}
	for k := 1; k <= 50; k++ {
		v, found := tree.Search(k)
		require.True(t, found, "key %d not found", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}
