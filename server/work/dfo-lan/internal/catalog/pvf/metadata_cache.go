package pvf

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"dfolan/internal/derivedcache"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
)

const metadataCacheMagic = "PVFMET01"
const metadataRawLimit = 1 << 30

var metadataCacheFormat = derivedcache.Format{Magic: metadataCacheMagic, PackedLimit: 512 << 20, RawLimit: metadataRawLimit}

// Metadata caches contain no source descriptor, body chunks, decoded scripts
// or runtime caches. The original immutable PVF remains open for every read.
type MetadataCacheStats struct{ Hits, Misses, Invalid, Writes, WriteErrors uint64 }

func (a *Archive) MetadataCacheStats() MetadataCacheStats { return a.metadataCache }

func metadataCacheKey(parser, source string) [32]byte {
	return sha256.Sum256([]byte(metadataCacheMagic + "\x00" + parser + "\x00" + source))
}

func metadataCachePath(dir string, key [32]byte) string {
	return filepath.Join(dir, fmt.Sprintf("archive-meta-%x.pvfc", key))
}

// Store packed native file records, sorted path hashes and the exact original
// pool bytes. Preparation keeps its existing fast raw-pool access, then shares
// the existing bounded compressed pool with its runtime views as before.
func encodeMetadata(w io.Writer, a *Archive) error {
	s := a.stringPools.state.Load()
	if s.compacted || a.format != FormatDFO20260901 {
		return fmt.Errorf("metadata cache requires unmodified native preparation metadata")
	}
	b := bufio.NewWriterSize(w, 128<<10)
	var header [headerSize + 24]byte
	copy(header[:headerSize], a.header.plain[:])
	binary.LittleEndian.PutUint64(header[48:56], uint64(len(a.compactIndex)))
	binary.LittleEndian.PutUint64(header[56:64], uint64(len(s.a)))
	binary.LittleEndian.PutUint64(header[64:72], uint64(len(s.w)))
	if _, err := b.Write(header[:]); err != nil {
		return err
	}
	if _, err := b.Write(a.compactTable); err != nil {
		return err
	}
	var row [12]byte
	for _, g := range a.groups {
		binary.LittleEndian.PutUint32(row[:4], uint32(g.compressedSize))
		binary.LittleEndian.PutUint32(row[4:8], uint32(g.originalSize))
		if _, err := b.Write(row[:8]); err != nil {
			return err
		}
	}
	for _, d := range a.compactIndex {
		binary.LittleEndian.PutUint64(row[:8], d.hash)
		binary.LittleEndian.PutUint32(row[8:], d.index)
		if _, err := b.Write(row[:]); err != nil {
			return err
		}
	}
	if _, err := b.Write(s.a); err != nil {
		return err
	}
	if _, err := b.Write(s.w); err != nil {
		return err
	}
	return b.Flush()
}

func decodeMetadata(reader io.Reader, a *Archive, h pvfHeader) error {
	r := bufio.NewReaderSize(reader, 128<<10)
	var header [headerSize + 24]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	if !bytes.Equal(header[:headerSize], h.plain[:]) {
		return fmt.Errorf("metadata cache native header mismatch")
	}
	n := binary.LittleEndian.Uint64(header[48:56])
	sizeA, sizeW := binary.LittleEndian.Uint64(header[56:64]), binary.LittleEndian.Uint64(header[64:72])
	// Bound all declared lengths before allocation, including overflow. The
	// outer envelope has already verified the entire compressed file digest.
	if n > uint64(h.fileCount) || sizeA > metadataRawLimit || sizeW > metadataRawLimit {
		return fmt.Errorf("metadata cache allocation budget exceeded")
	}
	tableSize, err := checkedMul(h.fileCount, fileItemSize)
	if err != nil {
		return err
	}
	groupSize, err := checkedMul(h.groupCount, groupItemSize)
	if err != nil {
		return err
	}
	rawSize := uint64(len(header)) + uint64(tableSize) + uint64(groupSize) + n*12 + sizeA + sizeW
	if rawSize > metadataRawLimit || a.bodyOff+h.bodySize != int(a.snapshot.Size) {
		return fmt.Errorf("metadata cache section budget/boundary mismatch")
	}
	a.header, a.format = h, FormatDFO20260901
	a.compactTable = make([]byte, tableSize)
	if _, err := io.ReadFull(r, a.compactTable); err != nil {
		return err
	}
	a.groups = make([]groupItem, h.groupCount)
	var row [12]byte
	previous := 0
	for i := range a.groups {
		if _, err := io.ReadFull(r, row[:8]); err != nil {
			return err
		}
		g := groupItem{readInt32(row[:4]), readInt32(row[4:8])}
		if g.compressedSize <= previous || g.compressedSize > h.bodySize || g.originalSize < 0 || g.originalSize > 256<<20 {
			return fmt.Errorf("metadata cache group %d bounds", i)
		}
		a.groups[i], previous = g, g.compressedSize
	}
	if previous != h.bodySize {
		return fmt.Errorf("metadata cache body coverage mismatch")
	}
	a.compactIndex = make([]directoryEntry, int(n))
	seen := make([]byte, (h.fileCount+7)/8)
	for i := range a.compactIndex {
		if _, err := io.ReadFull(r, row[:]); err != nil {
			return err
		}
		d := directoryEntry{binary.LittleEndian.Uint64(row[:8]), binary.LittleEndian.Uint32(row[8:])}
		if uint64(d.index) >= uint64(h.fileCount) || seen[d.index/8]&(1<<(d.index%8)) != 0 {
			return fmt.Errorf("metadata cache invalid/duplicate file index")
		}
		seen[d.index/8] |= 1 << (d.index % 8)
		if i > 0 {
			prev := a.compactIndex[i-1]
			if d.hash < prev.hash || d.hash == prev.hash && d.index <= prev.index {
				return fmt.Errorf("metadata cache path hashes not ordered")
			}
		}
		a.compactIndex[i] = d
	}
	poolA, poolW := make([]byte, int(sizeA)), make([]byte, int(sizeW))
	if _, err := io.ReadFull(r, poolA); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, poolW); err != nil {
		return err
	}
	if _, err := r.ReadByte(); err != io.EOF {
		return fmt.Errorf("metadata cache trailing or damaged payload: %v", err)
	}
	a.stringPools = newRuntimeStringPools(poolA, poolW)
	return nil
}

func (a *Archive) MetadataCacheFile() string { return a.metadataCacheFile }
