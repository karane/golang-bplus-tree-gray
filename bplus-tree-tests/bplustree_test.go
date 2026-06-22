package bplustree

import (
	"math/rand"
	"sort"
	"testing"
)
func TestSearchEmptyTree(t *testing.T) {
	tree := New()
	_, found := tree.Search(42)
	if found {
		t.Fatal("expected nothing in empty tree")
	}
}

func TestInsertAndSearch(t *testing.T) {
	tree := New()
	tree.Insert(10, "ten")
	tree.Insert(20, "twenty")
	tree.Insert(5, "five")

	cases := []struct {
		key      int
		expected string
	}{
		{10, "ten"},
		{20, "twenty"},
		{5, "five"},
	}
	for _, c := range cases {
		v, ok := tree.Search(c.key)
		if !ok {
			t.Errorf("key %d not found", c.key)
		}
		if v != c.expected {
			t.Errorf("key %d: got %v, want %v", c.key, v, c.expected)
		}
	}
}

func TestSearchMissingKey(t *testing.T) {
	tree := New()
	tree.Insert(1, "a")
	tree.Insert(2, "b")
	_, found := tree.Search(99)
	if found {
		t.Fatal("key 99 should not exist")
	}
}

// Update 

func TestUpdateExistingKey(t *testing.T) {
	tree := New()
	tree.Insert(7, "original")
	tree.Insert(7, "updated")
	v, ok := tree.Search(7)
	if !ok {
		t.Fatal("key 7 not found after update")
	}
	if v != "updated" {
		t.Errorf("got %v, want 'updated'", v)
	}
	if tree.Size() != 1 {
		t.Errorf("size should be 1, got %d", tree.Size())
	}
}


func TestSize(t *testing.T) {
	tree := New()
	for i := 0; i < 20; i++ {
		tree.Insert(i, i)
	}
	if tree.Size() != 20 {
		t.Errorf("expected size 20, got %d", tree.Size())
	}
}


func TestManyInsertsSequential(t *testing.T) {
	tree := New()
	n := 100
	for i := 0; i < n; i++ {
		tree.Insert(i, i*10)
	}
	for i := 0; i < n; i++ {
		v, ok := tree.Search(i)
		if !ok {
			t.Fatalf("key %d not found", i)
		}
		if v != i*10 {
			t.Fatalf("key %d: got %v, want %v", i, v, i*10)
		}
	}
}

func TestManyInsertsReverse(t *testing.T) {
	tree := New()
	n := 100
	for i := n - 1; i >= 0; i-- {
		tree.Insert(i, i)
	}
	for i := 0; i < n; i++ {
		_, ok := tree.Search(i)
		if !ok {
			t.Fatalf("key %d not found after reverse insert", i)
		}
	}
}

func TestManyInsertsRandom(t *testing.T) {
	tree := New()
	rng := rand.New(rand.NewSource(42))
	keys := rng.Perm(200)
	for _, k := range keys {
		tree.Insert(k, k)
	}
	for _, k := range keys {
		v, ok := tree.Search(k)
		if !ok {
			t.Fatalf("key %d not found", k)
		}
		if v != k {
			t.Fatalf("key %d: got %v, want %v", k, v, k)
		}
	}
}


func TestRangeEmpty(t *testing.T) {
	tree := New()
	for i := 0; i < 20; i++ {
		tree.Insert(i*10, i)
	}
	result := tree.Range(1000, 2000)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestRangeSingleElement(t *testing.T) {
	tree := New()
	tree.Insert(5, "five")
	result := tree.Range(5, 5)
	if len(result) != 1 || result[0][0] != 5 {
		t.Errorf("expected [{5 five}], got %v", result)
	}
}

func TestRangeAllElements(t *testing.T) {
	tree := New()
	for i := 1; i <= 10; i++ {
		tree.Insert(i, i)
	}
	result := tree.Range(1, 10)
	if len(result) != 10 {
		t.Fatalf("expected 10 results, got %d", len(result))
	}
	for i, pair := range result {
		if pair[0] != i+1 {
			t.Errorf("position %d: got key %v, want %v", i, pair[0], i+1)
		}
	}
}

func TestRangeSubset(t *testing.T) {
	tree := New()
	for i := 0; i < 50; i++ {
		tree.Insert(i, i)
	}
	result := tree.Range(10, 20)
	if len(result) != 11 {
		t.Fatalf("expected 11 results (10..20), got %d", len(result))
	}
	for i, pair := range result {
		expected := 10 + i
		if pair[0] != expected {
			t.Errorf("position %d: got %v, want %v", i, pair[0], expected)
		}
	}
}

func TestRangeResultsAreSorted(t *testing.T) {
	tree := New()
	rng := rand.New(rand.NewSource(7))
	for _, k := range rng.Perm(100) {
		tree.Insert(k, k)
	}
	result := tree.Range(10, 60)
	for i := 1; i < len(result); i++ {
		if result[i][0].(int) < result[i-1][0].(int) {
			t.Fatalf("range not sorted at index %d: %v before %v", i, result[i-1][0], result[i][0])
		}
	}
}



func TestDeleteMissingKey(t *testing.T) {
	tree := New()
	tree.Insert(1, "a")
	if tree.Delete(99) {
		t.Fatal("delete of non-existent key should return false")
	}
	if tree.Size() != 1 {
		t.Errorf("size should still be 1, got %d", tree.Size())
	}
}

func TestDeleteAndSearch(t *testing.T) {
	tree := New()
	for i := 0; i < 20; i++ {
		tree.Insert(i, i)
	}
	for i := 0; i < 20; i += 2 {
		if !tree.Delete(i) {
			t.Fatalf("delete(%d) returned false", i)
		}
	}
	if tree.Size() != 10 {
		t.Errorf("expected size 10, got %d", tree.Size())
	}
	for i := 0; i < 20; i++ {
		_, found := tree.Search(i)
		if i%2 == 0 && found {
			t.Errorf("key %d should have been deleted", i)
		}
		if i%2 != 0 && !found {
			t.Errorf("key %d should still exist", i)
		}
	}
}

func TestDeleteAll(t *testing.T) {
	tree := New()
	n := 50
	for i := 0; i < n; i++ {
		tree.Insert(i, i)
	}
	for i := 0; i < n; i++ {
		if !tree.Delete(i) {
			t.Fatalf("delete(%d) failed", i)
		}
	}
	if tree.Size() != 0 {
		t.Errorf("expected empty tree, got size %d", tree.Size())
	}
	_, found := tree.Search(0)
	if found {
		t.Fatal("found key 0 in empty tree")
	}
}

func TestDeleteThenInsert(t *testing.T) {
	tree := New()
	for i := 0; i < 30; i++ {
		tree.Insert(i, i)
	}
	for i := 0; i < 15; i++ {
		tree.Delete(i)
	}
	for i := 0; i < 15; i++ {
		tree.Insert(i, i*100)
	}
	for i := 0; i < 30; i++ {
		v, ok := tree.Search(i)
		if !ok {
			t.Fatalf("key %d not found after re-insert", i)
		}
		expected := i
		if i < 15 {
			expected = i * 100
		}
		if v != expected {
			t.Errorf("key %d: got %v, want %v", i, v, expected)
		}
	}
}


func TestInsertSearchDeleteConsistency(t *testing.T) {
	tree := New()
	rng := rand.New(rand.NewSource(13))
	reference := make(map[int]int)

	for i := 0; i < 300; i++ {
		k := rng.Intn(200)
		reference[k] = k * 2
		tree.Insert(k, k*2)
	}

	for k, v := range reference {
		got, ok := tree.Search(k)
		if !ok {
			t.Fatalf("key %d missing", k)
		}
		if got != v {
			t.Fatalf("key %d: got %v, want %v", k, got, v)
		}
	}
	keys := make([]int, 0, len(reference))
	for k := range reference {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys[:len(keys)/2] {
		delete(reference, k)
		tree.Delete(k)
	}

	for k, v := range reference {
		got, ok := tree.Search(k)
		if !ok {
			t.Fatalf("survivor key %d missing", k)
		}
		if got != v {
			t.Fatalf("survivor key %d: got %v, want %v", k, got, v)
		}
	}
}

func BenchmarkInsert(b *testing.B) {
	tree := New()
	for i := 0; i < b.N; i++ {
		tree.Insert(i, i)
	}
}

func BenchmarkSearch(b *testing.B) {
	tree := New()
	for i := 0; i < 10_000; i++ {
		tree.Insert(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(i % 10_000)
	}
}

func BenchmarkRange(b *testing.B) {
	tree := New()
	for i := 0; i < 10_000; i++ {
		tree.Insert(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Range(1000, 2000)
	}
}
