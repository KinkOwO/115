package gamedata

import (
	"crypto/sha256"
	"dfolan/internal/derivedcache"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// parser changes, embedded rules and dependency/toolchain changes. Tests may
// override it with DFO_PVF_CACHE_IDENTITY so a test-binary rebuild does not
// discard the parsed-projection cache between runs.
var derivedParserIdentity = sync.OnceValues(func() (string, error) {
	if id := strings.TrimSpace(os.Getenv("DFO_PVF_CACHE_IDENTITY")); id != "" {
		return id, nil
	}
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

var itemCacheFormat = derivedcache.Format{Magic: derivedCacheMagic, PackedLimit: maxDerivedPacked, RawLimit: maxDerivedRaw}

func loadDerived(path string, key [32]byte, decode func(*json.Decoder) error) (bool, error) {
	return derivedcache.Load(path, key, itemCacheFormat, func(r io.Reader) error {
		d := json.NewDecoder(r)
		if err := decode(d); err != nil {
			return err
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return fmt.Errorf("derived cache trailing or damaged content: %v", err)
		}
		return nil
	})
}

func saveDerived(path string, key [32]byte, encode func(*json.Encoder) error) error {
	return derivedcache.Save(path, key, itemCacheFormat, func(w io.Writer) error { return encode(json.NewEncoder(w)) })
}
