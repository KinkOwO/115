package protocol

import (
	"bytes"
	"testing"
)

func TestTagEquipmentStoredUnsealRollNativeNestedList(t *testing.T) {
	// Observed bash slot14/template100060157: column block
	// 014a000010000017000002ff00000000. Native reader consumes triples.
	record := OrdinaryItem(14, 100060157, 0)
	record[76] = 9 // First field immediately after the nested list.
	item := DetailedWorn{Slot: 14, Template: 100060157, Durability: 60, Record: record[:]}
	empty, err := TagEquipment([]DetailedWorn{item})
	if err != nil {
		t.Fatal(err)
	}
	copy(record[60:76], []byte{1, 74, 0, 0, 16, 0, 0, 23, 0, 0, 2, 255, 0, 0, 0, 0})
	before := record
	got, err := TagEquipment([]DetailedWorn{item})
	if err != nil {
		t.Fatal(err)
	}
	// 1452C19F0 sits after row prefix, auxiliary count and u32 period.
	want := append(bytes.Clone(empty[:46]), 1, 74, 16, 23, 2, 255)
	want = append(want, empty[47:]...)
	if !bytes.Equal(got, want) || got[52] != 9 {
		t.Fatalf("nested-list vector/next-field cursor mismatch: %x", got[46:53])
	}
	if record != before {
		t.Fatal("encoding changed the saved roll")
	}
}

func TestTagRandomOptionsNativeCountsAndUnsupportedData(t *testing.T) {
	for _, tc := range []struct {
		block [16]byte
		want  []byte
	}{
		{want: []byte{0}},
		{block: [16]byte{11: 255}, want: []byte{0}},
		{block: [16]byte{2, 11, 12, 0, 21, 22, 0, 31, 32, 0, 7, 255}, want: []byte{2, 11, 21, 31, 12, 22, 32, 7, 255}},
		{block: [16]byte{3, 11, 12, 13, 21, 22, 23, 31, 32, 33, 7, 255}, want: []byte{3, 11, 21, 31, 12, 22, 32, 13, 23, 33, 7, 255}},
	} {
		var record [CurrentItemRecordSize]byte
		copy(record[60:76], tc.block[:])
		got, err := tagRandomOptions(record)
		if err != nil || !bytes.Equal(got, tc.want) {
			t.Fatalf("block %x: got %x, error %v", tc.block, got, err)
		}
	}
	for _, block := range [][16]byte{
		{4}, {0, 1}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
		{1, 74, 0, 0, 16, 0, 0, 23, 0, 0, 2, 0},
		{1, 74, 1, 0, 16, 0, 0, 23, 0, 0, 2, 255},
		{1, 74, 0, 0, 16, 0, 0, 23, 0, 0, 2, 255, 1},
	} {
		var record [CurrentItemRecordSize]byte
		copy(record[60:76], block[:])
		if _, err := tagRandomOptions(record); err == nil {
			t.Fatalf("unsupported data silently discarded: %x", block)
		}
	}
}
