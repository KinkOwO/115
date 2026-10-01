package pvf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// directoryEntry contains no Go pointers. Hash matches are always checked
// against the original path, including duplicates and hash collisions.
type directoryEntry struct {
	hash  uint64
	index uint32
}

type archiveFile struct {
	mu   sync.Mutex
	file *os.File
	refs int
}

type archiveLease struct {
	backing *archiveFile
	once    sync.Once
	err     error
}

func (l *archiveLease) close() error {
	l.once.Do(func() { l.err = l.backing.release() })
	return l.err
}

func (a *Archive) attachCleanup() {
	if a.backing == nil {
		return
	}
	a.lease = &archiveLease{backing: a.backing}
	// The cleanup argument owns no pointer back to Archive, so cache-list
	// cycles cannot prevent descriptor cleanup when a caller omits Close.
	a.cleanup = runtime.AddCleanup(a, func(lease *archiveLease) { lease.close() }, a.lease)
}

func (f *archiveFile) retain() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == 0 {
		return false
	}
	f.refs++
	return true
}
func (f *archiveFile) release() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refs--
	if f.refs == 0 {
		return f.file.Close()
	}
	return nil
}

// OpenReadOnly verifies the complete file before parsing metadata. Only the
// directory and string pools stay in memory; compressed bodies use ReadAt on
// the same open file. Views own independent leases on this immutable source.
func OpenReadOnly(options Options, expectedChecksum string) (*Archive, error) {
	if strings.TrimSpace(options.Path) == "" {
		return nil, ErrPathRequired
	}
	limit := options.MaxBytes
	if limit == 0 {
		limit = DefaultMaxBytes
	}
	if limit < 0 {
		return nil, fmt.Errorf("pvf max bytes must be positive")
	}
	f, err := openImmutableFile(options.Path)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pvf is not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%w: got %d want <= %d", ErrTooLarge, info.Size(), limit)
	}
	hasher := sha256.New()
	n, err := io.CopyBuffer(hasher, f, make([]byte, 1024*1024))
	if err != nil {
		return nil, err
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if n != info.Size() {
		return nil, fmt.Errorf("PVF size changed during verification")
	}
	if checksum != strings.ToLower(expectedChecksum) {
		return nil, fmt.Errorf("inner PVF source mismatch: got %s expected %s", checksum, expectedChecksum)
	}
	header := make([]byte, headerSize)
	if _, err = f.ReadAt(header, 0); err != nil {
		return nil, err
	}
	plain := append([]byte(nil), header...)
	decryptProtected("iNfO", plain)
	if readInt32(plain[:4]) != magicSignature {
		plain = append(plain[:0], header...)
		decryptGuard(plain)
		decrypt("HeaD", plain)
		if readInt32(plain[:4]) != magicSignature {
			plain = append(plain[:0], header...)
			decryptProtected("hEAd", plain)
		}
	}
	if readInt32(plain[:4]) != magicSignature {
		return nil, fmt.Errorf("%w: unsupported archive format", ErrInvalidArchive)
	}
	h := parseHeaderSizes(plain)
	if err = validateHeader(h); err != nil {
		return nil, err
	}
	tableSize, err := checkedMul(h.fileCount, fileItemSize)
	if err != nil {
		return nil, err
	}
	groupSize, err := checkedMul(h.groupCount, groupItemSize)
	if err != nil {
		return nil, err
	}
	metadataSize := int64(headerSize) + int64(tableSize) + int64(h.hashSize) + int64(h.nameSize) + int64(groupSize)
	if metadataSize > info.Size() || metadataSize > int64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("%w: metadata boundaries", ErrInvalidArchive)
	}
	a := &Archive{snapshot: Snapshot{Path: options.Path, Size: info.Size(), Checksum: checksum, LoadedAt: time.Now().UTC()},
		data: make([]byte, int(metadataSize)), compactDirectory: true, readOnlyView: true,
		backing: &archiveFile{file: f, refs: 1}, maxChunkBytes: 64 * 1024 * 1024, maxTexts: 2048}
	if _, err = f.ReadAt(a.data, 0); err != nil {
		return nil, err
	}
	if err = a.parse(); err != nil {
		return nil, err
	}
	// The original metadata buffer also contains compressed pools/hash tables.
	// Keep just the original packed file records after parsing.
	a.data = nil
	a.attachCleanup()
	success = true
	return a, nil
}

func (a *Archive) Close() error {
	if a == nil {
		return nil
	}
	var err error
	a.closeOnce.Do(func() {
		a.closed.Store(true)
		a.ReleaseReadCaches()
		if a.lease != nil {
			a.cleanup.Stop()
			err = a.lease.close()
		}
	})
	return err
}

func (a *Archive) sourceSize() int64 {
	if a.backing != nil {
		return a.snapshot.Size
	}
	return int64(len(a.data))
}

func pathHash(key string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(key); i++ {
		h = (h ^ uint64(key[i])) * 1099511628211
	}
	return h
}

func (a *Archive) itemAt(idx int) fileItem {
	if !a.compactDirectory {
		return a.items[idx]
	}
	p := a.compactTable[idx*fileItemSize:]
	return fileItem{readInt32(p), readInt32(p[4:]), readInt32(p[8:]), readInt32(p[12:]), readInt32(p[16:]), readInt32(p[20:])}
}

func (a *Archive) fileAt(idx int) File {
	if !a.compactDirectory {
		return a.files[idx]
	}
	r := a.itemAt(idx)
	name, dir := a.resolveString(r.nameOffset), a.resolveString(r.pathOffset)
	return File{Index: idx, Path: dir, Name: name, ArchivePath: joinArchivePath(dir, name), DataType: r.dataType, Size: r.dataSize}
}

func (a *Archive) buildCompactIndex() {
	a.compactIndex = make([]directoryEntry, 0, a.FileCount())
	for i := 0; i < a.FileCount(); i++ {
		key := pathKey(a.fileAt(i).ArchivePath)
		if key != "" {
			a.compactIndex = append(a.compactIndex, directoryEntry{pathHash(key), uint32(i)})
		}
	}
	sort.Slice(a.compactIndex, func(i, j int) bool {
		x, y := a.compactIndex[i], a.compactIndex[j]
		return x.hash < y.hash || x.hash == y.hash && x.index < y.index
	})
}

func (a *Archive) lookupPath(value string) (int, bool) {
	key := pathKey(value)
	if !a.compactDirectory {
		i, ok := a.pathIdx[key]
		return i, ok
	}
	h := pathHash(key)
	i := sort.Search(len(a.compactIndex), func(i int) bool { return a.compactIndex[i].hash >= h })
	for ; i < len(a.compactIndex) && a.compactIndex[i].hash == h; i++ {
		idx := int(a.compactIndex[i].index)
		if pathKey(a.fileAt(idx).ArchivePath) == key {
			return idx, true
		}
	}
	return 0, false
}

// IterateFilesUnder avoids materializing unrelated paths when a consumer
// needs one directory subtree. Native directory offsets are reused heavily.
// Names containing separators take the general path to preserve normalization.
func (a *Archive) IterateFilesUnder(prefix string, visit func(File) error) error {
	if a == nil || visit == nil {
		return fmt.Errorf("invalid archive file iterator")
	}
	prefix = pathKey(prefix)
	if prefix == "" {
		return a.IterateFiles(visit)
	}
	prefix += "/"
	if !a.compactDirectory {
		return a.IterateFiles(func(f File) error {
			if strings.HasPrefix(pathKey(f.ArchivePath), prefix) {
				return visit(f)
			}
			return nil
		})
	}
	dirs := make(map[int]bool)
	for i := 0; i < a.FileCount(); i++ {
		r := a.itemAt(i)
		matches, known := dirs[r.pathOffset]
		if !known {
			matches = strings.HasPrefix(pathKey(a.resolveString(r.pathOffset))+"/", prefix)
			dirs[r.pathOffset] = matches
		}
		if !matches && !a.nameHasSeparator(r.nameOffset) {
			continue
		}
		f := a.fileAt(i)
		if strings.HasPrefix(pathKey(f.ArchivePath), prefix) {
			if err := visit(f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Archive) nameHasSeparator(offset int) bool {
	if offset < 0 {
		return false
	}
	if offset&1 == 0 {
		for i := offset >> 1; i < len(a.strA) && a.strA[i] != 0; i++ {
			if a.strA[i] == '/' || a.strA[i] == '\\' {
				return true
			}
		}
	} else {
		for i := (offset >> 1) * 2; i+1 < len(a.strW); i += 2 {
			lo, hi := a.strW[i], a.strW[i+1]
			if lo == 0 && hi == 0 {
				break
			}
			if hi == 0 && (lo == '/' || lo == '\\') {
				return true
			}
		}
	}
	return false
}
