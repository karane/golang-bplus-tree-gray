// write and read fixed-size structs in a file using Seek.
package persiststructs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Person is the struct we want to persist.
type Person struct {
	ID   uint32
	Age  uint8
	Name [32]byte 
}

// PersonSize is the encoded byte size of one Person: 4 + 1 + 32 = 37 bytes.
const PersonSize = 37

// NameFromString copies s (up to 32 bytes) into a [32]byte array.
func NameFromString(s string) [32]byte {
	var n [32]byte
	copy(n[:], s)
	return n
}

// NameToString trims trailing zero bytes from a [32]byte name.
func NameToString(n [32]byte) string {
	end := len(n)
	for end > 0 && n[end-1] == 0 {
		end--
	}
	return string(n[:end])
}

// SeekStore persists Person records in a flat file using Seek.
// Records are stored at fixed offsets: index * PersonSize.
type SeekStore struct {
	f *os.File
}

// OpenSeekStore opens (or creates) a store file at path.
func OpenSeekStore(path string) (*SeekStore, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("OpenSeekStore: %w", err)
	}
	return &SeekStore{f: f}, nil
}

// Close closes the underlying file.
func (s *SeekStore) Close() error { return s.f.Close() }

// Write encodes p and writes it at position idx in the file.
func (s *SeekStore) Write(idx int, p Person) error {
	offset := int64(idx) * PersonSize
	if _, err := s.f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("Seek: %w", err)
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, p); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	_, err := s.f.Write(buf.Bytes())
	return err
}

// Read seeks to position idx and decodes the Person stored there.
func (s *SeekStore) Read(idx int) (Person, error) {
	offset := int64(idx) * PersonSize
	if _, err := s.f.Seek(offset, io.SeekStart); err != nil {
		return Person{}, fmt.Errorf("Seek: %w", err)
	}
	var p Person
	if err := binary.Read(s.f, binary.BigEndian, &p); err != nil {
		return Person{}, fmt.Errorf("decode: %w", err)
	}
	return p, nil
}

// Count returns how many Person records fit in the current file size.
func (s *SeekStore) Count() (int, error) {
	info, err := s.f.Stat()
	if err != nil {
		return 0, err
	}
	return int(info.Size()) / PersonSize, nil
}
