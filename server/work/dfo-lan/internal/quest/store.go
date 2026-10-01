package quest

import (
	"context"
	"dfolan/internal/character"
	"encoding/json"
)

// QuestState is the durable objective state owned by the quest domain.
type QuestState struct {
	ID            uint16 `json:"id"`
	Status        string `json:"status"`
	Progress      uint32 `json:"progress"`
	ConfigVersion string `json:"config_version"`
	ProgressModel string `json:"progress_model"`
}

// Position is the geometry consumed by quest proximity rules.
type Position struct {
	Town, Area uint32
	X, Y       uint16
}

// Store is the persistence capability consumed by quest rules. Implementations
// preserve the existing transactions and event receipts.
type Store interface {
	Quests(context.Context, int64, int64) ([]QuestState, error)
	AcceptQuestGroups(context.Context, int64, int64, uint16, string, uint32, uint32, [][]uint32, uint32, string) (QuestState, error)
	AdventureEquipmentRegistered(context.Context, int64, int64, uint32) (bool, error)
	CompleteQuestObjective(context.Context, int64, int64, uint16, string, string) (bool, error)
	MarkMeetNPCQuest(context.Context, int64, int64, uint16, string, string) error
	RecordQuestMapClear(context.Context, int64, int64, string, uint32, string, string, []uint16) (bool, error)
	CompleteQuestUseObjective(context.Context, int64, int64, uint16, string, string, string, uint32) (bool, error)
	CommitOdysseyGraduation(context.Context, int64, int64, string, func(character.Character, bool) (json.RawMessage, json.RawMessage, []uint16, error)) (character.Character, bool, error)
}
