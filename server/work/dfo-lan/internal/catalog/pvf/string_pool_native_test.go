package pvf

import (
	"crypto/sha256"
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
	beforeCount := a.FileCount()
	fullIdx := sampledDirectoryIndices(t, a, "")
	subtreeIdx := sampledDirectoryIndices(t, a, "equipment")
	beforeFiles := digestArchiveIndices(t, a, fullIdx)
	beforeSubtree := digestArchiveIndices(t, a, subtreeIdx)
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
	if a.FileCount() != beforeCount {
		t.Fatal("directory file count changed")
	}
	// The full count above is exhaustive; the expanded records are compared on
	// the same deterministic cross-range sample. Set DFO_PVF_ARCHIVE_FULL_SWEEP=1
	// for every record.
	if got := digestArchiveIndices(t, a, fullIdx); got != beforeFiles {
		t.Fatal("complete directory differs")
	}
	if got := digestArchiveIndices(t, a, subtreeIdx); got != beforeSubtree {
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
