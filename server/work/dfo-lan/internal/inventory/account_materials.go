package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Account material storage ("soul storage"): the 115 client pins seventeen
// fixed stackable templates to the account-shared list 35. The template to
// slot mapping below is verified against client/DFO.exe.i64:
//
//	sub_145ACA5E0 cube fragments -> 363+i (dword_14A9288E0)
//	sub_145AD74A0 souls          -> 369+i (unk_14A9288F8)
//	sub_145AD6B70 old souls      -> 375+i (dword_14A928910)
//
// and sub_14500CE40 routes every one of these templates to container 35.
const (
	AccountMaterialSpace    = 35
	AccountMaterialSlotBase = 363
)

var accountMaterialSlotByTemplate = map[uint32]uint16{
	3033: 363, 3034: 364, 3035: 365, 3036: 366, 3037: 367, 3262: 368,
	10100115: 369, 10100116: 370, 10099773: 371, 10099774: 372, 10099775: 373, 10158124: 374,
	10361512: 375, 10361513: 376, 10361514: 377, 10361515: 378, 10361516: 379,
}

var accountMaterialTemplateBySlot = func() map[uint16]uint32 {
	m := make(map[uint16]uint32, len(accountMaterialSlotByTemplate))
	for t, s := range accountMaterialSlotByTemplate {
		m[s] = t
	}
	return m
}()

// AccountMaterialSlot reports the fixed storage slot of one of the seventeen
// account-shared material templates.
func AccountMaterialSlot(template uint32) (uint16, bool) {
	s, ok := accountMaterialSlotByTemplate[template]
	return s, ok
}

// AccountMaterialDelta is one swept stack: the amount of a template that
// moved from a character bag into the account storage.
type AccountMaterialDelta struct {
	Slot     uint16 `json:"slot"`
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

// AccountMaterials is the account-scoped storage content: fixed slot ->
// stack count. It persists per account, not per character.
type AccountMaterials struct {
	Version string            `json:"version"`
	Counts  map[uint16]uint32 `json:"counts,omitempty"`
}

func NewAccountMaterials() AccountMaterials {
	return AccountMaterials{Version: "account-materials-v1"}
}

// ReadAccountMaterials tolerates an empty/absent document (fresh account).
func ReadAccountMaterials(raw json.RawMessage) (AccountMaterials, error) {
	m := NewAccountMaterials()
	if len(raw) == 0 || string(raw) == "null" {
		return m, nil
	}
	if e := json.Unmarshal(raw, &m); e != nil {
		return m, e
	}
	if m.Version != "account-materials-v1" {
		return m, fmt.Errorf("unsupported account material storage")
	}
	for slot, n := range m.Counts {
		if _, ok := accountMaterialTemplateBySlot[slot]; !ok {
			return m, fmt.Errorf("invalid account material slot")
		}
		if n == 0 {
			delete(m.Counts, slot)
		}
	}
	return m, nil
}

func (m AccountMaterials) Save() (json.RawMessage, error) {
	if m.Version != "account-materials-v1" {
		return nil, fmt.Errorf("unsupported account material storage")
	}
	return json.Marshal(m)
}

// Count reports the stored amount of one template.
func (m AccountMaterials) Count(template uint32) uint32 {
	slot, ok := accountMaterialSlotByTemplate[template]
	if !ok {
		return 0
	}
	return m.Counts[slot]
}

// Add stacks amount onto the fixed slot of a storage template.
func (m AccountMaterials) Add(template, amount uint32) (AccountMaterials, uint16, error) {
	slot, ok := accountMaterialSlotByTemplate[template]
	if !ok {
		return m, 0, fmt.Errorf("not an account material template")
	}
	if amount == 0 {
		return m, slot, fmt.Errorf("invalid account material amount")
	}
	out := AccountMaterials{Version: m.Version, Counts: map[uint16]uint32{}}
	for s, n := range m.Counts {
		out.Counts[s] = n
	}
	if uint64(out.Counts[slot])+uint64(amount) > math.MaxUint32 {
		return m, 0, fmt.Errorf("account material overflow")
	}
	out.Counts[slot] += amount
	return out, slot, nil
}

// Spend removes amount from the fixed slot of a storage template, the exact
// counterpart of Add. The 115 client consumes these materials straight out of
// the account-shared store when a skill costs one (a BOSS_CHECK-adjacent CMD 18
// carries the storage slot, e.g. 367 for 无色小晶块), so a stored count has to be
// able to go down as well as up.
func (m AccountMaterials) Spend(template, amount uint32) (AccountMaterials, uint16, error) {
	slot, ok := accountMaterialSlotByTemplate[template]
	if !ok {
		return m, 0, fmt.Errorf("not an account material template")
	}
	if amount == 0 {
		return m, slot, fmt.Errorf("invalid account material amount")
	}
	if m.Counts[slot] < amount {
		return m, slot, fmt.Errorf("insufficient account material")
	}
	out := AccountMaterials{Version: m.Version, Counts: map[uint16]uint32{}}
	for s, n := range m.Counts {
		out.Counts[s] = n
	}
	remaining := out.Counts[slot] - amount
	if remaining == 0 {
		// Rows() omits zero-count slots, so dropping it keeps the panel clean.
		delete(out.Counts, slot)
	} else {
		out.Counts[slot] = remaining
	}
	return out, slot, nil
}

// Rows renders the authoritative snapshot rows. Zero-count slots are
// omitted, matching the storage panel's empty-cell representation. A spend
// removes a slot when its count reaches zero.
func (m AccountMaterials) Rows() [][protocol.CurrentItemRecordSize]byte {
	slots := make([]uint16, 0, len(m.Counts))
	for slot, n := range m.Counts {
		if _, ok := accountMaterialTemplateBySlot[slot]; !ok || n == 0 {
			continue
		}
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(slots))
	for _, slot := range slots {
		rows = append(rows, protocol.OrdinaryItem(slot, accountMaterialTemplateBySlot[slot], m.Counts[slot]))
	}
	return rows
}

// SweepAccountMaterials moves every account-shared material stack out of the
// ordinary bag. Existing archives may legitimately hold them (for example
// disjoint clear cube fragments granted before the storage existed); sweeping
// at bootstrap migrates them without loss.
func SweepAccountMaterials(b Bag) (Bag, []AccountMaterialDelta, error) {
	totals := map[uint16]AccountMaterialDelta{}
	kept := make([]BagItem, 0, len(b.Items))
	for _, i := range b.Items {
		slot, ok := accountMaterialSlotByTemplate[i.Template]
		if !ok || i.Amount == 0 {
			kept = append(kept, i)
			continue
		}
		d := totals[slot]
		d.Slot, d.Template = slot, i.Template
		if uint64(d.Amount)+uint64(i.Amount) > math.MaxUint32 {
			return b, nil, fmt.Errorf("account material sweep overflow")
		}
		d.Amount += i.Amount
		totals[slot] = d
	}
	if len(totals) == 0 {
		return b, nil, nil
	}
	out := b
	out.Items = kept
	deltas := make([]AccountMaterialDelta, 0, len(totals))
	for _, d := range totals {
		deltas = append(deltas, d)
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].Slot < deltas[j].Slot })
	return out, deltas, nil
}

// ApplyDeltas stacks swept amounts onto the storage.
func (m AccountMaterials) ApplyDeltas(deltas []AccountMaterialDelta) (AccountMaterials, error) {
	out := m
	var e error
	for _, d := range deltas {
		if slot, ok := accountMaterialSlotByTemplate[d.Template]; !ok || slot != d.Slot {
			return m, fmt.Errorf("invalid account material delta")
		}
		if out, _, e = out.Add(d.Template, d.Amount); e != nil {
			return m, e
		}
	}
	return out, nil
}

// StorageRowTemplate resolves the fixed template rendered at a storage slot.
func StorageRowTemplate(slot uint16) (uint32, bool) {
	t, ok := accountMaterialTemplateBySlot[slot]
	return t, ok
}

// SweepDeltasRow renders sweep deltas as ordinary rows (diagnostics/tests).
func SweepDeltaRows(deltas []AccountMaterialDelta) [][protocol.CurrentItemRecordSize]byte {
	rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(deltas))
	for _, d := range deltas {
		rows = append(rows, protocol.OrdinaryItem(d.Slot, d.Template, d.Amount))
	}
	return rows
}
