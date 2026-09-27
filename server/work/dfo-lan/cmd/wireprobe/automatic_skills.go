package main

// automaticSkillRefresh re-sends the skill rows after the character's level
// changes. The source grants behind them are level-gated: an advancement's
// starter rows, and the awakening rows whose own [required level] can sit
// above the awakening threshold (the second awakening unlocks at 75 while part
// of its block is 85-level). Either block is enough reason to refresh, so a
// profession carrying only awakening grants still gets the frame.
func (w *worldSession) automaticSkillRefresh() ([]outboundPacket, error) {
	if w.characters == nil || w.characters.Learning == nil {
		return nil, nil
	}
	prof := w.characters.Catalog.Professions[w.role.Profession]
	if len(prof.AdvancementSkills) == 0 && len(prof.AwakeningSkills) == 0 {
		return nil, nil
	}
	p, err := w.characters.EntrySkills(w.role)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"automatic_skills_updated", 0, 19, p}}
	return appendSkillPresetRestore(plan, w.characters, w.role, "skill_preset_restored_after_automatic_skills")
}
