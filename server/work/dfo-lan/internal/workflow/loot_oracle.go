package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/oracleresearch"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Oracle685Service 是活动 685「奥拉蔻的装备模块研究」的**生产**。
//
// ## 一次生产在一个事务里完成三件事
//
//	① 扣掉神秘能量（`[production] [material]`，费用 = 基准 × (已产次数 + 1)）
//	② 推进本自然周的已产次数（周二 00:00 UTC 归零，见 oracleresearch.WeekStartOf）
//	③ 发一封系统邮件：**模块 × 1 + Oracle's Keepsake × 5**
//
// 三件事压在 `SendSystemMailWithCharacterUpdate` 里（与 313 领奖、签到、礼包同一条机制）：
// 状态更新与邮件在同一事务，事件键保证**同一次生产只入账一次**（重放读回既有回执）。
//
// ## 为什么邮件在事务外先算、事务内再验
//
// 产出（模块号）取决于**已产次数**，而次数要在加锁后的事务里读才准。
// 所以这里先乐观地读一次状态算出邮件附件，事务内再复核次数是否仍然一致 ——
// 不一致就拒绝（调用方可以重试），**不**把算错的附件发出去。
type Oracle685Service struct {
	Store   *database.Store
	Catalog *oracleresearch.Catalog
	// Loot 是物品目录，`inventory.Bag.Consume` 要用它校验"这是一件源里的堆叠物"。
	Loot catalog.LootCatalog
	// Bag 是背包槽位策略 + Equipment 是装备目录：组装把成品**直接塞回背包**（不走邮件），
	// 而成品是**装备**（10xxxxxxx）⇒ 必须走 `Bag.AddEquipment`，那条路要这两样。
	Bag       inventory.BagRules
	Equipment *inventory.EquipmentCatalog
}

// Oracle685Receipt 是一次生产的回执（写日志与应答帧都用它）。
type Oracle685Receipt struct {
	WeekStart   int64
	Produced    uint32
	Cost        uint32
	Module      uint32
	ResultItem  uint32
	ResultCount uint32
}

// Produce 执行一次生产。`applied=false` 且 err==nil 表示事件键命中（重放）。
func (s *Oracle685Service) Produce(ctx context.Context, role database.Character, now time.Time,
	sender, text string) (database.Character, Oracle685Receipt, bool, error) {
	var receipt Oracle685Receipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, receipt, false, fmt.Errorf("oracle685 service unavailable")
	}
	// ★★ **必须先用库里那份最新角色**，不能用调用方手里的内存快照。
	// 2026-10-12 实机踩到：事件键里带了"已产次数"，而次数来自内存快照 ⇒ 第一次生产后内存没刷新，
	// 之后每次算出的键**都是同一个** ⇒ 全被幂等拒掉（`applied=false`）、状态永远停在 8、
	// 客户端每次收到拒绝 ⇒ 面板一直弹 "Please try again later."、按钮失效。
	// 判据就在会话日志里：`oracle_production_done applied=True cost=90 produced_after=9` 之后
	// 连着 12 条 `applied=False cost=90 produced_after=9`。
	if fresh, ferr := s.Store.AdminCharacter(ctx, role.ID); ferr == nil {
		role = fresh
	}
	weekStart := oracleresearch.WeekStartOf(now)
	st, err := oracleresearch.ReadState(role.State)
	if err != nil {
		return role, receipt, false, err
	}
	st = st.Normalize(weekStart)
	cost := s.Catalog.ProductionCostFor(st.Produced)
	module, ok := s.Catalog.ModuleAt(st.Produced)
	if !ok {
		return role, receipt, false, fmt.Errorf("oracle685 source declares no module to produce")
	}
	receipt = Oracle685Receipt{
		WeekStart: weekStart, Produced: st.Produced, Cost: cost, Module: module,
		ResultItem: s.Catalog.ProductionResultItem, ResultCount: s.Catalog.ProductionResultCount,
	}
	grants := []database.GrantItem{
		{Template: module, Amount: 1},
		{Template: s.Catalog.ProductionResultItem, Amount: s.Catalog.ProductionResultCount},
	}
	// 附件容器取 0（普通背包）：模块与遗物都是普通堆叠物。
	assets, err := database.SystemMailAssets(0, grants)
	if err != nil {
		return role, receipt, false, err
	}
	// 事件键按「期(周) + 本条第几次」定 ⇒ 同一次生产重放只入账一次；下一次生产自然得到新键。
	key := fmt.Sprintf("oracle685:production:%d:%d", weekStart, st.Produced)

	_, applied, err := s.Store.SendSystemMailWithCharacterUpdate(ctx, role.AccountID, role.ID,
		role.ConfigVersion, key, sender, text, assets,
		func(current database.Character) (json.RawMessage, error) {
			cur, e := oracleresearch.ReadState(current.State)
			if e != nil {
				return nil, e
			}
			cur = cur.Normalize(weekStart)
			if cur.Produced != st.Produced {
				// 乐观读到的次数已经过期（并发点了生产）⇒ 拒绝，绝不发错附件。
				return nil, fmt.Errorf("oracle685 production count moved: %d → %d", st.Produced, cur.Produced)
			}
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, e
			}
			// 先数够不够，再逐件扣（`Consume` 一次一件、扣空即移除该格）。
			have := uint64(0)
			for _, row := range bag.Items {
				if row.Template == s.Catalog.ProductionMaterial {
					have += uint64(row.Amount)
				}
			}
			if have < uint64(cost) {
				return nil, fmt.Errorf("oracle685 needs %d × %d, bag holds %d",
					cost, s.Catalog.ProductionMaterial, have)
			}
			left := cost
			for left > 0 {
				done := false
				for _, row := range bag.Items {
					if row.Template != s.Catalog.ProductionMaterial || row.Amount == 0 {
						continue
					}
					var ce error
					if bag, _, ce = bag.Consume(s.Loot, row.Slot, s.Catalog.ProductionMaterial); ce != nil {
						return nil, ce
					}
					left--
					done = true
					break
				}
				if !done {
					return nil, fmt.Errorf("oracle685 material vanished mid-consume")
				}
			}
			withBag, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, e
			}
			next := cur
			next.Produced = cur.Produced + 1
			return oracleresearch.WriteState(withBag, next)
		})
	if err != nil {
		return role, receipt, false, err
	}
	receipt.Produced = st.Produced + 1
	// 与 313 的 Claim 一样返回**入参那个 role**：状态已在事务里写库，调用方要新的读回来。
	return role, receipt, applied, nil
}

// Oracle685CombineReceipt 是一次「组装」的回执。
type Oracle685CombineReceipt struct {
	SetModule   uint32
	PartModule  uint32
	ResultItem  uint32
	ResultCount uint32
	// ChangedRows 是这次组装**改动过的背包行**（两件模块 + 新装备）。
	// 派发层用它推 N14（`protocol.InventoryUpdate`），客户端立刻看到成品进包。
	ChangedRows [][protocol.CurrentItemRecordSize]byte
	// RowsUsed 是这次组装之后、本自然周已用过的组装槽行（推 2809 板要用它）。
	RowsUsed [3]bool
}

// Combine 执行一次组装：槽位号 → 两件模块 → 查配方 → 各扣 1 → 成品直接进背包。
//
// ## 两个设计选择都有依据（不是随手定的）
//
//  1. **不走邮件**：脚本里 `[production]` 与 `[disjoint stackable]` 都带 `[mail info]`，
//     而 `[combine]` **没有** ⇒ 成品直接进背包。仓库里已有 `Bag.Add(c, rules, id, amount)`。
//  2. **不用事件键**：`[combine]` 没有"每次一条"的语义 —— 同一对模块可以合法地组装多次
//     （玩家本来就持有 2~3 份，实机三次组装用的就是三对不同的模块）。
//     所以走 `SaveCharacterState` 的「apply 是当前状态的纯函数」范式。
//     重放保护依赖客户端的"进行中"标志（`obj+2114`）：发请求前置 1、收到应答才清 0，期间按钮禁用。
//     ⚠️ 这是本文件里**唯一没有幂等键**的写路径，已记进待办（next202 §11）。
func (s *Oracle685Service) Combine(ctx context.Context, role database.Character, row int,
	setSlot, partSlot uint32) (database.Character, Oracle685CombineReceipt, error) {
	var receipt Oracle685CombineReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, receipt, fmt.Errorf("oracle685 service unavailable")
	}
	if row < 0 || row >= 3 {
		return role, receipt, fmt.Errorf("oracle685: 组装行号 %d 越界", row)
	}
	// 解锁门槛（脚本 result 槽的 [args]）：没解锁不给组装（官方：按已产次数顺序解锁）。
	if st, serr := oracleresearch.ReadState(role.State); serr == nil {
		st = st.Normalize(oracleresearch.WeekStartOf(time.Now()))
		if need := s.Catalog.UnlockForRow(row); st.Produced < need {
			return role, receipt, fmt.Errorf("oracle685: 第 %d 行组装槽需要已产 %d 次才解锁（当前 %d）",
				row+1, need, st.Produced)
		}
	}
	setModule, ok := s.Catalog.ModuleAtSlot(setSlot)
	if !ok {
		return role, receipt, fmt.Errorf("oracle685: 未知的套装槽位 %d", setSlot)
	}
	partModule, ok := s.Catalog.ModuleAtSlot(partSlot)
	if !ok {
		return role, receipt, fmt.Errorf("oracle685: 未知的部位槽位 %d", partSlot)
	}
	recipe, ok := s.Catalog.RecipeFor(setModule, partModule)
	if !ok {
		return role, receipt, fmt.Errorf("oracle685: 没有 %d + %d 这条配方", setModule, partModule)
	}
	receipt = Oracle685CombineReceipt{SetModule: setModule, PartModule: partModule,
		ResultItem: recipe.ResultItem, ResultCount: 1}
	// 与 Produce 同一条纪律：先用库里那份最新角色，别用调用方手里的内存快照。
	if fresh, ferr := s.Store.AdminCharacter(ctx, role.ID); ferr == nil {
		role = fresh
	}
	saved, err := s.Store.SaveCharacterState(ctx, role.AccountID, role.ID, role.ConfigVersion,
		func(current database.Character) (json.RawMessage, error) {
			st, e := oracleresearch.ReadState(current.State)
			if e != nil {
				return nil, e
			}
			st = st.Normalize(oracleresearch.WeekStartOf(time.Now()))
			// ★ 每行每周一次：官方文案「A slot will be deactivated after Assembling.」
			// 这是**服务端**的硬门禁 —— 按钮看起来还能点也不会重复扣料发装备。
			//
			// ⚠️ `DFO_ORACLE_ROWS_RESET=on` 是**给调试用的**绕过开关（把本自然周的行状态清掉，
			// 方便反复 A/B 那个「已用」tag）。它是全套 DFO_ORACLE_* 里最后一个还会被留下来的
			// 调试口子 ⇒ 收尾时必须确认未设置（10-12 那次遗留环境变量事故的教训）。
			if strings.EqualFold(strings.TrimSpace(os.Getenv("DFO_ORACLE_ROWS_RESET")), "on") {
				st.RowsUsed = [3]bool{}
			}
			if st.RowsUsed[row] {
				return nil, fmt.Errorf("oracle685: 本自然周第 %d 行组装槽已经用过（周二 09:00 UTC 重置）", row+1)
			}
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, e
			}
			before := bag
			// 先把两件都数够，再动手扣 —— 缺任何一件就整笔拒绝，绝不扣一半。
			for _, need := range []uint32{setModule, partModule} {
				have := uint64(0)
				for _, row := range bag.Items {
					if row.Template == need {
						have += uint64(row.Amount)
					}
				}
				if have == 0 {
					return nil, fmt.Errorf("oracle685: 组装需要模块 %d，背包里一件都没有", need)
				}
			}
			for _, need := range []uint32{setModule, partModule} {
				done := false
				for _, row := range bag.Items {
					if row.Template != need || row.Amount == 0 {
						continue
					}
					var ce error
					if bag, _, ce = bag.Consume(s.Loot, row.Slot, need); ce != nil {
						return nil, ce
					}
					done = true
					break
				}
				if !done {
					return nil, fmt.Errorf("oracle685: 模块 %d 在扣的过程中消失了", need)
				}
			}
			// ★ 成品是**装备**：`Bag.Add` 只收堆叠物（本机实测会以"不是源里的堆叠物"拒绝），
			// 装备必须走 `AddEquipment` —— 与 pickup / moon_rewards / black_purgatory 同一条路。
			var ae error
			if bag, _, ae = bag.AddEquipment(s.Equipment, s.Bag.EquipmentSlots, recipe.ResultItem, 1); ae != nil {
				return nil, ae
			}
			receipt.ChangedRows = inventory.ChangedItemRows(before, bag)
			withBag, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, e
			}
			// 置位「本自然周这一行已用过」，并随角色状态一起落库（与扣料同一事务）。
			st.RowsUsed[row] = true
			receipt.RowsUsed = st.RowsUsed
			return oracleresearch.WriteState(withBag, st)
		})
	if err != nil {
		return role, receipt, err
	}
	return saved, receipt, nil
}

// DisjointPick 是分解请求里的一条：模块**槽位号** + 件数。
type DisjointPick struct {
	Slot  uint32
	Count uint32
}

// Oracle685DisjointReceipt 是一次「分解」的回执。
type Oracle685DisjointReceipt struct {
	// Picks 是这次分解掉的（模块号, 件数），按请求顺序。
	Picks []Oracle685DisjointPick
	// Modules 是分解掉的模块总件数。
	Modules uint32
	// ResultItem / ResultCount 是产出的物品与总数量（= 每件产出 × Modules）。
	ResultItem  uint32
	ResultCount uint32
	// ChangedRows 是这次分解改动过的背包行（推 N14 用）。
	ChangedRows [][protocol.CurrentItemRecordSize]byte
}

// Oracle685DisjointPick 是回执里的一条（模块号 + 件数）。
type Oracle685DisjointPick struct {
	Module uint32
	Count  uint32
}

// Disjoint 执行一次分解：按 (槽位号, 件数) 扣模块，产出 Doom Oracle Box，**走系统邮件**。
//
// 与 Combine 的差别（都有依据，不是随手选）：
//   * **走邮件**：客户端 2414 handler 回包后弹的文案明写 "Items will be sent by mail."，
//     脚本 `[disjoint stackable]` 也带 `[mail info]` ⇒ 与生产同一条机制（事件键幂等）。
//   * **产出是堆叠物**（590015045 = `[Event] Doom Oracle Box (1)`），不是装备。
//
// ⚠️ 事件键用「本自然周第几次分解」：客户端收到应答后会**清空自己的选中列表**
//    （handler 里 `*(obj+2192) = *(obj+2184)`），所以同一份选择不会被重复提交；
//    协议里没有序号，重传无法自证 —— 与 Combine 同一条已知边界。
func (s *Oracle685Service) Disjoint(ctx context.Context, role database.Character,
	picks []DisjointPick, sender, text string) (database.Character, Oracle685DisjointReceipt, bool, error) {
	var receipt Oracle685DisjointReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, receipt, false, fmt.Errorf("oracle685 service unavailable")
	}
	if len(picks) == 0 {
		return role, receipt, false, fmt.Errorf("oracle685: 分解请求是空的")
	}
	// 槽位号 → 模块号；件数必须 > 0。
	grantsPer := s.Catalog.DisjointCount
	total := uint32(0)
	for _, p := range picks {
		if p.Count == 0 {
			return role, receipt, false, fmt.Errorf("oracle685: 分解条目件数为 0（槽 %d）", p.Slot)
		}
		module, ok := s.Catalog.ModuleAtSlot(p.Slot)
		if !ok {
			return role, receipt, false, fmt.Errorf("oracle685: 未知的模块槽位 %d", p.Slot)
		}
		receipt.Picks = append(receipt.Picks, Oracle685DisjointPick{Module: module, Count: p.Count})
		total += p.Count
	}
	receipt.Modules = total
	receipt.ResultItem = s.Catalog.DisjointItem
	receipt.ResultCount = grantsPer * total
	grants := []database.GrantItem{{Template: s.Catalog.DisjointItem, Amount: receipt.ResultCount}}
	assets, err := database.SystemMailAssets(0, grants)
	if err != nil {
		return role, receipt, false, err
	}
	if fresh, ferr := s.Store.AdminCharacter(ctx, role.ID); ferr == nil {
		role = fresh
	}
	// 事件键：本自然周第几次分解（分解次数存在 State 里，与扣料同一事务推进）。
	st0, err := oracleresearch.ReadState(role.State)
	if err != nil {
		return role, receipt, false, err
	}
	st0 = st0.Normalize(oracleresearch.WeekStartOf(time.Now()))
	key := fmt.Sprintf("oracle685:disjoint:%d:%d", st0.WeekStart, st0.Dismantles)

	_, applied, err := s.Store.SendSystemMailWithCharacterUpdate(ctx, role.AccountID, role.ID,
		role.ConfigVersion, key, sender, text, assets,
		func(current database.Character) (json.RawMessage, error) {
			st, e := oracleresearch.ReadState(current.State)
			if e != nil {
				return nil, e
			}
			st = st.Normalize(oracleresearch.WeekStartOf(time.Now()))
			if st.Dismantles != st0.Dismantles {
				return nil, fmt.Errorf("oracle685 dismantle count moved: %d → %d", st0.Dismantles, st.Dismantles)
			}
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, e
			}
			before := bag
			// 先数够再扣，缺任何一件整笔拒绝。
			for _, pick := range receipt.Picks {
				have := uint64(0)
				for _, row := range bag.Items {
					if row.Template == pick.Module {
						have += uint64(row.Amount)
					}
				}
				if have < uint64(pick.Count) {
					return nil, fmt.Errorf("oracle685: 分解需要 %d × %d，背包只有 %d",
						pick.Count, pick.Module, have)
				}
			}
			for _, pick := range receipt.Picks {
				left := pick.Count
				for left > 0 {
					done := false
					for _, row := range bag.Items {
						if row.Template != pick.Module || row.Amount == 0 {
							continue
						}
						var ce error
						if bag, _, ce = bag.Consume(s.Loot, row.Slot, pick.Module); ce != nil {
							return nil, ce
						}
						left--
						done = true
						break
					}
					if !done {
						return nil, fmt.Errorf("oracle685: 模块 %d 在扣的过程中消失了", pick.Module)
					}
				}
			}
			receipt.ChangedRows = inventory.ChangedItemRows(before, bag)
			withBag, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, e
			}
			st.Dismantles = st0.Dismantles + 1
			return oracleresearch.WriteState(withBag, st)
		})
	if err != nil {
		return role, receipt, false, err
	}
	return role, receipt, applied, nil
}
