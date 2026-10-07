package pvf

// 第 2 步验证（需要真档，用环境变量开启，默认跳过）：
// 在**真实内层 PVF** 上新增一条无害条目，然后做两级判据：
//
//	A. **字节级不变量**（比"逐条比对"更强）：
//	   - 文件表前缀（原有 N 条记录）逐字节相同；
//	   - 哈希流整段逐字节相同；
//	   - 组表除**最后一条**外逐字节相同，且最后一条只有两个字段随增量变化；
//	   - body 在"最后一块之前"的整段逐字节相同。
//	B. 用**只读视图**重开产物：条目数 +1、新条目查得到且内容一致、抽样既有条目内容一致。
//
// 用法：
//
//	$env:PVF_ADDENTRY_SOURCE="<真档路径>"; $env:PVF_ADDENTRY_OUT="<产物路径>"
//	go test ./internal/catalog/pvf/ -run RealAddEntry -v -timeout 60m

import (
	"bytes"
	"os"
	"testing"
)

func TestRealAddEntryPreservesEveryOtherByte(t *testing.T) {
	src := os.Getenv("PVF_ADDENTRY_SOURCE")
	out := os.Getenv("PVF_ADDENTRY_OUT")
	if src == "" || out == "" {
		t.Skip("未设 PVF_ADDENTRY_SOURCE / PVF_ADDENTRY_OUT")
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读真档失败：%v", err)
	}
	t.Logf("真档 %s：%.1f MB", src, float64(len(raw))/(1<<20))

	a, err := OpenBytes(raw)
	if err != nil {
		t.Fatalf("展开真档失败：%v", err)
	}
	origCount := len(a.Files())
	origRaw := raw
	raw = nil // 别让两份 726 MB 同时活着
	t.Logf("条目数 = %d，组数 = %d，bodySize = %d", origCount, len(a.groups), a.header.bodySize)

	script := []byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0} // 10 字节 = 2 个 5 字节单元
	const dir, name = "pvfmod/selfcheck", "probe.txt"
	if err := a.WriteEntryAdded(out, dir, name, script); err != nil {
		t.Fatalf("WriteEntryAdded: %v", err)
	}
	newRaw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("产物 %.1f MB（原 %.1f MB）", float64(len(newRaw))/(1<<20), float64(len(origRaw))/(1<<20))

	// ---- A. 字节级不变量 ----
	// 头部
	oh := append([]byte(nil), origRaw[:headerSize]...)
	decryptProtected("iNfO", oh)
	nh := append([]byte(nil), newRaw[:headerSize]...)
	decryptProtected("iNfO", nh)
	oldCount, oldBody, oldGroups, oldHashSize, oldNameSize :=
		readInt32(oh[24:28]), readInt32(oh[32:36]), readInt32(oh[36:40]), readInt32(oh[40:44]), readInt32(oh[44:48])
	newCount, newBody, newGroups, newHashSize, newNameSize :=
		readInt32(nh[24:28]), readInt32(nh[32:36]), readInt32(nh[36:40]), readInt32(nh[40:44]), readInt32(nh[44:48])
	if newCount != oldCount+1 {
		t.Fatalf("fileCount 应为 %d，实为 %d", oldCount+1, newCount)
	}
	if newGroups != oldGroups || newHashSize != oldHashSize {
		t.Fatalf("组数/哈希流长度不该变：groups %d→%d，hashSize %d→%d", oldGroups, newGroups, oldHashSize, newHashSize)
	}

	// 文件表前缀：原有 N 条记录必须逐字节相同
	oldTable := origRaw[headerSize : headerSize+oldCount*fileItemSize]
	newTable := newRaw[headerSize : headerSize+newCount*fileItemSize]
	if !bytes.Equal(oldTable, newTable[:len(oldTable)]) {
		t.Fatal("文件表前缀被改动（原有条目记录不是逐字节相同）")
	}

	// 各级偏移先算出来并**显式自检**（不要靠推理，越界就让错误信息说话）
	oldHashOff := headerSize + oldCount*fileItemSize
	newHashOff := headerSize + newCount*fileItemSize
	oldGroupOff := oldHashOff + oldHashSize + oldNameSize
	newGroupOff := newHashOff + newHashSize + newNameSize
	oldBodyOff := oldGroupOff + oldGroups*groupItemSize
	newBodyOff := newGroupOff + newGroups*groupItemSize
	t.Logf("原档头 fileCount=%d bodySize=%d groupCount=%d hashSize=%d nameSize=%d", oldCount, oldBody, oldGroups, oldHashSize, oldNameSize)
	t.Logf("新档头 fileCount=%d bodySize=%d groupCount=%d hashSize=%d nameSize=%d", newCount, newBody, newGroups, newHashSize, newNameSize)
	t.Logf("原档偏移 hash=%d group=%d body=%d（len=%d）", oldHashOff, oldGroupOff, oldBodyOff, len(origRaw))
	t.Logf("新档偏移 hash=%d group=%d body=%d（len=%d）", newHashOff, newGroupOff, newBodyOff, len(newRaw))
	if oldGroupOff < 0 || oldGroups <= 0 || oldGroupOff+oldGroups*groupItemSize > len(origRaw) {
		t.Fatalf("原档组表越界：off=%d count=%d", oldGroupOff, oldGroups)
	}
	if newGroupOff < 0 || newGroups <= 0 || newGroupOff+newGroups*groupItemSize > len(newRaw) {
		t.Fatalf("新档组表越界：off=%d count=%d", newGroupOff, newGroups)
	}
	// 新记录必须是最后一条，且 chunkIndex 指向最后一块
	rec := newTable[len(oldTable):]
	if got := readInt32(rec[8:12]); got != a.header.groupCount-1 {
		t.Fatalf("新记录 chunkIndex = %d，期望最后一块 %d", got, a.header.groupCount-1)
	}
	if got := readInt32(rec[16:20]); got != len(script) {
		t.Fatalf("新记录 dataSize = %d，期望 %d", got, len(script))
	}

	// 哈希流整段逐字节相同
	if !bytes.Equal(origRaw[oldHashOff:oldHashOff+oldHashSize], newRaw[newHashOff:newHashOff+oldHashSize]) {
		t.Fatal("哈希流被改动")
	}

	// 组表：除最后一条外逐字节相同。
	// ⚠ 比对用**密文原始字节**（等价即可），但**读字段必须先解 `Gidx` 保护**
	// —— 直接按明文读会读出负数（本项目实测踩到）。
	oldGroupsB := origRaw[oldGroupOff : oldGroupOff+oldGroups*groupItemSize]
	newGroupsB := newRaw[newGroupOff : newGroupOff+newGroups*groupItemSize]
	if !bytes.Equal(oldGroupsB[:len(oldGroupsB)-groupItemSize], newGroupsB[:len(newGroupsB)-groupItemSize]) {
		t.Fatal("组表里除最后一条外的记录被改动")
	}
	oldGroupsPlain := append([]byte(nil), oldGroupsB...)
	decryptProtected("Gidx", oldGroupsPlain)
	newGroupsPlain := append([]byte(nil), newGroupsB...)
	decryptProtected("Gidx", newGroupsPlain)
	lastIdx := (oldGroups - 1) * groupItemSize
	oldLastEnd := readInt32(oldGroupsPlain[lastIdx:])
	oldLastOrig := readInt32(oldGroupsPlain[lastIdx+4:])
	newLastEnd := readInt32(newGroupsPlain[lastIdx:])
	newLastOrig := readInt32(newGroupsPlain[lastIdx+4:])
	if newLastOrig != oldLastOrig+len(script) {
		t.Fatalf("最后一块 originalSize 应为 %d（+%d），实为 %d", oldLastOrig+len(script), len(script), newLastOrig)
	}
	if newLastEnd != newBody {
		t.Fatalf("最后一块累计末尾 %d 应等于 bodySize %d", newLastEnd, newBody)
	}
	t.Logf("最后一块 originalSize %d → %d，累计末尾 %d → %d", oldLastOrig, newLastOrig, oldLastEnd, newLastEnd)

	// body：最后一块之前整段逐字节相同（前缀长度 = 倒数第二组的累计末尾）
	prefix := 0
	if oldGroups > 1 {
		prefix = readInt32(oldGroupsPlain[(oldGroups-2)*groupItemSize:])
	}
	if !bytes.Equal(origRaw[oldBodyOff:oldBodyOff+prefix], newRaw[newBodyOff:newBodyOff+prefix]) {
		t.Fatal("body 在最后一块之前的整段被改动")
	}
	t.Logf("body 最后一块之前的前缀 %.1f MB 逐字节相同 ✅", float64(prefix)/(1<<20))
	if newBody != len(newRaw)-newBodyOff {
		t.Fatalf("bodySize 与产物长度不自洽：%d vs %d", newBody, len(newRaw)-newBodyOff)
	}
	// body 不必变大：最后一块被重压（BestCompression），若比原档压得更狠 delta 就是负的。
	t.Logf("bodySize %d → %d（delta %+d）", oldBody, newBody, newBody-oldBody)

	// 串池：**不可能逐字节相同**（整段被重新压缩，实测 nameSize 43,706,621 → 43,559,478），
	// 正确的不变量是**语义前缀**：新池的解码内容必须以旧池为前缀
	//（既有的 magic 偏移因此仍然指向同一批串）。
	oldA, oldW := decodePools(t, origRaw)
	newA, newW := decodePools(t, newRaw)
	if !bytes.HasPrefix(newA, oldA) {
		t.Fatal("strA 不是以旧池为前缀（既有条目的路径串被改动了）")
	}
	if !bytes.HasPrefix(newW, oldW) {
		t.Fatal("strW 不是以旧池为前缀")
	}
	t.Logf("串池前缀不变量成立：strA %d → %d 字节，strW %d → %d 字节 ✅",
		len(oldA), len(newA), len(oldW), len(newW))

	// ---- B. 只读视图重开 + 抽样既有条目 ----
	added, err := OpenReadOnly(Options{Path: out, MaxBytes: 4 << 30}, "")
	if err != nil {
		t.Fatalf("只读重开失败：%v", err)
	}
	defer added.Close()
	if got := len(added.Files()); got != origCount+1 {
		t.Fatalf("重开后条目数 = %d，期望 %d", got, origCount+1)
	}
	idx := added.FindFileIndex(dir + "/" + name)
	if idx < 0 {
		t.Fatal("重开后查不到新条目")
	}
	got, err := added.FileRaw(idx)
	if err != nil || !bytes.Equal(got, script) {
		t.Fatalf("新条目内容不一致：%v got=%d want=%d", err, len(got), len(script))
	}
	t.Logf("新条目 idx=%d 读回 %d 字节 ✅", idx, len(got))

	step := origCount / 500
	if step < 1 {
		step = 1
	}
	checked := 0
	for i := 0; i < origCount; i += step {
		want, err1 := a.FileRaw(i)
		now, err2 := added.FileRaw(i)
		if err1 != nil || err2 != nil || !bytes.Equal(want, now) {
			t.Fatalf("抽样第 %d 条不一致：%v / %v", i, err1, err2)
		}
		checked++
	}
	t.Logf("抽样既有条目 %d 条，内容全部一致 ✅", checked)
}

// decodePools 按当前文件头定位名字段，解出两个串池（strA / strW）。
func decodePools(t *testing.T, raw []byte) ([]byte, []byte) {
	t.Helper()
	hdr := append([]byte(nil), raw[:headerSize]...)
	decryptProtected("iNfO", hdr)
	fileCount := readInt32(hdr[24:28])
	hashSize := readInt32(hdr[40:44])
	nameOff := headerSize + fileCount*fileItemSize + hashSize
	names := raw[nameOff:]
	pos := 8
	a, err := decryptPStringBuffer(names, &pos, "stAs", 0xe7adf7ea)
	if err != nil {
		t.Fatalf("解 strA 失败：%v", err)
	}
	w, err := decryptPStringBuffer(names, &pos, "stWs", 0xb8dea7ac)
	if err != nil {
		t.Fatalf("解 strW 失败：%v", err)
	}
	return a, w
}
