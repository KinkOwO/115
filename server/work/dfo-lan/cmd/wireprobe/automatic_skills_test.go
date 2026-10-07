package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/testfixture"
	"encoding/json"
	"testing"
)

func TestAutomaticSkillLevelRefresh(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.auto-skills-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := character.LoadLearningCatalog(testfixture.SkillCatalogPath(t, "release"), c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	service := &character.Service{Catalog: c, Learning: l}
	st := character.State{Level: 15, Advancement: 2, AllJobsPilot: true, SourceSHA256: c.Professions[11].RawSHA256, InitialSkills: c.Professions[11].InitialSkills}
	raw, _ := json.Marshal(st)
	w := worldSession{characters: service, role: database.Character{Profession: 11, ConfigVersion: c.Source.SaveIdentity(), State: raw}}
	plan, err := w.automaticSkillRefresh()
	if err != nil || len(plan) != 1 {
		t.Fatal(plan, err)
	}
	w.characters = nil
	plan, err = w.automaticSkillRefresh()
	if err != nil || len(plan) != 0 {
		t.Fatal("legacy world changed", err)
	}
}

// 二觉段的自授予同样按等级刷新：职业只带 awakening_skills（没有
// advancement_skills）时也必须发帧，否则 85 级才授予的二觉技能永远不会随升级
// 到达客户端。两个授予块都为空的旧目录（demonic swordman / creator mage 在本
// 目录里就是这种形状）保持不发帧。
func TestAutomaticSkillRefreshCoversAwakeningGrants(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := character.LoadLearningCatalog(testfixture.SkillCatalogPath(t, "next27"), c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	prof := c.Professions[0]
	if len(prof.AwakeningSkills) == 0 {
		t.Fatal("swordman carries no awakening grants")
	}
	prof.AdvancementSkills = nil
	c.Professions[0] = prof
	st := character.State{Level: 85, Advancement: 1, Awakening: 2, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills}
	raw, _ := json.Marshal(st)
	service := &character.Service{Catalog: c, Learning: l}
	w := worldSession{characters: service, role: database.Character{Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw}}
	plan, err := w.automaticSkillRefresh()
	if err != nil || len(plan) != 1 {
		t.Fatal("awakening-only profession did not refresh", plan, err)
	}
	w.characters.Catalog.Professions[0] = catalog.Profession{RawSHA256: prof.RawSHA256}
	plan, err = w.automaticSkillRefresh()
	if err != nil || len(plan) != 0 {
		t.Fatal("profession without grants changed", err)
	}
}
