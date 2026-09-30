package pvf

// 本文件为汉化/本地化补丁提供底层能力：枚举 PVF 内部所有字符串引用点，
// 在保持原字符串池全部旧偏移有效的前提下向池尾追加新文本，重写引用点，
// 并重新组装内层归档（可再经外层 RSA/AES 加壳回写成客户端 Script.pvf）。
//
// 设计取舍：字符串池只追加、不覆盖、不重排。这样任何我们没能枚举到的
// （例如指向某个字符串中段的）旧偏移仍然指向原来的英文文本，不会读到垃圾。

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf16"
)

// StringRefKind 标识一个字符串引用点所在的位置。
type StringRefKind string

const (
	// RefFileName 表示文件表的 name 字段（同时也是池内偏移）。
	RefFileName StringRefKind = "file_name"
	// RefFilePath 表示文件表的 path 字段。
	RefFilePath StringRefKind = "file_path"
	// RefToken 表示脚本正文里的一个 token 值。
	RefToken StringRefKind = "token"
)

// StringRef 是一个池内字符串的引用点。
type StringRef struct {
	Kind     StringRefKind
	FileIdx  int
	TokenIdx int // 脚本内第几个 5 字节单元；非脚本引用为 -1
	TokenTyp byte
	Chunk    int // 脚本引用：所属 chunk
	ChunkOff int // 脚本引用：chunk 内字节偏移
	AbsOff   int // 文件表引用：归档内绝对字节偏移
	Value    int // magicOffset 原值
	IsWide   bool
	Text     string
}

// IterateStringRefs 枚举归档中全部字符串引用点。
func (a *Archive) IterateStringRefs(fn func(StringRef) error) error {
	if a == nil {
		return fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	for idx := range a.items {
		item := a.items[idx]
		base := headerSize + idx*fileItemSize
		for _, f := range []struct {
			kind StringRefKind
			rel  int
			val  int
		}{
			{RefFileName, 0, item.nameOffset},
			{RefFilePath, 4, item.pathOffset},
		} {
			text := a.resolveString(f.val)
			if text == "" {
				continue
			}
			if err := fn(StringRef{
				Kind: f.kind, FileIdx: idx, TokenIdx: -1,
				AbsOff: base + f.rel, Value: f.val,
				IsWide: f.val&1 != 0, Text: text,
			}); err != nil {
				return err
			}
		}
	}
	for idx := range a.files {
		item := a.items[idx]
		if item.dataType != 1 {
			continue
		}
		raw, err := a.readRawIndex(idx)
		if err != nil {
			return err
		}
		if len(raw)%5 != 0 {
			continue
		}
		chunk, err := a.chunk(item.chunkIndex)
		if err != nil {
			return err
		}
		_ = chunk
		for off := 0; off < len(raw); off += 5 {
			typ := raw[off]
			if typ != 3 && typ != 6 && typ != 8 {
				continue
			}
			value := readInt32(raw[off+1 : off+5])
			text := a.resolveString(value)
			if text == "" {
				continue
			}
			if err := fn(StringRef{
				Kind: RefToken, FileIdx: idx, TokenIdx: off / 5, TokenTyp: typ,
				Chunk: item.chunkIndex, ChunkOff: item.dataOffset + off + 1,
				Value: value, IsWide: value&1 != 0, Text: text,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// TokenTypeHistogram 统计全部脚本 token 的类型分布。
func (a *Archive) TokenTypeHistogram() (map[int]int, error) {
	hist := map[int]int{}
	for idx := range a.files {
		if a.items[idx].dataType != 1 {
			continue
		}
		raw, err := a.readRawIndex(idx)
		if err != nil {
			return nil, err
		}
		for off := 0; off+5 <= len(raw); off += 5 {
			hist[int(raw[off])]++
		}
	}
	return hist, nil
}

// ScriptString samples per token type.
type TokenSample struct {
	Type  int      `json:"type"`
	Path  string   `json:"path"`
	Index int      `json:"index"`
	Texts []string `json:"texts"`
}

// SampleTokenStrings 每种 token 类型返回最多 limit 个样例字符串，便于人工判断。
func (a *Archive) SampleTokenStrings(limit int) ([]TokenSample, error) {
	per := map[int][]string{}
	paths := map[int]string{}
	for idx := range a.files {
		if a.items[idx].dataType != 1 {
			continue
		}
		raw, err := a.readRawIndex(idx)
		if err != nil {
			return nil, err
		}
		for off := 0; off+5 <= len(raw); off += 5 {
			typ := int(raw[off])
			if len(per[typ]) >= limit {
				continue
			}
			value := readInt32(raw[off+1 : off+5])
			switch typ {
			case 3, 6, 8:
				if s := a.resolveString(value); s != "" {
					per[typ] = append(per[typ], s)
					paths[typ] = a.files[idx].ArchivePath
				}
			default:
				per[typ] = append(per[typ], fmt.Sprintf("%d", value))
				paths[typ] = a.files[idx].ArchivePath
			}
		}
	}
	types := make([]int, 0, len(per))
	for t := range per {
		types = append(types, t)
	}
	sort.Ints(types)
	out := make([]TokenSample, 0, len(types))
	for _, t := range types {
		out = append(out, TokenSample{Type: t, Path: paths[t], Texts: per[t]})
	}
	return out, nil
}

// DistinctString 是所有被引用的池内字符串及其引用次数。
type DistinctString struct {
	Text   string `json:"text"`
	Refs   int    `json:"refs"`
	Tok3   int    `json:"tok3"`
	Tok6   int    `json:"tok6"`
	Tok8   int    `json:"tok8"`
	Wide   bool   `json:"wide"`
	Sample string `json:"sample"`
}

// DistinctStrings 汇总全部被引用字符串（按引用次数降序）。
func (a *Archive) DistinctStrings() ([]DistinctString, error) {
	byText := map[string]*DistinctString{}
	err := a.IterateStringRefs(func(ref StringRef) error {
		d, ok := byText[ref.Text]
		if !ok {
			d = &DistinctString{Text: ref.Text, Wide: ref.IsWide}
			byText[ref.Text] = d
		}
		d.Refs++
		switch ref.TokenTyp {
		case 3:
			d.Tok3++
		case 6:
			d.Tok6++
		case 8:
			d.Tok8++
		}
		if d.Sample == "" && a.files[ref.FileIdx].ArchivePath != "" {
			d.Sample = a.files[ref.FileIdx].ArchivePath
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]DistinctString, 0, len(byText))
	for _, d := range byText {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Refs != out[j].Refs {
			return out[i].Refs > out[j].Refs
		}
		return out[i].Text < out[j].Text
	})
	return out, nil
}

// PoolBytes 返回解密后的 ANSI / UTF-16 字符串池副本。
func (a *Archive) PoolBytes() (strA, strW []byte) {
	return append([]byte(nil), a.strA...), append([]byte(nil), a.strW...)
}

// LocalizeStats 描述一次本地化重写的结果。
type LocalizeStats struct {
	Refs       int `json:"refs"`
	Translated int `json:"translated"`
	Missed     int `json:"missed"`
	NewStrA    int `json:"new_str_a"`
	NewStrW    int `json:"new_str_w"`
	OutSize    int `json:"out_size"`
}

// Translator 把原文映射为译文；返回 false 表示保持原文。
type Translator func(text string) (string, bool)

// Localize 生成一个经过字符串替换的新内层归档。
func (a *Archive) Localize(out string, tr Translator) (LocalizeStats, error) {
	var stats LocalizeStats
	if a == nil || a.readOnlyView || a.format != FormatDFO20260901 {
		return stats, fmt.Errorf("localize requires a dfo inner archive")
	}
	type pending struct {
		refs []StringRef
		text string
	}
	order := make([]*pending, 0, 1024)
	index := map[string]*pending{}
	err := a.IterateStringRefs(func(ref StringRef) error {
		stats.Refs++
		p, ok := index[ref.Text]
		if !ok {
			p = &pending{text: ref.Text}
			index[ref.Text] = p
			order = append(order, p)
		}
		p.refs = append(p.refs, ref)
		return nil
	})
	if err != nil {
		return stats, err
	}

	newA := append([]byte(nil), a.strA...)
	newW := append([]byte(nil), a.strW...)
	remap := map[string]int{}
	for _, p := range order {
		translated, ok := tr(p.text)
		if !ok || translated == "" || translated == p.text {
			stats.Missed++
			continue
		}
		wide := p.refs[0].IsWide
		if !wide && !isASCII(translated) {
			wide = true
		}
		var magic int
		if wide {
			off := len(newW)
			newW = appendUTF16(newW, translated)
			stats.NewStrW++
			magic = (off>>1)<<1 | 1
		} else {
			off := len(newA)
			newA = append(newA, []byte(translated)...)
			newA = append(newA, 0)
			stats.NewStrA++
			magic = off << 1
		}
		remap[p.text] = magic
		stats.Translated++
	}

	// 收集需要打补丁的 chunk（脚本 token）与文件表字段。
	tokenPatches := map[int][]struct {
		off   int
		value int
	}{}
	fieldPatches := map[int]int{}
	for _, p := range order {
		magic, ok := remap[p.text]
		if !ok {
			continue
		}
		for _, ref := range p.refs {
			switch ref.Kind {
			case RefToken:
				tokenPatches[ref.Chunk] = append(tokenPatches[ref.Chunk], struct {
					off   int
					value int
				}{ref.ChunkOff, magic})
			case RefFileName, RefFilePath:
				fieldPatches[ref.AbsOff] = magic
			}
		}
	}

	// 重新压缩全部 chunk（未打补丁的 chunk 原样保留原压缩字节）。
	body := make([]byte, 0, len(a.data))
	groupBytes := make([]byte, len(a.groups)*groupItemSize)
	cumulative := 0
	for idx := range a.groups {
		var compressed []byte
		if patches, ok := tokenPatches[idx]; ok {
			plain, err := a.chunk(idx)
			if err != nil {
				return stats, err
			}
			modified := append([]byte(nil), plain...)
			for _, pt := range patches {
				if pt.off < 0 || pt.off+4 > len(modified) {
					return stats, fmt.Errorf("token patch out of range in chunk %d", idx)
				}
				binary.LittleEndian.PutUint32(modified[pt.off:pt.off+4], uint32(int32(pt.value)))
			}
			compressed, err = compressZlib(modified)
			if err != nil {
				return stats, err
			}
			binary.LittleEndian.PutUint32(groupBytes[idx*groupItemSize+4:], uint32(len(modified)))
		} else {
			start := 0
			if idx > 0 {
				start = a.groups[idx-1].compressedSize
			}
			raw := append([]byte(nil), a.data[a.bodyOff+start:a.bodyOff+a.groups[idx].compressedSize]...)
			compressed = raw
			binary.LittleEndian.PutUint32(groupBytes[idx*groupItemSize+4:], uint32(a.groups[idx].originalSize))
		}
		// 非补丁 chunk 的字节已是加密态，保持原样；补丁 chunk 需要重新加密。
		if _, ok := tokenPatches[idx]; ok {
			decryptProtected("mAIn", compressed)
		}
		body = append(body, compressed...)
		cumulative += len(compressed)
		binary.LittleEndian.PutUint32(groupBytes[idx*groupItemSize:], uint32(cumulative))
	}
	decryptProtected("Gidx", groupBytes)

	// 文件表：原样拷贝，仅改写 name/path 偏移。
	tableSize := len(a.items) * fileItemSize
	fileTable := append([]byte(nil), a.data[headerSize:headerSize+tableSize]...)
	for abs, magic := range fieldPatches {
		rel := abs - headerSize
		if rel < 0 || rel+4 > len(fileTable) {
			return stats, fmt.Errorf("field patch out of range")
		}
		binary.LittleEndian.PutUint32(fileTable[rel:rel+4], uint32(int32(magic)))
	}

	// 名字段：保留 8 字节前缀，重建两个字符串池。
	hashOffset := headerSize + tableSize
	nameOffset := hashOffset + a.header.hashSize
	namePrefix := append([]byte(nil), a.data[nameOffset:nameOffset+8]...)
	if len(namePrefix) < 8 {
		return stats, fmt.Errorf("name section prefix is short")
	}
	encA, err := encodePStringBuffer(newA, "stAs", 0xe7adf7ea)
	if err != nil {
		return stats, err
	}
	encW, err := encodePStringBuffer(newW, "stWs", 0xb8dea7ac)
	if err != nil {
		return stats, err
	}
	nameSection := append([]byte(nil), namePrefix...)
	nameSection = append(nameSection, encA...)
	nameSection = append(nameSection, encW...)

	// 头部：更新 bodySize / nameSize 后重新加密。
	hdr := append([]byte(nil), a.header.plain[:]...)
	binary.LittleEndian.PutUint32(hdr[32:36], uint32(len(body)))
	binary.LittleEndian.PutUint32(hdr[44:48], uint32(len(nameSection)))
	decryptProtected("iNfO", hdr)

	hashes := append([]byte(nil), a.data[hashOffset:nameOffset]...)

	f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return stats, err
	}
	defer f.Close()
	for _, part := range [][]byte{hdr, fileTable, hashes, nameSection, groupBytes, body} {
		if _, err := f.Write(part); err != nil {
			return stats, err
		}
	}
	if err := f.Sync(); err != nil {
		return stats, err
	}
	if err := f.Close(); err != nil {
		return stats, err
	}
	info, err := os.Stat(out)
	if err == nil {
		stats.OutSize = int(info.Size())
	}
	return stats, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func appendUTF16(dst []byte, s string) []byte {
	for _, u := range utf16.Encode([]rune(s)) {
		dst = append(dst, byte(u), byte(u>>8))
	}
	return append(dst, 0, 0)
}

// encodePStringBuffer 与 decryptPStringBuffer 对称：压缩后异或加密。
func encodePStringBuffer(plain []byte, key string, xorConst uint32) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(plain); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	encrypted := buf.Bytes()
	decryptPString(key, encrypted)
	out := make([]byte, 0, 8+len(encrypted))
	var head [8]byte
	binary.LittleEndian.PutUint32(head[0:4], uint32(len(encrypted))^xorConst)
	binary.LittleEndian.PutUint32(head[4:8], uint32(len(plain))^uint32(len(encrypted)))
	out = append(out, head[:]...)
	out = append(out, encrypted...)
	return out, nil
}

// DumpPoolStrings 把一个池按 NUL 切分成 (偏移, 文本) 列表。
func DumpPoolStrings(pool []byte, wide bool) []struct {
	Off  int
	Text string
} {
	out := []struct {
		Off  int
		Text string
	}{}
	if wide {
		for off := 0; off+1 < len(pool); {
			end := off
			for end+1 < len(pool) && !(pool[end] == 0 && pool[end+1] == 0) {
				end += 2
			}
			if end > off {
				out = append(out, struct {
					Off  int
					Text string
				}{off, decodeUTF16LE(pool[off:end])})
			}
			off = end + 2
		}
		return out
	}
	for off := 0; off < len(pool); {
		end := bytes.IndexByte(pool[off:], 0)
		if end < 0 {
			break
		}
		if end > 0 {
			out = append(out, struct {
				Off  int
				Text string
			}{off, readUTF8String(pool, off)})
		}
		off += end + 1
	}
	return out
}

// GuessLanguage 粗略判断文本里是否含有 CJK 字符。
func GuessLanguage(text string) string {
	han, latin, other := 0, 0, 0
	for _, r := range text {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF, r >= 0x3400 && r <= 0x4DBF:
			han++
		case r < 0x80 && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')):
			latin++
		default:
			other++
		}
	}
	switch {
	case han > 0 && han >= latin:
		return "cjk"
	case latin > 0:
		return "latin"
	default:
		return "other"
	}
}

// PoolStringCount 统计池中非空字符串条数。
func PoolStringCount(pool []byte, wide bool) int {
	return len(DumpPoolStrings(pool, wide))
}

var _ = strings.TrimSpace
