package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// addEntryFixture 造一份展开态（内存）归档，供"新增条目"的往返验证使用。
func addEntryFixture(t *testing.T) *Archive {
	t.Helper()
	a, err := OpenBytes(compactFixture(t))
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	return a
}

// hashSectionOf 按当前文件头算出哈希流那一段的原始字节（用于断言"逐字节没动"）。
func hashSectionOf(t *testing.T, raw []byte) []byte {
	t.Helper()
	hdr := append([]byte(nil), raw[:headerSize]...)
	decryptProtected("iNfO", hdr)
	fileCount := readInt32(hdr[24:28])
	hashSize := readInt32(hdr[40:44])
	off := headerSize + fileCount*fileItemSize
	if off+hashSize > len(raw) {
		t.Fatalf("哈希流越界：off=%d size=%d len=%d", off, hashSize, len(raw))
	}
	return raw[off : off+hashSize]
}

// TestWriteEntryAddedRoundTrip 是 Tier C 的第一道门：新增一条路径后，
// ① 新条目读得回来且内容逐字节一致；② **除它以外每一条路径与内容都不变**；
// ③ 哈希流逐字节原样；④ 六个节段的边界校验全部通过（能打开就是过了）。
func TestWriteEntryAddedRoundTrip(t *testing.T) {
	a := addEntryFixture(t)
	before := a.Files()

	// 5 字节对齐的合法脚本体（PVF 单元 = type + u32）
	script := make([]byte, 15)
	for i := range script {
		script[i] = byte(i + 1)
	}

	out := filepath.Join(t.TempDir(), "added.pvf")
	if err := a.WriteEntryAdded(out, "Dir", "added.stk", script); err != nil {
		t.Fatalf("WriteEntryAdded: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	added, err := OpenBytes(raw)
	if err != nil {
		t.Fatalf("读回失败（节段边界校验没过）：%v", err)
	}
	after := added.Files()

	// ① 条目数 +1，新条目可读且内容一致
	if len(after) != len(before)+1 {
		t.Fatalf("条目数：%d → %d，期望 +1", len(before), len(after))
	}
	idx := added.FindFileIndex("Dir/added.stk")
	if idx < 0 {
		t.Fatalf("新条目查不到：Dir/added.stk；现有 %v", pathsOf(after))
	}
	got, err := added.FileRaw(idx)
	if err != nil {
		t.Fatalf("读新条目失败：%v", err)
	}
	if !bytes.Equal(got, script) {
		t.Fatalf("新条目内容不一致：got %d 字节 want %d 字节", len(got), len(script))
	}
	if f := after[idx]; f.DataType != 1 || f.Size != len(script) {
		t.Fatalf("新条目元数据不对：%+v", f)
	}

	// ② 除新条目外，逐条路径 + 内容都不变
	for i := range before {
		if before[i].ArchivePath != after[i].ArchivePath {
			t.Fatalf("第 %d 条路径被改动：%q → %q", i, before[i].ArchivePath, after[i].ArchivePath)
		}
		want, err := a.FileRaw(i)
		if err != nil {
			t.Fatalf("原档读第 %d 条失败：%v", i, err)
		}
		now, err := added.FileRaw(i)
		if err != nil {
			t.Fatalf("新档读第 %d 条失败：%v", i, err)
		}
		if !bytes.Equal(want, now) {
			t.Fatalf("第 %d 条（%s）内容被改动：%d vs %d 字节",
				i, before[i].ArchivePath, len(want), len(now))
		}
	}

	// ③ 哈希流逐字节原样
	if !bytes.Equal(hashSectionOf(t, compactFixture(t)), hashSectionOf(t, raw)) {
		t.Fatal("哈希流被改动了（本阶段要求逐字节原样）")
	}
}

func TestWriteEntryAddedRefusals(t *testing.T) {
	a := addEntryFixture(t)
	out := filepath.Join(t.TempDir(), "x.pvf")

	if err := a.WriteEntryAdded(out, "Dir", "entry.stk", make([]byte, 5)); err == nil {
		t.Fatal("已存在的路径必须被拒绝（那属于 WriteEntryCopy 的职责）")
	}
	if err := a.WriteEntryAdded(out, "Dir", "bad.stk", make([]byte, 7)); err == nil {
		t.Fatal("非 5 字节对齐的脚本体必须被拒绝")
	}
	if err := a.WriteEntryAdded(out, "Dir", "empty.stk", nil); err == nil {
		t.Fatal("空脚本体必须被拒绝")
	}
	if err := a.WriteEntryAdded(out, "Dir", "big.stk", make([]byte, (1024*1024)+5)); err == nil {
		t.Fatal("超过 1MB 的脚本体必须被拒绝")
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("被拒绝的调用不该留下任何输出文件")
	}
}

func pathsOf(files []File) string {
	var b bytes.Buffer
	for i, f := range files {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s(type=%d)", f.ArchivePath, f.DataType)
	}
	return b.String()
}

// twoChunkFixture 造一份**两个 chunk** 的展开态归档。
//
// 为什么必须单独有它：compactFixture 只有**一个** chunk，于是
// `startOf(last) == 0`、body 前缀为空 —— WriteEntryAdded 里"拷贝前若干 chunk 的原始字节、
// 只重压最后一块、并把累计长度整体平移"那条路**根本走不到**。
// 真实内层 PVF 有 82,571 组，走的正是那条路。
func twoChunkFixture(t *testing.T) (*Archive, []byte) {
	t.Helper()
	pool := []byte{0}
	add := func(s string) int {
		off := len(pool) * 2
		pool = append(pool, []byte(s)...)
		pool = append(pool, 0)
		return off
	}
	dir := add("Dir")
	n0 := add("a.stk")
	n1 := add("b.stk")
	n2 := add("c.stk")

	chunk0 := bytes.Repeat([]byte{0x11}, 12)
	chunk1 := bytes.Repeat([]byte{0x22}, 8)
	pack := func(b []byte) []byte {
		var buf bytes.Buffer
		z := zlib.NewWriter(&buf)
		z.Write(b)
		z.Close()
		p := buf.Bytes()
		decryptProtected("mAIn", p)
		return p
	}
	p0, p1 := pack(chunk0), pack(chunk1)

	strA, err := encodePStringBuffer(pool, "stAs", 0xe7adf7ea)
	if err != nil {
		t.Fatal(err)
	}
	strW, err := encodePStringBuffer([]byte{0, 0}, "stWs", 0xb8dea7ac)
	if err != nil {
		t.Fatal(err)
	}
	names := append(make([]byte, 8), strA...)
	names = append(names, strW...)

	hashes := make([]byte, 8) // n=0, m=0：parse 接受，且本阶段要求原样
	decryptProtected("HSrm", hashes)

	groups := make([]byte, 2*groupItemSize)
	binary.LittleEndian.PutUint32(groups[0:], uint32(len(p0)))
	binary.LittleEndian.PutUint32(groups[4:], uint32(len(chunk0)))
	binary.LittleEndian.PutUint32(groups[8:], uint32(len(p0)+len(p1)))
	binary.LittleEndian.PutUint32(groups[12:], uint32(len(chunk1)))
	decryptProtected("Gidx", groups)

	rows := []struct{ name, chunk, off, size int }{{n0, 0, 0, 6}, {n1, 0, 6, 6}, {n2, 1, 0, 8}}
	table := make([]byte, len(rows)*fileItemSize)
	for i, r := range rows {
		p := table[i*fileItemSize:]
		for j, v := range []int{r.name, dir, r.chunk, r.off, r.size, 1} {
			binary.LittleEndian.PutUint32(p[j*4:], uint32(int32(v)))
		}
	}

	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header, magicSignature)
	for off, v := range map[int]int{24: len(rows), 32: len(p0) + len(p1), 36: 2, 40: len(hashes), 44: len(names)} {
		binary.LittleEndian.PutUint32(header[off:], uint32(v))
	}
	decryptProtected("iNfO", header)

	raw := append(header, table...)
	raw = append(raw, hashes...)
	raw = append(raw, names...)
	raw = append(raw, groups...)
	raw = append(raw, p0...)
	raw = append(raw, p1...)

	a, err := OpenBytes(raw)
	if err != nil {
		t.Fatalf("两 chunk 样本造出来打不开：%v", err)
	}
	return a, raw
}

// TestWriteEntryAddedMultiChunkKeepsEarlierChunksByteExact 钉住多 chunk 那条路：
// 只有最后一块被重压，**前面每一块的压缩字节必须逐字节不变**，
// 且落在前面那些块里的既有条目读出来内容一模一样。
func TestWriteEntryAddedMultiChunkKeepsEarlierChunksByteExact(t *testing.T) {
	a, raw := twoChunkFixture(t)
	before := a.Files()
	if len(before) != 3 {
		t.Fatalf("样本条目数不对：%d", len(before))
	}

	script := make([]byte, 10)
	for i := range script {
		script[i] = byte(0xA0 + i)
	}
	out := filepath.Join(t.TempDir(), "added2.pvf")
	if err := a.WriteEntryAdded(out, "Dir", "d.stk", script); err != nil {
		t.Fatalf("WriteEntryAdded: %v", err)
	}
	addedRaw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	added, err := OpenBytes(addedRaw)
	if err != nil {
		t.Fatalf("两 chunk 样本读回失败：%v", err)
	}

	// ① 前面那些块的压缩字节逐字节不变（body 前缀只在最后一块之前截止）
	oldFirstPackLen := a.groups[0].compressedSize
	if !bytes.Equal(raw[a.bodyOff:a.bodyOff+oldFirstPackLen], addedRaw[added.bodyOff:added.bodyOff+oldFirstPackLen]) {
		t.Fatal("非最后一块的压缩字节被改动了")
	}
	// ② 组表：第 0 组两项不变；第 1 组 originalSize 长大
	if added.groups[0] != a.groups[0] {
		t.Fatalf("第 0 组记录被改动：%+v → %+v", a.groups[0], added.groups[0])
	}
	if added.groups[1].originalSize != a.groups[1].originalSize+len(script) {
		t.Fatalf("第 1 组 originalSize 没按新增长度长大：%d → %d（增量 %d）",
			a.groups[1].originalSize, added.groups[1].originalSize, len(script))
	}
	if added.groups[1].compressedSize != added.header.bodySize {
		t.Fatal("最后一组的累计末尾必须等于 bodySize")
	}
	// ③ 既有条目逐条不变，新条目读得回来
	for i := range before {
		want, _ := a.FileRaw(i)
		now, err := added.FileRaw(i)
		if err != nil || !bytes.Equal(want, now) {
			t.Fatalf("既有条目 %d（%s）被改动：%v", i, before[i].ArchivePath, err)
		}
	}
	idx := added.FindFileIndex("Dir/d.stk")
	if idx < 0 {
		t.Fatalf("新条目查不到；现有 %v", pathsOf(added.Files()))
	}
	got, err := added.FileRaw(idx)
	if err != nil || !bytes.Equal(got, script) {
		t.Fatalf("新条目内容不一致：%v got=%d want=%d", err, len(got), len(script))
	}
	// ④ 哈希流仍逐字节原样
	if !bytes.Equal(hashSectionOf(t, raw), hashSectionOf(t, addedRaw)) {
		t.Fatal("哈希流被改动了")
	}
}
