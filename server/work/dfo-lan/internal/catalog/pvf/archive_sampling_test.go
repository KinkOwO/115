package pvf

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"os"
	"testing"
)

// archiveSampleCap bounds the deterministic cross-range sampling used by the
// heavyweight native directory parity tests. The full record sweep stays
// available with DFO_PVF_ARCHIVE_FULL_SWEEP=1 (historical audit convention,
// server/AGENTS.md §0).
const archiveSampleCap = 500

func archiveFullSweep() bool { return os.Getenv("DFO_PVF_ARCHIVE_FULL_SWEEP") == "1" }

// sampledArchiveIndexes returns up to archiveSampleCap indexes evenly spread
// across [0,total), always including the first and last. DFO_PVF_ARCHIVE_FULL_SWEEP=1
// returns every index. The result is identical for two archives parsed from the
// same source, so callers can compare sampled records in lockstep.
func sampledArchiveIndexes(total int) []int {
	if total <= 0 {
		return nil
	}
	if archiveFullSweep() || total <= archiveSampleCap {
		out := make([]int, total)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, archiveSampleCap)
	for i := range out {
		out[i] = int(int64(i) * int64(total-1) / int64(archiveSampleCap-1))
	}
	return out
}

// sampledDirectoryIndices returns the file indexes to compare for a directory
// subtree. It walks the subtree once so callers can reuse the indexes after
// compaction instead of re-resolving every path.
func sampledDirectoryIndices(t *testing.T, a *Archive, prefix string) []int {
	t.Helper()
	if prefix == "" {
		return sampledArchiveIndexes(a.FileCount())
	}
	var matches []int
	if err := a.IterateFilesUnder(prefix, func(f File) error {
		matches = append(matches, f.Index)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if archiveFullSweep() || len(matches) <= archiveSampleCap {
		return matches
	}
	out := make([]int, archiveSampleCap)
	for i := range out {
		out[i] = matches[int(int64(i)*int64(len(matches)-1)/int64(archiveSampleCap-1))]
	}
	return out
}

// hashArchiveFile folds one directory record into a digest. It matches the
// fields the exhaustive nativeDirectoryDigest covers.
func hashArchiveFile(h hash.Hash, f File) {
	var numeric [24]byte
	binary.LittleEndian.PutUint64(numeric[:8], uint64(f.Index))
	binary.LittleEndian.PutUint64(numeric[8:16], uint64(f.DataType))
	binary.LittleEndian.PutUint64(numeric[16:], uint64(f.Size))
	h.Write(numeric[:])
	for _, s := range []string{f.Path, f.Name, f.ArchivePath} {
		binary.LittleEndian.PutUint64(numeric[:8], uint64(len(s)))
		h.Write(numeric[:8])
		h.Write([]byte(s))
	}
}

// digestArchiveIndices hashes the directory records at the given indexes.
func digestArchiveIndices(t *testing.T, a *Archive, indices []int) [32]byte {
	t.Helper()
	h := sha256.New()
	for _, i := range indices {
		f, err := a.FileInfo(i)
		if err != nil {
			t.Fatal(err)
		}
		hashArchiveFile(h, f)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}
