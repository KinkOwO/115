package pvf

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/derivedcache"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMetadataCacheColdHotSourceAndViewLifetime(t *testing.T) {
	raw := compactFixture(t)
	source := filepath.Join(t.TempDir(), "source.pvf")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	sha := fmt.Sprintf("%x", sha256.Sum256(raw))
	open := func(parser string) *Archive {
		a, err := OpenReadOnlyCached(Options{Path: source}, sha, dir, parser)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}
	cold := open("parser-1")
	if cold.MetadataCacheStats().Writes != 1 {
		t.Fatal(cold.MetadataCacheStats())
	}
	hot := open("parser-1")
	if hot.MetadataCacheStats().Hits != 1 {
		t.Fatal(hot.MetadataCacheStats())
	}
	if !bytes.Equal(cold.compactTable, hot.compactTable) || !reflect.DeepEqual(cold.compactIndex, hot.compactIndex) || !reflect.DeepEqual(cold.groups, hot.groups) || !reflect.DeepEqual(cold.stringPools.state.Load(), hot.stringPools.state.Load()) {
		t.Fatal("metadata differs")
	}
	if hot.data != nil || hot.backing == nil || hot.closed.Load() || hot.stringPools.state.Load().compacted {
		t.Fatal("restored archive ownership changed")
	}
	paths := []string{"DIR/entry.stk", "dir/second.txt"}
	for _, p := range paths {
		x, err := cold.ReadRaw(p)
		if err != nil {
			t.Fatal(err)
		}
		y, err := hot.ReadRaw(p)
		if err != nil || !bytes.Equal(x, y) {
			t.Fatal(p, err)
		}
	}
	if idx, ok := hot.lookupPath("dir/entry.stk"); !ok || idx != 0 {
		t.Fatal("duplicate path did not retain native first record", idx, ok)
	}
	view, err := hot.ReadOnlyView(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	if err := hot.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	hot.Close()
	if _, err := hot.ReadRaw(paths[0]); err == nil {
		t.Fatal("closed source readable")
	}
	if x, err := view.ReadRaw(paths[0]); err != nil || len(x) != 10 {
		t.Fatal("view lost held native source", err)
	}
	if a, err := OpenReadOnlyCached(Options{Path: source}, strings.Repeat("0", 64), dir, "parser-1"); err == nil {
		a.Close()
		t.Fatal("cache bypassed expected SHA")
	}
	if a, err := OpenReadOnlyCached(Options{Path: source, MaxBytes: int64(len(raw) - 1)}, sha, dir, "parser-1"); err == nil {
		a.Close()
		t.Fatal("cache bypassed source size limit")
	}
	version := open("parser-2")
	if version.MetadataCacheStats().Hits != 0 || version.MetadataCacheStats().Writes != 1 {
		t.Fatal("parser version did not invalidate", version.MetadataCacheStats())
	}
	cold.Close()
	version.Close()
	view.Close()
	changed := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint32(changed[headerSize+2*fileItemSize+20:], 3)
	if err := os.WriteFile(source, changed, 0600); err != nil {
		t.Fatal(err)
	}
	a, err := OpenReadOnlyCached(Options{Path: source}, "", dir, "parser-1")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.MetadataCacheStats().Hits != 0 || a.MetadataCacheStats().Writes != 1 || a.Snapshot().Checksum == sha {
		t.Fatal("changed same-path source reused old cache", a.MetadataCacheStats())
	}
}

func TestMetadataCacheCorruptionAndWriteFailureFallback(t *testing.T) {
	raw := compactFixture(t)
	source := filepath.Join(t.TempDir(), "source.pvf")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a, err := OpenReadOnlyCached(Options{Path: source}, "", dir, "parser")
	if err != nil {
		t.Fatal(err)
	}
	key := metadataCacheKey("parser", a.Snapshot().Checksum)
	p := metadataCachePath(dir, key)
	var image bytes.Buffer
	if err := encodeMetadata(&image, a); err != nil {
		t.Fatal(err)
	}
	indexOffset := headerSize + 24 + len(a.compactTable) + len(a.groups)*8
	want, err := a.ReadRaw("dir/entry.stk")
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	mutations := []func([]byte){
		func(b []byte) { b[0] ^= 1 },
		func(b []byte) { binary.LittleEndian.PutUint64(b[48:56], 4) },
		func(b []byte) { binary.LittleEndian.PutUint64(b[56:64], metadataRawLimit+1) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[indexOffset+8:], 3) },
		func(b []byte) { copy(b[indexOffset+12+8:indexOffset+24], b[indexOffset+8:indexOffset+12]) },
		func(b []byte) { binary.LittleEndian.PutUint64(b[indexOffset+12:], 0) },
	}
	for i, mutate := range mutations {
		b := append([]byte(nil), image.Bytes()...)
		mutate(b)
		if err := derivedcache.Save(p, key, metadataCacheFormat, func(w io.Writer) error { _, err := w.Write(b); return err }); err != nil {
			t.Fatal(err)
		}
		c, err := OpenReadOnlyCached(Options{Path: source}, "", dir, "parser")
		if err != nil {
			t.Fatal(err)
		}
		if c.MetadataCacheStats().Invalid != 1 || c.MetadataCacheStats().Writes != 1 || c.MetadataCacheStats().Hits != 0 {
			t.Fatal(i, c.MetadataCacheStats())
		}
		got, err := c.ReadRaw("dir/entry.stk")
		c.Close()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("invalid cache leaked partial archive", i, err)
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)-1] ^= 1
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := OpenReadOnlyCached(Options{Path: source}, "", dir, "parser")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.MetadataCacheStats().Invalid != 1 || c.MetadataCacheStats().Writes != 1 {
		t.Fatal(c.MetadataCacheStats())
	}
	blocked := filepath.Join(t.TempDir(), "unrelated-file")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := OpenReadOnlyCached(Options{Path: source}, "", blocked, "parser")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.MetadataCacheStats().WriteErrors != 1 {
		t.Fatal(d.MetadataCacheStats())
	}
	if got, err := d.ReadRaw("dir/entry.stk"); err != nil || !bytes.Equal(got, want) {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(blocked); err != nil || string(b) != "keep" {
		t.Fatal("unrelated file modified", err)
	}
	for _, disabled := range []string{"", "-"} {
		d, err := OpenReadOnlyCached(Options{Path: source}, "", disabled, "parser")
		if err != nil {
			t.Fatal(err)
		}
		d.Close()
		if d.MetadataCacheStats() != (MetadataCacheStats{}) {
			t.Fatal("disabled cache attempted IO")
		}
	}
}

func TestMetadataCacheLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete cached metadata parity")
	}
	dir := t.TempDir()
	a, err := OpenReadOnlyCached(Options{Path: p, MaxBytes: 1 << 30}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), dir, "native-test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.MetadataCacheStats().Writes != 1 {
		t.Fatal(a.MetadataCacheStats())
	}
	b, err := OpenReadOnlyCached(Options{Path: p, MaxBytes: 1 << 30}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), dir, "native-test")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if b.MetadataCacheStats().Hits != 1 {
		t.Fatal(b.MetadataCacheStats())
	}
	if a.FileCount() != b.FileCount() || !bytes.Equal(a.compactTable, b.compactTable) || !reflect.DeepEqual(a.compactIndex, b.compactIndex) || !reflect.DeepEqual(a.groups, b.groups) || nativePoolDigest(t, a.stringPools) != nativePoolDigest(t, b.stringPools) {
		t.Fatal("complete native metadata differs")
	}
	// The raw compact table bytes and file count above stay exhaustive; the
	// expanded directory records are compared on a deterministic cross-range
	// sample so the default run stays fast. Set DFO_PVF_ARCHIVE_FULL_SWEEP=1 for
	// every record.
	full := sampledDirectoryIndices(t, a, "")
	subtree := sampledDirectoryIndices(t, a, "equipment")
	if digestArchiveIndices(t, a, full) != digestArchiveIndices(t, b, full) || digestArchiveIndices(t, a, subtree) != digestArchiveIndices(t, b, subtree) {
		t.Fatal("complete native directory differs")
	}
	paths := []string{"list/map.lst", "list/dungeon.lst", "list/equipment.lst", "list/stackable.lst"}
	for _, p := range paths {
		x, err := a.ReadRaw(p)
		if err != nil {
			t.Fatal(err)
		}
		y, err := b.ReadRaw(p)
		if err != nil || !bytes.Equal(x, y) {
			t.Fatal("source LIST read differs", p, err)
		}
	}
	view, err := b.ReadOnlyView(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	before := nativePoolDigest(t, b.stringPools)
	if err := b.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	if before != nativePoolDigest(t, b.stringPools) {
		t.Fatal("restored pool compaction changed bytes")
	}
	b.Close()
	if _, err := view.Tokens(paths[0]); err != nil {
		t.Fatal("cached source child failed after parent close", err)
	}
	t.Logf("all metadata/path indexes/pool bytes and %d files match native parsing; child views remain native", a.FileCount())
}
