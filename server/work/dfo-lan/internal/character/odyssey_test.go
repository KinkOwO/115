package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func odysseyGrowthFixture(t *testing.T) (*ProgressionService, storage.Character) {
	t.Helper()
	o, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadCharacters("../../configs/characters.alljobs-pilot.json")
	if e != nil {
		t.Fatal(e)
	}
	p, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	prof := c.Professions[0]
	state := State{Level: 1, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourceSHA256: prof.RawSHA256}
	raw, _ := json.Marshal(state)
	r := storage.Character{Profession: 0, Name: "GrowTest", WireID: 1, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	r.Request = req
	return &ProgressionService{Catalog: p, Professions: c, Rules: progression.Rules{LevelCap: 115}, Odyssey: o}, r
}

func TestOdysseyGiftCapacityAndSource(t *testing.T) {
	s, r := odysseyGrowthFixture(t)
	if s.Odyssey.ClearLevels[100004934] != 15 || s.Odyssey.EntryLevels[100004935] != 15 {
		t.Fatal("source route changed")
	}
	normal := r
	normal.Request = nil
	if _, e := s.ApplyOdysseyTarget(normal, 15); e == nil {
		t.Fatal("ordinary role admitted")
	}
	c := OdysseyGiftCatalog(s.Odyssey)
	if len(c.Items) != 3 {
		t.Fatal("gift pool widened")
	}
	b := inventory.Bag{Version: "ordinary-bag-v1"}
	rules := inventory.BagRules{Source: c.Source.Checksum, Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1}
	b, slot, e := b.Add(c, rules, 10419348, 1)
	if e != nil || slot != 65 {
		t.Fatal(slot, e)
	}
	for n := uint16(66); n <= 120; n++ {
		b.Items = append(b.Items, inventory.BagItem{Slot: n, Template: 10419348, Amount: 1})
	}
	if _, _, e = b.Add(c, rules, 10419349, 1); e == nil {
		t.Fatal("full bag accepted")
	}
}

func TestOdysseySourceTargetGrowthPreservesState(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	var fields map[string]json.RawMessage
	json.Unmarshal(role.State, &fields)
	fields["untouched_marker"] = json.RawMessage(`{"value":42}`)
	role.State, _ = json.Marshal(fields)
	next, e := s.ApplyOdysseyTarget(role, 15)
	if e != nil {
		t.Fatal(e)
	}
	var before, after State
	json.Unmarshal(role.State, &before)
	json.Unmarshal(next.State, &after)
	if after.Level != 15 || after.Experience != s.Catalog.Thresholds[13] || after.Attributes["[hp max]"] <= before.Attributes["[hp max]"] || after.SkillPoints[0] <= before.SkillPoints[0] {
		t.Fatal("missing growth", after.Level, after.SkillPoints)
	}
	json.Unmarshal(next.State, &fields)
	if string(fields["untouched_marker"]) != `{"value":42}` {
		t.Fatal("custom field lost")
	}
	retry, e := s.ApplyOdysseyTarget(next, 15)
	if e != nil || !bytes.Equal(next.State, retry.State) {
		t.Fatal("replay awarded again", e)
	}
	for _, level := range []byte{25, 60, 65, 90, 100, 115} {
		next, e = s.ApplyOdysseyTarget(next, level)
		if e != nil {
			t.Fatalf("target %d: %v", level, e)
		}
	}
	retry, e = s.ApplyOdysseyTarget(next, 15)
	if e != nil || !bytes.Equal(next.State, retry.State) {
		t.Fatal("old dungeon downgraded role", e)
	}
	for level, id := range s.Odyssey.Gifts {
		if _, _, e := s.ApplyOdysseyGift(next, level, id); e != nil {
			t.Fatal(level, id, e)
		}
	}
	if _, _, e := s.ApplyOdysseyGift(role, 60, 10419348); e == nil {
		t.Fatal("early gift")
	}
}

func TestOdysseyCatchupOnlyFromPersistedClear(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	if target, e := s.odysseyRecordedTarget(role); e != nil || target != 0 {
		t.Fatal(target, e)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(role.State, &fields)
	fields["dungeon_best_times"] = json.RawMessage(`{"100004934:normal:solo":35000,"3:normal:solo":100}`)
	role.State, _ = json.Marshal(fields)
	if target, e := s.odysseyRecordedTarget(role); e != nil || target != 15 {
		t.Fatal(target, e)
	}
	role.Request = nil
	if target, e := s.odysseyRecordedTarget(role); e != nil || target != 0 {
		t.Fatal("ordinary catchup", target, e)
	}
}

func TestCreatedAsOdysseyIgnoresLauncherMode(t *testing.T) {
	_, role := odysseyGrowthFixture(t)
	if !CreatedAsOdyssey(role) || !OdysseyRole(role) {
		t.Fatal("creation marker lost")
	}
	// 启动器的全局开关只应影响 OdysseyRole 这类内容开关，不得改变角色自身的
	// 创建标记：城镇准入必须与客户端一致（客户端按角色标记判定）。
	t.Setenv("DFO_ODYSSEY_MODE", "0")
	if OdysseyRole(role) {
		t.Fatal("debug override ignored")
	}
	if !CreatedAsOdyssey(role) {
		t.Fatal("override changed the per-character marker")
	}
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	normal := role
	normal.Request = nil
	if !OdysseyRole(normal) || CreatedAsOdyssey(normal) {
		t.Fatal("override must stay global and leave the marker alone")
	}
}
