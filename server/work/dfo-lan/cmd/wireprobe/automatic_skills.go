package main

func (w *worldSession) automaticSkillRefresh() ([]outboundPacket, error) {
	if w.characters == nil || w.characters.Learning == nil || len(w.characters.Catalog.Professions[w.role.Profession].AdvancementSkills) == 0 {
		return nil, nil
	}
	p, err := w.characters.EntrySkills(w.role)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"automatic_skills_updated", 0, 19, p}}, nil
}
