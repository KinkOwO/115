package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	// No omitempty: an unallocated VP block still has to serialize, otherwise
	// the client decodes an empty variation section and the panel reads blank
	// until the next character switch.
	SkillVariations [2]SkillVariationState `json:"skill_variations"`
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
	// OdysseyGraduated marks a max-level Arad Odyssey character that became a
	// regular character. omitempty keeps old binaries reading the state as a
	// plain unknown field, so a rollback never invalidates the save.
	OdysseyGraduated bool `json:"odyssey_graduated,omitempty"`
}
type Service struct {
	DisableActorAppearance bool
	DetailedWornCandidate  bool
	Store                  *storage.Store
	Catalog                catalog.Characters
	Rules                  Rules
	Learning               *LearningCatalog
	// Equipment 与 WearRules 是创建期的可选依赖：两者都带上、且与 Catalog 同一份
	// 源快照时，Create 会按源 [create equipment list] 给新角色穿上其转职槽的初始
	// 装备。缺任一项或不匹配都只是不投影，不构成拒绝创建的理由。
	Equipment *inventory.EquipmentCatalog
	WearRules inventory.WearRules
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
	s.applyCreationAdvancement(&initial, req, prof)
	// 源 [create equipment list] 按转职槽给出初始装备。有数据就投影到穿戴栏；
	// 没有数据或依赖缺失时保持原样，创建照样成功。
	worn := s.creationWorn(prof, initial.Advancement, initial.Level)
	if len(worn) > 0 {
		initial.EquipmentPending = false
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
	if len(worn) > 0 {
		state, e = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn})
		if e != nil {
			return storage.Character{}, e
		}
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
//
// The worn set is projected into the native 0x145639840 block. The block is
// the only place the client learns an explicit per-slot model binding, and
// AppearanceProbe below carries it. The entry-time packet keeps the explicit
// block empty on purpose: with count=0 the client fills every slot's model
// from the worn item objects (live-verified 2026-09-18: weapon, armour and
// title all render correctly on first entry). The post-move refresh is the
// path that does need explicit rows - see AppearanceProbe.
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
	// The entry-time block is built from the list-row equipment projection
	// (protocol.UserInfoBasicProbe falls back to EquipmentAppearance(r.Equipment)).
	// The roster channel proves the value shape live: the same rows render the
	// select-screen model correctly. An empty block leaves every slot's
	// 0x405 binding at zero and the client reports "no weapon equipped"
	// (user report 20260918). Cosmetic only: an undecorable worn row must
	// never refuse entry (user ruling 20260918).
	equipment, err := wornAppearance(role.State)
	if err != nil {
		equipment = nil
	}
	if s.DisableActorAppearance {
		equipment = nil
	}
	// The mode-0 creature segment is the town-follower display path
	// (docs/宠物显示实现-G0198 §2.1): u32 slot-26 template + dstr name + u8
	// present. It is read before any appearance clearing, so a creature
	// survives DisableActorAppearance exactly like the native client.
	creatureItemID, creatureName := wornCreature(role.State)
	return protocol.UserInfoBasicProbe(protocol.EntryBasicProbe{
		ActorServerID: role.WireID, Context: channelContext,
		Character: protocol.CharacterRow{Name: role.Name, Profession: role.Profession, Advancement: advancement, Level: state.Level, Odyssey: odyssey, Equipment: equipment, CreatureItemID: creatureItemID, CreatureName: creatureName},
		// The explicit per-slot block must stay empty on the entry path. A
		// block holding a client-rejected slot is worse than an empty one:
		// the reader replaces the projection wholesale, so a knight wearing
		// a [support weapon] would ship a block whose only row is slot 24 -
		// which the client's cosmetic-layer slot set rejects - and slot 12
		// (the weapon/held-visual row the client does accept) would never
		// reach it. Slots 14..25 are template-driven from the id13/id14 item
		// rows and must not be sent as binding rows.
		Appearance: nil,
	})
}

// AppearanceProbe is the worn-appearance variant of the mode0 userinfo packet:
// the same 0x145637a20 shape, but the 0x145639840 block carries one entry per
// client-visible worn slot instead of an empty count. It is the payload the
// equipment-move flow sends after its resync chain so the client can rebuild
// the actor with the new per-slot model bindings - the "change the look and
// the world model follows" channel (C9).
//
// Each row carries the piece's own template ID. Two independent sources pin
// that value shape: (a) the sibling build's live-validated mode0 appearance
// summary stores {appearance slot, item ID} per row, and (b) the earlier
// live failure where the second cell of [equipment type] (a small class
// number, 17..21) was sent instead made the client drop the weapon with
// "no weapon equipped" - an item lookup that cannot resolve. A template ID is
// valid by construction; a class number is not.
//
// state is the character's stored state object, which is the shape ReadBag
// looks in. Handing it the inventory sub-object instead yields an empty bag
// without an error, because the bag keys are looked up one level up.
func (s *Service) AppearanceProbe(role storage.Character, channelContext [2]byte) ([]byte, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	rows, err := s.wornAppearance(role.State)
	if err != nil {
		return nil, err
	}
	// The post-move mode-0 refresh rebuilds the actor wholesale, so it must
	// re-project the creature segment too; omitting it here hides the town
	// follower right after a CMD19 equip/unequip touches the worn set.
	creatureItemID, creatureName := wornCreature(role.State)
	// wire 字节是 advancement | awakening<<4（见 protocol/advancement.go：低 4 位转职
	// 分支、bit4..6 觉醒阶段）。这里曾只写 state.Advancement，于是每次换装重发的
	// mode0 userinfo 都把觉醒阶段抹成 0；客户端收到后以为"刚从无觉醒变成 N 觉"，每次
	// 穿脱都弹一次「N次觉醒」对话框（实机 2026-09-23：开箱换上奥德赛装备必弹，弹窗
	// 写着"你的角色属性已提升 / 你已学会以下技能"）。列表与进城路径都用
	// WireAdvancement，只有这个换装刷新漏了。
	advancement, err := state.WireAdvancement()
	if err != nil {
		return nil, err
	}
	return protocol.UserInfoBasicProbe(protocol.EntryBasicProbe{
		ActorServerID: role.WireID, Context: channelContext,
		Character:  protocol.CharacterRow{Name: role.Name, Profession: role.Profession, Advancement: advancement, Level: state.Level, CreatureItemID: creatureItemID, CreatureName: creatureName},
		Appearance: rows,
	})
}

// wornAppearance projects the character's worn equipment onto the native
// equipped-appearance block, one entry per worn slot in ascending slot order.
// Each row binds the piece's own template ID; see AppearanceProbe for the
// value provenance. state must be the whole stored state object (ReadBag
// looks up the bag keys one level above the inventory sub-object).
//
// No body-slot filter is applied here on purpose: the client's own entry trace
// accepts body-equipment rows 14..25 verbatim ("equip : 24 - 骑士之盾"), and
// the 0x145a8a780 key set is the cosmetic-layer table, a different coordinate
// system from the [equipment type] slot space.
//
// Slots above the client's equipped-appearance table (protocol tops out at 25)
// are skipped, not emitted: the creature rides worn slot 26 and its visuals
// travel the creature packets, while a slot-26 row here makes the native
// encoder reject the whole block - live 2026-09-21, every CMD19 equip on a
// creature-wearing character failed with "equipped appearance slot 26 exceeds
// the client table" and the client showed its generic move-refusal notice.
const maxWornAppearanceSlot = 25

func (s *Service) wornAppearance(state json.RawMessage) ([]protocol.EquippedAppearance, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	if len(bag.Worn) == 0 {
		return nil, nil
	}

	rows := make([]protocol.EquippedAppearance, 0, len(bag.Worn))
	for _, w := range bag.Worn {
		if w.Slot > maxWornAppearanceSlot {
			continue
		}
		rows = append(rows, protocol.EquippedAppearance{Slot: byte(w.Slot), Model: w.Template})
	}
	if len(rows) == 0 {
		return nil, nil
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, nil
}

func (s *Service) IsOdyssey(role storage.Character) (bool, error) {
	return OdysseyRole(role), nil
}
