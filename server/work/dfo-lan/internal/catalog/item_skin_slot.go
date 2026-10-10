package catalog

// SkinSlotAction projects the single native action from a stackable script.
// It does not use the translated name or keep another list of ticket IDs.
func SkinSlotAction(s ScriptRecord) string {
	cells := sectionCells(s.Cells, "[action type]")
	if len(cells) != 1 || cells[0].Type != 6 {
		return ""
	}
	switch cells[0].Text {
	case "[open aura skin slot]", "[open creature skin slot]":
		return cells[0].Text
	}
	return ""
}
