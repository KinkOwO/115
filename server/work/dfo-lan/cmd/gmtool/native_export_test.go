package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeSlotExportPreservesOutputOnSourceFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "slots.json")
	before := []byte("existing artifact")
	if err := os.WriteFile(out, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildEquipmentSlots(out, filepath.Join(dir, "missing.pvf")); err == nil {
		t.Fatal("missing source accepted")
	}
	after, err := os.ReadFile(out)
	if err != nil || string(after) != string(before) {
		t.Fatal("existing output damaged", err)
	}
}
