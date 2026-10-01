package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"testing"
)

// This exhaustive local proof compares the joint scan to independent native
// importers, including every price refusal and exact-path material boundary.
func TestJointItemBasicsLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for joint item parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
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
	a.ReleaseReadCaches()
	prices, err := ImportShopPrices(a, index)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prices, joint.Prices) {
		t.Fatal("joint buy/sell prices or refusals differ")
	}
	a.ReleaseReadCaches()
	materials, err := ImportItemMaterials(a, index)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(materials, joint.Materials) {
		t.Fatal("joint material order/pairs/exact-path policy differs")
	}
	t.Logf("native parity: items=%d periods=%d prices=%d materials=%d", len(index.Items), len(periods.Templates), len(prices.Items), len(materials.Items))
}
