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
	present := false
	for _, cell := range script {
		if cell.Type == 3 && cell.Text == "[target character]" {
			present = true
			break
		}
	}
	if !present {
		return nil, true
	}
	cells := cells(script, "[target character]")
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

func targetCharacterAllowed(rows []targetCharacter, job string, advancement, awakening byte) bool {
	if len(rows) == 0 {
		return true
	}
	for _, row := range rows {
		if (row.job == "[all]" || row.job == job) &&
			(row.advancement == -1 || row.advancement == int32(advancement)) &&
			(row.awakening == -1 || row.awakening == int32(awakening)) {
			return true
		}
	}
	return false
}
