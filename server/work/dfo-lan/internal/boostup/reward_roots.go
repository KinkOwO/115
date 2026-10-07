package boostup

// Both variants and captured graduation mails are roots; not just auto-open
// steps9/11. Nested contents are resolved by the shared box importer.
func (c *Catalog) RewardRoots() []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	add := func(id uint32) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, s := range c.Steps {
		for _, r := range s.Rewards {
			add(r.Item)
		}
		for _, r := range s.BufferRewards {
			add(r.Item)
		}
	}
	if c.ReservedMail != nil {
		add(c.ReservedMail.Item)
	}
	for _, r := range c.ChallengeLevelRewards {
		add(r.Item)
	}
	return out
}
