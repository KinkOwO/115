package pvf

import (
	"encoding/binary"
	"testing"
)

// Row index 91 contains '[' as its low byte. This is a valid trailer entry,
// not a pool character; the header's trailer boundary disambiguates them.
func TestReadCTPUsesDeclaredTrailerBoundary(t *testing.T) {
	const rows = 92
	const label = "[entry]"
	cellEnd := ctpHeaderSize + rows*ctpRecordHeader
	trailerEnd := cellEnd + 24 + rows*8
	raw := make([]byte, trailerEnd+1+len(label))
	binary.LittleEndian.PutUint32(raw, 1)
	binary.LittleEndian.PutUint32(raw[4:], rows)
	binary.LittleEndian.PutUint32(raw[20:], uint32(cellEnd-4))
	binary.LittleEndian.PutUint32(raw[28:], uint32(trailerEnd-4))
	for i := 0; i < rows; i++ {
		at := ctpHeaderSize + i*ctpRecordHeader
		binary.LittleEndian.PutUint64(raw[at+8:], uint64(len(label)))
		binary.LittleEndian.PutUint64(raw[at+20:], ^uint64(0))
	}
	binary.LittleEndian.PutUint64(raw[cellEnd+8:], uint64(len(label)))
	binary.LittleEndian.PutUint64(raw[cellEnd+16:], rows)
	for i := 0; i < rows; i++ {
		binary.LittleEndian.PutUint64(raw[cellEnd+24+i*8:], uint64(i))
	}
	copy(raw[trailerEnd+1:], label)
	table, err := ReadCTP("row-index-bracket.ctp", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Records) != rows || len(table.Columns) != 1 || table.Columns[0].Name != label || table.Columns[0].Records[91] != 91 || table.Records[0].Name != label {
		t.Fatal("trailer byte consumed as pool")
	}
	binary.LittleEndian.PutUint32(raw[28:], uint32(len(raw)))
	if _, err := ReadCTP("bad-trailer.ctp", raw); err == nil {
		t.Fatal("out-of-range trailer accepted")
	}
}
