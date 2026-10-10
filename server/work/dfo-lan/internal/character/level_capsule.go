package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CapsuleLevelRow struct{ Minimum, Maximum, Target byte }

// JumpLevelCapsule is parsed from the owned stackable's native definition.
// There is no template or target-level list maintained by the executor.
type JumpLevelCapsule struct {
	Path, SHA256 string
	Rows         []CapsuleLevelRow
	Minimum      byte
	Maximum      byte
	SeriaOnly    bool
}

func ParseJumpLevelCapsule(s catalog.ScriptRecord) (JumpLevelCapsule, error) {
	r := JumpLevelCapsule{Path: s.Path, SHA256: s.SHA256, Minimum: 1, Maximum: 255}
	// An additional source requirement/reward must not be silently ignored.
	// These are the closed native headers, including non-gameplay presentation
	// fields; unfamiliar definitions remain unconsumed until their grammar is read.
	allowed := map[string]bool{}
	for _, name := range []string{"name", "explain", "grade", "rarity", "usable job", "attach type", "icon", "icon marks", "field image", "stackable type", "item group name", "move wav", "minimum level", "maximum level", "action type", "increase status type", "action usable place", "impossible contents", "display exp up result level", "display new area ui", "purchase limit"} {
		allowed["["+name+"]"] = true
	}
	for _, t := range s.Cells {
		if t.Type == 3 && !allowed[strings.Replace(t.Text, "[/", "[", 1)] {
			return r, fmt.Errorf("unsupported capsule source header %s", t.Text)
		}
	}
	field := func(tag string) ([]pvf.Token, error) {
		var cells []pvf.Token
		seen, active := false, false
		for _, t := range s.Cells {
			if t.Type == 3 {
				active = t.Text == tag
				if active {
					if seen {
						return nil, fmt.Errorf("duplicate capsule field %s", tag)
					}
					seen = true
				}
				continue
			}
			if active {
				cells = append(cells, t)
			}
		}
		return cells, nil
	}
	expect := func(tag, text string) error {
		v, err := field(tag)
		if err != nil {
			return err
		}
		if len(v) != 1 || v[0].Type != 6 || v[0].Text != text {
			return fmt.Errorf("unsupported capsule %s", tag)
		}
		return nil
	}
	if len(s.Cells) == 0 || s.Path == "" || len(s.SHA256) != 64 {
		return r, fmt.Errorf("capsule source missing")
	}
	if err := expect("[action type]", "[jumping lv]"); err != nil {
		return r, err
	}
	if err := expect("[usable job]", "[all]"); err != nil {
		return r, err
	}
	// Other source place/job grammars must be closed independently.
	if err := expect("[action usable place]", "[seria room]"); err != nil {
		return r, err
	}
	r.SeriaOnly = true
	for tag, dest := range map[string]*byte{"[minimum level]": &r.Minimum, "[maximum level]": &r.Maximum} {
		v, err := field(tag)
		if err != nil {
			return r, err
		}
		if len(v) == 0 {
			continue
		}
		if len(v) != 1 || v[0].Type != 0 || v[0].Value < 1 || v[0].Value > 255 {
			return r, fmt.Errorf("invalid capsule %s", tag)
		}
		*dest = byte(v[0].Value)
	}
	v, err := field("[increase status type]")
	if err != nil {
		return r, err
	}
	if len(v) < 4 || v[0].Type != 6 || v[0].Text != "[levelUpByTable]" || (len(v)-1)%3 != 0 {
		return r, fmt.Errorf("unsupported capsule level table")
	}
	for i := 1; i < len(v); i += 3 {
		for _, t := range v[i : i+3] {
			if t.Type != 0 || t.Value < 1 || t.Value > 255 {
				return r, fmt.Errorf("invalid capsule level row")
			}
		}
		row := CapsuleLevelRow{byte(v[i].Value), byte(v[i+1].Value), byte(v[i+2].Value)}
		if row.Minimum > row.Maximum || row.Target <= row.Maximum {
			return r, fmt.Errorf("capsule table is not an increasing level jump")
		}
		for _, prior := range r.Rows {
			if row.Minimum <= prior.Maximum && prior.Minimum <= row.Maximum {
				return r, fmt.Errorf("overlapping capsule level rows")
			}
		}
		r.Rows = append(r.Rows, row)
	}
	contents, err := field("[impossible contents]")
	if err != nil {
		return r, err
	}
	for _, t := range contents {
		if t.Type != 6 || (t.Text != "ban redeemitem" && t.Text != "hardcore mode") {
			return r, fmt.Errorf("unsupported capsule content restriction")
		}
	}
	if r.Minimum > r.Maximum {
		return r, fmt.Errorf("invalid capsule level bounds")
	}
	return r, nil
}

func (r JumpLevelCapsule) Target(level byte) (byte, error) {
	if level < r.Minimum || level > r.Maximum {
		return 0, fmt.Errorf("角色等级不在胶囊可用范围内")
	}
	for _, row := range r.Rows {
		if row.Minimum <= level && level <= row.Maximum {
			return row.Target, nil
		}
	}
	return 0, fmt.Errorf("胶囊等级表没有当前等级的升级规则")
}

type LevelCapsuleReceipt struct {
	Template       uint32 `json:"template"`
	Slot           uint16 `json:"slot"`
	Before, Target byte
	Gain           uint64 `json:"experience_gain"`
	Remaining      uint32 `json:"remaining"`
	Path, SHA256   string
}

// ApplyLevelCapsule is pure. Its caller commits the level, SP, attributes,
// complete bag and receipt in the same existing dual-engine transaction.
func (s *ProgressionService) ApplyLevelCapsule(role Character, c catalog.LootCatalog, slot uint16, inSeria bool, now time.Time) (Character, LevelCapsuleReceipt, error) {
	var receipt LevelCapsuleReceipt
	if c.Source.SaveIdentity() != role.ConfigVersion || c.Source.Checksum != s.Catalog.Source.Checksum {
		return role, receipt, fmt.Errorf("capsule/progression source mismatch")
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return role, receipt, err
	}
	// Only the ordinary creation path is implemented. No special-mode or
	// redemption operation is admitted through this consumption handler.
	if state.CreationMode != 0 {
		return role, receipt, fmt.Errorf("该角色模式尚未支持使用跳级胶囊")
	}
	prof, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return role, receipt, fmt.Errorf("capsule profession source missing")
	}
	if state.Advancement == 0 && len(prof.AdvancementGrowth) > 0 {
		return role, receipt, fmt.Errorf("请先完成转职再使用胶囊")
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return role, receipt, err
	}
	var owned inventory.BagItem
	for _, item := range bag.Items {
		if item.Slot == slot {
			owned = item
			break
		}
	}
	if owned.Template == 0 || owned.Amount == 0 {
		return role, receipt, fmt.Errorf("胶囊所在背包格为空")
	}
	if protocol.StoredItemExpired(owned.ExpireTime, now.Unix()) {
		return role, receipt, fmt.Errorf("升级胶囊已过期")
	}
	item, ok := c.Items[owned.Template]
	if !ok || item.Kind != "stackable" {
		return role, receipt, fmt.Errorf("capsule source stackable missing")
	}
	script, err := c.ItemScript(owned.Template)
	if err != nil {
		return role, receipt, err
	}
	rule, err := ParseJumpLevelCapsule(script)
	if err != nil {
		return role, receipt, err
	}
	if rule.SeriaOnly && !inSeria {
		return role, receipt, fmt.Errorf("只能在赛丽亚房间使用升级胶囊")
	}
	target, err := rule.Target(state.Level)
	if err != nil {
		return role, receipt, err
	}
	if target <= state.Level || target > s.Rules.LevelCap || int(target)-2 >= len(s.Catalog.Thresholds) {
		return role, receipt, fmt.Errorf("capsule target exceeds source growth boundary")
	}
	experience := s.Catalog.Thresholds[int(target)-2]
	if experience <= state.Experience {
		return role, receipt, fmt.Errorf("capsule would not increase the saved experience")
	}
	gain := experience - state.Experience
	updated, result, err := s.ApplyGain(role, gain)
	if err != nil {
		return role, receipt, err
	}
	if result.Level != target {
		return role, receipt, fmt.Errorf("capsule target and growth table disagree")
	}
	bag, remaining, err := bag.Consume(c, slot, owned.Template)
	if err != nil {
		return role, receipt, err
	}
	updated.State, err = inventory.SaveBag(updated.State, bag)
	if err != nil {
		return role, receipt, err
	}
	receipt = LevelCapsuleReceipt{Template: owned.Template, Slot: slot, Before: state.Level, Target: target, Gain: gain, Remaining: remaining, Path: rule.Path, SHA256: rule.SHA256}
	return updated, receipt, nil
}

func (s *ProgressionService) UseLevelCapsule(ctx context.Context, role Character, c catalog.LootCatalog, slot uint16, inSeria bool, key string, now time.Time) (Character, bool, error) {
	if s == nil || nilPersistence(s.Store) || key == "" {
		return role, false, fmt.Errorf("capsule persistence unavailable")
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-jump-level-capsule-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		updated, receipt, err := s.ApplyLevelCapsule(current, c, slot, inSeria, now)
		if err != nil {
			return nil, nil, err
		}
		proof, err := json.Marshal(receipt)
		return updated.State, proof, err
	})
	if err == nil && applied {
		s.notifyLevelUp(ctx, role, saved)
	}
	return saved, applied, err
}
