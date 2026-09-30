package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// NPCMove records the native [role] [move town] target from one .npc script.
// A role-data in-progress list is retained as an authorization condition.
type NPCMove struct {
	NPCID      uint32   `json:"npc_id"`
	TargetNPC  uint32   `json:"target_npc"`
	Quests     []uint32 `json:"in_progress_quests,omitempty"`
	ScriptPath string   `json:"script_path"`
	SHA256     string   `json:"sha256"`
}

type NPCMoveCatalog struct {
	SourceChecksum string    `json:"source_checksum"`
	Moves          []NPCMove `json:"moves"`
}

type NPCPlace struct{ Town, Area uint32 }

func (w *WorldCatalog) indexNPCTeleports() {
	w.NPCPlaces = make(map[uint32][]NPCPlace)
	w.EpisodeReturns = make(map[uint32]NPCPlace)
	for _, area := range w.Areas {
		place := NPCPlace{area.Town, area.Area}
		for _, script := range append([]ScriptRecord{area.Map}, area.ImportedScripts...) {
			for _, npc := range sourcePhaseNPCs(script) {
				w.addNPCPlace(npc.ID, place)
			}
		}
		for _, npc := range area.PhaseNPCs {
			w.addNPCPlace(npc.ID, place)
		}
	}
	// Areas is a map. Place membership has no priority; stabilize its slices
	// so repeated loads and source comparisons cannot depend on map iteration.
	for id, places := range w.NPCPlaces {
		sort.Slice(places, func(i, j int) bool {
			if places[i].Town != places[j].Town {
				return places[i].Town < places[j].Town
			}
			return places[i].Area < places[j].Area
		})
		w.NPCPlaces[id] = places
	}
	rows, err := ParseIndex(w.TownIndex.Cells)
	if err != nil || len(rows) != len(w.Towns) {
		return
	}
	for i, town := range w.Towns {
		var episode bool
		var returnNPC uint32
		for j, c := range town.Cells {
			if c.Type == 3 && c.Text == "[episode town]" {
				episode = true
			}
			if c.Type == 3 && c.Text == "[return npc]" && j+1 < len(town.Cells) && town.Cells[j+1].Type == 0 && town.Cells[j+1].Value > 0 {
				returnNPC = uint32(town.Cells[j+1].Value)
			}
		}
		if episode && returnNPC != 0 && len(w.NPCPlaces[returnNPC]) == 1 {
			w.EpisodeReturns[rows[i].ID] = w.NPCPlaces[returnNPC][0]
		}
	}
}

func (w *WorldCatalog) addNPCPlace(id uint32, place NPCPlace) {
	for _, existing := range w.NPCPlaces[id] {
		if existing == place {
			return
		}
	}
	w.NPCPlaces[id] = append(w.NPCPlaces[id], place)
}

func ImportNPCMoves(a *pvf.Archive) (NPCMoveCatalog, error) {
	out := NPCMoveCatalog{SourceChecksum: a.Snapshot().Checksum}
	index, err := ReadScript(a, "list/npc.lst")
	if err != nil {
		return out, err
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		rec, err := ResolveScript(a, row.Path)
		if err != nil {
			return out, fmt.Errorf("npc %d: %w", row.ID, err)
		}
		var target uint32
		inRole, inRoleData, inMoveData, inProgress := false, false, false, false
		hasMoveRoleData := false
		var quests []uint32
		for _, c := range rec.Cells {
			if c.Type == 3 || c.Type == 6 {
				switch c.Text {
				case "[role]":
					inRole = true
				case "[/role]":
					inRole = false
				case "[npc role data]":
					inRoleData = true
				case "[/npc role data]":
					inRoleData, inMoveData, inProgress = false, false, false
				case "[move town]":
					if inRoleData {
						inMoveData = true
						hasMoveRoleData = true
					}
				case "[in progress quest]":
					inProgress = inRoleData && inMoveData
				case "[/in progress quest]":
					inProgress = false
				default:
					if inRoleData && inMoveData && c.Text != "[move town]" {
						inMoveData = false
					}
				}
				continue
			}
			if inProgress && c.Type == 0 && c.Value > 0 {
				quests = append(quests, uint32(c.Value))
			}
		}
		for i, c := range rec.Cells {
			if c.Type == 3 && c.Text == "[role]" {
				inRole = true
				continue
			}
			if c.Type == 3 && c.Text == "[/role]" {
				inRole = false
				continue
			}
			if inRole && c.Type == 6 && c.Text == "[move town]" && i+1 < len(rec.Cells) && rec.Cells[i+1].Type == 0 && rec.Cells[i+1].Value > 0 {
				target = uint32(rec.Cells[i+1].Value)
				break
			}
		}
		if target != 0 && (!hasMoveRoleData || len(quests) != 0) {
			out.Moves = append(out.Moves, NPCMove{NPCID: row.ID, TargetNPC: target, Quests: quests, ScriptPath: rec.Path, SHA256: rec.SHA256})
		}
	}
	return out, nil
}

func loadNPCMoves(worldFile, checksum string) ([]NPCMove, error) {
	file := filepath.Join(filepath.Dir(worldFile), "npc-teleport.generated.json")
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c NPCMoveCatalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.SourceChecksum != checksum || len(c.Moves) == 0 {
		return nil, fmt.Errorf("npc teleport catalog source mismatch or empty")
	}
	return c.Moves, nil
}
