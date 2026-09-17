package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// NOTI53: native 1452feaf0 reads u8 status, i32 balance, u32 shop state.
// The native balance is signed and negative values are reset to zero.
func CeraBalance(balance uint64) ([]byte, error) {
	if balance > math.MaxInt32 {
		return nil, fmt.Errorf("CERA balance exceeds native signed range")
	}
	p := add32([]byte{1}, uint32(balance))
	// 1452feb54 stores this at shop+0x2fad4. Predicate1446ec740
	// returns true for0, which routes purchases to the privilege/web-store
	// notice before CMD64 is sent. The native constructor defaults to1.
	return add32(p, 1), nil
}

// CeraPurchaseCancelled closes the native buy-in-progress state without a
// success acknowledgement. Handler145267d40 first clears pending purchases,
// then unconditionally reads u8+5*u32 even when dispatch error code is zero.
// Code0 suppresses a guessed error message and skips product-specific handling.
// This response grants nothing and must never follow a committed purchase.
func CeraPurchaseCancelled() []byte {
	return append(Refusal(0), make([]byte, 21)...)
}

// CeraPurchasePilotSuccess is the current-build ordinary single-item layout.
// Category -1 searches all25 categories in144700530. Equal batch markers
// require the extra two words; -1 suppresses the optional reward update.
// The final quantity drives140887530's pending-cart decrement. Neutral fields
// are restricted to the non-gift, non-avatar pilot and still need live UI QA.
func CeraPurchasePilotSuccess(product uint32) ([]byte, error) {
	return CeraPurchaseOrdinarySuccess(product, 1)
}

func CeraPurchaseOrdinarySuccess(product, quantity uint32) ([]byte, error) {
	// Authorization belongs to the PVF catalog and delivery handler, not the
	// byte encoder. A new ordinary SKU uses the same native packet layout.
	if product == 0 || product > math.MaxInt32 || quantity == 0 || quantity > 56 {
		return nil, fmt.Errorf("invalid ordinary purchase response")
	}
	p := []byte{1, 0}
	for _, v := range []uint32{math.MaxUint32, product, 0, 0, 0} {
		p = add32(p, v)
	}
	p = add16(p, 0)
	for _, v := range []uint32{math.MaxUint32, 0, 0, 0, 0, quantity} {
		p = add32(p, v)
	}
	return append(p, 0), nil
}

type CeraCartItem struct {
	Option   byte   `json:"option"`
	Kind     byte   `json:"kind"`
	Product  uint32 `json:"product"`
	Quantity uint32 `json:"quantity"`
}

// DecodeCeraCart covers the ordinary nongift path 140889a1f -> 14088dff0.
// Gift strings and avatar customizations need separate layouts.
func DecodeCeraCart(p []byte) ([]CeraCartItem, error) {
	if len(p) < 3 || p[0] != 0 || p[1] != 0 {
		return nil, fmt.Errorf("unsupported CERA purchase variant")
	}
	count := int(p[2])
	end := 3 + count*12
	if count == 0 || count > 32 || len(p) < end {
		return nil, fmt.Errorf("invalid CERA cart count or length")
	}
	if err := padding(p[end:], 16); err != nil {
		return nil, err
	}
	items := make([]CeraCartItem, 0, count)
	for i := 0; i < count; i++ {
		at := 3 + i*12
		if p[at+10] != 0 || p[at+11] != 0 {
			return nil, fmt.Errorf("customized CERA item needs its native layout")
		}
		item := CeraCartItem{p[at], p[at+1], binary.LittleEndian.Uint32(p[at+2:]), binary.LittleEndian.Uint32(p[at+6:])}
		if item.Product == 0 || item.Quantity == 0 {
			return nil, fmt.Errorf("empty CERA product or quantity")
		}
		items = append(items, item)
	}
	return items, nil
}
