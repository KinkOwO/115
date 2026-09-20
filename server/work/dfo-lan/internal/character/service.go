package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Rules struct {
	OdysseyPilot     bool `json:"odyssey_pilot,omitempty"`
	AllJobsPilot     bool `json:"all_jobs_pilot,omitempty"`
	MaxCharacters    int  `json:"max_characters"`
	InitialLevel     byte `json:"initial_level"`
	SwordmasterPilot bool `json:"swordmaster_pilot,omitempty"`
}
type State struct {
	AllJobsPilot    bool                   `json:"all_jobs_pilot,omitempty"`
	Level           byte                   `json:"level"`
	Experience      uint64                 `json:"experience,omitempty"`
	SkillPoints     [2]uint16              `json:"skill_points,omitempty"`
	TechniquePoints [2]uint16              `json:"technique_points,omitempty"`
	CurrencySlot2   uint32                 `json:"currency_slot2,omitempty"`
	Advancement     byte                   `json:"advancement"`
	Awakening       byte                   `json:"awakening,omitempty"`
	SkillVariations [2]SkillVariationState `json:"skill_variations,omitempty"`
	Attributes      map[string]float32     `json:"attributes"`
	InitialSkills   []int32                `json:"initial_skill_cells"`
	LearnedSkills   [2]map[uint16]byte     `json:"learned_skills,omitempty"`
	SkillSlots      [2]map[uint16]uint16   `json:"skill_slots,omitempty"`
	SourcePath      string                 `json:"source_path"`
	SourceSHA256    string                 `json:"source_sha256"`
	// Create equipment cells are intentionally unresolved until the native
	// grow-type/slot selection semantics are verified. Never substitute IDs.
	EquipmentPending bool   `json:"equipment_pending"`
	CreationOptions  []byte `json:"creation_options,omitempty"`
	CreationMode     byte   `json:"creation_mode,omitempty"`
	SwordmasterPilot bool   `json:"swordmaster_pilot,omitempty"`
}
type Service struct {
	DisableActorAppearance bool
	DetailedWornCandidate  bool
	Store                  *storage.Store
	Catalog                catalog.Characters
	Rules                  Rules
	Learning               *LearningCatalog
}

func New(s *storage.Store, c catalog.Characters, r Rules) (*Service, error) {
	if s == nil || len(c.Professions) == 0 || r.MaxCharacters < 1 || r.MaxCharacters > 65534 || r.InitialLevel < 1 {
		return nil, errors.New("incomplete character service configuration")
	}
	return &Service{Store: s, Catalog: c, Rules: r}, nil
}
func (s *Service) CheckName(ctx context.Context, p []byte) ([]byte, error) {
	name, e := protocol.DecodeNameRequest(p)
	if e != nil {
		return nil, e
	}
	exists, e := s.Store.NameExists(ctx, name)
	if e != nil {
		return nil, e
	}
	if exists {
		return nil, errors.New("character name already exists; native refusal code pending")
	}
	return []byte{1}, nil
}
func (s *Service) Create(ctx context.Context, account int64, p []byte) (storage.Character, error) {
	req, e := protocol.DecodeCreateRequest(p)
	if e != nil {
		return storage.Character{}, e
	}
	prof, ok := s.Catalog.Professions[req.Profession]
	if !ok {
		return storage.Character{}, fmt.Errorf("profession %d is absent from source data", req.Profession)
	}
	// A repeated confirmation for the same account/request returns its saved
	// role. Never reset an existing character's state on a client retry.
	existing, e := s.Store.Characters(ctx, account)
	if e != nil {
		return storage.Character{}, e
	}
	for _, c := range existing {
		if strings.EqualFold(c.Name, req.Name) {
			old, parseErr := protocol.DecodeCreateRequest(c.Request)
			if c.Name == req.Name && c.Profession == req.Profession && parseErr == nil && bytes.Equal(old.Options, req.Options) {
				return c, nil
			}
			return storage.Character{}, errors.New("character name already exists with another creation request")
		}
	}
	initial := State{Level: s.Rules.InitialLevel, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256, EquipmentPending: true}
	initial.setCreationOptions(req.Options)
	if s.Rules.AllJobsPilot && len(req.Options) == 12 && req.Options[8] != 0 {
		adv := req.Options[8]
		if len(prof.AdvancementGrowth[adv]) > 0 {
			initial.Advancement, initial.AllJobsPilot = adv, true
		}
	}
	if s.Rules.SwordmasterPilot && req.Profession == 0 && len(req.Options) == 12 && req.Options[8] == 1 && (req.Options[10] == 0 || req.Options[10] == 2) {
		if len(prof.SwordmasterGrowth) > 0 {
			initial.Advancement, initial.SwordmasterPilot = 1, true
		}
	}
	// The naming window always sends the advancement slot it had selected
	// (protocol.CreateRequest.GrowthType). Without it the character stays on
	// the unadvanced base profession, which is what the client renders as the
	// base job name. Record it unconditionally: a slot the snapshot ships no
	// [growtype N] block for simply leaves the per-level growth ledger empty
	// and must never refuse the creation itself. Pilot channels set
	// Advancement themselves and bypass this assignment.
	if !initial.AllJobsPilot && !initial.SwordmasterPilot {
		initial.Advancement = req.GrowthType
	}
	state, e := json.Marshal(initial)
	if e != nil {
		return storage.Character{}, e
	}
	return s.Store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: req.Name, Profession: req.Profession, Request: append([]byte(nil), p...), ConfigVersion: s.Catalog.Source.Checksum, State: state}, s.Rules.MaxCharacters)
}

func (s *State) setCreationOptions(options []byte) {
	s.CreationOptions = append([]byte(nil), options...)
	s.CreationMode = 0
	// Five checksum-verified 115US requests: ordinary=0, Odyssey=2 at
	// option 10. Option 8 is 1 in both modes; it is not a mode selector.
	if len(options) >= 12 {
		s.CreationMode = options[10]
	}
}

func (s *Service) List(ctx context.Context, account int64) ([]byte, error) {
	return s.ListWithFatigue(ctx, account, nil, time.Time{})
}

func (s *Service) ListWithFatigue(ctx context.Context, account int64, fatigue *FatigueService, now time.Time) ([]byte, error) {
	chars, e := s.Store.Characters(ctx, account)
	if e != nil {
		return nil, e
	}
	rows := make([]protocol.CharacterRow, 0, len(chars))
	for slot, c := range chars {
		var state State
		if e = json.Unmarshal(c.State, &state); e != nil {
			return nil, e
		}
		row := protocol.CharacterRow{Slot: uint16(slot), Name: c.Name, Profession: c.Profession, Advancement: state.Advancement, Level: state.Level}
		row.Advancement, e = state.WireAdvancement()
		if e != nil {
			return nil, e
		}
		row.Equipment, e = wornAppearance(c.State)
		if e != nil {
			return nil, e
		}
		row.Odyssey, e = s.IsOdyssey(c)
		if e != nil {
			return nil, e
		}
		if fatigue != nil {
			fp, err := fatigue.State(ctx, account, c.ID, now)
			if err != nil {
				return nil, err
			}
			if fp.Limit > fp.Used {
				row.FatigueRemaining = fp.Limit - fp.Used
			}
		}
		rows = append(rows, row)
	}
	return protocol.CharacterList(uint16(s.Rules.MaxCharacters), rows)
}

// RosterSlot deliberately resolves within the account's ordered roster. The
// database identity and legacy wire_id column remain durable storage keys.
func (s *Service) RosterSlot(ctx context.Context, account, characterID int64) (uint16, error) {
	roles, err := s.Store.Characters(ctx, account)
	if err != nil {
		return 0, err
	}
	for slot, role := range roles {
		if role.ID == characterID {
			return uint16(slot), nil
		}
	}
	return 0, errors.New("created character is absent from account roster")
}

func (s *Service) Select(ctx context.Context, account int64, p []byte) (storage.Character, error) {
	slot, e := protocol.DecodeSelectRequest(p)
	if e != nil {
		return storage.Character{}, e
	}
	roles, e := s.Store.Characters(ctx, account)
	if e != nil {
		return storage.Character{}, e
	}
	if uint64(slot) >= uint64(len(roles)) {
		return storage.Character{}, errors.New("selected slot is absent from account roster")
	}
	return roles[slot], nil
}

// EntryBasicProbe uses the persisted character identity and state. This is a
// packet experiment only; world placement and actor allocation are separate.
func (s *Service) EntryBasicProbe(role storage.Character, channelContext [2]byte) ([]byte, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	odyssey, err := s.IsOdyssey(role)
	if err != nil {
		return nil, err
	}
	advancement, err := state.WireAdvancement()
	if err != nil {
		return nil, err
	}
	appearance, err := wornAppearance(role.State)
	if err != nil {
		return nil, err
	}
	if s.DisableActorAppearance {
		appearance = nil
	}
	return protocol.UserInfoBasicProbe(protocol.EntryBasicProbe{
		ActorServerID: role.WireID, Context: channelContext,
		Character: protocol.CharacterRow{Name: role.Name, Profession: role.Profession, Advancement: advancement, Level: state.Level, Odyssey: odyssey, Equipment: appearance},
	})
}

func (s *Service) IsOdyssey(role storage.Character) (bool, error) {
	return OdysseyRole(role), nil
}
