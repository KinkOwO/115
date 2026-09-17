package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	_ "embed"
	"encoding/json"
)

// Source COS hash d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e.
// Journal nodes are ordered. Every prior node must be complete, matching
// the client's 1402cd7a0 membership check, rather than a level-only unlock.
//
//go:embed odyssey_journal_routes.json
var odysseyJournalRoutes []byte

type odysseyJournalNode struct {
	Destination [4]uint32 `json:"destination"`
	Dungeons    []uint32  `json:"dungeons"`
}

func (s *ProgressionService) OdysseyJournalTeleport(role storage.Character, r protocol.AreaChangeRequest) bool {
	if s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != s.Odyssey.Source || r.Flag != 5 || r.TailFlags != [2]byte{} {
		return false
	}
	ids, err := s.odysseyCompleted(role)
	if err != nil {
		return false
	}
	completed := map[uint32]bool{}
	for _, id := range ids {
		completed[id] = true
	}
	var nodes []odysseyJournalNode
	if json.Unmarshal(odysseyJournalRoutes, &nodes) != nil {
		return false
	}
	target := [4]uint32{r.Town, r.Area, uint32(r.X), uint32(r.Y)}
	for _, node := range nodes {
		if node.Destination == target {
			return true
		}
		for _, id := range node.Dungeons {
			if !completed[id] {
				return false
			}
		}
	}
	return false
}
