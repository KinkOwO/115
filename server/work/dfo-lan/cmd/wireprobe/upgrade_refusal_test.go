package main

import (
	"dfolan/internal/inventory"
	"errors"
	"fmt"
	"testing"
)

func TestUpgradeRefusalCodesIgnoreMessageAndWrapping(t *testing.T) {
	for _, tc := range []struct {
		kind              inventory.RefusalKind
		reinforce, refine uint16
	}{
		{inventory.RefusalGeneric, 9000, 9000},
		{inventory.RefusalGold, 10, 9000},
		{inventory.RefusalMaterials, 22, 22},
		{inventory.RefusalLimit, 13, 17},
		{inventory.RefusalEquipment, 17, 23},
		{inventory.RefusalUnsupported, 23, 9000},
		{inventory.RefusalItems, 4, 4},
	} {
		// Deliberately misleading text must not override the actual failure kind.
		err := fmt.Errorf("operation: %w", inventory.Refuse(tc.kind, "金币不足 / 上限 / not in bag"))
		if got := reinforcementRefusalCode(err); got != tc.reinforce {
			t.Errorf("reinforce kind %d: %d, want %d", tc.kind, got, tc.reinforce)
		}
		if got := refineRefusalCode(err); got != tc.refine {
			t.Errorf("refine kind %d: %d, want %d", tc.kind, got, tc.refine)
		}
	}
	for _, err := range []error{nil, errors.New("金币不足")} {
		if reinforcementRefusalCode(err) != 9000 || refineRefusalCode(err) != 9000 {
			t.Fatal("unclassified failures must use the diagnostic fallback")
		}
	}
}

func TestEnchantRefusalCodeIgnoresMessage(t *testing.T) {
	for _, tc := range []struct {
		kind inventory.RefusalKind
		want uint16
	}{
		{inventory.RefusalItems, cmd272ErrItem},
		{inventory.RefusalEquipment, cmd272ErrEquipment},
		{inventory.RefusalGeneric, cmd272ErrGeneric},
	} {
		err := fmt.Errorf("wrapped: %w", inventory.Refuse(tc.kind, "宝珠不在背包，目标不是装备"))
		if got := enchantRefusalCode(err); got != tc.want {
			t.Errorf("kind %d: %d, want %d", tc.kind, got, tc.want)
		}
	}
}
