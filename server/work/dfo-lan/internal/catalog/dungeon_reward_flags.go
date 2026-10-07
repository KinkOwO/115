package catalog

// These bare flags come from the active DGN script, including scripts that
// retain repeated legacy experience weights alongside [disable exp].
func (d DungeonDefinition) ExperienceDisabled() bool {
	return d.hasRewardFlag("[disable exp]")
}

func (d DungeonDefinition) ItemDropsDisabled() bool {
	return d.hasRewardFlag("[disable drop item]")
}

func (d DungeonDefinition) hasRewardFlag(name string) bool {
	for _, cell := range d.Script.Cells {
		if cell.Type == 3 && cell.Text == name {
			return true
		}
	}
	return false
}
