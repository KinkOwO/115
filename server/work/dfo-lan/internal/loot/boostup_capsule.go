package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

type BoostCapsuleReceipt struct {
	Slot               uint16
	Item, Variant      uint32
	BeforeLevel, Level byte
}

// Class/level reads happen under the same row lock as consumption, not from
// the connection's potentially stale copy. The callback uses ordinary growth.
// grow/preflight 的存储编排与提交在 workflow.LootService.UseBoostCapsule
// （§7.2 E13）；这里只做可回滚的纯状态计算。
func (s *Service) PrepareBoostCapsule(role Role, c *boostup.Catalog, slot uint16, variant uint32, grow func(Role, byte) (Role, error)) (json.RawMessage, BoostCapsuleReceipt, error) {
	var receipt BoostCapsuleReceipt
	if c == nil || grow == nil || variant > 1 {
		return nil, receipt, fmt.Errorf("capsule source unavailable")
	}
	state, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, receipt, e
	}
	if state.Activated {
		return nil, receipt, fmt.Errorf("character already used event capsule")
	}
	var base struct {
		Level       byte `json:"level"`
		Advancement byte `json:"advancement"`
	}
	if e = json.Unmarshal(role.State, &base); e != nil {
		return nil, receipt, e
	}
	// The cap is inclusive: a level115 character may still enter the
	// unplayed teaching course. Never downgrade an above-cap character.
	if base.Level == 0 || base.Level > c.UsableLevel || base.Level > c.GoalLevel {
		return nil, receipt, fmt.Errorf("capsule source level limit")
	}
	if variant == 1 && !c.BufferEligible(role.Profession, base.Advancement) {
		return nil, receipt, fmt.Errorf("buffer capsule profession mismatch")
	}
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, receipt, e
	}
	var id uint32
	for _, row := range bag.Items {
		if row.Slot == slot && row.Amount > 0 {
			id = row.Template
			break
		}
	}
	def, ok := c.Capsules[id]
	if !ok || def.Variant != variant {
		return nil, receipt, fmt.Errorf("slot is not the selected source capsule")
	}
	bag, _, e = bag.Consume(s.Catalog, slot, id)
	if e != nil {
		return nil, receipt, e
	}
	next, e := grow(role, c.GoalLevel)
	if e != nil {
		return nil, receipt, e
	}
	if next.ID != role.ID || next.AccountID != role.AccountID || next.Profession != role.Profession {
		return nil, receipt, fmt.Errorf("capsule changed owner/profession")
	}
	var changed struct {
		Level byte `json:"level"`
	}
	if e = json.Unmarshal(next.State, &changed); e != nil || changed.Level != c.GoalLevel {
		return nil, receipt, fmt.Errorf("capsule growth target not reached")
	}
	next.State, e = inventory.SaveBag(next.State, bag)
	if e != nil {
		return nil, receipt, e
	}
	state.Activated = true
	if c.ReservedMail != nil && !state.MailSent {
		intent := *c.ReservedMail
		state.PendingMail = &intent
	}
	if !state.LevelBonusSent {
		state.PendingLevelBonus = c.CapsuleLevelMail(changed.Level)
	}
	state.Variant = variant
	state.Training, e = c.Begin()
	if e != nil {
		return nil, receipt, e
	}
	next.State, e = boostup.WriteState(next.State, state)
	receipt = BoostCapsuleReceipt{slot, id, variant, base.Level, changed.Level}
	return next.State, receipt, e
}
