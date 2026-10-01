package gamedata

import (
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const derivedCacheMagic = "PVFJNT01"
const derivedHeaderSize = 88
const maxDerivedPacked = 256 << 20
const maxDerivedRaw = 512 << 20

type derivedCacheCounters struct{ hits, misses, invalid, writes, writeErrors atomic.Uint64 }
type DerivedCacheStats struct{ Hits, Misses, Invalid, Writes, WriteErrors uint64 }

func (s *Source) DerivedCacheStats() DerivedCacheStats {
	c := &s.cacheStats
	return DerivedCacheStats{c.hits.Load(), c.misses.Load(), c.invalid.Load(), c.writes.Load(), c.writeErrors.Load()}
}

// Binding to the exact executable also invalidates caches for uncommitted
// parser changes, embedded rules and dependency/toolchain changes.
var derivedParserIdentity = sync.OnceValues(func() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
})

func derivedPath(dir string, key [32]byte) string {
	return filepath.Join(dir, fmt.Sprintf("joint-items-%x.pvfc", key))
}

func loadDerived(path string, key [32]byte, decode func(*json.Decoder) error) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() < derivedHeaderSize || info.Size()-derivedHeaderSize > maxDerivedPacked {
		return false, fmt.Errorf("invalid derived cache size")
	}
	var header [derivedHeaderSize]byte
	if _, err = io.ReadFull(f, header[:]); err != nil {
		return false, err
	}
	if string(header[:8]) != derivedCacheMagic || string(header[8:40]) != string(key[:]) {
		return false, fmt.Errorf("derived cache identity mismatch")
	}
	packed, raw := binary.LittleEndian.Uint64(header[72:80]), binary.LittleEndian.Uint64(header[80:88])
	if packed != uint64(info.Size()-derivedHeaderSize) || raw == 0 || raw > maxDerivedRaw {
		return false, fmt.Errorf("invalid derived cache lengths")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return false, err
	}
	if string(h.Sum(nil)) != string(header[40:72]) {
		return false, fmt.Errorf("derived cache checksum mismatch")
	}
	if _, err = f.Seek(derivedHeaderSize, io.SeekStart); err != nil {
		return false, err
	}
	z, err := zlib.NewReader(f)
	if err != nil {
		return false, err
	}
	defer z.Close()
	r := &countedReader{Reader: io.LimitReader(z, int64(raw)+1)}
	d := json.NewDecoder(r)
	if err = decode(d); err != nil {
		return false, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return false, fmt.Errorf("derived cache trailing or damaged content: %v", err)
	}
	if r.n != int64(raw) {
		return false, fmt.Errorf("derived cache expanded length mismatch")
	}
	return true, nil
}

type countedReader struct {
	io.Reader
	n int64
}

func (r *countedReader) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	r.n += int64(n)
	return n, e
}

type boundedWriter struct {
	io.Writer
	n, limit int64
}

func (w *boundedWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.limit-w.n {
		return 0, fmt.Errorf("derived cache size budget exceeded")
	}
	n, e := w.Writer.Write(b)
	w.n += int64(n)
	return n, e
}

// Publish a fully synced same-directory temporary file. Readers verify a held
// file before decoding; a competing rename or transient miss never publishes a
// partial catalog. Cache failure is an optimization failure, not a source error.
func saveDerived(path string, key [32]byte, encode func(*json.Encoder) error) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".joint-items-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { f.Close(); os.Remove(tmp) }()
	if _, err = f.Write(make([]byte, derivedHeaderSize)); err != nil {
		return err
	}
	h := sha256.New()
	packed := &boundedWriter{Writer: io.MultiWriter(f, h), limit: maxDerivedPacked}
	z, err := zlib.NewWriterLevel(packed, zlib.BestSpeed)
	if err != nil {
		return err
	}
	raw := &boundedWriter{Writer: z, limit: maxDerivedRaw}
	if err = encode(json.NewEncoder(raw)); err != nil {
		z.Close()
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	var header [derivedHeaderSize]byte
	copy(header[:8], derivedCacheMagic)
	copy(header[8:40], key[:])
	copy(header[40:72], h.Sum(nil))
	binary.LittleEndian.PutUint64(header[72:80], uint64(packed.n))
	binary.LittleEndian.PutUint64(header[80:88], uint64(raw.n))
	if _, err = f.WriteAt(header[:], 0); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
