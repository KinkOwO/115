package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WriteEntryCopy replaces one existing script in a NEW current-format inner
// archive. Paths/string pools/hash indexes are unchanged. Append within the
// owning chunk, retaining every other entry's original bytes and offsets.
// It never overwrites a file and never mutates the loaded source archive.
func (a *Archive) WriteEntryCopy(output, path string, replacement []byte) error {
	if a.format != FormatDFO20260901 || len(replacement) == 0 || len(replacement)%5 != 0 || len(replacement) > 1024*1024 {
		return fmt.Errorf("unsupported script replacement")
	}
	f, ok := a.FindFile(path)
	if !ok || f.DataType != 1 {
		return fmt.Errorf("existing script required")
	}
	item := a.items[f.Index]
	oldChunk, e := a.chunk(item.chunkIndex)
	if e != nil {
		return e
	}
	newChunk := make([]byte, 0, len(oldChunk)+len(replacement))
	newChunk = append(newChunk, oldChunk...)
	newChunk = append(newChunk, replacement...)
	if len(newChunk) > 256*1024*1024 {
		return fmt.Errorf("replacement chunk too large")
	}
	var compressed bytes.Buffer
	z, e := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	if e != nil {
		return e
	}
	if _, e = z.Write(newChunk); e != nil {
		return e
	}
	if e = z.Close(); e != nil {
		return e
	}
	body := compressed.Bytes()
	decryptProtected("mAIn", body)
	oldStart := 0
	if item.chunkIndex > 0 {
		oldStart = a.groups[item.chunkIndex-1].compressedSize
	}
	oldEnd := a.groups[item.chunkIndex].compressedSize
	delta := len(body) - (oldEnd - oldStart)
	if a.header.bodySize+delta < 0 || int64(a.header.bodySize)+int64(delta) > 2147483647 {
		return fmt.Errorf("replacement body overflow")
	}
	header := a.header.plain
	binary.LittleEndian.PutUint32(header[32:36], uint32(a.header.bodySize+delta))
	decryptProtected("iNfO", header[:])
	groupOffset := a.bodyOff - len(a.groups)*groupItemSize
	groups := make([]byte, len(a.groups)*groupItemSize)
	for i, g := range a.groups {
		end, size := g.compressedSize, g.originalSize
		if i >= item.chunkIndex {
			end += delta
		}
		if i == item.chunkIndex {
			size = len(newChunk)
		}
		binary.LittleEndian.PutUint32(groups[i*8:], uint32(end))
		binary.LittleEndian.PutUint32(groups[i*8+4:], uint32(size))
	}
	decryptProtected("Gidx", groups)
	recordOffset := headerSize + f.Index*fileItemSize
	record := append([]byte(nil), a.data[recordOffset:recordOffset+fileItemSize]...)
	binary.LittleEndian.PutUint32(record[12:16], uint32(len(oldChunk)))
	binary.LittleEndian.PutUint32(record[16:20], uint32(len(replacement)))
	dst, e := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer dst.Close()
	parts := [][]byte{header[:], a.data[headerSize:recordOffset], record, a.data[recordOffset+fileItemSize : groupOffset], groups, a.data[a.bodyOff : a.bodyOff+oldStart], body, a.data[a.bodyOff+oldEnd:]}
	for _, p := range parts {
		if _, e = io.Copy(dst, bytes.NewReader(p)); e != nil {
			return e
		}
	}
	if e = dst.Sync(); e != nil {
		return e
	}
	return dst.Close()
}
