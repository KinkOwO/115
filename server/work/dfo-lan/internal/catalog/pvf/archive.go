package pvf

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	headerSize     = 0x30
	fileItemSize   = 0x18
	groupItemSize  = 8
	magicSignature = 0x69706B6E
)

type ArchiveFormat string

const (
	// FormatNKPI 表示旧版 DNF PVF `nkpi` 归档格式。
	FormatNKPI ArchiveFormat = "nkpi"
	// FormatProtectedNKPI 表示 23.4.15.0 之后使用 UTF-16 seed 密钥流的 `nkpi` 归档格式。
	FormatProtectedNKPI ArchiveFormat = "protected_nkpi"
	// FormatDFO20260901 is the inner archive of the supplied 2.38.2.34 client.
	// The RSA/AES outer layer must first be removed into a separate file.
	FormatDFO20260901 ArchiveFormat = "dfo_20260901_inner"
)

var (
	ErrInvalidArchive = errors.New("pvf archive is invalid")
	ErrFileNotFound   = errors.New("pvf file not found")
)

type ArchiveSnapshot struct {
	Format       ArchiveFormat `json:"format"`
	Path         string        `json:"path"`
	Size         int64         `json:"size"`
	Checksum     string        `json:"checksum"`
	LoadedAt     string        `json:"loaded_at"`
	FileCount    int           `json:"file_count"`
	GroupCount   int           `json:"group_count"`
	CachedChunks int           `json:"cached_chunks"`
	CachedTexts  int           `json:"cached_texts"`
}

type PreloadResult struct {
	Groups int `json:"groups"`
	Cached int `json:"cached"`
}

type File struct {
	Index       int    `json:"index"`
	Path        string `json:"path"`
	Name        string `json:"name"`
	ArchivePath string `json:"archive_path"`
	DataType    int    `json:"data_type"`
	Size        int    `json:"size"`
}

type Archive struct {
	snapshot Snapshot
	format   ArchiveFormat
	header   pvfHeader

	// Memory archives retain all bytes; runtime archives retain packed metadata.
	data   []byte
	files  []File
	items  []fileItem
	groups []groupItem

	// pathIdx 保存归一化路径到文件表下标的映射，查询时避免扫描目录。
	pathIdx          map[string]int
	bodyOff          int
	strA             []byte
	strW             []byte
	compactDirectory bool
	compactTable     []byte
	compactIndex     []directoryEntry
	backing          *archiveFile
	lease            *archiveLease
	cleanup          runtime.Cleanup
	closeOnce        sync.Once
	closed           atomic.Bool

	// chunks 缓存已解密解压的 body chunk，texts 缓存已解码的脚本文本。
	chunks           sync.Map
	texts            sync.Map
	readOnlyView     bool
	cacheMu          sync.Mutex
	maxChunkBytes    int64
	cachedChunkBytes int64
	maxTexts         int
	chunkOrder       list.List
	chunkPositions   map[int]*list.Element
	textOrder        list.List
	textPositions    map[int]*list.Element
}

func Open(path string) (*Archive, error) {
	return LoadArchive(Options{Path: path})
}

func OpenBytes(data []byte) (*Archive, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty pvf data", ErrInvalidArchive)
	}
	copied := append([]byte(nil), data...)
	sum := sha256.Sum256(copied)
	return OpenArchive(&Bundle{
		snapshot: Snapshot{
			Size:     int64(len(copied)),
			Checksum: hex.EncodeToString(sum[:]),
			LoadedAt: time.Now().UTC(),
		},
		data: copied,
	})
}

func LoadArchive(options Options) (*Archive, error) {
	bundle, err := Load(options)
	if err != nil {
		return nil, err
	}
	archive, err := OpenArchive(bundle)
	if err != nil {
		return nil, err
	}
	return archive, nil
}

func OpenArchive(bundle *Bundle) (*Archive, error) {
	if bundle == nil || len(bundle.data) == 0 {
		return nil, fmt.Errorf("%w: empty pvf data", ErrInvalidArchive)
	}
	archive := &Archive{
		snapshot: bundle.snapshot,
		data:     bundle.data,
		pathIdx:  make(map[string]int),
	}
	if err := archive.parse(); err != nil {
		return nil, err
	}
	return archive, nil
}

func (a *Archive) Snapshot() ArchiveSnapshot {
	if a == nil {
		return ArchiveSnapshot{}
	}
	cachedChunks := 0
	a.chunks.Range(func(_, _ any) bool {
		cachedChunks++
		return true
	})
	cachedTexts := 0
	a.texts.Range(func(_, _ any) bool {
		cachedTexts++
		return true
	})
	return ArchiveSnapshot{
		Format:       a.format,
		Path:         a.snapshot.Path,
		Size:         a.snapshot.Size,
		Checksum:     a.snapshot.Checksum,
		LoadedAt:     a.snapshot.LoadedAt.Format(rfc3339Nano),
		FileCount:    a.FileCount(),
		GroupCount:   len(a.groups),
		CachedChunks: cachedChunks,
		CachedTexts:  cachedTexts,
	}
}

// Format 返回当前 PVF 归档格式，供 smoke 报告和兼容排查使用。
func (a *Archive) Format() ArchiveFormat {
	if a == nil {
		return ""
	}
	return a.format
}

func (a *Archive) Files() []File {
	if a == nil || a.FileCount() == 0 {
		return nil
	}
	out := make([]File, a.FileCount())
	for i := range out {
		out[i] = a.fileAt(i)
	}
	return out
}

// IterateFiles visits immutable directory values without copying the complete
// multi-million-entry file slice. The callback cannot mutate archive entries.
func (a *Archive) IterateFiles(fn func(File) error) error {
	if a == nil || fn == nil {
		return fmt.Errorf("invalid archive file iterator")
	}
	for i := 0; i < a.FileCount(); i++ {
		if err := fn(a.fileAt(i)); err != nil {
			return err
		}
	}
	return nil
}

func (a *Archive) FileCount() int {
	if a == nil {
		return 0
	}
	if a.compactDirectory {
		return len(a.compactTable) / fileItemSize
	}
	return len(a.files)
}

// ReleaseReadCaches discards temporary decoded chunks and text. Archive bytes,
// paths and string pools remain intact, so subsequent reads are still valid.
// A concurrent reader may repopulate an entry; this is not an archive close.
func (a *Archive) ReleaseReadCaches() {
	if a == nil {
		return
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	a.chunks.Clear()
	a.texts.Clear()
	a.chunkOrder.Init()
	a.textOrder.Init()
	a.chunkPositions = nil
	a.textPositions = nil
	a.cachedChunkBytes = 0
}

func (a *Archive) CanReadFileData() bool {
	if a == nil {
		return false
	}
	return a.format == FormatNKPI || a.format == FormatProtectedNKPI || a.format == FormatDFO20260901
}

func (a *Archive) FindFile(relativePath string) (File, bool) {
	if a == nil {
		return File{}, false
	}
	idx, ok := a.lookupPath(relativePath)
	if !ok {
		return File{}, false
	}
	return a.fileAt(idx), true
}

func (a *Archive) FindFileIndex(relativePath string) int {
	if a == nil {
		return -1
	}
	idx, ok := a.lookupPath(relativePath)
	if !ok {
		return -1
	}
	return idx
}

func (a *Archive) ReadText(relativePath string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	idx, ok := a.lookupPath(relativePath)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrFileNotFound, relativePath)
	}
	return a.readTextIndex(idx)
}

func (a *Archive) ReadRaw(relativePath string) ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	idx, ok := a.lookupPath(relativePath)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, relativePath)
	}
	return a.readRawIndex(idx)
}

// ResolveString exposes one script token's string-pool value. PVF type-1
// scripts store labels and quoted values as encoded offsets into the archive's
// shared ANSI/UTF-16 pools; callers must keep the encoded offset unchanged
// unless they rebuild every referring script token.
func (a *Archive) ResolveString(magicOffset int) string {
	if a == nil {
		return ""
	}
	return a.resolveString(magicOffset)
}

func (a *Archive) FileText(idx int) (string, error) {
	if a == nil {
		return "", fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	return a.readTextIndex(idx)
}

func (a *Archive) FileRaw(idx int) ([]byte, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	return a.readRawIndex(idx)
}

func (a *Archive) PreloadAll() (PreloadResult, error) {
	if a == nil {
		return PreloadResult{}, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	result := PreloadResult{Groups: len(a.groups)}
	for idx := range a.groups {
		if _, err := a.chunk(idx); err != nil {
			return result, err
		}
		result.Cached++
	}
	return result, nil
}

func (a *Archive) Bytes() []byte {
	if a == nil {
		return nil
	}
	if a.backing != nil {
		if a.closed.Load() {
			return nil
		}
		out := make([]byte, int(a.sourceSize()))
		if _, err := a.backing.file.ReadAt(out, 0); err != nil {
			runtime.KeepAlive(a)
			return nil
		}
		runtime.KeepAlive(a)
		return out
	}
	out := make([]byte, len(a.data))
	copy(out, a.data)
	return out
}

func (a *Archive) Reader() *bytes.Reader {
	if a == nil {
		return bytes.NewReader(nil)
	}
	if a.backing != nil {
		return bytes.NewReader(a.Bytes())
	}
	return bytes.NewReader(a.data)
}
