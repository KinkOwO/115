package gamedata

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type itemCacheIdentity struct {
	Format, Parser, Source, Policy, PolicyPath                      string
	Periods, Prices, Materials, Skins, Boosters, Enhancements, Fame bool
}

func itemDerivedKey(parser, source string, o catalog.ItemBasicOptions, enhancements bool, policyPath, policy string, fame bool) [32]byte {
	id := itemCacheIdentity{derivedCacheMagic, parser, source, policy, policyPath, o.Periods, o.Prices, o.Materials, o.Skins, o.Boosters, enhancements, fame}
	b, _ := json.Marshal(id)
	return sha256.Sum256(b)
}

func (s *Source) cachedItemCatalogs(o catalog.ItemBasicOptions, enhancements bool, policyPath string, fame bool) (JointItemCatalogs, error) {
	if s.mode != PVF || s.cacheDir == "" || s.cacheDir == "-" || len(o.Consumers) != 0 {
		return s.importItemCatalogs(o, enhancements, policyPath, fame)
	}
	parser, err := derivedParserIdentity()
	policy := ""
	if err == nil && enhancements {
		var b []byte
		b, err = os.ReadFile(policyPath)
		if err == nil {
			policy = fmt.Sprintf("%x", sha256.Sum256(b))
		}
	}
	if err != nil {
		log.Printf("PVF derived cache unavailable; native item import: %v", err)
		return s.importItemCatalogs(o, enhancements, policyPath, fame)
	}
	key := itemDerivedKey(parser, s.Snapshot().Checksum, o, enhancements, policyPath, policy, fame)
	path := derivedPath(s.cacheDir, key)
	started := time.Now()
	var cached JointItemCatalogs
	hit, readErr := loadDerived(path, key, func(d *json.Decoder) error { return decodeJointItems(d, &cached) })
	if hit {
		if readErr = s.validateItemCache(&cached, o, enhancements, fame); readErr == nil {
			s.cacheStats.hits.Add(1)
			log.Printf("PVF derived item cache hit key=%x elapsed=%s templates=%d", key[:8], time.Since(started), len(cached.Basics.Index.Items))
			return cached, nil
		}
	}
	s.cacheStats.misses.Add(1)
	if readErr != nil {
		s.cacheStats.invalid.Add(1)
		log.Printf("PVF derived item cache rejected key=%x; rebuilding from verified PVF: %v", key[:8], readErr)
	} else {
		log.Printf("PVF derived item cache miss key=%x", key[:8])
	}
	// Drop partially decoded data before the native scan. No cached global
	// activation or source-version rewrite occurs, even on decode failure.
	cached = JointItemCatalogs{}
	out, err := s.importItemCatalogs(o, enhancements, policyPath, fame)
	if err != nil {
		return JointItemCatalogs{}, err
	}
	if err = saveDerived(path, key, func(e *json.Encoder) error { return encodeJointItems(e, out) }); err != nil {
		s.cacheStats.writeErrors.Add(1)
		log.Printf("PVF derived item cache write skipped; native catalogs remain usable: %v", err)
	} else {
		s.cacheStats.writes.Add(1)
		log.Printf("PVF derived item cache stored key=%x", key[:8])
	}
	return out, nil
}

// Maps are streamed in bounded batches so serialization does not expand all
// 600k rows into a second large buffer. JSON preserves a zero *uint32 price as
// distinct from an absent purchase price (gob pointer flattening would not).
type cachedItemRow[V any] struct {
	ID    uint32
	Value V
}

const itemCacheBatch = 512
const maxCachedItems = 2_000_000

func encodeItemMap[V any](e *json.Encoder, rows map[uint32]V) error {
	n := len(rows)
	if rows == nil {
		n = -1
	}
	if err := e.Encode(n); err != nil {
		return err
	}
	batch := make([]cachedItemRow[V], 0, itemCacheBatch)
	for id, value := range rows {
		batch = append(batch, cachedItemRow[V]{id, value})
		if len(batch) == itemCacheBatch {
			if err := e.Encode(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return e.Encode(batch)
	}
	return nil
}

func decodeItemMap[V any](d *json.Decoder) (map[uint32]V, error) {
	var n int
	if err := d.Decode(&n); err != nil {
		return nil, err
	}
	if n < -1 || n > maxCachedItems {
		return nil, fmt.Errorf("invalid derived item count")
	}
	if n == -1 {
		return nil, nil
	}
	rows := make(map[uint32]V, n)
	for remaining := n; remaining > 0; {
		var batch []cachedItemRow[V]
		if err := d.Decode(&batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 || len(batch) > itemCacheBatch || len(batch) > remaining {
			return nil, fmt.Errorf("invalid derived item batch")
		}
		for _, r := range batch {
			if _, exists := rows[r.ID]; exists {
				return nil, fmt.Errorf("duplicate derived item %d", r.ID)
			}
			rows[r.ID] = r.Value
		}
		remaining -= len(batch)
	}
	return rows, nil
}

func encodeJointItems(e *json.Encoder, out JointItemCatalogs) error {
	header := out
	header.Basics.Index.Items = nil
	header.Basics.Boosters = nil
	var prices map[uint32]catalog.ShopPrice
	if out.Basics.Prices != nil {
		meta := *out.Basics.Prices
		prices = meta.Items
		meta.Items = nil
		header.Basics.Prices = &meta
	}
	if err := e.Encode(header); err != nil {
		return err
	}
	if err := encodeItemMap(e, out.Basics.Index.Items); err != nil {
		return err
	}
	if err := encodeItemMap(e, prices); err != nil {
		return err
	}
	return encodeItemMap(e, out.Basics.Boosters)
}

func decodeJointItems(d *json.Decoder, out *JointItemCatalogs) error {
	if err := d.Decode(out); err != nil {
		return err
	}
	var err error
	if out.Basics.Index.Items, err = decodeItemMap[catalog.ItemIndexEntry](d); err != nil {
		return err
	}
	prices, err := decodeItemMap[catalog.ShopPrice](d)
	if err != nil {
		return err
	}
	if out.Basics.Prices != nil {
		out.Basics.Prices.Items = prices
	} else if prices != nil {
		return fmt.Errorf("unexpected derived prices")
	}
	if out.Basics.Boosters, err = decodeItemMap[catalog.BoosterDefinition](d); err != nil {
		return err
	}
	if out.Basics.Materials != nil {
		m := out.Basics.Materials
		out.Basics.Materials, err = catalog.RestoreItemMaterials(m.Source, m.Items)
	}
	return err
}

func (s *Source) validateItemCache(out *JointItemCatalogs, o catalog.ItemBasicOptions, enhancements, fame bool) error {
	b := &out.Basics
	snapshot := s.Snapshot()
	if b.Index.Source.Checksum != snapshot.Checksum || b.Index.Source.Size != snapshot.Size || b.Index.Source.FileCount != snapshot.FileCount || b.Index.Source.Format != snapshot.Format {
		return fmt.Errorf("derived item source identity mismatch")
	}
	if err := b.Index.Validate(); err != nil {
		return err
	}
	if len(b.Index.IndexHashes) != 2 || b.ScriptsRead > uint64(len(b.Index.Items)) {
		return fmt.Errorf("invalid derived item provenance")
	}
	for _, kind := range []string{"equipment", "stackable"} {
		p := catalog.ResolveScriptPath(s.archive, "list/"+kind+".lst")
		raw, err := s.archive.ReadRaw(p)
		if err != nil {
			return err
		}
		if b.Index.IndexHashes[p] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			return fmt.Errorf("derived item LIST checksum mismatch")
		}
	}
	if (b.Periods != nil) != o.Periods || (b.Prices != nil) != o.Prices || (b.Materials != nil) != o.Materials || (b.Skins != nil) != o.Skins || (b.Boosters != nil) != o.Boosters || (out.Enhancements != nil) != enhancements || (out.Fame != nil) != fame {
		return fmt.Errorf("derived item domain scope mismatch")
	}
	b.Index.Source.Path, b.Index.Source.LoadedAt = snapshot.Path, snapshot.LoadedAt
	if b.Periods != nil {
		if b.Periods.Schema != catalog.ItemPeriodCatalogSchema || b.Periods.Source.Checksum != snapshot.Checksum {
			return fmt.Errorf("invalid derived periods")
		}
		b.Periods.Source.Path, b.Periods.Source.LoadedAt = snapshot.Path, snapshot.LoadedAt
	}
	if b.Prices != nil && b.Prices.Source != snapshot.Checksum || b.Materials != nil && b.Materials.Source != snapshot.Checksum {
		return fmt.Errorf("invalid derived commerce identity")
	}
	if b.Skins != nil {
		if b.Skins.Schema != catalog.SkinStorageSchema || b.Skins.Source.Checksum != snapshot.Checksum {
			return fmt.Errorf("invalid derived skins")
		}
		b.Skins.Source.Path, b.Skins.Source.LoadedAt = snapshot.Path, snapshot.LoadedAt
	}
	if out.Enhancements != nil {
		if err := out.Enhancements.Validate(); err != nil {
			return err
		}
		if out.Enhancements.Grimoires.Source != snapshot.Checksum || out.Enhancements.Gold.Source != snapshot.Checksum || out.Enhancements.Amplify.Source != snapshot.Checksum {
			return fmt.Errorf("invalid derived enhancement identity")
		}
	}
	if out.Fame != nil {
		if out.Fame.Source != snapshot.Checksum {
			return fmt.Errorf("invalid derived fame identity")
		}
		if _, err := character.NewFameRules(*out.Fame); err != nil {
			return err
		}
	}
	return nil
}
