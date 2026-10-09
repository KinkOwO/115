package character

import (
	"context"
	"encoding/json"
	"reflect"
	"time"
)

// EventStore supplies the atomic state update used by character rules.
type EventStore interface {
	CommitCharacterEvent(context.Context, int64, int64, string, string, string, func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error)
	HasGrowthPremium(context.Context, int64, time.Time) (bool, error)
	HasTacticianPremium(context.Context, int64, time.Time) (bool, error)
}

// Store is the persistence contract consumed by character creation and skills.
type Store interface {
	EventStore
	NameExists(context.Context, string) (bool, error)
	Characters(context.Context, int64) ([]Character, error)
	CreateCharacter(context.Context, Character, int) (Character, error)
	CompletedQuestIDs(context.Context, int64, string, []uint16) (map[int64][]uint16, error)
	AccountUnifiedOptions(context.Context, int64) (map[uint16]uint16, error)
	CharacterUnifiedOptions(context.Context, int64) (map[uint16]uint16, error)
	AdventureLevel(context.Context, int64, int64) (uint32, error)
	ChangeCharacterSlots(context.Context, int64, CharacterSlotChange, int) error
	// AccountSlotBonus returns the account-level extra character-slot count
	// (granted by the Character Slot Extension Kit on purchase); the effective
	// cap for create/list/slot-move is Rules.MaxCharacters + this value.
	AccountSlotBonus(context.Context, int64) (int32, error)
	CommitSkillLocks(context.Context, int64, int64, string, string, func([]uint16) ([]uint16, error)) ([]uint16, bool, error)
}

// ProgressionStore supplies progression commits and existing run ledgers.
type ProgressionStore interface {
	EventStore
	CharacterEventReceipt(context.Context, int64, int64, string) (json.RawMessage, error)
	RunMonsterExperience(context.Context, int64, int64, string) (uint64, error)
	RunFatigueLedger(context.Context, int64, int64, string) (int64, int64, error)
}

// FatigueStore supplies day/room accounting and atomic potion consumption.
type FatigueStore interface {
	HasGrowthPremium(context.Context, int64, time.Time) (bool, error)
	LoadFatigue(context.Context, int64, int64, string, uint16) (FatigueState, error)
	ConsumeRoomFatigue(context.Context, int64, int64, string, uint16, string, uint32, uint16) (FatigueState, bool, error)
	RunPaidFatigue(context.Context, int64, string) (bool, error)
	RecoverFatigue(context.Context, int64, int64, string, FatigueRecovery, func(Character) (json.RawMessage, error)) (Character, FatigueState, error)
}

// Interface injection must preserve the nil concrete-store constructor behavior.
func nilPersistence(store any) bool {
	if store == nil {
		return true
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}
