package main

import (
	"encoding/binary"
	"testing"
)

func TestParseAccountVaultGoldWithdraw(t *testing.T) {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint32(body, 5001765)
	amount, err := parseAccountVaultGoldWithdraw(body)
	if err != nil || amount != 5001765 {
		t.Fatalf("valid body: %d %v", amount, err)
	}
	if _, err = parseAccountVaultGoldWithdraw(make([]byte, 29)); err == nil {
		t.Fatal("accepted full frame length as body")
	}
	body[15] = 1
	if _, err = parseAccountVaultGoldWithdraw(body); err == nil {
		t.Fatal("accepted unknown reserved byte")
	}
}
