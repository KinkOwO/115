package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const MaxExpireTime = math.MaxInt32

// boosterEquipmentDurability 解析一件刚发放的装备应带的耐久。
//
// 走的是 EquipmentCatalog.Reward，也就是 AddEquipment（任务奖励 / GM 发放）
// 与掉落共用的那条规则：读源 .equ 的 [durability]。缺该段且部位属于
// durabilityOptional（首饰、称号、辅助装备、魔法石、耳环……）时返回 0 且无错，
// 那本来就是 0；其余情况解析不出来时返回错误，由调用方记日志后按 0 发放——
// 不能因此拒绝开箱，否则"拿不到东西"比"拿到 0 耐久"更糟。
func boosterEquipmentDurability(wear *workflow.WearService, id uint32) (uint16, error) {
	if wear == nil || wear.Catalog == nil {
		return 0, fmt.Errorf("no equipment catalog loaded")
	}
	return wear.Catalog.Reward(id)
}

type BoosterRewardCandidate = catalog.BoosterRewardCandidate
type BoosterRewardPool = catalog.BoosterRewardPool
type BoosterDefinition = catalog.BoosterDefinition

type ItemIndexInfo = catalog.ItemIndexEntry

type BoosterCatalog = catalog.BoosterCatalog

// payAsIsWrappers 是**声明了 [booster info]、但必须原样发给玩家**的模板。
//
// 源里两种东西共用 [booster info]，只能逐个判：
//   - 一次性礼盒 / 服务端载体 —— 玩家打不开或用不上，必须在这里展开。
//     10362480 的文案就是「不实际发放礼盒，以开封状态发放」；
//     10416752「Endkeeper of Order Oath Not Obtained」是「本次没拿到誓约」的空奖
//     占位，展开成什么都没有才是对的；10418293「Doom Oracle Orb (500)」展开成
//     10362429×500，名字里的 (500) 就是它的含量。
//   - 玩家会攒起来自己使用的材料 —— 应当原样发放。
//
// 10415192「Splendor Soul Crystal（光辉灵魂结晶）」是后者：它落在小深渊 maze 1
// 固定掉落表的**材料槽**上，而 maze 0 的同一槽位是 10362432「迷雾工商协会银币」
// —— 那个根本不是盒子，一直按原样发。两张表在这一格上是镜像的（一个给银币、
// 一个给结晶），所以官方给的也是结晶本身，玩家在 Scales UI 里自己消耗。
// 不豁免的话，展开会把它变成 Splendor Soul —— 名字和数量都不是玩家该看到的东西。
//
// 判据只能是具名：全库 37,387 个 [booster] 可堆叠物里 30,230 个没有 stack_limit，
// 其中既有真礼盒（box_08sealed / booster_gate* / package_*）也有材料，
// 「能不能堆叠」和 stackable_type 字符串都分不开这两类。所以这里逐个列并写明依据。
var payAsIsWrappers = map[uint32]string{
	10415192: "小深渊 maze 1 固定表的材料槽（maze 0 的同槽位是 10362432 银币，本就不是盒子）；官方发结晶本身，玩家自己使用",
}

// boosterBoxSource 把已加载的礼包目录接到奖励展开上：奖励表发的是外层包装，
// 这里回答两个问题 —— 开一层会出什么，以及某个模板到底是不是真物品。
type boosterBoxSource struct{ catalog *BoosterCatalog }

func (s boosterBoxSource) RewardBox(template uint32) (loot.RewardBox, bool) {
	if s.catalog == nil {
		return loot.RewardBox{}, false
	}
	if _, exempt := payAsIsWrappers[template]; exempt {
		return loot.RewardBox{}, false
	}
	def, ok := s.catalog.Definitions[template]
	if !ok || len(def.Pools) == 0 {
		return loot.RewardBox{}, false
	}
	out := loot.RewardBox{Pools: make([]loot.RewardBoxPool, 0, len(def.Pools))}
	for _, p := range def.Pools {
		pool := loot.RewardBoxPool{
			Draws:      p.DrawCount,
			Candidates: make([]loot.RewardBoxCandidate, 0, len(p.Candidates)),
		}
		for _, c := range p.Candidates {
			pool.Candidates = append(pool.Candidates, loot.RewardBoxCandidate{
				Template: c.Template, Weight: c.Weight, Count: c.Count,
			})
		}
		out.Pools = append(out.Pools, pool)
	}
	return out, true
}

// Item 覆盖装备与可堆叠物两类：一层展开既会出装备（本转职的基础装备），也会出
// 材料、虚拟道具、书和罐子。目录里没有的模板是源留的「空槽」，不是可发放物品。
func (s boosterBoxSource) Item(template uint32) bool {
	if s.catalog == nil {
		return false
	}
	item, ok := s.catalog.Items[template]
	if !ok {
		return false
	}
	return item.Kind == "equipment" || item.Kind == "stackable"
}

// Container recognises the box family. The catalog is the primary witness: it is
// built from the source's own [booster info] section, so membership means the
// template really is a container - including the ones whose stackable type does
// not advertise it. 10362480 is typed [virtual] and still declares [booster info]
// with [instantly open]; judging it by its type string let it reach the ground as
// a box nobody can open. The type string is kept as a fallback so a catalog
// generated before the structural export keeps behaving the way it did.
func (s boosterBoxSource) Container(template uint32) bool {
	if s.catalog == nil {
		return false
	}
	// 豁免的模板必须**两边都不认**：只从 RewardBox 摘掉会让它掉进 Container
	// 那支，被当成「本 build 打不开的盒子」直接丢掉。
	if _, exempt := payAsIsWrappers[template]; exempt {
		return false
	}
	if _, ok := s.catalog.Definitions[template]; ok {
		return true
	}
	item, ok := s.catalog.Items[template]
	if !ok {
		return false
	}
	// `[booster selection]` 是**选择箱**，不是「打不开的盒子」：模板自带
	// [booster select category] / [equipment] 候选表，玩家在客户端自己打开并从中挑一件，
	// 服务端既不展开也不该拦它 —— 它在 index.go 里是 typeConsumable，能正常落地进背包。
	// 判成容器会让整条奖励分支被判「unopenable」而**静默不发**
	// （2026-10-01 实测：千海天深渊奖励表里 10401416/10401429/10417539/10417540/
	// 10417548/10417549/10420581/10420594 这 8 个选择箱就是这样被丢掉的）。
	// 只按类型字符串排除：带 [booster info] 的真礼包已在上面按 Definitions 返回 true。
	if item.StackableType == "[booster selection]" {
		return false
	}
	return strings.Contains(strings.ToLower(item.StackableType), "booster")
}

type boosterEventStore interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
	CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error)
}

// 礼包复用本地原子契约事务：扣物品、续期和回执同时提交，重试只读回执。
func commitBoosterEvent(ctx context.Context, store boosterEventStore, role storage.Character, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	decode := func(receipt json.RawMessage) ([]storage.CashPremiumActivation, error) {
		var outcome struct {
			ActivatedPremiums []struct {
				Type     uint8 `json:"type"`
				Duration int64 `json:"duration"`
			} `json:"activated_premiums"`
		}
		if err := json.Unmarshal(receipt, &outcome); err != nil {
			return nil, err
		}
		var rewards []storage.CashPremiumActivation
		for _, p := range outcome.ActivatedPremiums {
			rewards = append(rewards, storage.CashPremiumActivation{Type: p.Type, DurationSecond: p.Duration})
		}
		return rewards, nil
	}
	if atomicStore, ok := store.(interface {
		CommitCharacterPremiumEvent(context.Context, int64, int64, string, string, string, func(storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error)) (storage.Character, bool, error)
	}); ok {
		return atomicStore.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, model, func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
			raw, receipt, err := apply(current)
			if err != nil {
				return nil, nil, nil, err
			}
			rewards, err := decode(receipt)
			return raw, receipt, rewards, err
		})
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, receipt, err := apply(current)
		if err != nil {
			return nil, nil, err
		}
		rewards, err := decode(receipt)
		if err != nil {
			return nil, nil, err
		}
		if len(rewards) > 0 {
			return nil, nil, fmt.Errorf("礼包契约需要支持原子续期的存储服务")
		}
		return raw, receipt, nil
	})
}

func (w *worldSession) openBoosterItem(
	ctx context.Context,
	store boosterEventStore,
	wear *workflow.WearService,
	lootSvc *loot.Service,
	boosterCat *BoosterCatalog,
	choices odysseyWeaponChoices,
	plaintext, raw []byte,
) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("booster open requires selected character in town")
	}
	if w.activeDungeon != nil {
		return nil, fmt.Errorf("booster open requires character in town")
	}

	req, err := protocol.DecodeBoosterUseRequest(plaintext)
	if err != nil {
		return nil, err
	}

	// 1. Check current inventory to see what box is in slot
	curBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	var boxItem *inventory.BagItem
	for i := range curBag.Items {
		if curBag.Items[i].Slot == req.Slot {
			boxItem = &curBag.Items[i]
			break
		}
	}
	if boxItem == nil {
		return nil, fmt.Errorf("booster box not found at slot %d", req.Slot)
	}

	// If this is the Odyssey weapon creation box and Odyssey rewards are enabled, delegate to existing handler
	if boxItem.Template == 10417789 && len(req.Selections) > 0 && odysseyRewardsEnabled() {
		sel := protocol.WeaponBoxSelection{
			Slot:     req.Slot,
			Category: [2]byte{byte(req.Category), byte(req.Category >> 8)},
			Template: req.Selections[0],
		}
		if choices.allows(sel) {
			if realStore, ok := store.(*storage.Store); ok {
				saved, plan, e := selectOdysseyWeapon(ctx, realStore, wear, w.role, choices, sel)
				if e == nil {
					w.role = saved
				}
				return plan, e
			}
		}
	}

	// 1b. Source selection boxes ([booster select category]). Their contents are
	// picked by the player, and by design they are absent from the fixed-content
	// booster catalog. Without this branch a pick-a-item box falls through to the
	// random-pool path, which answers "booster %d has no reward pool defined" and
	// leaves the client showing its generic failure notice ("The target inventory
	// is full") — the pick list never appears.
	//
	// Validation happens here; the actual hand-out stays on the verified
	// destination logic below (avatar / creature / equipment / stackable), because
	// a box may mix them (10335328 carries gear plus 1000/10000-strong stacks).
	// The Odyssey creation weapon box keeps its own handler above.
	if w.selectionBoxes != nil && boxItem.Template != 10417789 {
		if _, ok := w.selectionBoxes.ByTemplate(boxItem.Template); ok {
			if len(req.Selections) == 0 {
				// The client asks once before it can show the pick list. Answering
				// with a failure keeps that list from ever appearing, so answer with
				// an empty success and grant nothing: the pick arrives in a second
				// request, which is what the validation below guards.
				log.Printf("selection box %d at slot %d asked without a pick (category=%d): awaiting client selection", boxItem.Template, req.Slot, req.Category)
				return []outboundPacket{{"selection_box_awaiting_pick", 1, 160, protocol.BoosterOpenSuccess(boxItem.Template, req.Slot, nil)}}, nil
			}
			category := [2]byte{byte(req.Category), byte(req.Category >> 8)}
			items, missing, checked := w.selectionBoxes.Resolve(boxItem.Template, category, req.Selections)
			switch {
			case !checked:
				log.Printf("selection box %d: category %v has no exported item set (unmodelled content block or unknown category); the pick goes down the generic path", boxItem.Template, category)
			case len(missing) > 0:
				// 导出源来自 client-build/Script.inner.pvf，而客户端加载自己的 Script.pvf
				// （两份不是同一个构建），客户端 UI 给出的选择可能不在导出列表里。只记录，
				// 不拒绝——严格校验会拒掉客户端合法给出的选择。
				log.Printf("selection box %d: pick(s) %v outside the exported source range for category %v (client PVF may differ); granting anyway", boxItem.Template, missing, category)
			default:
				log.Printf("selection box %d: %d pick(s) matched the exported source range in category %v", boxItem.Template, len(items), category)
			}
		}
	}

	// 2. Direct contract item activation (e.g. Tactician, Conqueror, Growth, Cube, NeoPremium)
	if contract, isContract := cashshop.ResolveContract(boxItem.Template); isContract {
		if req.Amount != 1 || len(req.Selections) != 0 {
			return nil, fmt.Errorf("契约道具必须单独使用一件")
		}
		eventKey := fmt.Sprintf("contract-use:%d:%x", w.role.ID, sha256.Sum256(raw))
		saved, _, err := commitBoosterEvent(ctx, store, w.role, eventKey, "contract-use-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, err
			}
			boxIdx := -1
			for i, it := range b.Items {
				if it.Slot == req.Slot && it.Template == boxItem.Template {
					boxIdx = i
					break
				}
			}
			if boxIdx < 0 {
				return nil, nil, fmt.Errorf("contract item at slot %d not found", req.Slot)
			}
			if protocol.StoredItemExpired(b.Items[boxIdx].ExpireTime, time.Now().Unix()) {
				return nil, nil, fmt.Errorf("契约道具已过期")
			}
			if b.Items[boxIdx].Amount > 1 {
				b.Items[boxIdx].Amount--
			} else {
				b.Items = append(b.Items[:boxIdx], b.Items[boxIdx+1:]...)
			}
			rawBag, err := inventory.SaveBag(current.State, b)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(map[string]any{"activated_premiums": []map[string]any{{"type": contract.Type, "duration": contract.DurationSecond}}})
			return rawBag, receipt, err
		})
		if err != nil {
			return nil, err
		}

		recorded, err := store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, eventKey)
		if err != nil {
			return nil, err
		}
		var receipt struct {
			Premiums []storage.CashPremium `json:"premiums"`
		}
		if err = json.Unmarshal(recorded, &receipt); err != nil {
			return nil, err
		}

		w.role = saved
		finalBag, err := inventory.ReadBag(saved.State)
		if err != nil {
			return nil, err
		}

		var plan []outboundPacket
		mainRows := inventory.ChangedItemRows(curBag, finalBag)
		mainUpdate, err := protocol.InventoryUpdate(mainRows)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"contract_inventory_updated", 0, 14, mainUpdate})
		// 即时 NOTI66 与客户端闪退强相关（2026-09-27 实机确认），契约道具
		// 直接使用路径与商城购买、开箱共用 DFO_CONTRACT_PURCHASE_CRASH_FIX
		// 开关；修复开启（默认）时契约仍落库，选角时经 premiums_restored 恢复。
		if !contractPurchaseCrashFixEnabled() {
			for _, premium := range receipt.Premiums {
				if remaining := premium.EndTime - time.Now().Unix(); remaining > 0 {
					notice, err := protocol.PremiumActivationNotice(premium.Type, remaining)
					if err != nil {
						return nil, err
					}
					plan = append(plan, outboundPacket{"contract_special_item_noti", 0, 66, notice})
				}
			}
		}
		plan = append(plan, outboundPacket{"contract_use_ack", 1, 160, protocol.BoosterOpenSuccess(boxItem.Template, req.Slot, nil)})
		return plan, nil
	}

	// 3. Transactional execution of booster opening
	eventKey := fmt.Sprintf("booster-open:%d:%x", w.role.ID, sha256.Sum256(raw))
	type activatedPremium struct {
		Type     uint8 `json:"type"`
		Duration int64 `json:"duration"`
	}
	type outcome struct {
		BoxTemplate       uint32                        `json:"box_template"`
		Granted           []protocol.BoosterGrantedItem `json:"granted"`
		HasAvatars        bool                          `json:"has_avatars"`
		HasCreatures      bool                          `json:"has_creatures"`
		ActivatedPremiums []activatedPremium            `json:"activated_premiums,omitempty"`
		Premiums          []storage.CashPremium         `json:"premiums,omitempty"`
	}
	var res outcome

	saved, applied, err := commitBoosterEvent(ctx, store, w.role, eventKey, "booster-open-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		b, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		boxIdx := -1
		for i, it := range b.Items {
			if it.Slot == req.Slot {
				boxIdx = i
				break
			}
		}
		if boxIdx < 0 {
			return nil, nil, fmt.Errorf("box at slot %d not found", req.Slot)
		}
		box := b.Items[boxIdx]
		if box.Amount < req.Amount {
			return nil, nil, fmt.Errorf("insufficient box amount")
		}

		boxTemplate := box.Template
		boxHasExp := box.ExpireTime != 0

		// Determine items to grant
		var toGrant []protocol.BoosterGrantedItem
		if len(req.Selections) > 0 {
			for _, selTpl := range req.Selections {
				tpl := selTpl
				if tpl == 42 {
					tpl = 1
				}
				toGrant = append(toGrant, protocol.BoosterGrantedItem{
					Template: tpl,
					Count:    req.Amount,
				})
			}
		} else if boosterCat != nil {
			def, ok := boosterCat.Definitions[boxTemplate]
			if !ok || len(def.Pools) == 0 {
				return nil, nil, fmt.Errorf("booster %d has no reward pool defined", boxTemplate)
			}
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			for _, pool := range def.Pools {
				picks := pool.Pick(r)
				for _, p := range picks {
					tpl := p.Template
					if tpl == 42 {
						tpl = 1
					}
					toGrant = append(toGrant, protocol.BoosterGrantedItem{
						Template: tpl,
						Count:    p.Count * req.Amount,
					})
				}
			}
		}
		if len(toGrant) == 0 {
			return nil, nil, fmt.Errorf("no items granted from booster %d", boxTemplate)
		}

		// Consume the box
		if box.Amount > req.Amount {
			b.Items[boxIdx].Amount -= req.Amount
		} else {
			b.Items = append(b.Items[:boxIdx], b.Items[boxIdx+1:]...)
		}

		hasAvatars := false
		hasCreatures := false
		var activatedPremiums []activatedPremium

		// Avatar option mapping if selections had options
		optMap := make(map[uint32][]byte)
		for _, ao := range req.AvatarOptions {
			optMap[ao.Template] = append(optMap[ao.Template], ao.Option)
		}

		// Award items
		for _, g := range toGrant {
			// Destination 0: Contract Item (Direct activation without adding placeholder item to bag)
			if contract, isContract := cashshop.ResolveContract(g.Template); isContract {
				activatedPremiums = append(activatedPremiums, activatedPremium{
					Type:     contract.Type,
					Duration: contract.DurationSecond * int64(g.Count),
				})
				continue
			}

			var (
				kind     string
				itemPath string
			)
			if boosterCat != nil {
				if info, ok := boosterCat.Items[g.Template]; ok {
					kind = info.Kind
					itemPath = info.Path
				}
			}
			if kind == "" && lootSvc != nil {
				if info, ok := lootSvc.Catalog.Items[g.Template]; ok {
					kind = info.Kind
				}
			}
			if kind == "" && wear != nil && wear.Catalog != nil {
				if def, err := wear.Catalog.Definition(g.Template); err == nil {
					if strings.Contains(def.Path, "avatar") {
						kind = "avatar"
					} else if strings.Contains(def.Path, "creature") {
						kind = "equipment"
					} else {
						kind = "equipment"
					}
					itemPath = def.Path
				}
			}

			// Destination 1: Avatar (kind == "avatar" or path contains "avatar" or option specified)
			// 装扮目录有两种命名：avatar/ 与 at_avatar/；只认 "/avatar/" 会漏掉后者
			// （实机诊断 selection_box_audit_test.go 时发现），让它落进装备分支。
			isAvatar := kind == "avatar" || strings.Contains(itemPath, "avatar")
			if !isAvatar && len(req.AvatarOptions) > 0 {
				for _, ao := range req.AvatarOptions {
					if ao.Template == g.Template {
						isAvatar = true
						break
					}
				}
			}
			if isAvatar {
				hasAvatars = true
				occupied := map[uint16]bool{}
				if b.Special != nil {
					for _, av := range b.Special[1] {
						occupied[av.Slot] = true
					}
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(0); s < protocol.AvatarInventorySlots(b.AvatarExpansion); s++ {
						if !occupied[s] {
							occupied[s] = true
							if b.Special == nil {
								b.Special = map[byte][]inventory.BagEquipment{}
							}
							var per uint32
							if boxHasExp {
								per = MaxExpireTime
							}
							dur := uint16(0)
							if opts, ok := optMap[g.Template]; ok && len(opts) > 0 {
								dur = uint16(opts[0])
								optMap[g.Template] = opts[1:]
							}
							b.Special[1] = append(b.Special[1], inventory.BagEquipment{
								Slot:       s,
								Template:   g.Template,
								Durability: dur,
								Period:     per,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("avatar inventory full")
					}
				}
				continue
			}

			if kind == "equipment" && wear != nil && wear.Catalog != nil {
				gearKind, kindErr := wear.Catalog.EquipmentKind(g.Template)
				if kindErr == nil && inventory.IsPetGear(gearKind) {
					dur, durErr := boosterEquipmentDurability(wear, g.Template)
					if durErr != nil {
						return nil, nil, durErr
					}
					for cnt := uint32(0); cnt < g.Count; cnt++ {
						b, _, err = b.AddPetGear(inventory.BagEquipment{Template: g.Template, Durability: dur})
						if err != nil {
							return nil, nil, err
						}
					}
					hasCreatures = true
					continue
				}
			}
			// Destination 2: Creature (path contains "equipment/creature/")
			if strings.Contains(itemPath, "equipment/creature/") {
				hasCreatures = true
				occupied := map[uint16]bool{}
				if b.Special != nil {
					for _, cr := range b.Special[7] {
						occupied[cr.Slot] = true
					}
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(0); s < 140; s++ {
						if !occupied[s] {
							occupied[s] = true
							if b.Special == nil {
								b.Special = map[byte][]inventory.BagEquipment{}
							}
							b.Special[7] = append(b.Special[7], inventory.BagEquipment{
								Slot:     s,
								Template: g.Template,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("creature inventory full")
					}
				}
				continue
			}

			// Destination 3: Regular Equipment (kind == "equipment")
			if kind == "equipment" {
				// 耐久必须取源 .equ 的 [durability]，与 AddEquipment（任务/GM 发放）
				// 和掉落完全同一条规则。这里曾经写死 0，客户端会把刚开出来的
				// 装备当成"耐久 0 / 已损坏"：面板显示 0/60，穿上也不生效
				// （2026-09-22 玩家报告：开箱出的上衣 100051398、下装
				// 100101272/273/275 耐久全为 0）。源文件里这四件的
				// [durability] 分别是 60/50/50/50。
				dur, durErr := boosterEquipmentDurability(wear, g.Template)
				if durErr != nil {
					log.Printf("booster grant %d: durability unresolved (%v); granted at 0", g.Template, durErr)
				}
				occupied := map[uint16]bool{}
				for _, eq := range b.Equipment {
					occupied[eq.Slot] = true
				}
				for _, it := range b.Items {
					occupied[it.Slot] = true
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(9); s <= 64; s++ {
						if !occupied[s] {
							occupied[s] = true
							b.Equipment = append(b.Equipment, inventory.BagEquipment{
								Slot:       s,
								Template:   g.Template,
								Durability: dur,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("equipment inventory full")
					}
				}
				continue
			}

			// Destination 4: Stackable (kind == "stackable")
			if kind != "stackable" {
				return nil, nil, fmt.Errorf("item %d kind %s cannot be delivered as stackable", g.Template, kind)
			}
			var exp uint32
			if boxHasExp {
				exp = MaxExpireTime
			}
			if lootSvc != nil {
				// Ensure item is registered in catalog if missing
				if _, ok := lootSvc.Catalog.Items[g.Template]; !ok {
					stType := "[etc]"
					if boosterCat != nil {
						if info, ok := boosterCat.Items[g.Template]; ok && info.StackableType != "" {
							stType = info.StackableType
						}
					}
					lootSvc.Catalog.Items[g.Template] = catalog.LootItem{
						ID:            g.Template,
						Kind:          "stackable",
						StackableType: stType,
						StackLimit:    1000,
					}
				}
				var addErr error
				b, _, addErr = b.Add(lootSvc.Catalog, lootSvc.BagRules, g.Template, g.Count, exp)
				if addErr != nil {
					return nil, nil, addErr
				}
			}
		}

		receiptData := outcome{
			BoxTemplate:       boxTemplate,
			Granted:           toGrant,
			HasAvatars:        hasAvatars,
			HasCreatures:      hasCreatures,
			ActivatedPremiums: activatedPremiums,
		}
		receiptBytes, err := json.Marshal(receiptData)
		if err != nil {
			return nil, nil, err
		}

		rawBag, err := inventory.SaveBag(current.State, b)
		if err != nil {
			return nil, nil, err
		}
		return rawBag, receiptBytes, nil
	})
	if err != nil {
		return nil, err
	}

	receiptBytes, err := store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, eventKey)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(receiptBytes, &res); err != nil {
		return nil, err
	}

	w.role = saved
	finalBag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}

	// 3. Build response packets
	ack := protocol.BoosterOpenSuccess(res.BoxTemplate, req.Slot, res.Granted)

	var plan []outboundPacket

	mainRows := inventory.ChangedItemRows(curBag, finalBag)

	mainUpdate, err := protocol.InventoryUpdate(mainRows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"booster_main_inventory_updated", 0, 14, mainUpdate})

	if res.HasAvatars && len(finalBag.Special[1]) > 0 {
		avatarPayload, err := inventory.EquipmentPayload(1, finalBag.Special[1], false)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"booster_avatar_inventory_updated", 0, 14, avatarPayload})
	}
	if (res.HasCreatures || len(finalBag.PetItems) > 0) && len(finalBag.Special[7])+len(finalBag.PetItems) > 0 {
		creaturePayload, err := inventory.PetContainerBody(finalBag, false)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"booster_creature_inventory_updated", 0, 14, creaturePayload})
	}

	// 回执中的绝对到期时间只用于落账；客户端接收剩余秒数。
	// 即时 NOTI66 与客户端闪退强相关（2026-09-27 实机确认），booster 开箱
	// 路径与商城购买、契约道具使用共用 DFO_CONTRACT_PURCHASE_CRASH_FIX 开关；
	// 修复开启（默认）时契约仍落库，选角时经 premiums_restored 恢复。
	if !contractPurchaseCrashFixEnabled() {
		for _, p := range res.Premiums {
			if remaining := p.EndTime - time.Now().Unix(); remaining > 0 {
				notice, err := protocol.PremiumActivationNotice(p.Type, remaining)
				if err != nil {
					return nil, err
				}
				plan = append(plan, outboundPacket{"booster_special_item_noti", 0, 66, notice})
			}
		}
	}

	plan = append(plan, outboundPacket{"booster_open_ack", 1, 160, ack})

	_ = applied
	_ = strconv.Itoa
	return plan, nil
}
