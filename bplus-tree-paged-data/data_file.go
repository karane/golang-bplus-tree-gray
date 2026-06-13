package bplustree

// Data file page layout (slotted page, pages 1+):
//
//   [0:2]   slotCount   (uint16 LE)  -- number of records on this page
//   [2:4]   freeOffset  (uint16 LE)  -- low-water mark; records packed from end downward
//
//   Slot array (grows forward from byte 4):
//     each entry: [recOffset: uint16][recLen: uint16]  (4 bytes)
//
//   Free space: [4 + slotCount*4 : freeOffset]
//
//   Record data (packed from end of page toward the slot array):
//     raw bytes of each record
//
// Page 0 is the data file header:
//   [0:4]   magic      (0xDA7AF11E)
//   [4:8]   totalPages (int32 LE)

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	dataPageSizeInBytes   = 4096
	dataMagic             = uint32(0xDA7AF11E)
	pageHeaderSizeInBytes = 4 // slotCount(2) + freeOffset(2)
	slotEntrySizeInBytes  = 4 // recOffset(2) + recLen(2)
	slotLengthInBytes     = 2 // offset of recLen within a slot entry
)

// RecordPtr locates a record within the data file
type RecordPtr struct {
	PageID int32
	Slot   int32
}

type DataFile struct {
	f          *os.File
	totalPages int32
}

func OpenDataFile(path string) (*DataFile, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	d := &DataFile{f: f}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.Size() == 0 {
		d.totalPages = 1 // page 0 = header
		if err := d.writeFileHeader(); err != nil {
			f.Close()
			return nil, err
		}
	} else {
		if err := d.readFileHeader(); err != nil {
			f.Close()
			return nil, err
		}
	}
	return d, nil
}

func (d *DataFile) Close() error {
	return d.f.Close()
}

// maxRecordSize is the largest record that fits on a single data page.
func maxRecordSize() int {
	return dataPageSizeInBytes - pageHeaderSizeInBytes - slotEntrySizeInBytes
}

// Append writes value to the data file and returns its location.
func (d *DataFile) Append(value string) (RecordPtr, error) {
	rec := []byte(value)
	recSize := len(rec)
	if recSize > maxRecordSize() {
		return RecordPtr{}, fmt.Errorf("record size %d exceeds max %d", recSize, maxRecordSize())
	}

	var (
		pageID int32
		buf    []byte
	)

	// Try the last data page if one exists
	if d.totalPages > 1 {
		pageID = d.totalPages - 1
		var err error
		buf, err = d.readPage(pageID)
		if err != nil {
			return RecordPtr{}, err
		}

		slotCount := int(binary.LittleEndian.Uint16(buf[0:2]))
		freeOffset := int(binary.LittleEndian.Uint16(buf[2:4]))
		freeSpace := freeOffset - pageHeaderSizeInBytes - slotCount*slotEntrySizeInBytes

		if recSize+slotEntrySizeInBytes > freeSpace {
			buf = nil // page full; fall through to allocate
		}
	}

	if buf == nil {
		var err error
		pageID, buf, err = d.allocatePage()
		if err != nil {
			return RecordPtr{}, err
		}
	}

	slotCount := int(binary.LittleEndian.Uint16(buf[0:2]))
	freeOffset := int(binary.LittleEndian.Uint16(buf[2:4]))

	// Write record bytes growing from end of page downward.
	newFreeOffset := freeOffset - recSize
	copy(buf[newFreeOffset:freeOffset], rec)

	// Write slot entry.
	slotOff := pageHeaderSizeInBytes + slotCount*slotEntrySizeInBytes
	binary.LittleEndian.PutUint16(buf[slotOff:], uint16(newFreeOffset))
	binary.LittleEndian.PutUint16(buf[slotOff+slotLengthInBytes:], uint16(recSize))

	// Update page header.
	binary.LittleEndian.PutUint16(buf[0:2], uint16(slotCount+1))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(newFreeOffset))

	if err := d.writePage(pageID, buf); err != nil {
		return RecordPtr{}, err
	}

	return RecordPtr{PageID: pageID, Slot: int32(slotCount)}, nil
}

func (d *DataFile) allocatePage() (int32, []byte, error) {
	pageID := d.totalPages
	d.totalPages++
	buf := make([]byte, dataPageSizeInBytes)

	binary.LittleEndian.PutUint16(buf[0:2], 0)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(dataPageSizeInBytes))
	if err := d.writeFileHeader(); err != nil {
		return 0, nil, err
	}

	return pageID, buf, nil
}

// Read retrieves the record at ptr
func (d *DataFile) Read(ptr RecordPtr) (string, error) {
	buf, err := d.readPage(ptr.PageID)
	if err != nil {
		return "", err
	}

	slotCount := int(binary.LittleEndian.Uint16(buf[0:2]))
	if int(ptr.Slot) >= slotCount {
		return "", fmt.Errorf("slot %d out of range (slotCount=%d)", ptr.Slot, slotCount)
	}

	slotOff := pageHeaderSizeInBytes + int(ptr.Slot)*slotEntrySizeInBytes
	offset := int(binary.LittleEndian.Uint16(buf[slotOff:]))
	length := int(binary.LittleEndian.Uint16(buf[slotOff+slotLengthInBytes:]))

	return string(buf[offset : offset+length]), nil
}

func (d *DataFile) readFileHeader() error {
	buf := make([]byte, dataPageSizeInBytes)

	if _, err := d.f.ReadAt(buf, 0); err != nil {
		return fmt.Errorf("read data file header: %w", err)
	}

	if m := binary.LittleEndian.Uint32(buf[0:4]); m != dataMagic {
		return fmt.Errorf("bad data file magic: %#x", m)
	}
	d.totalPages = int32(binary.LittleEndian.Uint32(buf[4:8]))

	return nil
}

func (d *DataFile) writeFileHeader() error {
	buf := make([]byte, dataPageSizeInBytes)

	binary.LittleEndian.PutUint32(buf[0:4], dataMagic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(d.totalPages))

	if _, err := d.f.WriteAt(buf, 0); err != nil {
		return fmt.Errorf("write data file header: %w", err)
	}

	return nil
}

func (d *DataFile) readPage(id int32) ([]byte, error) {
	buf := make([]byte, dataPageSizeInBytes)
	if _, err := d.f.ReadAt(buf, int64(id)*dataPageSizeInBytes); err != nil {
		return nil, fmt.Errorf("read data page %d: %w", id, err)
	}

	return buf, nil
}

func (d *DataFile) writePage(id int32, buf []byte) error {
	if _, err := d.f.WriteAt(buf, int64(id)*dataPageSizeInBytes); err != nil {
		return fmt.Errorf("write data page %d: %w", id, err)
	}

	return nil
}
