package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"testing"
)

func TestVaultMoveNonVaultRequest(t *testing.T) {
	w := &worldSession{}
	rules := inventory.BagRules{}
	r := protocol.ItemMoveRequest{SourceList: 0, DestinationList: 0}
	plan, handled, err := w.moveVault(rules, r)
	if handled {
		t.Fatal("expected handled=false for non-vault move")
	}
	if plan != nil || err != nil {
		t.Fatalf("unexpected plan or error: plan=%v, err=%v", plan, err)
	}
}

func TestVaultMoveUninitializedSession(t *testing.T) {
	w := &worldSession{}
	rules := inventory.BagRules{}
	r := protocol.ItemMoveRequest{SourceList: 0, DestinationList: 2}
	_, handled, err := w.moveVault(rules, r)
	if !handled {
		t.Fatal("expected handled=true for vault move")
	}
	if err == nil {
		t.Fatal("expected error for uninitialized character/vault")
	}
}
