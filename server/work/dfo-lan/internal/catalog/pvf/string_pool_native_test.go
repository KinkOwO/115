package pvf

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
	"reflect"
	"testing"
	"time"
)

func nativePoolDigest(t *testing.T, p *runtimeStringPools) [2][32]byte {
	t.Helper()
	state := p.state.Load()
	var sums [2][32]byte
	for i := range 2 {
		h := sha256.New()
		raw, pool := state.a, state.compressedA
		if i == 1 {
			raw, pool = state.w, state.compressedW
		}
		if pool == nil {
			h.Write(raw)
		} else {
			for block := range pool.blocks {
				data, err := p.block(pool, poolBlockKey{i == 1, block})
				if err != nil {
					t.Fatal(err)
				}
				h.Write(data)
			}
		}
		copy(sums[i][:], h.Sum(nil))
	}
	return sums
}

func nativeDirectoryDigest(t *testing.T, a *Archive, prefix string) [32]byte {
	t.Helper()
	h := sha256.New()
	visit := func(file File) error {
		var numeric [24]byte
		binary.LittleEndian.PutUint64(numeric[:8], uint64(file.Index))
		binary.LittleEndian.PutUint64(numeric[8:16], uint64(file.DataType))
		binary.LittleEndian.PutUint64(numeric[16:], uint64(file.Size))
		h.Write(numeric[:])
		for _, s := range []string{file.Path, file.Name, file.ArchivePath} {
			binary.LittleEndian.PutUint64(numeric[:8], uint64(len(s)))
			h.Write(numeric[:8])
			h.Write([]byte(s))
		}
		return nil
	}
	var err error
	if prefix == "" {
		err = a.IterateFiles(visit)
	} else {
		err = a.IterateFilesUnder(prefix, visit)
	}
	if err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func TestCompressedPoolsLocalArchiveParity(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete pools/directory parity")
	}
	a, err := OpenReadOnly(Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	beforePools := nativePoolDigest(t, a.stringPools)
	beforeFiles := nativeDirectoryDigest(t, a, "")
	beforeSubtree := nativeDirectoryDigest(t, a, "equipment")
	paths := []string{"list/map.lst", "list/dungeon.lst"}
	tokens := make([][]Token, len(paths))
	for i, path := range paths {
		tokens[i], err = a.Tokens(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	view, err := a.ReadOnlyView(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Close()
	begin := time.Now()
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	t.Logf("pool compaction=%s", time.Since(begin))
	if view.stringPools != a.stringPools {
		t.Fatal("view did not share pool owner")
	}
	if a.stringPools.state.Load().w != nil {
		t.Fatal("native raw W pool retained")
	}
	if got := nativePoolDigest(t, a.stringPools); got != beforePools {
		t.Fatal("native pool bytes differ")
	}
	if got := nativeDirectoryDigest(t, a, ""); got != beforeFiles {
		t.Fatal("complete directory differs")
	}
	if got := nativeDirectoryDigest(t, a, "equipment"); got != beforeSubtree {
		t.Fatal("subtree admission/order differs")
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		got, err := view.Tokens(path)
		if err != nil || !reflect.DeepEqual(tokens[i], got) {
			t.Fatalf("view tokens %s differ after parent close: %v", path, err)
		}
	}
	if view.stringPools.cacheBytes > stringPoolCacheBytes {
		t.Fatal("native pool cache exceeded budget")
	}
	t.Logf("source=%s files=%d cached decoded pool bytes=%d", view.Snapshot().Checksum, a.FileCount(), view.stringPools.cacheBytes)
}
