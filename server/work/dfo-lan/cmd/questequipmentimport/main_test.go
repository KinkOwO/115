package main

import (
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateArgumentsRequiresOutputAndRejectsBase(t *testing.T) {
	if err := validateArguments("", ""); err == nil {
		t.Fatal("missing output accepted")
	}
	if err := validateArguments("old.json", "out.json"); err == nil {
		t.Fatal("legacy JSON seed accepted")
	}
	if err := validateArguments("", "out.json"); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCatalogRequiresExplicitOutputPath(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "selection.json")
	want := &inventory.EquipmentCatalog{Rows: []inventory.EquipmentDefinition{{ID: 7, Path: "equipment/test.equ"}}}
	if err := writeCatalog(want, output); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var got inventory.EquipmentCatalog
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0].ID != 7 || got.Rows[0].Path != "equipment/test.equ" {
		t.Fatalf("unexpected exported selection: %#v", got.Rows)
	}
}
