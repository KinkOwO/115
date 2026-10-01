package character

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const buffEnhancementKey = "buff_enhancement"

type buffRegistration struct {
	Kind        byte   `json:"kind"`
	Template    uint32 `json:"template"`
	Fingerprint string `json:"fingerprint"`
}
type buffEnhancementState struct {
	Version uint8              `json:"version"`
	Skill   uint16             `json:"skill"`
	Items   []buffRegistration `json:"items"`
}

func readBuffEnhancement(raw json.RawMessage) (buffEnhancementState, error) {
	var fields map[string]json.RawMessage
	var state buffEnhancementState
	if err := json.Unmarshal(raw, &fields); err != nil {
		return state, err
	}
	if body, ok := fields[buffEnhancementKey]; ok {
		if err := json.Unmarshal(body, &state); err != nil {
			return state, err
		}
		if state.Version != 1 {
			return state, fmt.Errorf("unsupported buff enhancement save version")
		}
	}
	return state, nil
}

// A conservative instance snapshot: ignore location only. Changing instance
// attributes requires re-registering. Never silently choose among identical
// duplicates or let a reused inventory slot point at an unrelated item.
func buffFingerprint(item inventory.BagEquipment) string {
	item.Slot = 0
	item.Record = append([]byte(nil), item.Record...)
	if len(item.Record) >= 2 {
		item.Record[0], item.Record[1] = 0, 0
	}
	raw, _ := json.Marshal(item)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func buffRows(b inventory.Bag, list byte) []inventory.BagEquipment {
	switch list {
	case 0:
		return b.Equipment
	case 3:
		return b.Worn
	case 1, 7:
		return b.Special[list]
	}
	return nil
}

func (s *Service) buffKind(item inventory.BagEquipment) (byte, error) {
	if s.Equipment == nil {
		return 0, fmt.Errorf("buff equipment source unavailable")
	}
	kind, err := s.Equipment.EquipmentKind(item.Template)
	if err != nil {
		return 0, err
	}
	slot, ok := s.WearRules.Slots[kind]
	if !ok || slot > 26 {
		return 0, fmt.Errorf("unsupported buff equipment source kind")
	}
	return byte(slot), item.ValidateRecord()
}

func (s *Service) resolveBuffItem(b inventory.Bag, reg buffRegistration) (protocol.BuffEnhancementItem, bool) {
	var result protocol.BuffEnhancementItem
	matches := 0
	for _, list := range []byte{0, 1, 3, 7} {
		for _, item := range buffRows(b, list) {
			if item.Template != reg.Template || buffFingerprint(item) != reg.Fingerprint {
				continue
			}
			kind, err := s.buffKind(item)
			if err != nil || kind != reg.Kind {
				continue
			}
			matches++
			result = protocol.BuffEnhancementItem{Exists: true, Kind: reg.Kind, List: list, Slot: item.Slot}
		}
	}
	return result, matches == 1
}

func (s *Service) applyBuffEnhancement(current Character, req protocol.BuffEnhancementRequest) (json.RawMessage, error) {
	state, err := readBuffEnhancement(current.State)
	if err != nil {
		return nil, err
	}
	var skills State
	if err = json.Unmarshal(current.State, &skills); err != nil {
		return nil, err
	}
	known, err := knownSkills(skills, 0)
	if err != nil {
		return nil, err
	}
	if req.Skill != 0 && known[req.Skill] == 0 {
		return nil, fmt.Errorf("buff skill is not learned")
	}
	state.Version, state.Skill = 1, req.Skill
	if req.Kind != 48 {
		var registration buffRegistration
		if !req.EmptyReference() {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, e
			}
			found := false
			for _, item := range buffRows(b, req.List) {
				if item.Slot != req.Slot {
					continue
				}
				kind, e := s.buffKind(item)
				if e != nil || kind != req.Kind {
					return nil, fmt.Errorf("buff item source kind mismatch")
				}
				registration = buffRegistration{req.Kind, item.Template, buffFingerprint(item)}
				if _, unique := s.resolveBuffItem(b, registration); !unique {
					return nil, fmt.Errorf("buff item identity is ambiguous")
				}
				found = true
			}
			if !found {
				return nil, fmt.Errorf("buff item is absent from owned inventory")
			}
		}
		kept := make([]buffRegistration, 0, len(state.Items)+1)
		for _, old := range state.Items {
			if old.Kind != req.Kind {
				kept = append(kept, old)
			}
		}
		if !req.EmptyReference() {
			kept = append(kept, registration)
		}
		state.Items = kept
	}
	sort.Slice(state.Items, func(i, j int) bool { return state.Items[i].Kind < state.Items[j].Kind })
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(current.State, &fields); err != nil {
		return nil, err
	}
	fields[buffEnhancementKey], err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func (s *Service) SaveBuffEnhancement(ctx context.Context, role Character, key string, req protocol.BuffEnhancementRequest) (Character, bool, error) {
	if s.Store == nil || role.ID == 0 || role.AccountID == 0 {
		return role, false, fmt.Errorf("buff enhancement requires owned selected character")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "buff-enhancement-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		next, err := s.applyBuffEnhancement(current, req)
		return next, json.RawMessage(`{}`), err
	})
}

func (s *Service) BuffEnhancementRestore(role Character) ([]byte, error) {
	state, err := readBuffEnhancement(role.State)
	if err != nil {
		return nil, err
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	items := []protocol.BuffEnhancementItem{}
	for _, registration := range state.Items {
		if item, ok := s.resolveBuffItem(b, registration); ok {
			items = append(items, item)
		}
	}
	// Always clear the preceding actor's cache, including old saves with no key.
	return protocol.BuffEnhancementAllData(state.Skill, items)
}
