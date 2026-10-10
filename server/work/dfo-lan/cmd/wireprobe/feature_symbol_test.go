package main

import (
	"dfolan/internal/inventory"
	"testing"
)

func TestSymbolOptionDisplaySuppressesSourceFloat32Noise(t *testing.T) {
	value := float64(float32(0.1))
	text := symbolOptionText(inventory.SymbolOptions{Dealer: map[string]float64{"[equipment damage rate]": value}, Buffer: map[string]float64{"[equipment buff]": 10}})
	if text != [2]string{"攻击强化 +0.1%", "增益量 +10"} {
		t.Fatalf("display exposes float representation: %v", text)
	}
	text = symbolOptionText(inventory.SymbolOptions{Dealer: map[string]float64{"[skill bonus rate]": value * 6}})
	if text[0] != "技能攻击力 +0.6%" {
		t.Fatal(text)
	}
}
