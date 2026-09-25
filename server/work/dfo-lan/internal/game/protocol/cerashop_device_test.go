package protocol

import (
	"encoding/binary"
	"testing"
)

func TestCeraShopDeviceRequestsDoNotCaptureNoticeReports(t *testing.T) {
	shopOpen := make([]byte, 24)
	binary.LittleEndian.PutUint32(shopOpen, 0x27)
	shopDraw := make([]byte, 24)
	binary.LittleEndian.PutUint32(shopDraw[8:], 7)
	if !IsCeraShopDeviceAction(shopOpen) || !IsCeraShopDeviceAction(shopDraw) {
		t.Fatal("captured shop device actions were not recognized")
	}
	if IsCeraShopDeviceAction(shopOpen[:8]) {
		t.Fatal("short shop action was accepted")
	}
	for _, action := range []uint32{0x10, 0x39} {
		refresh := make([]byte, 8)
		binary.LittleEndian.PutUint32(refresh, action)
		binary.LittleEndian.PutUint32(refresh[4:], 1)
		if !IsCeraShopDeviceRefresh(refresh) {
			t.Fatalf("device refresh %#x was not recognized", action)
		}
		refresh[4] = 0
		if IsCeraShopDeviceRefresh(refresh) {
			t.Fatalf("notice-shaped CMD495 %#x was captured", action)
		}
	}
	if IsCeraShopDeviceRefresh([]byte{0x3e, 0, 0, 0, 1}) {
		t.Fatal("notice report was captured")
	}
}

func TestCeraShopDeviceStateFixedReader(t *testing.T) {
	p, err := CeraShopDeviceState(9, 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 0x88 || binary.LittleEndian.Uint32(p) != 9 || binary.LittleEndian.Uint32(p[4:]) != 99 {
		t.Fatalf("unexpected window state header: %x", p[:8])
	}
	for _, b := range p[8:] {
		if b != 0 {
			t.Fatal("unverified window state field was filled")
		}
	}
	if _, err := CeraShopDeviceState(10, 0); err == nil {
		t.Fatal("out-of-cycle counter was accepted")
	}
}
