package gamedata

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/derivedcache"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"time"
)

var projectionFormat = derivedcache.Format{Magic: "PVFPRJ01", PackedLimit: 128 << 20, RawLimit: 256 << 20}

// Inputs include the effective caller policy, not just its filename. Source
// checksums here remain physical PVF identities, independent of savecontract.
func projectionKey(parser, source, domain string, inputs any) ([32]byte, error) {
	h := sha256.New()
	e := json.NewEncoder(h)
	if err := e.Encode([]string{projectionFormat.Magic, parser, source, domain}); err != nil {
		return [32]byte{}, err
	}
	if err := e.Encode(inputs); err != nil {
		return [32]byte{}, err
	}
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key, nil
}

// Avoid encoding a second large 600k-row JSON tree merely to identify an
// input. Every effective index field is still bound, in deterministic order.
func itemIndexIdentity(index catalog.ItemIndex) string {
	h := sha256.New()
	writeString := func(s string) {
		var n [8]byte
		binary.LittleEndian.PutUint64(n[:], uint64(len(s)))
		h.Write(n[:])
		io.WriteString(h, s)
	}
	writeString(index.Source.Checksum)
	b, _ := json.Marshal(index.IndexHashes)
	writeString(string(b))
	ids := make([]uint32, 0, len(index.Items))
	for id := range index.Items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var row [12]byte
	for _, id := range ids {
		r := index.Items[id]
		binary.LittleEndian.PutUint32(row[:4], id)
		binary.LittleEndian.PutUint32(row[4:8], r.ID)
		binary.LittleEndian.PutUint32(row[8:], r.StackLimit)
		h.Write(row[:])
		writeString(r.Path)
		writeString(r.Kind)
		writeString(r.StackableType)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Restore validates serialized state and reconstructs private lookup tables
// or independent source views. Never serialize live handles/global activation.
func cachedProjection[T any](s *Source, domain string, inputs any, load func() (T, error), restore func(T) (T, error)) (T, error) {
	if s.archive == nil || s.cacheDir == "" || s.cacheDir == "-" {
		return load()
	}
	parser, err := derivedParserIdentity()
	var key [32]byte
	if err == nil {
		key, err = projectionKey(parser, s.Snapshot().Checksum, domain, inputs)
	}
	if err != nil {
		log.Printf("PVF %s cache unavailable: %v", domain, err)
		return load()
	}
	path := filepath.Join(s.cacheDir, fmt.Sprintf("projection-%s-%x.pvfc", domain, key))
	s.protectCache(path)
	started := time.Now()
	var out T
	hit, readErr := derivedcache.Load(path, key, projectionFormat, func(r io.Reader) error {
		d := json.NewDecoder(r)
		if err := d.Decode(&out); err != nil {
			return err
		}
		var extra any
		if err := d.Decode(&extra); err != io.EOF {
			return fmt.Errorf("projection trailing content: %v", err)
		}
		return nil
	})
	if hit {
		out, readErr = restore(out)
		if readErr == nil {
			s.cacheStats.hits.Add(1)
			log.Printf("PVF projection cache hit domain=%s elapsed=%s", domain, time.Since(started))
			return out, nil
		}
	}
	s.cacheStats.misses.Add(1)
	if readErr != nil {
		s.cacheStats.invalid.Add(1)
		log.Printf("PVF projection cache rejected domain=%s; native rebuild: %v", domain, readErr)
	}
	var zero T
	out = zero
	out, err = load()
	if err != nil {
		return out, err
	}
	if err = derivedcache.Save(path, key, projectionFormat, func(w io.Writer) error { return json.NewEncoder(w).Encode(out) }); err != nil {
		s.cacheStats.writeErrors.Add(1)
		log.Printf("PVF projection cache write skipped domain=%s: %v", domain, err)
	} else {
		s.cacheStats.writes.Add(1)
		log.Printf("PVF projection cache stored domain=%s", domain)
	}
	return out, nil
}

func (s *Source) protectCache(path string) {
	if s.cacheFiles == nil {
		s.cacheFiles = map[string]bool{}
	}
	s.cacheFiles[filepath.Clean(path)] = true
}

func (s *Source) pruneDerivedCaches() {
	if s.cacheDir == "" || s.cacheDir == "-" {
		return
	}
	s.protectCache(s.archive.MetadataCacheFile())
	families := []derivedcache.Family{{Prefix: "archive-meta-", Magic: "PVFMET01"}, {Prefix: "joint-items-", Magic: derivedCacheMagic}}
	for _, domain := range []string{"equipment", "loot", "dungeons", "season", "roster", "terminal", "warps"} {
		families = append(families, derivedcache.Family{Prefix: "projection-" + domain + "-", Magic: projectionFormat.Magic})
	}
	result, err := derivedcache.Prune(s.cacheDir, families, s.cacheFiles, derivedcache.Retention{MaxBytes: 1 << 30, KeepPerFamily: 3, Grace: 24 * time.Hour}, time.Now())
	if err != nil {
		log.Printf("PVF cache retention skipped: %v", err)
	} else if result.Removed > 0 || result.Errors > 0 {
		log.Printf("PVF cache retention removed=%d bytes=%d skipped_errors=%d", result.Removed, result.Bytes, result.Errors)
	}
}

// Per-open cache counters/path diagnostics are not effective game inputs.
func stableWorldInput(w catalog.WorldCatalog) catalog.WorldCatalog {
	w.Source = pvf.ArchiveSnapshot{Checksum: w.Source.Checksum}
	return w
}
func stableDungeonInput(d catalog.DungeonCatalog) catalog.DungeonCatalog {
	d.Source = pvf.ArchiveSnapshot{Checksum: d.Source.Checksum}
	return d
}
func stableQuestInput(q catalog.QuestCatalog) catalog.QuestCatalog {
	q.Source = pvf.ArchiveSnapshot{Checksum: q.Source.Checksum}
	return q
}
