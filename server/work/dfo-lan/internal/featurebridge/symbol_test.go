package featurebridge

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestSymbolRequestRejectsTruncationAndTrailingBytes(t *testing.T) {
	b := new(bytes.Buffer)
	put := func(v any) {
		if e := binary.Write(b, binary.LittleEndian, v); e != nil {
			t.Fatal(e)
		}
	}
	put(uint64(2))
	put(uint64(7))
	b.WriteString(strings.Repeat("a", 64))
	put(uint16(10))
	b.WriteString("red_symbol")
	put(uint16(3))
	put(uint32(590004573))
	put(uint16(289))
	put(uint32(590004579))
	p := b.Bytes()
	r, e := DecodeSymbolRequest(p)
	if e != nil || r.Role != 2 || r.Sequence != 7 || r.Group != "red_symbol" || r.Socket != 3 || r.BagSlot != 289 || r.Template != 590004579 || r.ExpectedTemplate != 590004573 {
		t.Fatalf("%+v: %v", r, e)
	}
	for n := 0; n < len(p); n++ {
		if _, e := DecodeSymbolRequest(p[:n]); e == nil {
			t.Fatalf("accepted truncation %d", n)
		}
	}
	if _, e := DecodeSymbolRequest(append(append([]byte{}, p...), 0)); e == nil {
		t.Fatal("accepted trailing byte")
	}
	for _, offset := range []int{0, 16, 82} {
		bad := append([]byte{}, p...)
		if offset == 0 {
			clear(bad[:8])
		} else {
			bad[offset] = 0xff
		}
		if _, e := DecodeSymbolRequest(bad); e == nil {
			t.Fatalf("accepted corrupted identity at %d", offset)
		}
	}
}
func TestSymbolViewBoundsAndFrame(t *testing.T) {
	v := SymbolView{Role: 2, Source: strings.Repeat("a", 64), Groups: []SymbolGroup{{Name: "red_symbol", Mask: 1, Items: []SymbolItem{{Template: 590004573, Flags: 1}, {}}}}, Bag: []SymbolBagItem{{Slot: 289, Mask: 1, Template: 590004579, Amount: 9}}}
	p, e := v.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 140 {
		t.Fatalf("wire size %d", len(p))
	}
	for _, kind := range []uint16{SymbolSnapshot, SymbolExchange, SymbolRefresh, SymbolSnapshot | Response, SymbolExchange | Response, SymbolRefresh | Response} {
		frame, e := (Frame{Kind: kind, ID: 1, Body: p}).Bytes()
		if e != nil {
			t.Fatal(e)
		}
		r, e := Read(bytes.NewReader(frame))
		if e != nil || !bytes.Equal(r.Body, p) {
			t.Fatalf("%+v %v", r, e)
		}
	}
	for _, kind := range []uint16{0, 6, 0x1003, 0x8006} {
		if _, e := (Frame{Kind: kind, ID: 1}).Bytes(); e == nil {
			t.Fatalf("accepted unknown kind %x", kind)
		}
	}
	v.Bag = append(v.Bag, v.Bag[0])
	if _, e := v.Bytes(); e == nil {
		t.Fatal("accepted duplicate slot")
	}
	v.Bag = v.Bag[:1]
	v.Groups = append(v.Groups, v.Groups[0])
	if _, e := v.Bytes(); e == nil {
		t.Fatal("accepted duplicate group")
	}
}

func TestSymbolAttributeTrailerIsBoundedUTF8(t *testing.T) {
	v := SymbolView{Role: 2, Source: strings.Repeat("a", 64), Groups: []SymbolGroup{{Name: "red_symbol", Mask: 1, Items: []SymbolItem{{}}, Attributes: "力量 +60"}}, Attributes: "力量 +60", BuffAttributes: "无属性加成"}
	p, e := v.Bytes()
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(p, []byte("SAT1")) || !bytes.Contains(p, []byte("力量 +60")) {
		t.Fatal("missing attributes")
	}
	for _, bad := range []string{strings.Repeat("x", 4097), "a\x00b", string([]byte{255})} {
		v.Attributes = bad
		if _, e := v.Bytes(); e == nil {
			t.Fatal("accepted invalid text")
		}
	}
}

func TestSymbolStarTrailerRequiresBoundedProgress(t *testing.T) {
	v := SymbolView{Role: 2, Source: strings.Repeat("a", 64), Groups: []SymbolGroup{{Name: "red_symbol", Mask: 1, Items: []SymbolItem{{}}, Star: &SymbolStar{Grade: 0, Level: 1, Fixed: [2]string{"攻击强化 +0.1%", "增益量 +10"}}}}, Powers: [][2]uint32{{1986000001, 1986000002}}}
	p, err := v.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(p, []byte("SAT1")) || !bytes.Contains(p, []byte("SSR1")) || !bytes.Contains(p, []byte("攻击强化 +0.1%")) {
		t.Fatal("missing source option trailer")
	}
	for _, bad := range []SymbolStar{{Grade: 4, Level: 1}, {Grade: 0, Level: 0}, {Grade: 0, Level: 6}} {
		v.Groups[0].Star = &bad
		if _, err := v.Bytes(); err == nil {
			t.Fatalf("accepted invalid star: %+v", bad)
		}
	}
}
