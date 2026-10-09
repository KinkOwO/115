package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func knownSkills(s State, tree int) (map[uint16]byte, error) {
	base := initialSkills(s)
	out := map[uint16]byte{}
	for _, v := range base {
		out[v.ID] = v.Level
	}
	for id, level := range s.LearnedSkills[tree] {
		if id == 0 || level == 0 {
			return nil, fmt.Errorf("invalid learned skill state")
		}
		out[id] = level
	}
	return out, nil
}
func skillOrder(state State, known map[uint16]byte) []int {
	ids := make([]int, 0, len(known))
	seen := map[uint16]bool{}
	// Use the same normalized stream as knownSkills.  Persisted characters
	// created before the initial-skill repair may retain conditional or broken
	// triples; iterating the raw cells would reintroduce those IDs with a zero
	// level (or twice), which makes the entry-addition packet invalid.
	for _, skill := range initialSkills(state) {
		id := skill.ID
		if seen[id] {
			continue
		}
		ids = append(ids, int(id))
		seen[id] = true
	}
	var added []int
	for id := range known {
		if !seen[id] {
			added = append(added, int(id))
		}
	}
	sort.Ints(added)
	return append(ids, added...)
}
func (s *Service) skillRows(role Character, state State, tree int) ([]protocol.LearnedSkill, error) {
	known, e := s.knownSkills(role, state, tree)
	if e != nil {
		return nil, e
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	// Match the profession reference, as automaticSkills does. Rebuilding the
	// PVF string pool changes raw .chr hashes without changing this reference.
	if !ok || prof.Path != state.SourcePath {
		return nil, fmt.Errorf("skill profession source mismatch")
	}
	ids := skillOrder(state, known)
	slots := map[uint16]uint16{}
	used := map[uint16]bool{}
	// Persisted shortcut moves are authoritative, even when an older state
	// records only the moved rows. Reserve every explicit row first so a
	// source default cannot claim a slot that a later row already owns.
	for _, raw := range ids {
		id := uint16(raw)
		if v, ok := state.SkillSlots[tree][id]; ok {
			if v >= 255 || used[v] {
				return nil, fmt.Errorf("invalid/duplicate saved skill slot")
			}
			slots[id] = v
			used[v] = true
		}
	}
	for _, raw := range ids {
		id := uint16(raw)
		if _, ok := slots[id]; ok {
			continue
		}
		slot := uint16(65535)
		if byAdv := prof.AdvancementSkillSlots[state.Advancement]; byAdv != nil {
			if v, ok := byAdv[id]; ok {
				slot = v
			}
		}
		if slot == 65535 {
			if v, ok := prof.InitialSkillSlots[id]; ok {
				slot = v
			}
		}
		if slot != 65535 && slot >= 255 {
			return nil, fmt.Errorf("invalid source skill slot")
		}
		if slot != 65535 && used[slot] {
			// A saved custom row owns the slot. Put the source-default row into
			// the ordinary palette below instead of rejecting or overwriting it.
			slot = 65535
		}
		if slot != 65535 {
			used[slot] = true
		}
		slots[id] = slot
	}
	// Current145c7a730 searches the first free slot at14 after the quickbar.
	// The manager allocates512 entries, while CMD28/29 use an8-bit slot index;
	//255 is the request/reply sentinel, so ordinary projected slots stop at254.
	if s.Learning != nil {
		for _, raw := range ids {
			id := uint16(raw)
			_, ok, sourceErr := s.Learning.Definition(role.Profession, id)
			if sourceErr != nil {
				return nil, sourceErr
			}
			if !ok {
				return nil, fmt.Errorf("learned skill missing from profession")
			}
			if slots[id] != 65535 {
				continue
			}
			for slot := uint16(14); slot < 255; slot++ {
				if !used[slot] {
					slots[id] = slot
					used[slot] = true
					break
				}
			}
			if slots[id] == 65535 {
				return nil, fmt.Errorf("ordinary skill palette is full")
			}
		}
	}
	out := make([]protocol.LearnedSkill, 0, len(ids))
	var custom map[uint16][]uint32
	if len(state.SkillCommands) > 0 {
		parsed, err := protocol.DecodeSkillCommands(state.SkillCommands)
		if err != nil {
			return nil, fmt.Errorf("invalid saved skill commands: %w", err)
		}
		custom = parsed.Entries
	}
	for _, raw := range ids {
		id := uint16(raw)
		row := protocol.LearnedSkill{ID: id, Level: known[id], Slot: slots[id]}
		if source := prof.SkillCommands[id]; len(source) > 0 {
			row.Commands = append([]uint32(nil), source...)
		}
		if override, ok := custom[id]; ok {
			row.Commands = append([]uint32(nil), override...)
		}
		out = append(out, row)
	}
	return out, nil
}
func mergeSkillState(raw json.RawMessage, state State) (json.RawMessage, error) {
	p, e := json.Marshal(state)
	if e != nil {
		return nil, e
	}
	var old, fields map[string]json.RawMessage
	if e = json.Unmarshal(raw, &old); e != nil {
		return nil, e
	}
	if e = json.Unmarshal(p, &fields); e != nil {
		return nil, e
	}
	for k, v := range fields {
		old[k] = v
	}
	return json.Marshal(old)
}

// Source awakening grants may precede a player's prerequisite purchases (for
// example demonic swordman 255 -> 81). Check this request's skills and reject
// refunds that break dependencies, without requiring unrelated old gaps to be
// repaired before any ordinary purchase can succeed.
func (s *Service) validateLearningPrerequisites(job byte, known, changes map[uint16]byte, reduced map[uint16]bool) error {
	for id, level := range known {
		if level == 0 {
			continue
		}
		definition, _, sourceErr := s.Learning.Definition(job, id)
		if sourceErr != nil {
			return sourceErr
		}
		pre := definition.Ints("[pre required skill]")
		if len(pre)%2 != 0 {
			return fmt.Errorf("invalid skill prerequisite")
		}
		for i := 0; i < len(pre); i += 2 {
			_, changed := changes[id]
			if int(known[uint16(pre[i])]) < pre[i+1] && (changed || reduced[uint16(pre[i])]) {
				return fmt.Errorf("skill change would invalidate learned skill prerequisite")
			}
		}
	}
	return nil
}

func (s *Service) Learn(ctx context.Context, role Character, key string, req protocol.SkillPurchase) (Character, bool, error) {
	// ⚠️ 实验第二轮：cmd29 放开（第一轮只开 2179 时"不崩但没加点"，崩因在 cmd29）。
	if s.Learning == nil || req.Tree > 1 {
		return role, false, fmt.Errorf("learning service/source unavailable")
	}
	hasTactician, _ := s.Store.HasTacticianPremium(ctx, role.AccountID, time.Now())
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-learning-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		known, e := s.knownSkills(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		seen := map[uint16]bool{}
		base := initialSkills(state)
		floor := map[uint16]byte{}
		for _, v := range base {
			floor[v.ID] = v.Level
		}
		free, e := s.automaticSkills(current, state)
		if e != nil {
			return nil, nil, e
		}
		for id, rank := range free {
			if floor[id] < rank {
				floor[id] = rank
			}
		}
		awakened, e := s.awakeningSkills(current, state)
		if e != nil {
			return nil, nil, e
		}
		for id, rank := range awakened {
			if floor[id] < rank {
				floor[id] = rank
			}
		}
		points := int(state.SkillPoints[req.Tree])
		changes := map[uint16]byte{}
		reduced := map[uint16]bool{}
		var newlyLearned []uint16
		effectiveLevel := int(state.Level)
		if hasTactician {
			effectiveLevel += 5
		}
		for _, v := range req.Entries {
			d, ok, sourceErr := s.Learning.Definition(current.Profession, v.ID)
			if sourceErr != nil {
				return nil, nil, sourceErr
			}
			if !ok || seen[v.ID] || v.Delta == 0 || v.Refund > 1 {
				return nil, nil, fmt.Errorf("invalid job skill learning request")
			}
			seen[v.ID] = true
			target := int(known[v.ID]) + int(v.Delta)
			if v.Refund == 1 {
				target = int(known[v.ID]) - int(v.Delta)
				if target < int(floor[v.ID]) || target < 0 {
					return nil, nil, fmt.Errorf("cannot refund source initial ranks or absent skill")
				}
				for lv := int(known[v.ID]); lv > target; lv-- {
					cost, err := d.costForLevel(state, effectiveLevel, lv, known)
					if err != nil {
						return nil, nil, err
					}
					points += cost
					if points > 65535 {
						return nil, nil, fmt.Errorf("SP refund overflow")
					}
				}
			}
			for lv := int(known[v.ID]) + 1; lv <= target; lv++ {
				cost, e := d.costForLevel(state, effectiveLevel, lv, known)
				if e != nil {
					return nil, nil, fmt.Errorf("skill %d rank %d: %w", v.ID, lv, e)
				}
				points -= cost
				if points < 0 {
					return nil, nil, fmt.Errorf("not enough SP")
				}
			}
			if known[v.ID] == 0 && target > 0 {
				newlyLearned = append(newlyLearned, v.ID)
			}
			reduced[v.ID] = target < int(known[v.ID])
			known[v.ID] = byte(target)
			changes[v.ID] = byte(target)
		}
		if e := s.validateLearningPrerequisites(current.Profession, known, changes, reduced); e != nil {
			return nil, nil, e
		}
		if len(changes) == 0 && req.Intensions == nil && req.Options == nil {
			return nil, nil, fmt.Errorf("empty learning request")
		}
		if e := s.applyVariations(current.Profession, &state, known, req); e != nil {
			return nil, nil, e
		}
		if state.LearnedSkills[req.Tree] == nil {
			state.LearnedSkills[req.Tree] = map[uint16]byte{}
		}
		for id, lv := range changes {
			if lv == 0 {
				delete(state.LearnedSkills[req.Tree], id)
				delete(state.SkillSlots[req.Tree], id)
			} else {
				state.LearnedSkills[req.Tree][id] = lv
			}
		}
		if state.Advancement > 0 {
			for id := range state.LearnedSkills[req.Tree] {
				d, ok, sourceErr := s.Learning.Definition(current.Profession, id)
				if sourceErr != nil {
					return nil, nil, sourceErr
				}
				if ok {
					if !d.ForAdvancement(int(state.Advancement)) && !d.ForAwakening(int(state.Advancement), int(state.Awakening)) {
						delete(state.LearnedSkills[req.Tree], id)
						delete(state.SkillSlots[req.Tree], id)
					}
				}
			}
		}
		state.SkillPoints[req.Tree] = uint16(points)
		rows, e := s.skillRows(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		s.placeNewShortcuts(current.Profession, rows, newlyLearned)
		state.SkillSlots[req.Tree] = map[uint16]uint16{}
		for _, v := range rows {
			state.SkillSlots[req.Tree][v.ID] = v.Slot
		}
		p, e := mergeSkillState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		// 活动 662 第三关（技能进化点）与技能保存同事务判定：变体已通过源校验后，
		// 按活动源的 [condition] 点数把关卡推进。未装载活动时原样返回。
		p, _, e = s.completeBoostVPSave(p, state, req)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"skills": changes, "sp": points, "source": s.Learning.Source.SaveIdentity()})
		return p, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// Auto-bind only newly purchased active skills. Rank upgrades never undo a
// player's existing layout; a full bar keeps the skill in its book slot.
func (s *Service) placeNewShortcuts(job byte, rows []protocol.LearnedSkill, fresh []uint16) {
	used := map[uint16]bool{}
	for _, row := range rows {
		used[row.Slot] = true
	}
	for _, id := range fresh {
		if !s.Learning.index[job][id].Active() {
			continue
		}
		for i := range rows {
			if rows[i].ID != id || rows[i].Slot < 14 {
				continue
			}
			for slot := uint16(0); slot < 14; slot++ {
				if !used[slot] {
					delete(used, rows[i].Slot)
					rows[i].Slot = slot
					used[slot] = true
					break
				}
			}
		}
	}
}
func (s *Service) MoveSkill(ctx context.Context, role Character, key string, req protocol.SkillMove) (Character, bool, error) {
	if s.Learning == nil || req.Tree > 1 || req.From == 255 || req.To == 255 || req.From == req.To {
		return role, false, fmt.Errorf("invalid ordinary skill move")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-slots-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		// 拖放也分页：第二页的栏位必须落到 SkillSlots[1]。
		rows, e := s.skillRows(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		var from, to uint16
		slots := map[uint16]uint16{}
		for _, v := range rows {
			slots[v.ID] = v.Slot
			if v.Slot == uint16(req.From) {
				from = v.ID
			}
			if v.Slot == uint16(req.To) {
				to = v.ID
			}
		}
		if from == 0 {
			return nil, nil, fmt.Errorf("source slot is empty")
		}
		// 只有源表明确写着 [passive] 的才拒绝。觉醒/VP 行的 [type] 常常解析不出
		// 来（atmage 138/139 是 "default"、priest 133/134/250/253 整行没有），
		// 用 Active() 会把它们误判成被动，玩家就再也注册不上快捷键。
		for _, id := range []uint16{from, to} {
			if id != 0 && !s.Learning.index[current.Profession][id].ShortcutCapable() {
				return nil, nil, fmt.Errorf("passive skill cannot occupy a shortcut")
			}
		}
		slots[from] = uint16(req.To)
		if to != 0 {
			slots[to] = uint16(req.From)
		}
		state.SkillSlots[req.Tree] = slots
		p, e := mergeSkillState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		r, e := json.Marshal(req)
		return p, r, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// MoveSkillTotal applies CMD 2179 CHANGE_SKILLSLOT_TOTAL: the sequential swap
// list the client emits after 自动加点 to lay out its shortcut bar, right after
// the learn burst. It used to reach the gateway as an unimplemented sample, so
// the bar stayed in whatever order the server had last computed and the layout
// the player saw in the auto-set preview was never the one they got.
//
// Sources name the state the previous pair produced, so the pairs are applied
// in order; an empty target receives the source skill outright.
func (s *Service) MoveSkillTotal(ctx context.Context, role Character, key string, req protocol.SkillSlotTotal) (Character, bool, error) {
	// ⚠️ 实验第六轮（21:5x）：放行 tree≤1 受理第二页布局并落库。
	// 与协议层 DecodeSkillSlotTotal 保持一致。
	if s.Learning == nil || req.Tree > 1 || len(req.Pairs) == 0 {
		return role, false, fmt.Errorf("invalid skill slot total")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-skill-slots-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		rows, e := s.skillRows(current, state, int(req.Tree))
		if e != nil {
			return nil, nil, e
		}
		if e := s.applySkillSlotSwaps(current.Profession, rows, req.Pairs); e != nil {
			return nil, nil, e
		}
		// The list is the layout the client is about to display; persist the
		// same rows so the next skillRows cannot re-lay them somewhere else.
		state.SkillSlots[req.Tree] = map[uint16]uint16{}
		for _, v := range rows {
			state.SkillSlots[req.Tree][v.ID] = v.Slot
		}
		p, e := mergeSkillState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		r, e := json.Marshal(req)
		return p, r, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// applySkillSlotSwaps mutates rows in place, one sequential swap per pair.
//
// The quick bar must not receive a passive skill; palette destinations are
// unrestricted. The guard uses ShortcutCapable rather than Active on purpose:
// the awakening and VP rows routinely have a [type] the export cannot resolve
// (atmage 138/139 read "default", priest 133/134/250/253 have no line at all),
// and those are exactly the rows 自动加点 puts on the bar - treating an
// unresolvable [type] as passive would refuse the whole layout, which is the
// same trap MoveSkill already had to have fixed.
func (s *Service) applySkillSlotSwaps(profession byte, rows []protocol.LearnedSkill, pairs []protocol.SkillSlotSwap) error {
	// 槽位交换语义：页1 autoset 抓包（08:35，source/target 覆盖面板槽 7..31）
	// 长期验证正确；页2 抓包（10-09 22:00）target 全为快捷栏 0..13，source 为面板槽
	// 8..28，同样按槽位对应用（同一源槽被引用多次 = 链式交换，逐对生效）。
	// 实验七：不改变落库语义，只恢复由 2179 受理后的 NOTI19 刷新（见 skill_flow case 2179）。
	bySlot := map[uint16]int{}
	for i, r := range rows {
		if r.Slot < 255 {
			bySlot[uint16(r.Slot)] = i
		}
	}
	capable := func(i int) bool { return s.Learning.index[profession][rows[i].ID].ShortcutCapable() }
	for _, pair := range pairs {
		from, to := uint16(pair.Source), uint16(pair.Target)
		i, ok := bySlot[from]
		if !ok {
			return fmt.Errorf("skill slot total source %d is empty", from)
		}
		j, ok := bySlot[to]
		if to < 14 && !capable(i) {
			return fmt.Errorf("passive skill cannot occupy a shortcut")
		}
		if ok && from < 14 && !capable(j) {
			return fmt.Errorf("passive skill cannot occupy a shortcut")
		}
		if ok {
			rows[j].Slot = from
		}
		rows[i].Slot = to
		delete(bySlot, from)
		bySlot[to] = i
		if ok {
			bySlot[from] = j
		}
	}
	return nil
}

func (s *Service) LearningResponse(role Character, req protocol.SkillPurchase) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	rows, e := s.skillRows(role, state, int(req.Tree))
	if e != nil {
		return nil, e
	}
	byID := map[uint16]protocol.LearnedSkill{}
	for _, v := range rows {
		byID[v.ID] = v
	}
	var changed []protocol.LearnedSkill
	for _, v := range req.Entries {
		row, ok := byID[v.ID]
		if !ok {
			if v.Refund != 1 {
				return nil, fmt.Errorf("learned response missing skill")
			}
			row = protocol.LearnedSkill{ID: v.ID, Slot: 65535}
		}
		changed = append(changed, row)
	}
	p, e := protocol.SkillPurchaseSuccess(req.Tree, state.SkillPoints[req.Tree], state.TechniquePoints[req.Tree], changed)
	if e != nil {
		return nil, e
	}
	v := state.SkillVariations[req.Tree]
	// A third-awakened character's Learn response must always carry the full
	// g1/g2 blocks (empty slots included). A persisted block that is empty on
	// disk decodes as a nil variation section, and the client clears the VP
	// panel on every such response — the "Learn Skill spent my VP" bug.
	if variationUnlocked(&state) {
		fillVariationSlots(&v)
	}
	// ⚠️ 这里的 mode 必须是 **req.Tree（页）**，不能是 req.Mode。
	// 实机（2026-10-09 23:42）auto set 之后的批量加点包：Tree=0 而 Mode=1 ——
	// Mode 是"这次是 auto set 批量"的标志，不是页。用 req.Mode 会发出一帧
	// "页号=0 但 mode=1"自相矛盾的响应，客户端据此不刷新 VP 面板
	// ⇒ 表现就是 auto set 后 enhance 看起来没设置，重登才恢复（登录走
	// VariationRestore，那里 mode=tree 是自洽的，所以重登正常）。
	return protocol.SkillPurchaseVariations(p, req.Tree, v.Intensions, v.Options)
}
