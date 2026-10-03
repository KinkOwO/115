package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"dfolan/internal/world"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"
)

// unlockRefresh re-publishes the actor's detail packets after a reward that
// changed the extended-slot unlock byte.
//
// The armoury draws its padlocks from the byte the native USERINFO1 reader
// stores at character +0x198 (14563d692), so the new value has to arrive in a
// fresh mode 1 addition - no other packet carries it, and the NOTI328 refresh
// only re-reads a state the client already holds. A mode 0 must precede that
// addition, and it resets the client's worn containers, so the worn rows
// follow immediately. This is the same trio dungeonEntryPlan sends on every
// mid-session actor rebuild.
func (w *worldSession) unlockRefresh(role storage.Character) ([]outboundPacket, error) {
	if w.characters == nil {
		return nil, nil
	}
	visual, e := w.characters.EntryBasicProbe(role, [2]byte{})
	if e != nil {
		return nil, e
	}
	addition, e := w.characters.EntryAddition(role)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{
		{"reward_actor_appearance_sent", 0, 2, visual},
		{"reward_actor_addition_sent", 0, 2, addition},
	}
	if worn, e := inventory.WornSpaceUpdate(role.State); e == nil && len(worn) > 0 {
		plan = append(plan, outboundPacket{"reward_worn_visuals_sent", 0, 14, worn})
	}
	return plan, nil
}

func (w *worldSession) finishQuest(r protocol.QuestSubmitRequest) ([]outboundPacket, error) {
	if w == nil || w.quests == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("quest submit before character selection")
	}
	if w.answeredQuests[r.ID] {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, e := (&workflow.QuestService{Store: w.store, Quest: w.quests, Rewards: w.rewards}).Finish(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	result.Role.WireID = w.role.WireID
	active, e := w.quests.Active(ctx, result.Role)
	if e != nil {
		return nil, e
	}
	done, e := w.quests.Completed(ctx, result.Role)
	if e != nil {
		return nil, e
	}
	triggers, e := protocol.QuestTriggers(active)
	if e != nil {
		return nil, e
	}
	completed, e := protocol.CompletedQuests(done)
	if e != nil {
		return nil, e
	}
	experience, e := character.ExperiencePayload(result.Role)
	if e != nil {
		return nil, e
	}
	var plan []outboundPacket
	// The native terminal ACK mutates a live quest object. After a reconnect,
	// restore saved state without dereferencing that already-completed object.
	if result.Applied {
		// 图鉴后两步引导的源奖励为0号物品、数量0，原生CMD34仍会
		// 据此打开空奖励窗。仅对已核实的零奖励引导使用原生完成通知；
		// 若规则变化产生真实奖励，继续走正常结算，不能吞掉奖励展示。
		quietGuide := (r.ID == 21651 || r.ID == 21652) &&
			result.Receipt.Experience == 0 && result.Receipt.Gold == 0 &&
			len(result.Receipt.Items) == 0 && len(result.Receipt.Consumed) == 0 &&
			result.Receipt.UnlockedEquipment == 0
		if quietGuide {
			body, err := protocol.QuestFinishedWithoutRewardWindow(r.ID)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"图鉴零奖励任务完成（原生通知候选1/3）", 0, 1668, body})
		} else {
			ack, err := protocol.QuestFinishedNoItems(r.ID, result.Receipt.Experience)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"quest_finished", 1, 34, ack})
		}
	}
	// Resend the ordinary bag whenever the reward changed it - items OR gold.
	// The gold balance rides the bag as the slot-0/template-0 row (Bag.Rows),
	// so a quest that pays only gold (its [job] item tuples do not match this
	// character) must still refresh it; otherwise the wallet never ticks up in
	// town even though the balance is committed. Live capture 20260911T215854
	// shows quest 3149 crediting 4300 gold with no items and no NOTI13, so the
	// on-screen number stayed put until the next relog.
	if len(result.Receipt.Items) > 0 || len(result.Receipt.Consumed) > 0 || result.Receipt.Gold > 0 {
		accountMaterial := false
		for _, item := range result.Receipt.Items {
			if _, ok := inventory.AccountMaterialSlot(item.Template); ok {
				accountMaterial = true
				break
			}
		}
		if accountMaterial {
			// Account-shared materials never stay in the bag: sweep them into
			// the account storage and precede the list0 snapshot with the
			// list35 storage snapshot so the client harvest adopts them.
			var materials inventory.AccountMaterials
			result.Role, materials, e = sweepAccountMaterials(ctx, w.store, result.Role)
			if e != nil {
				return nil, e
			}
			storageBody, e := accountMaterialSnapshot(materials)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"quest_account_materials_committed", 0, 13, storageBody})
			soulBody, e := radiantSoulSnapshot(materials)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"quest_radiant_souls_committed", 0, 13, soulBody})
		}
		bag, e := inventory.ReadBag(result.Role.State)
		if e != nil {
			return nil, e
		}
		body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"quest_inventory_committed", 0, 13, body})
		if len(bag.PetItems)+len(bag.Special[7]) > 0 {
			petBody, petErr := inventory.PetContainerBody(bag, true)
			if petErr != nil {
				return nil, petErr
			}
			plan = append(plan, outboundPacket{"quest_pet_container_committed", 0, 13, petBody})
		}
	}
	// A reward that opened an extended equipment slot has to be republished:
	// the client keeps showing the padlock until the new unlock byte arrives
	// in a USERINFO1 addition, even though the slot is already saved.
	if result.Receipt.UnlockedEquipment != 0 {
		refresh, e := w.unlockRefresh(result.Role)
		if e != nil {
			return nil, e
		}
		plan = append(plan, refresh...)
	}
	plan = append(plan, outboundPacket{"quest_experience", 0, 37, experience}, outboundPacket{"quest_triggers_updated", 0, 291, triggers}, outboundPacket{"completed_quests_updated", 0, 342, completed})
	w.role, w.level = result.Role, experience[0]
	available, e := w.availableQuestPayload(ctx)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"available_quests_updated", 0, 21, available})
	return plan, nil
}

func (w *worldSession) availableQuestPayload(ctx context.Context) ([]byte, error) {
	ids, e := w.quests.Available(ctx, w.role)
	if e != nil {
		return nil, e
	}
	return protocol.AvailableQuests(w.level, ids)
}

func (w *worldSession) actQuestRefresh(ctx context.Context) ([]outboundPacket, error) {
	active, err := w.quests.Active(ctx, w.role)
	if err != nil {
		return nil, err
	}
	done, err := w.quests.Completed(ctx, w.role)
	if err != nil {
		return nil, err
	}
	triggers, err := protocol.QuestTriggers(active)
	if err != nil {
		return nil, err
	}
	completed, err := protocol.CompletedQuests(done)
	if err != nil {
		return nil, err
	}
	available, err := w.availableQuestPayload(ctx)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{
		{"act_quest_triggers_updated", 0, 291, triggers},
		{"act_completed_quests_updated", 0, 342, completed},
		{"act_available_quests_updated", 0, 21, available},
	}, nil
}

func (w *worldSession) questInteraction(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.quests == nil {
		return nil, fmt.Errorf("quest check requires an owned character")
	}
	if len(p) != 16 || binary.LittleEndian.Uint16(p) != 33 {
		return nil, fmt.Errorf("unsupported quest check request")
	}
	for _, v := range p[4:] {
		if v != 0 {
			return nil, fmt.Errorf("unsupported quest check fields")
		}
	}
	id := binary.LittleEndian.Uint16(p[2:])
	if w.activeDungeon != nil {
		// A story scene ends with SET_QUEST_TRIGGER from its final layer map
		// instead of a boss check (quest 3191, live 2026-09-22): settle the
		// [clear map] objective against the owned run and sync triggers.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		active, e := w.quests.SceneTrigger(ctx, w.role, w.activeDungeon, id)
		if e != nil || active == nil {
			return nil, e
		}
		body, e := protocol.QuestTriggers(active)
		if e != nil {
			return nil, e
		}
		plan := []outboundPacket{{"quest_scene_trigger", 0, 291, body}}
		// [MERGE-20260928-SCENE-CLEAR-COMPLETE] 任务结算成功 = 本次攻略收尾。
		// 这类「[clear map] 剧情副本」没有可击杀的收尾 BOSS（BOSS 设计成不死，
		// 客户端血条显示 Immortal），客户端不会发 CMD117，副本自身的 tryComplete
		// 永远等不到信号 —— 实机 100004786「墨色瘟疫之匣」就是剧情播完不弹结算。
		// 这里以任务触发作为完成依据，补上副本的完成结算。
		w.activeDungeon.MarkSceneCompleted()
		done, e := w.completeDungeon()
		if e != nil {
			return nil, e
		}
		if len(done) > 0 {
			// 主循环只在 dungeonRequest 段发送 plan 时按 "dungeon_clear_enabled"
			// 置位 completionSent（main.go:2745），quest 段绕过了那段。而客户端
			// 收到 clear_enabled 后会立刻发 CMD46 请求结果，dungeonResult 要求
			// Completed() && completionSent —— 不置位就会被拒成
			// "result before committed boss completion"（实机 2026-09-28 04:26:51）。
			w.completionSent = true
		}
		return append(plan, done...), nil
	}
	d, ok := w.quests.Catalog.Quests[uint32(id)]
	if !ok {
		return nil, nil
	}
	if d.Kind == "[reach the range]" {
		r, valid := quest.ReachNPCObjective(d)
		if !valid || (!questLineageShowsReachNPC(d, r.NPC, w.state.Position, w.quests.Catalog) &&
			!questReachNPCAtSourcePlacement(w.service, w.state.Position, r)) {
			return nil, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		applied, e := w.quests.ReachNPCFromClient(ctx, w.role, id, r.NPC)
		if e != nil || !applied {
			return nil, e
		}
		active, e := w.quests.Active(ctx, w.role)
		if e != nil {
			return nil, e
		}
		body, e := protocol.QuestTriggers(active)
		if e != nil {
			return nil, e
		}
		return []outboundPacket{{"quest_npc_reach_objective", 0, 291, body}}, nil
	}
	if d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 {
		return nil, nil
	}
	npc := uint32(d.ObjectiveCells[0].Value)
	// A [sub type] 1 quest is settled by the conversation the client already
	// validated against its own resolved target, which for the three
	// extended-slot quests (649/650/2636) is an NPC that stands in a different
	// town than the raw objective. The positional test stays for the ordinary
	// form, so a request naming a quest whose NPC is genuinely elsewhere is
	// still refused, and the passive ProximityProgress walk keeps its own
	// in-area requirement (it never consults this path).
	communicated := w.communicationQuest == id && w.communicationNPC == npc &&
		w.communicationTown == w.state.Position.Town &&
		w.communicationArea == w.state.Position.Area &&
		time.Now().Before(w.communicationUntil)
	if !communicated && !quest.AllowsRemoteNPCInteraction(d) && !allowsQuestVisibleNPCInteraction(id, npc, w.state.Position, d, w.quests.Catalog) &&
		!allowsQuestPhaseNPCInteraction(w.service, npc, w.state.Position, d, w.quests.Catalog) &&
		!allowsALullPhaseNPCInteraction(w.service, id, npc, w.state.Position, d, w.quests.Catalog) &&
		(w.service == nil || !w.service.HasNPC(w.state.Position, npc)) {
		return nil, fmt.Errorf("quest %d NPC %d is absent from current source area %d/%d", id, npc, w.state.Position.Town, w.state.Position.Area)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := w.quests.MeetNPC(ctx, w.role, id, npc); e != nil {
		return nil, e
	}
	if communicated {
		w.communicationQuest = 0
		w.communicationNPC = 0
	}
	active, e := w.quests.Active(ctx, w.role)
	if e != nil {
		return nil, e
	}
	body, e := protocol.QuestTriggers(active)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"quest_npc_objective", 0, 291, body}}, nil
}

// Prey_01 (6200) declares NPC 8000 both as its objective and in two
// [npc visibility] blocks, but no static town map contains that NPC. The
// current client repeatedly sends CMD33 for 6200 while in Black Market 54/1.
// Accept only that native request in that area; MeetNPC still requires the
// character to own an accepted quest with the matching objective and version.
func allowsQuestVisibleNPCInteraction(id uint16, npc uint32, at storage.WorldPosition, d catalog.QuestDefinition, quests catalog.QuestCatalog) bool {
	// Preserve the user-confirmed 6200 behavior even with the relaxation off.
	if id == 6200 && npc == 8000 && at.Town == 54 && at.Area == 1 && questShowsObjectiveNPCOnAccept(d, npc) {
		return true
	}
	// Live CMD33 in Black Market and the source quest chain prove this form:
	// completing 6356 reveals NPC 100000175, which quest 6357 meets.
	if id == 6357 && npc == 100000175 && at.Town == 54 && at.Area == 1 && questShownByPrerequisiteOnClear(d, npc, quests) {
		return true
	}
	// A quest chain can reveal an NPC several steps before the meeting quest.
	// A source [go guide] in that chain pins the NPC to a town area.
	if questLineageShowsGuidedNPC(d, npc, at, quests, true) {
		return true
	}
	return os.Getenv("DFO_QUEST_VISIBLE_NPC_RELAX") == "1" &&
		(questShowsObjectiveNPCOnAccept(d, npc) || questShownByPrerequisiteOnClear(d, npc, quests) ||
			questLineageShowsGuidedNPC(d, npc, at, quests, false))
}

// A phase map is an area-specific source of NPC placement, but its NPCs are
// not all visible at once. Require the accepted meet quest's objective and
// completion NPC to match a source visibility rule as well as a phase map row.
func allowsQuestPhaseNPCInteraction(service *world.Service, npc uint32, at storage.WorldPosition, d catalog.QuestDefinition, quests catalog.QuestCatalog) bool {
	if service == nil || !service.HasPhaseNPC(at, npc) {
		return false
	}
	return questShowsObjectiveNPCOnAccept(d, npc) || questLineageShowsVisibleNPC(d, npc, at, quests, false, true)
}

// Follow source prerequisites, not numeric quest adjacency. A clear/show rule
// remains effective through later quests until a clear/hide rule supersedes it.
// The guide supplies the area for the normal path; the opt-in path only needs
// the source visibility rule. MeetNPC still validates the accepted quest.
func questLineageShowsGuidedNPC(d catalog.QuestDefinition, npc uint32, at storage.WorldPosition, quests catalog.QuestCatalog, requireGuide bool) bool {
	// A matching source guide proves the current area; the opt-in no-guide
	// path keeps its older, stricter treatment of this quest's clear/hide.
	return questLineageShowsVisibleNPC(d, npc, at, quests, requireGuide, requireGuide)
}

func questLineageShowsVisibleNPC(d catalog.QuestDefinition, npc uint32, at storage.WorldPosition, quests catalog.QuestCatalog, requireGuide, allowCurrentClearHide bool) bool {
	if npc == 0 || npc > 0x7fffffff || len(d.Pending) != 0 ||
		d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 ||
		d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value != int32(npc) ||
		!questCompletionNPCMatches(d, npc) {
		return false
	}
	// The current quest's clear/hide is a future effect, but only let it be
	// ignored when a guide or a source phase-map row proves the current area.
	return questLineageShowsNPC(d, npc, at, quests, requireGuide, allowCurrentClearHide)
}

// A subtype-0 reach objective may name an NPC distinct from its completion
// NPC. Its own clear/hide rule takes effect after this objective completes.
func questLineageShowsReachNPC(d catalog.QuestDefinition, npc uint32, at storage.WorldPosition, quests catalog.QuestCatalog) bool {
	r, ok := quest.ReachNPCObjective(d)
	return ok && r.NPC == npc && questLineageShowsNPC(d, npc, at, quests, true, true)
}

// For an NPC placed by the current area's source map, accept the native
// range trigger only within the quest script's configured extents.
func questReachNPCAtSourcePlacement(service *world.Service, at storage.WorldPosition, r quest.NPCReachObjective) bool {
	if service == nil || r.W <= 0 || r.H <= 0 {
		return false
	}
	position, found := service.NPCPosition(at, r.NPC)
	if !found {
		return false
	}
	dx := int64(at.X) - int64(position[0])
	dy := int64(at.Y) - int64(position[1])
	multiplier := quest.NPCDistanceMultiplier()
	return math.Abs(float64(dx)) <= float64(r.W)*multiplier &&
		math.Abs(float64(dy)) <= float64(r.H)*multiplier
}

func questLineageShowsNPC(d catalog.QuestDefinition, npc uint32, at storage.WorldPosition, quests catalog.QuestCatalog, requireGuide, allowCurrentClearHide bool) bool {
	seen := make(map[uint32]bool)
	var visit func(catalog.QuestDefinition, int) (bool, bool)
	visit = func(current catalog.QuestDefinition, depth int) (bool, bool) {
		if depth > 16 || seen[current.ID] {
			return false, false
		}
		seen[current.ID] = true
		defer delete(seen, current.ID)
		guideFound, guide := questGoGuideTargetsNPC(current.Script.Cells, npc, at)
		shown, found := false, false
		if depth > 0 {
			shown, found = questVisibilityOnClear(current.Script.Cells, npc)
		} else if currentShown, currentFound := questVisibilityOnClear(current.Script.Cells, npc); !allowCurrentClearHide && currentFound && !currentShown {
			// A current quest that hides the NPC is not evidence of a
			// presently visible conversation target.
			return false, false
		}
		bestShown, bestGuide := shown, guide
		for _, parentID := range questSourcePrerequisiteIDs(current.Script.Cells) {
			parent, ok := quests.Quests[parentID]
			if !ok {
				continue
			}
			parentShown, parentGuide := visit(parent, depth+1)
			pathShown := shown
			if !found {
				pathShown = parentShown
			}
			pathGuide := parentGuide
			if guideFound {
				pathGuide = guide
			}
			if pathShown && (!requireGuide || pathGuide) {
				return true, pathGuide
			}
			if pathShown {
				bestShown, bestGuide = true, pathGuide
			}
		}
		return bestShown, bestGuide
	}
	shown, guide := visit(d, 0)
	return shown && (!requireGuide || guide)
}

func questSourcePrerequisiteIDs(cells []pvf.Token) []uint32 {
	for i, c := range cells {
		if c.Type != 3 || c.Text != "[pre required quest]" {
			continue
		}
		var ids []uint32
		for j := i + 1; j < len(cells) && cells[j].Type != 3; j++ {
			if cells[j].Type == 0 && cells[j].Value > 0 {
				ids = append(ids, uint32(cells[j].Value))
			}
		}
		return ids
	}
	return nil
}

func questGoGuideTargetsNPC(cells []pvf.Token, npc uint32, at storage.WorldPosition) (found bool, matches bool) {
	for i, c := range cells {
		if c.Type == 3 && c.Text == "[go guide]" && i+3 < len(cells) &&
			cells[i+1].Type == 0 && cells[i+2].Type == 0 && cells[i+3].Type == 0 &&
			cells[i+3].Value == int32(npc) {
			found = true
			matches = cells[i+1].Value == int32(at.Town) && cells[i+2].Value == int32(at.Area)
		}
	}
	return found, matches
}

// Source [npc] groups can name several NPCs before the next field tag.
func questVisibilityTargetsNPC(cells []pvf.Token, start int, npc uint32) bool {
	for i := start; i < len(cells) && cells[i].Type != 3; i++ {
		if cells[i].Type == 0 && cells[i].Value == int32(npc) {
			return true
		}
	}
	return false
}

func questVisibilityOnClear(cells []pvf.Token, npc uint32) (show bool, found bool) {
	for i, c := range cells {
		if c.Type != 3 || c.Text != "[npc visibility]" {
			continue
		}
		var target, clear bool
		var visibility string
		for j := i + 1; j+1 < len(cells); j++ {
			field := cells[j]
			if field.Type != 3 {
				continue
			}
			if field.Text == "[/npc visibility]" {
				break
			}
			next := cells[j+1]
			switch field.Text {
			case "[npc]":
				target = questVisibilityTargetsNPC(cells, j+1, npc)
			case "[condition]":
				clear = next.Type == 6 && next.Text == "[clear]"
			case "[visibility]":
				if next.Type == 6 {
					visibility = next.Text
				}
			}
		}
		if target && clear && (visibility == "[show]" || visibility == "[hide]") {
			show, found = visibility == "[show]", true
		}
	}
	return show, found
}

// A visible quest NPC can be absent from the static town map because the
// client adds it from quest state. MeetNPC separately verifies ownership and
// accepted state.
func questShowsObjectiveNPCOnAccept(d catalog.QuestDefinition, npc uint32) bool {
	if npc == 0 || npc > 0x7fffffff || len(d.Pending) != 0 || len(d.Script.Cells) < 2 ||
		d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 ||
		d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value != int32(npc) {
		return false
	}
	if !questCompletionNPCMatches(d, npc) {
		return false
	}
	return questVisibilityShows(d.Script.Cells, npc, "[accept]")
}

func questCompletionNPCMatches(d catalog.QuestDefinition, npc uint32) bool {
	for i, c := range d.Script.Cells {
		if c.Type == 3 && c.Text == "[complete npc index]" && i+1 < len(d.Script.Cells) &&
			d.Script.Cells[i+1].Type == 0 && d.Script.Cells[i+1].Value == int32(npc) {
			return true
		}
	}
	return false
}

// A preceding quest can reveal the NPC only after it is cleared. The exported
// quest catalog predates prerequisite-group parsing, so read the source cells
// rather than relying on its empty Prerequisites projection.
func questShownByPrerequisiteOnClear(d catalog.QuestDefinition, npc uint32, quests catalog.QuestCatalog) bool {
	if npc == 0 || npc > 0x7fffffff || len(d.Pending) != 0 ||
		d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 ||
		d.ObjectiveCells[0].Type != 0 || d.ObjectiveCells[0].Value != int32(npc) ||
		!questCompletionNPCMatches(d, npc) {
		return false
	}
	for i, c := range d.Script.Cells {
		if c.Type != 3 || c.Text != "[pre required quest]" {
			continue
		}
		for j := i + 1; j < len(d.Script.Cells) && d.Script.Cells[j].Type != 3; j++ {
			parentID := d.Script.Cells[j].Value
			if d.Script.Cells[j].Type != 0 || parentID <= 0 {
				continue
			}
			parent, ok := quests.Quests[uint32(parentID)]
			if ok && questVisibilityShows(parent.Script.Cells, npc, "[clear]") {
				return true
			}
		}
	}
	return false
}

func questVisibilityShows(cells []pvf.Token, npc uint32, condition string) bool {
	for i, c := range cells {
		if c.Type != 3 || c.Text != "[npc visibility]" {
			continue
		}
		var target, accept, show bool
		for j := i + 1; j+1 < len(cells); j++ {
			field := cells[j]
			if field.Type != 3 {
				continue
			}
			if field.Text == "[/npc visibility]" {
				break
			}
			next := cells[j+1]
			switch field.Text {
			case "[npc]":
				target = questVisibilityTargetsNPC(cells, j+1, npc)
			case "[condition]":
				accept = next.Type == 6 && next.Text == condition
			case "[visibility]":
				show = next.Type == 6 && next.Text == "[show]"
			}
		}
		if target && accept && show {
			return true
		}
	}
	return false
}
