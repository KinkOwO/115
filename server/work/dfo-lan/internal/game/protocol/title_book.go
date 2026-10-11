package protocol

import (
	"encoding/binary"
	"fmt"
)

// Current 2.38.2.34: sender 145AE7C0D writes five u32s for CMD404/405;
// reply readers 145298DD0/145298940 consume four u32s after success.
// Native CMD404 capture: 20261006_002853, slot51, template786000583,
// category4, bookslot10. The remaining bytes are transport zero padding.
type TitleBookTransfer struct{ Space, Slot, Template, Category, BookSlot uint32 }

func DecodeTitleBookTransfer(p []byte) (TitleBookTransfer, error) {
	var r TitleBookTransfer
	if len(p) < 20 || !zeroTail(p[20:]) {
		return r, fmt.Errorf("invalid title book transfer length/padding")
	}
	r = TitleBookTransfer{binary.LittleEndian.Uint32(p), binary.LittleEndian.Uint32(p[4:]), binary.LittleEndian.Uint32(p[8:]), binary.LittleEndian.Uint32(p[12:]), binary.LittleEndian.Uint32(p[16:])}
	if (r.Space != 0 && r.Space != 3) || r.Slot > 65534 || r.Category >= 5 || r.BookSlot > 65534 || r.Template == 0 {
		return r, fmt.Errorf("invalid title book reference")
	}
	return r, nil
}
func zeroTail(p []byte) bool {
	for _, v := range p {
		if v != 0 {
			return false
		}
	}
	return true
}
func TitleBookTransferSuccess(r TitleBookTransfer) []byte {
	return add32(add32(add32(add32([]byte{1}, r.Space), r.Slot), r.Category), r.BookSlot)
}

// NOTI344 reader 1452E8C20: flag/u16 actor/u32 category/u32 count;
// a compact row has31 fixed bytes plus a length-delimited random-options blob.
// It is NOT the181-byte ordinary inventory row.
type TitleBookRow struct {
	Slot                        uint16
	Instance, Template          uint32
	Reinforcement               byte
	Durability                  uint16
	Sealed                      byte
	Enchant                     uint32
	EnchantUpgrade, AmplifyType byte
	AmplifyValue                uint16
	RandomOptions               []byte
	Clone                       uint32
	Flag                        byte
}

func TitleBookList(category uint32, rows []TitleBookRow) ([]byte, error) {
	if category >= 5 || len(rows) > 65535 {
		return nil, fmt.Errorf("invalid title book category/count")
	}
	p := add32(add32(add16([]byte{0}, 0), category), uint32(len(rows)))
	seen := map[uint16]bool{}
	for _, r := range rows {
		if r.Template == 0 || seen[r.Slot] || len(r.RandomOptions) > 4 {
			return nil, fmt.Errorf("invalid title book row")
		}
		seen[r.Slot] = true
		// 1452E8D9D reads the template; 1452E8DA7 reads item data.
		// The first value reaches the PVF lookup through 14603E2A0.
		p = add32(add32(add16(p, r.Slot), r.Template), r.Instance)
		p = append(add16(append(p, r.Reinforcement), r.Durability), r.Sealed)
		p = append(add32(p, r.Enchant), r.EnchantUpgrade, r.AmplifyType)
		p = add32(add16(p, r.AmplifyValue), uint32(len(r.RandomOptions)))
		p = append(p, r.RandomOptions...)
		p = append(add32(p, r.Clone), r.Flag)
	}
	return p, nil
}

// CMD560 sender144255670 (select) /144255910 (clear), reader144249220.
func DecodeTitleAppearance(p []byte) (uint32, error) {
	if len(p) < 4 || !zeroTail(p[4:]) {
		return 0, fmt.Errorf("invalid clone title length/padding")
	}
	return binary.LittleEndian.Uint32(p), nil
}
func TitleAppearanceSuccess(template uint32) []byte { return add32([]byte{1}, template) }
