package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// WriteEntryAdded 向一份**新的** current-format 内层归档**追加一条新脚本**。
//
// 与 WriteEntryCopy 的分工：
//   - WriteEntryCopy：替换**已存在**的脚本 —— 路径/串池/哈希流都不动，只把新字节追加进 owning chunk；
//   - WriteEntryAdded：**新增路径** —— 串池要变长、文件表要多一条记录、数据要追加进 chunk。
//
// 哈希流（`HSrm` 段）**逐字节原样拷贝**。依据：服务端的路径查找只建在文件表上
// （`parse.go:283-287` 用文件表逐条填 `pathIdx`），哈希流只被当作偏移/长度与尺寸自洽校验的对象
// （`parse.go:65-87`、`l10n.go:394`、`compact.go:155`），**从不参与解析**。
// ⇒ 服务端可见的新增条目不需要动它；客户端可见的新增条目另议
// （见 `docs/todo/PVF条目级mod扩展方案.md` §0.1）。
//
// 只写新文件（O_CREATE|O_EXCL），绝不就地改原件。
func (a *Archive) WriteEntryAdded(output, dir, name string, script []byte) error {
	if a == nil || a.readOnlyView {
		return fmt.Errorf("cannot rewrite a read-only PVF view")
	}
	if a.format != FormatDFO20260901 {
		return fmt.Errorf("entry addition requires the current inner format, got %q", a.format)
	}
	if len(script) == 0 || len(script)%5 != 0 || len(script) > 1024*1024 {
		return fmt.Errorf("unsupported script body: 需非空、5 字节对齐、≤1MB（got %d）", len(script))
	}
	name = strings.TrimSpace(name)
	dir = strings.TrimSpace(dir)
	if name == "" {
		return fmt.Errorf("entry name required")
	}
	archivePath := joinArchivePath(dir, name)
	if _, ok := a.FindFile(archivePath); ok {
		return fmt.Errorf("entry already exists: %s（已存在的路径请用 WriteEntryCopy）", archivePath)
	}
	// 本函数依赖**展开态**：文件表、串池、组表都要在手上（read-only/compact 视图不保留它们）。
	if a.compactDirectory || len(a.items) != a.header.fileCount || len(a.groups) == 0 || a.strA == nil || a.strW == nil {
		return fmt.Errorf("entry addition requires an expanded (memory) archive view")
	}
	if len(a.data) < headerSize {
		return fmt.Errorf("archive body unavailable")
	}

	// —— 1) 串池：按 l10n.go:300-307 的同一套约定追加 ——
	newA := append([]byte(nil), a.strA...)
	newW := append([]byte(nil), a.strW...)
	dirMagic, _ := appendPoolString(&newA, &newW, dir)
	nameMagic, _ := appendPoolString(&newA, &newW, name)

	// —— 2) chunk：追加到最后一块，重算该组与 body ——
	last := len(a.groups) - 1
	oldChunk, err := a.chunk(last)
	if err != nil {
		return fmt.Errorf("读最后一块失败：%w", err)
	}
	newChunk := make([]byte, 0, len(oldChunk)+len(script))
	newChunk = append(newChunk, oldChunk...)
	newChunk = append(newChunk, script...)
	packed, err := sealChunk(newChunk)
	if err != nil {
		return err
	}
	startOf := func(i int) int {
		if i == 0 {
			return 0
		}
		return a.groups[i-1].compressedSize
	}
	delta := len(packed) - (a.groups[last].compressedSize - startOf(last))
	newBodySize := a.header.bodySize + delta
	if newBodySize <= 0 || newBodySize > 2147483647 {
		return fmt.Errorf("body size overflow")
	}

	groupBytes := make([]byte, len(a.groups)*groupItemSize)
	for i, g := range a.groups {
		end, size := g.compressedSize, g.originalSize
		if i == last {
			end = g.compressedSize + delta
			size = len(newChunk)
		}
		binary.LittleEndian.PutUint32(groupBytes[i*groupItemSize:], uint32(end))
		binary.LittleEndian.PutUint32(groupBytes[i*groupItemSize+4:], uint32(size))
	}
	decryptProtected("Gidx", groupBytes)

	body := make([]byte, 0, newBodySize)
	body = append(body, a.data[a.bodyOff:a.bodyOff+startOf(last)]...)
	body = append(body, packed...)
	if len(body) != newBodySize {
		return fmt.Errorf("body 长度不自洽：%d ≠ %d", len(body), newBodySize)
	}

	// —— 3) 文件表：原样拷贝 + 追加一条 0x18 记录 ——
	tableSize := a.header.fileCount * fileItemSize
	if headerSize+tableSize > len(a.data) {
		return fmt.Errorf("file table exceeds archive")
	}
	fileTable := append([]byte(nil), a.data[headerSize:headerSize+tableSize]...)
	rec := make([]byte, fileItemSize)
	// 字段顺序取自 parse.go parseFiles：nameOffset / pathOffset / chunkIndex / dataOffset / dataSize / dataType
	for j, v := range []int{nameMagic, dirMagic, last, len(oldChunk), len(script), 1} {
		binary.LittleEndian.PutUint32(rec[j*4:], uint32(int32(v)))
	}
	fileTable = append(fileTable, rec...)

	// —— 4) 哈希流：逐字节原样（服务端解析不读它） ——
	hashOffset := headerSize + tableSize
	nameOffset := hashOffset + a.header.hashSize
	if nameOffset+8 > len(a.data) {
		return fmt.Errorf("name section prefix unavailable")
	}
	hashes := append([]byte(nil), a.data[hashOffset:nameOffset]...)

	// —— 5) 名字段：8 字节前缀 + 两个重建后的池 ——
	namePrefix := append([]byte(nil), a.data[nameOffset:nameOffset+8]...)
	encA, err := encodePStringBuffer(newA, "stAs", 0xe7adf7ea)
	if err != nil {
		return fmt.Errorf("编码 strA 失败：%w", err)
	}
	encW, err := encodePStringBuffer(newW, "stWs", 0xb8dea7ac)
	if err != nil {
		return fmt.Errorf("编码 strW 失败：%w", err)
	}
	nameSection := append(namePrefix, encA...)
	nameSection = append(nameSection, encW...)

	// —— 6) 头部：fileCount+1 / bodySize / nameSize（hashSize 不变）——
	hdr := append([]byte(nil), a.header.plain[:]...)
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(a.header.fileCount+1))
	binary.LittleEndian.PutUint32(hdr[32:36], uint32(newBodySize))
	binary.LittleEndian.PutUint32(hdr[44:48], uint32(len(nameSection)))
	decryptProtected("iNfO", hdr)

	// —— 7) 落盘：只写新文件；写失败由调用方删除半成品 ——
	dst, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer dst.Close()
	for _, part := range [][]byte{hdr, fileTable, hashes, nameSection, groupBytes, body} {
		if _, err := dst.Write(part); err != nil {
			return err
		}
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	return dst.Close()
}

// appendPoolString 按 l10n.go:300-307 的约定把 s 追加进窄池或宽池，返回 magic 偏移。
//
// 约定（decode.go:52 resolveString）：magic&1==1 ⇒ strW，字节偏移 (magic>>1)*2；
// magic&1==0 ⇒ strA，字节偏移 magic>>1。串一律 NUL 结尾（appendUTF16 自带）。
func appendPoolString(strA, strW *[]byte, s string) (magic int, wide bool) {
	if s == "" {
		return 0, false
	}
	if isASCII(s) {
		off := len(*strA)
		*strA = append(*strA, []byte(s)...)
		*strA = append(*strA, 0)
		return off << 1, false
	}
	off := len(*strW)
	*strW = appendUTF16(*strW, s)
	return (off>>1)<<1 | 1, true
}

// sealChunk 把展开后的 chunk 压成 body 里那一格：zlib 最优压缩 → mAIn 保护。
func sealChunk(plain []byte) ([]byte, error) {
	var compressed bytes.Buffer
	z, err := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := z.Write(plain); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	body := compressed.Bytes()
	decryptProtected("mAIn", body)
	return body, nil
}
