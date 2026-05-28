package persiststructs

import (
	"os"
	"testing"
)

func tempSeekStore(t *testing.T) (*SeekStore, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "seekstore-*.bin")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	s, err := OpenSeekStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return s, func() {
		s.Close()
		os.Remove(f.Name())
	}
}

func TestWriteAndReadBack(t *testing.T) {
	s, cleanup := tempSeekStore(t)
	defer cleanup()

	want := Person{ID: 1, Age: 30, Name: NameFromString("Alice")}
	if err := s.Write(0, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read(0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Fatalf("roundtrip mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestMultipleRecordsAtDifferentIndexes(t *testing.T) {
	s, cleanup := tempSeekStore(t)
	defer cleanup()

	people := []Person{
		{ID: 1, Age: 20, Name: NameFromString("Alice")},
		{ID: 2, Age: 35, Name: NameFromString("Bob")},
		{ID: 3, Age: 28, Name: NameFromString("Carol")},
	}
	for i, p := range people {
		if err := s.Write(i, p); err != nil {
			t.Fatalf("Write[%d]: %v", i, err)
		}
	}

	for i, want := range people {
		got, err := s.Read(i)
		if err != nil {
			t.Fatalf("Read[%d]: %v", i, err)
		}
		if got != want {
			t.Errorf("record[%d] mismatch: got %+v want %+v", i, got, want)
		}
	}
}

func TestOverwriteRecord(t *testing.T) {
	s, cleanup := tempSeekStore(t)
	defer cleanup()

	s.Write(0, Person{ID: 1, Age: 20, Name: NameFromString("Alice")})
	updated := Person{ID: 1, Age: 21, Name: NameFromString("Alice-updated")}
	s.Write(0, updated)

	got, err := s.Read(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != updated {
		t.Fatalf("overwrite failed: got %+v", got)
	}
}

func TestWriteNonContiguous(t *testing.T) {
	s, cleanup := tempSeekStore(t)
	defer cleanup()

	// Write at index 5, skipping 0-4.
	want := Person{ID: 99, Age: 50, Name: NameFromString("Zara")}
	if err := s.Write(5, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.Read(5)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Fatalf("non-contiguous write failed: got %+v", got)
	}
}

func TestCount(t *testing.T) {
	s, cleanup := tempSeekStore(t)
	defer cleanup()

	for i := 0; i < 4; i++ {
		s.Write(i, Person{ID: uint32(i)})
	}
	n, err := s.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("Count = %d, want 4", n)
	}
}

func TestNameRoundtrip(t *testing.T) {
	cases := []string{"Alice", "Bob", "A very long name here!!!", ""}
	for _, name := range cases {
		n := NameFromString(name)
		got := NameToString(n)
		if got != name {
			t.Errorf("NameToString(%q) = %q", name, got)
		}
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	f, _ := os.CreateTemp("", "seekstore-reopen-*.bin")
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	want := Person{ID: 7, Age: 42, Name: NameFromString("Dave")}

	s1, _ := OpenSeekStore(path)
	s1.Write(0, want)
	s1.Close()

	// Reopen and read back - verifies data actually hit disk.
	s2, _ := OpenSeekStore(path)
	defer s2.Close()
	got, err := s2.Read(0)
	if err != nil {
		t.Fatalf("Read after reopen: %v", err)
	}
	if got != want {
		t.Fatalf("data lost across reopen: got %+v want %+v", got, want)
	}
}
