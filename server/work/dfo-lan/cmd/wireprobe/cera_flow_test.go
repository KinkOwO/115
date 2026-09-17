package main

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

type ceraFake struct {
	balance uint64
	calls   int
	account int64
	err     error
}

func (f *ceraFake) AccountCera(_ context.Context, a int64) (uint64, error) {
	f.calls++
	f.account = a
	return f.balance, f.err
}
func TestCeraQueryReadsLiveLedger(t *testing.T) {
	s := &ceraFake{balance: 100}
	ctx := context.Background()
	a, e := ceraQuery(ctx, s, 7, nil)
	if e != nil || binary.LittleEndian.Uint32(a[1:5]) != 100 {
		t.Fatal(e)
	}
	s.balance = 500
	b, e := ceraQuery(ctx, s, 7, nil)
	if e != nil || binary.LittleEndian.Uint32(b[1:5]) != 500 || s.account != 7 || s.calls != 2 {
		t.Fatal("stale balance")
	}
	if _, e = ceraQuery(ctx, s, 7, []byte{1}); e == nil || s.calls != 2 {
		t.Fatal("malformed query read ledger")
	}
	s.err = errors.New("offline")
	if _, e = ceraQuery(ctx, s, 7, nil); e == nil {
		t.Fatal("database failure hidden")
	}
	for i := 0; i < BodySampleLimit+2; i++ {
		if !retainRequestBody(63, map[uint16]int{63: 100}) {
			t.Fatal("query lost after cap")
		}
	}
}

func TestCeraPurchaseAlwaysRetainsBody(t *testing.T) {
	seen := map[uint16]int{64: BodySampleLimit}
	for i := 0; i < 32; i++ {
		if !retainRequestBody(64, seen) {
			t.Fatalf("purchase %d discarded after diagnostic sample cap", i+1)
		}
	}
	if retainRequestBody(2127, map[uint16]int{2127: BodySampleLimit}) {
		t.Fatal("unrelated telemetry cap was disabled")
	}
}
