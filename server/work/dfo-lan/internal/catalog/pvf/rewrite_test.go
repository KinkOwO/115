package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEntryCopyPreservesSiblingChunksAndRefusesOverwrite(t *testing.T) {
	chunks := [][]byte{[]byte("abcdefghij"), []byte("klmno")}
	a := &Archive{format: FormatDFO20260901, bodyOff: headerSize + 3*fileItemSize + 2*groupItemSize, pathIdx: map[string]int{"target.co": 0}}
	a.header.fileCount = 3
	a.header.groupCount = 2
	a.data = make([]byte, a.bodyOff)
	for _, raw := range chunks {
		var b bytes.Buffer
		z := zlib.NewWriter(&b)
		z.Write(raw)
		z.Close()
		p := b.Bytes()
		decryptProtected("mAIn", p)
		a.data = append(a.data, p...)
		a.groups = append(a.groups, groupItem{len(a.data) - a.bodyOff, len(raw)})
	}
	a.header.bodySize = len(a.data) - a.bodyOff
	binary.LittleEndian.PutUint32(a.header.plain[32:], uint32(a.header.bodySize))
	a.items = []fileItem{{chunkIndex: 0, dataOffset: 0, dataSize: 5, dataType: 1}, {chunkIndex: 0, dataOffset: 5, dataSize: 5, dataType: 1}, {chunkIndex: 1, dataOffset: 0, dataSize: 5, dataType: 1}}
	for i, f := range a.items {
		p := a.data[headerSize+i*24:]
		binary.LittleEndian.PutUint32(p[8:], uint32(f.chunkIndex))
		binary.LittleEndian.PutUint32(p[12:], uint32(f.dataOffset))
		binary.LittleEndian.PutUint32(p[16:], uint32(f.dataSize))
		binary.LittleEndian.PutUint32(p[20:], 1)
	}
	a.files = []File{{Index: 0, DataType: 1, ArchivePath: "target.co"}}
	sourceCopy := append([]byte(nil), a.data...)
	out := filepath.Join(t.TempDir(), "copy.pvf")
	if e := a.WriteEntryCopy(out, "target.co", []byte("0123456789")); e != nil {
		t.Fatal(e)
	}
	p, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	header := append([]byte(nil), p[:headerSize]...)
	decryptProtected("iNfO", header)
	if int(binary.LittleEndian.Uint32(header[32:])) != len(p)-a.bodyOff {
		t.Fatal("header body size stale")
	}
	g := append([]byte(nil), p[a.bodyOff-16:a.bodyOff]...)
	decryptProtected("Gidx", g)
	previous := 0
	decoded := [][]byte{}
	for i := 0; i < 2; i++ {
		end := int(binary.LittleEndian.Uint32(g[i*8:]))
		encoded := append([]byte(nil), p[a.bodyOff+previous:a.bodyOff+end]...)
		decryptProtected("mAIn", encoded)
		raw, e := zlibBytes(encoded)
		if e != nil {
			t.Fatal(e)
		}
		if len(raw) != int(binary.LittleEndian.Uint32(g[i*8+4:])) {
			t.Fatal("group size stale")
		}
		decoded = append(decoded, raw)
		previous = end
	}
	if !bytes.Equal(decoded[0][:10], chunks[0]) || !bytes.Equal(decoded[1], chunks[1]) {
		t.Fatal("sibling or other group changed")
	}
	if !bytes.Equal(p[headerSize+24:a.bodyOff-16], a.data[headerSize+24:a.bodyOff-16]) {
		t.Fatal("other file records changed")
	}
	row := p[headerSize : headerSize+24]
	off, size := binary.LittleEndian.Uint32(row[12:]), binary.LittleEndian.Uint32(row[16:])
	if string(decoded[0][off:off+size]) != "0123456789" {
		t.Fatal("replacement row wrong")
	}
	if !bytes.Equal(a.data, sourceCopy) {
		t.Fatal("loaded source mutated")
	}
	if e = a.WriteEntryCopy(out, "target.co", []byte("ABCDE")); e == nil {
		t.Fatal("existing output overwritten")
	}
	again, _ := os.ReadFile(out)
	if !bytes.Equal(p, again) {
		t.Fatal("refused overwrite changed output")
	}
}
