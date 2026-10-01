package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// applyCreationAdvancement 按建号请求把转职槽落账。创建与老角色修复共用这一处判定，
// 避免两套条件打架：pilot 开关、12 字节 option 布局、option[8] 的槽位、以及该槽在源里
// 确实有成长段（AdvancementGrowth 非空）。返回是否真的落了账。
func (s *Service) applyCreationAdvancement(initial *State, req protocol.CreateRequest, prof catalog.Profession) bool {
	applied := false
	if s.Rules.AllJobsPilot && len(req.Options) == 12 && req.Options[8] != 0 {
		adv := req.Options[8]
		if len(prof.AdvancementGrowth[adv]) > 0 {
			initial.Advancement, initial.AllJobsPilot = adv, true
			applied = true
		}
	}
	if s.Rules.SwordmasterPilot && req.Profession == 0 && len(req.Options) == 12 && req.Options[8] == 1 && (req.Options[10] == 0 || req.Options[10] == 2) {
		if len(prof.SwordmasterGrowth) > 0 {
			initial.Advancement, initial.SwordmasterPilot = 1, true
			applied = true
		}
	}
	return applied
}

// CreationPreview 报告显式选定的角色"按源数据还缺什么"，本身不落盘（调用方决定是否写回）。
//
// 覆盖两件事，都是 2026-09-20 之前那批角色缺的：
//  1. 转职落账：建号请求 option[8] 选定的槽位当时没有写进 state.Advancement（启动链
//     用的是不含 growtype 分段数据的角色目录）。**只在当前仍是未转职（advancement==0）
//     时**按建号请求补，已转职的角色一律不动 —— 不重置已有进度。
//  2. 初始穿戴：按补好之后的 advancement 从源 [create equipment list] 取同一转职槽的
//     穿戴，只补当前没有的槽位；已有同槽穿戴一律保留。
//
// 返回新的 state JSON（无变化时为 nil）与人类可读的变更说明。技能不需要落库：
// 转职段起始技能与退点下限由 automaticSkills/knownSkills 在运行时按 advancement 现算。
func (s *Service) CreationPreview(role Character) (json.RawMessage, []string, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, nil, fmt.Errorf("角色配置版本与当前目录不一致：%s", role.ConfigVersion)
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, nil, e
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	if !ok {
		return nil, nil, fmt.Errorf("角色职业 %d 不在当前目录里", role.Profession)
	}
	var changes []string
	if state.Advancement == 0 && len(role.Request) > 0 {
		if req, e := protocol.DecodeCreateRequest(role.Request); e == nil {
			before := state.Advancement
			if s.applyCreationAdvancement(&state, req, prof) && state.Advancement != before {
				changes = append(changes, fmt.Sprintf("advancement %d -> %d（按建号请求 option[8]=%d，all_jobs_pilot=%v swordmaster_pilot=%v）",
					before, state.Advancement, req.Options[8], state.AllJobsPilot, state.SwordmasterPilot))
			}
		}
	}
	projected := s.creationWorn(prof, state.Advancement, state.Level)
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	occupied := map[uint16]bool{}
	for _, w := range bag.Worn {
		occupied[w.Slot] = true
	}
	added := 0
	for _, w := range projected {
		if occupied[w.Slot] {
			continue
		}
		bag.Worn = append(bag.Worn, w)
		occupied[w.Slot] = true
		added++
		changes = append(changes, fmt.Sprintf("worn slot %d <- 模板 %d（耐久 %d）", w.Slot, w.Template, w.Durability))
	}
	if len(changes) == 0 {
		return nil, nil, nil
	}
	if added > 0 {
		state.EquipmentPending = false
	}
	raw, e := json.Marshal(state)
	if e != nil {
		return nil, nil, e
	}
	if added > 0 {
		raw, e = inventory.SaveBag(raw, bag)
		if e != nil {
			return nil, nil, e
		}
	}
	return raw, changes, nil
}
