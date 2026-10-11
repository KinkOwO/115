package character

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"testing"
)

func TestTruthFameFromCurrentSourceAndCoreIndependence(t *testing.T) {
	filename := os.Getenv("DFO_TRUTH_TEST_ARCHIVE")
	if filename == "" {
		t.Skip("set DFO_TRUTH_TEST_ARCHIVE for current source audit")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: filename, MaxBytes: 1024 * 1024 * 1024}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := NewFameItemImport(a)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := InstallFameRules(b.rules)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	svc := &Service{Equipment: &inventory.EquipmentCatalog{Source: a.Snapshot()}}
	for level := byte(0); level <= 4; level++ {
		raw, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err = inventory.SaveOathTruth(raw, inventory.OathTruthState{Level: level})
		if err != nil {
			t.Fatal(err)
		}
		fame, err := svc.EquipmentFameBreakdown(raw)
		if err != nil || fame.Total != uint32(level)*400 {
			t.Fatal("truth source fame without equipped core", level, fame, err)
		}
	}
}
