package pvf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// Compiled `.ctp` tables (contents/2026/apocalypse/etc/*.ctp) are a container
// of their own, not a PVF script. The layout below was recovered from the
// client's binary reader (sub_1474D11A0 → sub_1474D0C00 per record, and
// sub_1474D0580 per cell) and is verified against two independent files:
// the walk must consume exactly the declared record count and end where the
// trailer starts, the trailer must end where the string pool starts, and every
// name reference must resolve to a printable pool slice.
//
//	header  : 9 x u32 at 0x00 — version, recordCount, 0, 32, 0, cellEnd, 0,
//	          trailerEnd, 0. Both offset fields read four bytes short of the
//	          real boundary, so callers must not rely on their absolute value.
//	record  : u64 nameLo, u64 nameHi, u32 flags, u64 parent, u64 nCells,
//	          u64 nRefs, nCells x cell, nRefs x ref      (44 byte fixed head)
//	cell    : u32 tag followed by a tag-sized payload
//	ref     : u64 nameLo, u64 nameHi, u64 n, n x u64
//	trailer : column name -> the record indices (rows) that column owns
//	pool    : packed ASCII tags, no separators; character 0 is the first '['
const (
	ctpHeaderSize   = 0x24
	ctpRecordHeader = 44
	ctpHeaderSlack  = 4 // declared offsets sit this far before the real boundary
)

const (
	ctpCellName        = 1
	ctpCellNameIndexed = 2
	ctpCellByte        = 3
	ctpCellFloat       = 4
	ctpCellFloatAlt    = 5
)

// ctpCellPayload maps a cell tag to its payload width. A tag outside the map
// carries no payload (the client treats it as padding), which is why unknown
// tags are tolerated rather than rejected.
var ctpCellPayload = map[uint32]int{1: 16, 2: 20, 3: 1, 4: 8, 5: 8, 6: 5, 7: 5}

// CTPCell is one value. Kind names the encoding so consumers never have to
// re-interpret the tag themselves.
type CTPCell struct {
	Tag   uint32  `json:"tag"`
	Kind  string  `json:"kind"`
	Float float64 `json:"float,omitempty"`
	Byte  byte    `json:"byte,omitempty"`
	Name  string  `json:"name,omitempty"`
	Index uint32  `json:"index,omitempty"`
	Raw   string  `json:"raw,omitempty"`
}

// CTPRef is a child reference: a column name plus the record indices it owns.
type CTPRef struct {
	Name   string   `json:"name"`
	Values []uint64 `json:"values"`
}

// CTPRecord is one row. Parent is the record index of the owning container and
// is -1 for a top level row, which is how the column tree is expressed.
type CTPRecord struct {
	Index  int       `json:"index"`
	Offset int       `json:"offset"`
	Name   string    `json:"name"`
	Flags  uint32    `json:"flags"`
	Parent int       `json:"parent"`
	Cells  []CTPCell `json:"cells"`
	Refs   []CTPRef  `json:"refs"`
}

// CTPColumn is one trailer entry.
type CTPColumn struct {
	Name    string   `json:"name"`
	Records []uint64 `json:"records"`
}

// CTPTable is a fully decoded `.ctp` entry.
type CTPTable struct {
	Path        string      `json:"path"`
	SHA256      string      `json:"sha256"`
	Bytes       int         `json:"bytes"`
	Version     uint32      `json:"version"`
	RecordCount uint32      `json:"recordCount"`
	PoolChars   int         `json:"poolChars"`
	PoolTags    []string    `json:"poolTags"`
	Records     []CTPRecord `json:"records"`
	Columns     []CTPColumn `json:"columns"`
}

// RecordsOf returns every row carrying the given column name, in file order.
func (t *CTPTable) RecordsOf(name string) []CTPRecord {
	var out []CTPRecord
	for _, r := range t.Records {
		if r.Name == name {
			out = append(out, r)
		}
	}
	return out
}

// Column returns the trailer entry for a column name, or nil when the column
// is absent. Absence is meaningful for columns such as [allow coin], which only
// some operation blocks carry.
func (t *CTPTable) Column(name string) *CTPColumn {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

// Floats returns the float values of a record in file order.
func (r CTPRecord) Floats() []float64 {
	var out []float64
	for _, c := range r.Cells {
		if c.Kind == "float" {
			out = append(out, c.Float)
		}
	}
	return out
}

// Float returns the i-th float value of a record.
func (r CTPRecord) Float(i int) (float64, bool) {
	f := r.Floats()
	if i < 0 || i >= len(f) {
		return 0, false
	}
	return f[i], true
}

// Texts returns the string values of a record in file order.
func (r CTPRecord) Texts() []string {
	var out []string
	for _, c := range r.Cells {
		if c.Kind == "name" || c.Kind == "name_indexed" {
			out = append(out, c.Name)
		}
	}
	return out
}

// Ints returns the float values of a record that are whole numbers, which is
// how the container stores small integers (ids, counts, weights).
func (r CTPRecord) Ints() []int64 {
	var out []int64
	for _, f := range r.Floats() {
		out = append(out, int64(f))
	}
	return out
}

// CTP decodes a compiled table entry. Every structural assumption is asserted;
// a reshaped container is refused instead of silently misread.
func (a *Archive) CTP(path string) (*CTPTable, error) {
	f, ok := a.FindFile(path)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, path)
	}
	if f.DataType != 4 {
		return nil, fmt.Errorf("entry %s is data type %d, not a compiled table", path, f.DataType)
	}
	raw, err := a.ReadRaw(path)
	if err != nil {
		return nil, err
	}
	return ReadCTP(path, raw)
}

// ReadCTP decodes a compiled table from its raw bytes.
func ReadCTP(path string, raw []byte) (*CTPTable, error) {
	if len(raw) < ctpHeaderSize {
		return nil, fmt.Errorf("table %s is shorter than the header: %d bytes", path, len(raw))
	}
	version := readUint32(raw[0x00:0x04])
	recordCount := readUint32(raw[0x04:0x08])
	cellEnd := int(readUint32(raw[0x14:0x18])) + ctpHeaderSlack
	trailerEnd := int(readUint32(raw[0x1c:0x20])) + ctpHeaderSlack
	if version != 1 {
		return nil, fmt.Errorf("table %s has unsupported version %d", path, version)
	}
	if cellEnd <= ctpHeaderSize || cellEnd > len(raw) {
		return nil, fmt.Errorf("table %s declares a cell region ending at %#x of %d bytes", path, cellEnd, len(raw))
	}
	if trailerEnd < cellEnd || trailerEnd >= len(raw) {
		return nil, fmt.Errorf("table %s declares a trailer ending at %#x after cells %#x in %d bytes", path, trailerEnd, cellEnd, len(raw))
	}
	poolStart, ok := ctpPoolStart(raw, trailerEnd)
	if !ok {
		return nil, fmt.Errorf("table %s has no string pool after trailer %#x", path, trailerEnd)
	}
	pool := string(raw[poolStart:])

	table := &CTPTable{
		Path:        path,
		Bytes:       len(raw),
		Version:     version,
		RecordCount: recordCount,
		PoolChars:   len(pool),
	}
	sum := sha256.Sum256(raw)
	table.SHA256 = hex.EncodeToString(sum[:])
	table.PoolTags = ctpPoolTags(pool)

	off := ctpHeaderSize
	for i := 0; i < int(recordCount); i++ {
		rec, next, err := readCTPRecord(raw, pool, off, len(table.Records), len(raw))
		if err != nil {
			return nil, fmt.Errorf("table %s record %d at %#x: %w", path, i, off, err)
		}
		table.Records = append(table.Records, rec)
		off = next
	}
	if off > poolStart {
		return nil, fmt.Errorf("table %s record stream overran the pool (%#x > %#x)", path, off, poolStart)
	}
	for _, rec := range table.Records {
		if rec.Parent >= len(table.Records) {
			return nil, fmt.Errorf("table %s record %d names parent %d of %d records",
				path, rec.Index, rec.Parent, len(table.Records))
		}
	}
	columns, err := readCTPColumns(raw, pool, off, poolStart)
	if err != nil {
		return nil, fmt.Errorf("table %s trailer: %w", path, err)
	}
	table.Columns = columns
	return table, nil
}

// ctpPoolStart starts at the declared trailer boundary and skips NUL padding.
// Trailer row indices can contain 0x5b, so a bracket search from the cell end
// would mistake an index byte for the beginning of the string pool.
func ctpPoolStart(raw []byte, from int) (int, bool) {
	for from < len(raw) && raw[from] == 0 {
		from++
	}
	if from < len(raw) && raw[from] == '[' {
		return from, true
	}
	return 0, false
}

// ctpPoolTags lists every bracketed tag in pool order.
func ctpPoolTags(pool string) []string {
	var out []string
	for i := 0; i < len(pool); {
		open := strings.IndexByte(pool[i:], '[')
		if open < 0 {
			break
		}
		open += i
		shut := strings.IndexByte(pool[open:], ']')
		if shut < 0 {
			break
		}
		out = append(out, pool[open:open+shut+1])
		i = open + shut + 1
	}
	return out
}

// ctpName resolves a (lo, hi) pool character range. Unlike the client, which
// reads until a NUL, the range itself is authoritative: hi-lo equals the
// string length. An unresolvable range is a shape error, not a default.
func ctpName(pool string, lo, hi uint64) (string, error) {
	if lo > hi {
		lo, hi = hi, lo
	}
	if hi > uint64(len(pool)) {
		return "", fmt.Errorf("name range [%d,%d) exceeds the %d character pool", lo, hi, len(pool))
	}
	s := pool[lo:hi]
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			return "", fmt.Errorf("name range [%d,%d) is not printable", lo, hi)
		}
	}
	return s, nil
}

func readCTPRecord(raw []byte, pool string, off, index, limit int) (CTPRecord, int, error) {
	if off+ctpRecordHeader > limit {
		return CTPRecord{}, 0, fmt.Errorf("truncated record header")
	}
	rec := CTPRecord{Index: index, Offset: off, Cells: []CTPCell{}, Refs: []CTPRef{}}
	name, err := ctpName(pool, readUint64(raw[off:off+8]), readUint64(raw[off+8:off+16]))
	if err != nil {
		return CTPRecord{}, 0, err
	}
	rec.Name = name
	rec.Flags = readUint32(raw[off+16 : off+20])
	parent := int64(readUint64(raw[off+20 : off+28]))
	if parent < -1 {
		return CTPRecord{}, 0, fmt.Errorf("parent %d is not a record index", parent)
	}
	rec.Parent = int(parent)
	nCells := readUint64(raw[off+28 : off+36])
	nRefs := readUint64(raw[off+36 : off+44])
	if nCells > 1<<20 || nRefs > 1<<20 {
		return CTPRecord{}, 0, fmt.Errorf("implausible cell/ref counts (%d, %d)", nCells, nRefs)
	}
	off += ctpRecordHeader

	for i := uint64(0); i < nCells; i++ {
		if off+4 > limit {
			return CTPRecord{}, 0, fmt.Errorf("truncated cell tag at %#x", off)
		}
		tag := readUint32(raw[off : off+4])
		off += 4
		size := ctpCellPayload[tag]
		if off+size > limit {
			return CTPRecord{}, 0, fmt.Errorf("cell payload for tag %d overruns at %#x", tag, off)
		}
		cell, err := readCTPCell(tag, raw[off:off+size], pool)
		if err != nil {
			return CTPRecord{}, 0, fmt.Errorf("cell %d: %w", i, err)
		}
		rec.Cells = append(rec.Cells, cell)
		off += size
	}
	for i := uint64(0); i < nRefs; i++ {
		if off+24 > limit {
			return CTPRecord{}, 0, fmt.Errorf("truncated ref at %#x", off)
		}
		refName, err := ctpName(pool, readUint64(raw[off:off+8]), readUint64(raw[off+8:off+16]))
		if err != nil {
			return CTPRecord{}, 0, err
		}
		n := readUint64(raw[off+16 : off+24])
		off += 24
		if n > 1<<20 || off+8*int(n) > limit {
			return CTPRecord{}, 0, fmt.Errorf("ref %s declares %d values at %#x", refName, n, off)
		}
		ref := CTPRef{Name: refName, Values: make([]uint64, 0, n)}
		for v := uint64(0); v < n; v++ {
			ref.Values = append(ref.Values, readUint64(raw[off:off+8]))
			off += 8
		}
		rec.Refs = append(rec.Refs, ref)
	}
	return rec, off, nil
}

func readCTPCell(tag uint32, payload []byte, pool string) (CTPCell, error) {
	cell := CTPCell{Tag: tag}
	switch tag {
	case ctpCellFloat, ctpCellFloatAlt:
		cell.Kind = "float"
		cell.Float = math.Float64frombits(readUint64(payload))
	case ctpCellByte:
		cell.Kind = "byte"
		cell.Byte = payload[0]
	case ctpCellName:
		cell.Kind = "name"
		name, err := ctpName(pool, readUint64(payload[0:8]), readUint64(payload[8:16]))
		if err != nil {
			return cell, err
		}
		cell.Name = name
	case ctpCellNameIndexed:
		cell.Kind = "name_indexed"
		cell.Index = readUint32(payload[0:4])
		name, err := ctpName(pool, readUint64(payload[4:12]), readUint64(payload[12:20]))
		if err != nil {
			return cell, err
		}
		cell.Name = name
	default:
		cell.Kind = "opaque"
		cell.Raw = hex.EncodeToString(payload)
	}
	return cell, nil
}

func readCTPColumns(raw []byte, pool string, off, end int) ([]CTPColumn, error) {
	var out []CTPColumn
	for off < end {
		if raw[off] == 0 && allZero(raw[off:end]) {
			// A single NUL pad byte separates the trailer from the pool.
			off = end
			break
		}
		if off+24 > end {
			return nil, fmt.Errorf("truncated entry at %#x", off)
		}
		name, err := ctpName(pool, readUint64(raw[off:off+8]), readUint64(raw[off+8:off+16]))
		if err != nil {
			return nil, err
		}
		n := readUint64(raw[off+16 : off+24])
		off += 24
		if n > 1<<20 || off+8*int(n) > end {
			return nil, fmt.Errorf("entry %s declares %d rows at %#x", name, n, off)
		}
		col := CTPColumn{Name: name, Records: make([]uint64, 0, n)}
		for i := uint64(0); i < n; i++ {
			col.Records = append(col.Records, readUint64(raw[off:off+8]))
			off += 8
		}
		out = append(out, col)
	}
	if off != end {
		return nil, fmt.Errorf("trailer ended at %#x, expected %#x", off, end)
	}
	return out, nil
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func readUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func readUint64(b []byte) uint64 {
	return uint64(readUint32(b[0:4])) | uint64(readUint32(b[4:8]))<<32
}
