package quest

import "dfolan/internal/catalog/pvf"

// A [target character] row is (profession, advancement branch, awakening
// stage). Rows are alternatives; -1 in either numeric column is a wildcard.
type targetCharacter struct {
	job         string
	advancement int32
	awakening   int32
}

func targetCharacters(script []pvf.Token) ([]targetCharacter, bool) {
	return targetCharacterRows(script, "[target character]")
}

// nonTargetCharacters reads every [non target character] section: the
// professions named there are excluded from the quest (Silent City faction
// lead-ins mark the [at swordman] alternate lineage this way, while their
// *_atS sibling quests name the same lineage under [target character]).
// Ignoring this reverse gate offered both spellings of one faction quest to
// the same character, which read in the quest book as duplicate entries.
func nonTargetCharacters(script []pvf.Token) ([]targetCharacter, bool) {
	return targetCharacterRows(script, "[non target character]")
}

func targetCharacterRows(script []pvf.Token, section string) ([]targetCharacter, bool) {
	present := false
	for _, cell := range script {
		if cell.Type == 3 && cell.Text == section {
			present = true
			break
		}
	}
	if !present {
		return nil, true
	}
	cells := cells(script, section)
	if len(cells) == 0 || len(cells)%3 != 0 {
		return nil, false
	}
	rows := make([]targetCharacter, 0, len(cells)/3)
	for i := 0; i < len(cells); i += 3 {
		if cells[i].Type != 6 || cells[i+1].Type != 0 || cells[i+2].Type != 0 ||
			cells[i+1].Value < -1 || cells[i+2].Value < -1 {
			return nil, false
		}
		rows = append(rows, targetCharacter{cells[i].Text, cells[i+1].Value, cells[i+2].Value})
	}
	return rows, true
}

func targetCharacterMatches(row targetCharacter, job string, advancement, awakening byte) bool {
	return (row.job == "[all]" || row.job == job) &&
		(row.advancement == -1 || row.advancement == int32(advancement)) &&
		(row.awakening == -1 || row.awakening == int32(awakening))
}

// targetCharacterAllowed applies both gates: a [non target character] row that
// matches the character excludes the quest outright, and (when present) at
// least one [target character] row must match. Quest sources use the two
// sections as the positive and negative faces of one profession filter; only
// the positive face was honoured until the Silent City faction duplicates.
func targetCharacterAllowed(targets, nonTargets []targetCharacter, job string, advancement, awakening byte) bool {
	for _, row := range nonTargets {
		if targetCharacterMatches(row, job, advancement, awakening) {
			return false
		}
	}
	if len(targets) == 0 {
		return true
	}
	for _, row := range targets {
		if targetCharacterMatches(row, job, advancement, awakening) {
			return true
		}
	}
	return false
}
