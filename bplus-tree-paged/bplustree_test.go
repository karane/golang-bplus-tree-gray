package bplustree

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runSuite runs the same logical test suite against any Store backend
func runSuite(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()

	t.Run("SearchEmpty", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()
		_, found, err := tree.Search(1)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("InsertAndSearchSingle", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(42, "forty-two")
		v, found, err := tree.Search(42)

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "forty-two", v)
	})

	t.Run("SearchMissingKey", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(1, "one")
		tree.Insert(3, "three")
		_, found, err := tree.Search(2)

		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("DuplicateKeyOverwrites", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(1, "first")
		tree.Insert(1, "second")
		v, found, err := tree.Search(1)

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "second", v)
	})

	t.Run("LeafSplit", func(t *testing.T) {
		// order=3: third insert triggers leaf split
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(1, "one")
		tree.Insert(2, "two")
		tree.Insert(3, "three")

		for k := int64(1); k <= 3; k++ {
			_, found, err := tree.Search(k)
			require.NoError(t, err)
			assert.True(t, found, "key %d not found after leaf split", k)
		}
	})

	t.Run("InternalNodeSplit", func(t *testing.T) {
		// 7 inserts with order=3 forces both leaf and internal splits
		tree := Open(newStore(t))
		defer tree.Close()

		for k := int64(1); k <= 7; k++ {
			tree.Insert(k, fmt.Sprintf("val%d", k))
		}

		for k := int64(1); k <= 7; k++ {
			v, found, err := tree.Search(k)
			require.NoError(t, err)
			require.True(t, found, "key %d not found", k)
			assert.Equal(t, fmt.Sprintf("val%d", k), v)
		}
	})

	t.Run("InsertOrderedAllSearchable", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		for k := int64(1); k <= 20; k++ {
			tree.Insert(k, fmt.Sprintf("val%d", k))
		}

		for k := int64(1); k <= 20; k++ {
			v, found, err := tree.Search(k)
			require.NoError(t, err)
			require.True(t, found, "key %d not found", k)
			assert.Equal(t, fmt.Sprintf("val%d", k), v)
		}
	})

	t.Run("InsertReverseOrderAllSearchable", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		for k := int64(20); k >= 1; k-- {
			tree.Insert(k, fmt.Sprintf("val%d", k))
		}

		for k := int64(1); k <= 20; k++ {
			v, found, err := tree.Search(k)
			require.NoError(t, err)
			require.True(t, found, "key %d not found", k)
			assert.Equal(t, fmt.Sprintf("val%d", k), v)
		}
	})

	// --- structural tests ---

	// Single insert: root page is a leaf with exactly one key/value.
	t.Run("StructureSingleInsert", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		tree.Insert(10, "ten")

		root, err := store.LoadNode(store.RootID())
		require.NoError(t, err)
		assert.True(t, root.isLeaf)
		assert.Equal(t, []int64{10}, root.keys)
		assert.Equal(t, []string{"ten"}, root.values)
		assert.Empty(t, root.children)
		assert.Equal(t, noPage, root.next)
	})

	// Two inserts, no split yet: root is still a leaf with both keys sorted.
	t.Run("StructureTwoInserts", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		tree.Insert(20, "twenty")
		tree.Insert(10, "ten")

		root, err := store.LoadNode(store.RootID())
		require.NoError(t, err)
		assert.True(t, root.isLeaf)
		assert.Equal(t, []int64{10, 20}, root.keys)
		assert.Equal(t, []string{"ten", "twenty"}, root.values)
		assert.Empty(t, root.children)
		assert.Equal(t, noPage, root.next)
	})

	// Third insert with order=3 triggers a leaf split.
	// Root becomes internal with keys=[2] and two leaf children.
	// left.next page ID == right page ID.
	t.Run("StructureLeafSplit", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		tree.Insert(1, "one")
		tree.Insert(2, "two")
		tree.Insert(3, "three")

		root, err := store.LoadNode(store.RootID())
		require.NoError(t, err)
		assert.False(t, root.isLeaf, "root should be internal after split")
		assert.Equal(t, []int64{2}, root.keys)
		require.Len(t, root.children, 2)

		left, err := store.LoadNode(root.children[0])
		require.NoError(t, err)
		right, err := store.LoadNode(root.children[1])
		require.NoError(t, err)

		assert.True(t, left.isLeaf)
		assert.Equal(t, []int64{1}, left.keys)
		assert.Equal(t, []string{"one"}, left.values)

		assert.True(t, right.isLeaf)
		assert.Equal(t, []int64{2, 3}, right.keys)
		assert.Equal(t, []string{"two", "three"}, right.values)

		assert.Equal(t, right.id, left.next, "left.next should be the right leaf's page ID")
		assert.Equal(t, noPage, right.next, "rightmost leaf next should be noPage")
	})

	// Four inserts with order=3: two leaf splits, root stays internal with 3 children.
	// Root keys=[2,3]; leaves hold [1], [2], [3,4] with a valid next-pointer chain.
	t.Run("StructureTwoLeafSplits", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		for k := int64(1); k <= 4; k++ {
			tree.Insert(k, fmt.Sprintf("v%d", k))
		}

		root, err := store.LoadNode(store.RootID())
		require.NoError(t, err)
		assert.False(t, root.isLeaf)
		assert.Equal(t, []int64{2, 3}, root.keys)
		require.Len(t, root.children, 3)

		l0, err := store.LoadNode(root.children[0])
		require.NoError(t, err)
		l1, err := store.LoadNode(root.children[1])
		require.NoError(t, err)
		l2, err := store.LoadNode(root.children[2])
		require.NoError(t, err)

		assert.Equal(t, []int64{1}, l0.keys)
		assert.Equal(t, []int64{2}, l1.keys)
		assert.Equal(t, []int64{3, 4}, l2.keys)

		assert.Equal(t, l1.id, l0.next)
		assert.Equal(t, l2.id, l1.next)
		assert.Equal(t, noPage, l2.next)
	})

	// Seven reverse-order inserts: walk next-pointer chain from leftmost leaf
	// and assert all keys appear in sorted order.
	t.Run("StructureLeafLinkedList", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		for k := int64(7); k >= 1; k-- {
			tree.Insert(k, fmt.Sprintf("v%d", k))
		}

		// Descend to leftmost leaf.
		nodeID := store.RootID()
		for {
			n, err := store.LoadNode(nodeID)
			require.NoError(t, err)
			if n.isLeaf {
				break
			}
			nodeID = n.children[0]
		}

		var got []int64
		for nodeID != noPage {
			n, err := store.LoadNode(nodeID)
			require.NoError(t, err)
			got = append(got, n.keys...)
			nodeID = n.next
		}
		assert.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7}, got)
	})
}

func TestWithMemStore(t *testing.T) {
	runSuite(t, func(t *testing.T) Store {
		return NewMemStore(3)
	})
}

func TestWithPagedStore(t *testing.T) {
	runSuite(t, func(t *testing.T) Store {
		path := filepath.Join(t.TempDir(), "tree.db")
		s, err := OpenPagedStore(path, 3)
		require.NoError(t, err)

		return s
	})
}

func TestPagedStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	// Write phase
	s, err := OpenPagedStore(path, 5)
	require.NoError(t, err)
	tree := Open(s)

	for k := int64(1); k <= 30; k++ {
		tree.Insert(k, fmt.Sprintf("val%d", k))
	}

	require.NoError(t, tree.Close())

	// Read phase: reopen the same file; order is ignored, read from header
	s2, err := OpenPagedStore(path, 0)
	require.NoError(t, err)
	tree2 := Open(s2)
	defer tree2.Close()

	for k := int64(1); k <= 30; k++ {
		v, found, err := tree2.Search(k)
		require.NoError(t, err, "search %d", k)
		require.True(t, found, "key %d not found after reopen", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

func TestOrderTooLargeForPageSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tree.db")
	_, err := OpenPagedStore(path, 60) // order 60 needs 4497 bytes > 4096

	assert.Error(t, err)
}
