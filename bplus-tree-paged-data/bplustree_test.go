package bplustree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// runSuite runs the same logical test suite against any Store backend.
func runSuite(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()

	t.Run("SearchEmpty", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		_, found, err := tree.Search(1)
		assert.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("InsertAndSearchSingle", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		assert.NoError(t, tree.Insert(42, "forty-two"))
		v, found, err := tree.Search(42)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "forty-two", v)
	})

	t.Run("SearchMissingKey", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(1, "one")
		tree.Insert(3, "three")
		_, found, err := tree.Search(2)
		assert.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("DuplicateKeyOverwrites", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		tree.Insert(1, "first")
		tree.Insert(1, "second")
		v, found, err := tree.Search(1)

		assert.NoError(t, err)
		assert.True(t, found)
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
			assert.NoError(t, err)
			assert.True(t, found, "key %d not found after leaf split", k)
		}
	})

	t.Run("InternalNodeSplit", func(t *testing.T) {
		// 7 inserts with order=3 forces both leaf and internal splits
		tree := Open(newStore(t))
		defer tree.Close()

		for k := int64(1); k <= 7; k++ {
			assert.NoError(t, tree.Insert(k, fmt.Sprintf("val%d", k)))
		}

		for k := int64(1); k <= 7; k++ {
			v, found, err := tree.Search(k)
			assert.NoError(t, err)
			assert.True(t, found, "key %d not found", k)
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
			assert.NoError(t, err)
			assert.True(t, found, "key %d not found", k)
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
			assert.NoError(t, err)
			assert.True(t, found, "key %d not found", k)
			assert.Equal(t, fmt.Sprintf("val%d", k), v)
		}
	})

	// --- structural tests (keys and tree shape only; values are opaque ptrs) ---

	t.Run("StructureSingleInsert", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		assert.NoError(t, tree.Insert(10, "ten"))

		root, err := store.LoadNode(store.RootID())

		assert.NoError(t, err)
		assert.True(t, root.isLeaf)
		assert.Equal(t, []int64{10}, root.keys)
		assert.Len(t, root.ptrs, 1)
		assert.Empty(t, root.children)
		assert.Equal(t, noPage, root.next)
	})

	t.Run("StructureTwoInserts", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		assert.NoError(t, tree.Insert(20, "twenty"))
		assert.NoError(t, tree.Insert(10, "ten"))

		root, err := store.LoadNode(store.RootID())
		assert.NoError(t, err)
		assert.True(t, root.isLeaf)
		assert.Equal(t, []int64{10, 20}, root.keys)
		assert.Len(t, root.ptrs, 2)
		assert.Empty(t, root.children)
		assert.Equal(t, noPage, root.next)
	})

	t.Run("StructureLeafSplit", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		assert.NoError(t, tree.Insert(1, "one"))
		assert.NoError(t, tree.Insert(2, "two"))
		assert.NoError(t, tree.Insert(3, "three"))

		root, err := store.LoadNode(store.RootID())
		assert.NoError(t, err)
		assert.False(t, root.isLeaf, "root should be internal after split")
		assert.Equal(t, []int64{2}, root.keys)
		assert.Len(t, root.children, 2)

		left, err := store.LoadNode(root.children[0])
		assert.NoError(t, err)
		right, err := store.LoadNode(root.children[1])
		assert.NoError(t, err)

		assert.True(t, left.isLeaf)
		assert.Equal(t, []int64{1}, left.keys)
		assert.Len(t, left.ptrs, 1)

		assert.True(t, right.isLeaf)
		assert.Equal(t, []int64{2, 3}, right.keys)
		assert.Len(t, right.ptrs, 2)

		assert.Equal(t, right.id, left.next, "left.next should be the right leaf's page ID")
		assert.Equal(t, noPage, right.next, "rightmost leaf next should be noPage")
	})

	t.Run("StructureTwoLeafSplits", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		for k := int64(1); k <= 4; k++ {
			assert.NoError(t, tree.Insert(k, fmt.Sprintf("v%d", k)))
		}

		root, err := store.LoadNode(store.RootID())
		assert.NoError(t, err)
		assert.False(t, root.isLeaf)
		assert.Equal(t, []int64{2, 3}, root.keys)
		assert.Len(t, root.children, 3)

		l0, _ := store.LoadNode(root.children[0])
		l1, _ := store.LoadNode(root.children[1])
		l2, _ := store.LoadNode(root.children[2])

		assert.Equal(t, []int64{1}, l0.keys)
		assert.Equal(t, []int64{2}, l1.keys)
		assert.Equal(t, []int64{3, 4}, l2.keys)

		assert.Equal(t, l1.id, l0.next)
		assert.Equal(t, l2.id, l1.next)
		assert.Equal(t, noPage, l2.next)
	})

	t.Run("StructureLeafLinkedList", func(t *testing.T) {
		store := newStore(t)
		tree := Open(store)
		defer tree.Close()

		for k := int64(7); k >= 1; k-- {
			assert.NoError(t, tree.Insert(k, fmt.Sprintf("v%d", k)))
		}

		nodeID := store.RootID()
		for {
			n, err := store.LoadNode(nodeID)
			assert.NoError(t, err)
			if n.isLeaf {
				break
			}
			nodeID = n.children[0]
		}

		var got []int64
		for nodeID != noPage {
			n, err := store.LoadNode(nodeID)
			assert.NoError(t, err)
			got = append(got, n.keys...)
			nodeID = n.next
		}

		assert.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7}, got)
	})

	// --- data file specific tests ---

	t.Run("LongValueRoundTrip", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		long := strings.Repeat("x", 500)
		assert.NoError(t, tree.Insert(1, long))
		v, found, err := tree.Search(1)

		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, long, v)
	})

	t.Run("LongValueExceedsPageCapacity", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		// maxRecordSize for a 4096-byte page = 4096 - 4 - 4 = 4088 bytes
		// This test only applies to PagedStore; MemStore has no size limit.
		tooBig := strings.Repeat("z", 5000)
		err := tree.Insert(1, tooBig)
		// MemStore never errors on size; only assert for PagedStore via the suite name.
		// We skip the assertion here and cover it in the PagedStore-specific test below.
		_ = err
	})

	t.Run("MultipleDataPagesAllReadable", func(t *testing.T) {
		tree := Open(newStore(t))
		defer tree.Close()

		// Each value is ~2000 bytes; two records fill a data page, forcing multiple pages.
		val := strings.Repeat("a", 2000)
		n := 20

		for k := int64(1); k <= int64(n); k++ {
			assert.NoError(t, tree.Insert(k, val))
		}

		for k := int64(1); k <= int64(n); k++ {
			v, found, err := tree.Search(k)
			assert.NoError(t, err)
			assert.True(t, found, "key %d not found", k)
			assert.Equal(t, val, v)
		}
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
		assert.NoError(t, err)
		return s
	})
}

func TestPagedStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	// Write phase
	s, err := OpenPagedStore(path, 5)
	assert.NoError(t, err)
	tree := Open(s)

	for k := int64(1); k <= 30; k++ {
		assert.NoError(t, tree.Insert(k, fmt.Sprintf("val%d", k)), "insert %d", k)
	}
	assert.NoError(t, tree.Close())

	// Read phase: reopen the same file; order is ignored, read from header
	s2, err := OpenPagedStore(path, 0)
	assert.NoError(t, err)
	tree2 := Open(s2)
	defer tree2.Close()

	for k := int64(1); k <= 30; k++ {
		v, found, err := tree2.Search(k)
		assert.NoError(t, err, "search %d", k)
		assert.True(t, found, "key %d not found after reopen", k)
		assert.Equal(t, fmt.Sprintf("val%d", k), v)
	}
}

func TestOrderTooLargeForPageSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tree.db")
	// order 210 needs 20*210-7 = 4193 bytes > 4096
	_, err := OpenPagedStore(path, 210)

	assert.Error(t, err)
}

func TestDataFileCreatedAlongsideIndexFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tree.db")
	s, err := OpenPagedStore(path, 5)

	assert.NoError(t, err)
	tree := Open(s)
	assert.NoError(t, tree.Insert(1, "hello"))
	assert.NoError(t, tree.Close())

	_, err = os.Stat(path + ".data")
	assert.NoError(t, err, "data file should exist alongside the index file")
}

func TestDataFileRecordTooBig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tree.db")
	s, err := OpenPagedStore(path, 5)
	assert.NoError(t, err)
	tree := Open(s)
	defer tree.Close()

	tooBig := strings.Repeat("z", 5000)
	err = tree.Insert(1, tooBig)

	assert.Error(t, err, "inserting a record larger than a data page should fail")
}
