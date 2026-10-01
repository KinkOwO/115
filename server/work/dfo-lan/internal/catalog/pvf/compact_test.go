package pvf

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func compactFixture(t *testing.T) []byte {
	t.Helper()
	pool := []byte{0}
	add := func(s string) int {
		off := len(pool) * 2
		pool = append(pool, []byte(s)...)
		pool = append(pool, 0)
		return off
	}
	dir := add("Dir")
	name := add("entry.stk")
	other := add("second.txt")
	label := add("[price]")
	body := []byte{3, 0, 0, 0, 0, 0, 123, 0, 0, 0, 'O', 0, 'K', 0}
	binary.LittleEndian.PutUint32(body[1:], uint32(label))
	var buf bytes.Buffer
	z := zlib.NewWriter(&buf)
	z.Write(body)
	z.Close()
	packed := buf.Bytes()
	decryptProtected("mAIn", packed)
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
	hashes := make([]byte, 8)
	decryptProtected("HSrm", hashes)
	groups := make([]byte, 8)
	binary.LittleEndian.PutUint32(groups, uint32(len(packed)))
	binary.LittleEndian.PutUint32(groups[4:], uint32(len(body)))
	decryptProtected("Gidx", groups)
	table := make([]byte, 3*fileItemSize)
	for i, row := range []struct{ name, off, size, typ int }{{name, 0, 10, 1}, {other, 10, 4, 3}, {name, 5, 5, 1}} {
		p := table[i*fileItemSize:]
		for j, v := range []int{row.name, dir, 0, row.off, row.size, row.typ} {
			binary.LittleEndian.PutUint32(p[j*4:], uint32(v))
		}
	}
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header, magicSignature)
	for off, v := range map[int]int{24: 3, 32: len(packed), 36: 1, 40: len(hashes), 44: len(names)} {
		binary.LittleEndian.PutUint32(header[off:], uint32(v))
	}
	decryptProtected("iNfO", header)
	out := append(header, table...)
	out = append(out, hashes...)
	out = append(out, names...)
	out = append(out, groups...)
	return append(out, packed...)
}

func TestCompactFileArchiveParityAndViewLifetime(t *testing.T) {
	raw := compactFixture(t)
	path := filepath.Join(t.TempDir(), "source.pvf")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := OpenReadOnly(Options{Path: path}, fmt.Sprintf("%x", sha256.Sum256(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	memory, err := OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.data) != 0 || len(a.files) != 0 || len(a.items) != 0 || !reflect.DeepEqual(a.Files(), memory.Files()) {
		t.Fatal("compact directory differs or retains expanded metadata")
	}
	if a.FindFileIndex("./DIR\\ENTRY.STK") != 0 {
		t.Fatal("normalization/first duplicate changed")
	}
	for _, f := range memory.Files() {
		got, err := a.FileRaw(f.Index)
		want, e := memory.FileRaw(f.Index)
		if err != nil || e != nil || !bytes.Equal(got, want) {
			t.Fatal(f, err, e)
		}
	}
	if !bytes.Equal(a.Bytes(), raw) {
		t.Fatal("explicit raw archive read differs")
	}
	view, err := a.ReadOnlyView([]string{"dir/second.txt"})
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	if view.FileCount() != 1 || view.FindFileIndex("dir/second.txt") != 0 {
		t.Fatal("view not remapped")
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = a.FileRaw(0); err == nil {
		t.Fatal("closed parent accepted read")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				v, e := view.ReadText("dir/second.txt")
				if e != nil || v != "OK" {
					t.Errorf("view read: %q %v", v, e)
				}
				view.ReleaseReadCaches()
			}
		}()
	}
	wg.Wait()
	if runtime.GOOS == "windows" {
		if f, e := os.OpenFile(path, os.O_WRONLY, 0); e == nil {
			f.Close()
			t.Fatal("runtime source accepted concurrent write")
		}
	}
	backing := view.backing
	view.Close()
	if _, err := backing.file.Stat(); err == nil {
		t.Fatal("last lease did not close source")
	}
}

func TestCompactPathHashCollisionChecksFullPath(t *testing.T) {
	raw := compactFixture(t)
	a, err := OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	a.compactDirectory = true
	a.compactTable = append([]byte(nil), a.data[headerSize:headerSize+3*fileItemSize]...)
	a.files = nil
	a.items = nil
	h := pathHash("dir/second.txt")
	a.compactIndex = []directoryEntry{{h, 0}, {h, 1}, {h, 2}}
	if got := a.FindFileIndex("dir/second.txt"); got != 1 {
		t.Fatalf("collision resolved to %d", got)
	}
	if a.FindFileIndex("not-present") != -1 {
		t.Fatal("absent path accepted")
	}
}

func TestReadOnlySourceRejectsIdentityAndTruncation(t *testing.T) {
	raw := compactFixture(t)
	p := filepath.Join(t.TempDir(), "test.pvf")
	os.WriteFile(p, raw, 0600)
	if a, e := OpenReadOnly(Options{Path: p}, fmt.Sprintf("%064x", 0)); e == nil {
		a.Close()
		t.Fatal("wrong source accepted")
	}
	if a, e := OpenReadOnly(Options{Path: p, MaxBytes: 1}, fmt.Sprintf("%x", sha256.Sum256(raw))); e == nil {
		a.Close()
		t.Fatal("size limit ignored")
	}
	for _, n := range []int{0, 47, len(raw) - 1} {
		os.WriteFile(p, raw[:n], 0600)
		if a, e := OpenReadOnly(Options{Path: p}, fmt.Sprintf("%x", sha256.Sum256(raw[:n]))); e == nil {
			a.Close()
			t.Fatalf("truncation %d accepted", n)
		}
	}
}

func TestBoundedChunkCacheRetainsRecentHits(t *testing.T) {
	a := &Archive{maxChunkBytes: 6, maxTexts: 2}
	a.cacheChunk(0, []byte{1, 2})
	a.cacheChunk(1, []byte{3, 4})
	a.cacheChunk(2, []byte{5, 6})
	a.cachedChunk(0)
	a.cacheChunk(3, []byte{7, 8})
	if _, ok := a.cachedChunk(0); !ok {
		t.Fatal("recently read chunk evicted")
	}
	if _, ok := a.cachedChunk(1); ok {
		t.Fatal("oldest unused chunk retained")
	}
	if _, ok := a.cachedChunk(2); !ok {
		t.Fatal("cache was cleared wholesale")
	}
	if a.cachedChunkBytes != 6 {
		t.Fatal(a.cachedChunkBytes)
	}
}

func TestFileSubtreeIteratorMatchesFullEnumeration(t *testing.T) {
	a, err := OpenBytes(compactFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, compact := range []bool{false, true} {
		if compact {
			a.compactDirectory = true
			a.compactTable = append([]byte(nil), a.data[headerSize:headerSize+3*fileItemSize]...)
		}
		for _, prefix := range []string{"DIR", "di", "unrelated", "./dir\\"} {
			var got, want []File
			if err := a.IterateFilesUnder(prefix, func(f File) error { got = append(got, f); return nil }); err != nil {
				t.Fatal(err)
			}
			a.IterateFiles(func(f File) error {
				if strings.HasPrefix(pathKey(f.ArchivePath), pathKey(prefix)+"/") {
					want = append(want, f)
				}
				return nil
			})
			if !reflect.DeepEqual(got, want) {
				t.Fatal(compact, prefix, got, want)
			}
		}
	}
	// A name containing a relative directory must still use full normalization.
	off := len(a.strA) * 2
	a.strA = append(a.strA, []byte("../other/entry.stk\x00")...)
	binary.LittleEndian.PutUint32(a.compactTable, uint32(off))
	var files []File
	a.IterateFilesUnder("other", func(f File) error { files = append(files, f); return nil })
	if len(files) != 1 || files[0].ArchivePath != "other/entry.stk" {
		t.Fatal("relative filename was skipped", files)
	}
}

func TestReadOnlyCleanupReleasesDescriptorWithCacheCycles(t *testing.T) {
	raw := compactFixture(t)
	p := filepath.Join(t.TempDir(), "cleanup.pvf")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	backing := func() *archiveFile {
		a, err := OpenReadOnly(Options{Path: p}, fmt.Sprintf("%x", sha256.Sum256(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.ReadRaw("dir/entry.stk"); err != nil {
			t.Fatal(err)
		}
		return a.backing
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if _, err := backing.file.Stat(); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("unreachable archive cache cycle retained descriptor")
}
