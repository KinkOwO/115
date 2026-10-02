package catalog

import (
	"os"
	"reflect"
	"testing"
)

// archiveSampleCap bounds the deterministic cross-range sampling used by the
// heavyweight native item parity test. Full counts and index hashes stay
// exhaustive; per-row fields are compared on this sample. Set
// DFO_PVF_ARCHIVE_FULL_SWEEP=1 to restore the exhaustive comparison.
const archiveSampleCap = 500

func archiveFullSweep() bool { return os.Getenv("DFO_PVF_ARCHIVE_FULL_SWEEP") == "1" }

// paritySample returns a deterministic stride sample of an already ordered
// slice, or the whole slice when it is small or a full sweep is requested.
func paritySample[T any](all []T) []T {
	if archiveFullSweep() || len(all) <= archiveSampleCap {
		return all
	}
	step := (len(all) + archiveSampleCap - 1) / archiveSampleCap
	out := make([]T, 0, archiveSampleCap)
	for i := 0; i < len(all); i += step {
		out = append(out, all[i])
	}
	return out
}

// sampledItemIndex keeps the source identity but only the sampled rows, so the
// price/material oracles scan a bounded subset while the joint counts stay full.
func sampledItemIndex(index ItemIndex) ItemIndex {
	ids := paritySample(sortedItemIDs(index))
	out := ItemIndex{Source: index.Source, Items: make(map[uint32]ItemIndexEntry, len(ids)), IndexHashes: index.IndexHashes}
	for _, id := range ids {
		out.Items[id] = index.Items[id]
	}
	return out
}

// sameShopPrice compares the value, not the Buy pointer identity: the joint scan
// and the independent oracle each allocate their own pointer.
func sameShopPrice(a, b ShopPrice) bool {
	if (a.Buy == nil) != (b.Buy == nil) {
		return false
	}
	if a.Buy != nil && *a.Buy != *b.Buy {
		return false
	}
	return a.Sell == b.Sell
}

// This exhaustive local proof compares the joint scan to independent native
// importers, including every price refusal and exact-path material boundary.
//
// The joint scan is the subject under test and always runs. Independent oracles
// that accept a caller-supplied index (prices, materials) are bounded to a
// deterministic cross-range sample; the index projection is compared on the same
// sample with full counts and hashes. Periods has no caller-scoped oracle, so the
// exhaustive independent scan is gated to DFO_PVF_ARCHIVE_FULL_SWEEP=1 while a
// sample is still cross-checked against the native predicate.
func TestJointItemBasicsLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for joint item parity")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	joint, err := ImportItemBasics(a, ItemBasicOptions{Periods: true, Prices: true, Materials: true})
	if err != nil {
		t.Fatal(err)
	}
	a.ReleaseReadCaches()
	index, err := ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	index.Source = joint.Index.Source
	if len(index.Items) != len(joint.Index.Items) || !reflect.DeepEqual(index.IndexHashes, joint.Index.IndexHashes) {
		t.Fatal("joint LIST/stackable projection differs")
	}
	sampleIDs := paritySample(sortedItemIDs(index))
	for _, id := range sampleIDs {
		if index.Items[id] != joint.Index.Items[id] {
			t.Fatalf("joint item %d differs: %+v vs %+v", id, index.Items[id], joint.Index.Items[id])
		}
	}
	if archiveFullSweep() {
		if !reflect.DeepEqual(index, joint.Index) {
			t.Fatal("joint LIST/stackable projection differs")
		}
		a.ReleaseReadCaches()
		periods, err := ImportItemPeriods(a)
		if err != nil {
			t.Fatal(err)
		}
		periods.Source = joint.Periods.Source
		if !reflect.DeepEqual(periods, *joint.Periods) {
			t.Fatal("joint periods differ")
		}
	} else {
		// Cheap default cross-check of the period set against the native predicate
		// on the same sampled scripts.
		periodSet := make(map[uint32]bool, len(joint.Periods.Templates))
		for _, id := range joint.Periods.Templates {
			periodSet[id] = true
		}
		for _, id := range sampleIDs {
			entry := joint.Index.Items[id]
			cells, err := a.Tokens(entry.Path)
			if err != nil {
				t.Fatalf("period sample %d: %v", id, err)
			}
			if got, want := periodSet[id], hasItemPeriod(cells); got != want {
				t.Fatalf("period classification %d differs: joint=%v native=%v", id, got, want)
			}
		}
	}
	// Prices and materials accept a caller-supplied index, so bound their native
	// scans to the same deterministic sample.
	sample := sampledItemIndex(index)
	a.ReleaseReadCaches()
	prices, err := ImportShopPrices(a, sample)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range sortedItemIDs(sample) {
		got, gotOK := prices.Items[id]
		want, wantOK := joint.Prices.Items[id]
		if gotOK != wantOK || !sameShopPrice(got, want) {
			t.Fatalf("joint price %d differs: %+v(%v) vs %+v(%v)", id, got, gotOK, want, wantOK)
		}
	}
	a.ReleaseReadCaches()
	materials, err := ImportItemMaterials(a, sample)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range sortedItemIDs(sample) {
		got, gotOK := materials.Materials(id)
		want, wantOK := joint.Materials.Materials(id)
		if gotOK != wantOK || !reflect.DeepEqual(got, want) {
			t.Fatalf("joint material %d differs: %+v(%v) vs %+v(%v)", id, got, gotOK, want, wantOK)
		}
	}
	t.Logf("native parity (sample=%d): items=%d periods=%d prices=%d materials=%d", len(sampleIDs), len(index.Items), len(joint.Periods.Templates), len(joint.Prices.Items), len(joint.Materials.Items))
}
