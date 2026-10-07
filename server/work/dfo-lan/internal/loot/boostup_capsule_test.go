package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"testing"
)

func TestBoostCapsuleCandidatePreservesOwnerAndRequiresRealItem(t *testing.T) {
	ls := &Service{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{7: {ID: 7, Kind: "stackable", StackableType: "[waste]", StackLimit: 1}}}}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":1,"advancement":3,"keep":123}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 7, Amount: 1}}})
	if e != nil {
		t.Fatal(e)
	}
	role := Role{ID: 1, AccountID: 9, Profession: 0, State: raw}
	c := &boostup.Catalog{GoalLevel: 115, UsableLevel: 115, Capsules: map[uint32]boostup.Capsule{7: {Item: 7, Variant: 0}}, Steps: []boostup.Step{{Number: 1, Guide: "none", Mission: "equip item"}}}
	grow := func(r Role, target byte) (Role, error) {
		var f map[string]json.RawMessage
		_ = json.Unmarshal(r.State, &f)
		f["level"], _ = json.Marshal(target)
		r.State, _ = json.Marshal(f)
		return r, nil
	}
	c.ReservedMail = &boostup.GraduationMail{Item: 590015954, Count: 1, Sender: "Starter Boost", Body: "reserved"}
	c.ChallengeLevelRewards = map[byte]boostup.Reward{115: {Item: 590015933, Count: 1}}
	changed, receipt, e := ls.PrepareBoostCapsule(role, c, 65, 0, grow)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(changed)
	if e != nil || len(bag.Items) != 0 {
		t.Fatal("capsule not consumed", e)
	}
	st, e := boostup.ReadState(changed)
	if e != nil || !st.Activated || st.Training.Step != 1 || st.Training.Phase != 1 || receipt.BeforeLevel != 1 {
		t.Fatal("event did not begin", st, e)
	}
	c.ReservedMail.Item = 999
	if st.PendingMail == nil || st.PendingMail.Item != 590015954 || st.MailSent {
		t.Fatal("reserved mail not frozen with activation")
	}
	delete(c.ChallengeLevelRewards, 115)
	if st.PendingLevelBonus == nil || st.PendingLevelBonus.Item != 590015933 || st.LevelBonusSent {
		t.Fatal("level bonus not frozen separately")
	}
	original, _ := inventory.ReadBag(role.State)
	if len(original.Items) != 1 {
		t.Fatal("candidate mutated input")
	}
	if _, _, e = ls.PrepareBoostCapsule(role, c, 66, 0, grow); e == nil {
		t.Fatal("wrong slot accepted")
	}
	if _, _, e = ls.PrepareBoostCapsule(role, c, 65, 1, grow); e == nil {
		t.Fatal("variant/profession not checked")
	}
	if out, _, e := ls.PrepareBoostCapsule(role, c, 65, 0, func(Role, byte) (Role, error) {
		return Role{}, errors.New("growth failed")
	}); e == nil || out != nil {
		t.Fatal("failed growth consumed capsule")
	}
	var atCap map[string]json.RawMessage
	_ = json.Unmarshal(role.State, &atCap)
	atCap["level"] = json.RawMessage(`115`)
	maxRole := role
	maxRole.State, _ = json.Marshal(atCap)
	maxState, receipt, e := ls.PrepareBoostCapsule(maxRole, c, 65, 0, func(r Role, target byte) (Role, error) { return r, nil })
	if e != nil || receipt.BeforeLevel != 115 || receipt.Level != 115 {
		t.Fatal("at-cap character cannot start training", e)
	}
	maxEvent, e := boostup.ReadState(maxState)
	if e != nil || !maxEvent.Activated || maxEvent.Training.Step != 1 {
		t.Fatal(maxEvent, e)
	}
	atCap["level"] = json.RawMessage(`116`)
	maxRole.State, _ = json.Marshal(atCap)
	if _, _, e = ls.PrepareBoostCapsule(maxRole, c, 65, 0, grow); e == nil {
		t.Fatal("above cap accepted")
	}
	role.State = changed
	if _, _, e = ls.PrepareBoostCapsule(role, c, 65, 0, grow); e == nil {
		t.Fatal("activated role consumed again")
	}
}
