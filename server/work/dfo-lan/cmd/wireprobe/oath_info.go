package main

// noti 2838 ENDKEEPER_OF_ORDER_REWARD：向客户端下发「引子 / 誓约」的两个档位。
//
// 为什么必须发（见 docs/protocol/endkeeper-of-order-primer-20260926.md §20）：
//
//	客户端单例 qword_14E6388B8 的构造器 sub_1406560D0 里硬写
//
//	    *(_DWORD *)(a1 + 88) = 72;   // primer
//	    *(_DWORD *)(a1 + 92) = 72;   // oath
//
//	而 noti 2838 的解析器 sub_140656A00 正是用 8 字节载荷（2 × u32）写这两个字段：
//
//	    v3 = 72; v4 = 72;
//	    sub_146EA0BE0(&v3, 8);        // 载荷 [0:4) -> v3, [4:8) -> v4
//	    *(_DWORD *)(v0 + 88) = v3;
//	    *(_DWORD *)(v0 + 92) = v4;
//
//	脚本（contents/2026/endkeeperoforder/.../Primer_Proc.act cell 6–9）把它们读成
//
//	    c:primer_rarity_progress_max = getPrimerGrade()   // = 单例 +88
//	    c:oath_rarity_progress_max   = getOathGrade()     // = 单例 +92
//
//	再用 max(oath, primer) 当**下标**与**闸门**：
//
//	  * 收尾动作 `[SET GROUP ACTION] end (max-40)` —— 而 `o:hp_limit` / `o:delay_time`
//	    都只有 8 项 ⇒ 合法下标 0..7 ⇒ max ∈ {40..47, 70, 71}；
//	  * `END_TRIGGER` 要 `now >= max` **两侧都成立**才开 `DIE_TRIGGER`；
//	  * 通往隐藏 BOSS 的 `summon_orthaire` 被包在 `now == max` 里。
//
//	72 越出这个值域 ⇒ 下标 32 越界 ⇒ 收尾动作选不中、不放结束动画、不 `DESTROY`、
//	**客户端永不发 C2S 39**；同时 `oath_max == 45 → nox_is_orthaire` 等四条选路分支
//	全不成立 ⇒ 奥尔泰尔永不登场。一个哨兵同时堵死三件事。
//
// 实机定案（2026-09-27 07:1x，四轮差分注入，服务端兜底开着以便退出）：
//
//	注入 (45,45) -> primer_max=45 oath_max=45，探针实时读数 45/45，c:nox_index=109019264
//	注入 (71,45) -> 71/45                                  c:nox_index=109019264
//	注入 (45,71) -> 45/71                                  c:nox_index=0
//	注入 (71,71) -> 71/71                                  c:nox_index=0
//
//	⇒ [0:4) = primer、[4:8) = oath，4/4 与实时 getter 逐字吻合；且 **只有 oath=45**
//	  会把 `c:nox_index` 选成 109019264（orderchroniclerorthaire）= 第四档「太初」的隐藏 BOSS。
//
// 档位值域（八档，与 scale_primer.mob 的 `Primer_00..07` 一一对应）：

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// oathGradeTiers 是脚本真正接受的八个档位（其它值一律让 `max-40` 越界）。
var oathGradeTiers = map[uint16]string{
	40: "normal",
	41: "rare",
	42: "unique",
	43: "legendary",
	44: "epic",
	45: "primitive",
	70: "rainbow1",
	71: "rainbow2",
}

// 默认档位**不**是一个常数，也**不**看装备。它由「征兆」决定（业主 2026-09-27 定调
// B1，见 cmd/wireprobe/omen_state.go）：征兆**集齐四档并在天平结算过**之后，下一场
// 下发 oath=45（第四档「太初」，唯一召唤奥尔泰尔的档），通关时兑现并清零。
//
// 曾经恒发 45：隐藏 BOSS（代表必出太初）场场登场。之后改成按穿戴装备算，但客户端
// **脱不下誓约槽**（实机 4 次 move 全是穿进、无一次被拒）⇒ 穿上 primeval 誓约就永久
// 45 ⇒ 又变回场场出。所以装备表只留作诊断通道（-oath-grades-from-gear）。
// 再之后是「通关 N 场保底」（-oath-progress-clears）：它没有任何出处，也把「稀有」
// 变成一个与玩法无关的计数 ⇒ 同样退化成诊断兜底，只在 -omen-state 关着时生效。

// parseOathGrades 解析 "primer,oath"。空串 = 不覆盖，返回零值，
// 由通关保底决定档位（见 oath_progress.go）。
//
// 只接受八档内的值：发域外值等于把「打不死」原样复制一遍，所以宁可启动就报错。
func parseOathGrades(spec string) ([2]uint16, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		// 零值 = 未覆盖：oathInfoPackets 按通关保底算。
		return [2]uint16{}, nil
	}
	parts := strings.Split(spec, ",")
	if len(parts) != 2 {
		return [2]uint16{}, fmt.Errorf("bad oath grades %q: want primer,oath", spec)
	}
	var out [2]uint16
	for i, part := range parts {
		v, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil {
			return [2]uint16{}, fmt.Errorf("bad oath grades %q: %w", spec, err)
		}
		if _, ok := oathGradeTiers[uint16(v)]; !ok {
			return [2]uint16{}, fmt.Errorf("bad oath grades %q: %d is outside the eight tiers "+
				"the client's script accepts (40..45, 70, 71)", spec, v)
		}
		out[i] = uint16(v)
	}
	return out, nil
}

// oathInfoPayload 是 noti 2838 的载荷：两个 little-endian u32，共 8 字节。
//
// 长度必须是 8：解析器先 `sub_146EA0BE0(&v3, 8)` 定长读一次，短了会踩客户端自己的
// 长度护栏（loc_146EA0C30 是故意的空写陷阱，2026-09-27 06:1x 用 8 字节的 2839 撞过）。
func oathInfoPayload(primer, oath uint16) []byte {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p[0:4], uint32(primer))
	binary.LittleEndian.PutUint32(p[4:8], uint32(oath))
	return p
}

// oathInfoPackets 返回进本时应随加载应答下发的档位通知。
//
// 时机很关键：必须在天平开场执行 `Primer_Proc.act` cell 6–9 之前到位，所以挂在
// 副本加载应答的同一个 plan 里（见 dungeon_flow.go 的 loading 分支）。
// 这也是一份**常驻状态**：客户端只在收到它时才会覆盖构造器里的 72/72 兜底。
// oathGradeTableDefaultPath 是生成器写表的默认位置。
const oathGradeTableDefaultPath = "configs/oath-grades.json"

// loadOathGradeTable 解析表路径。
//
// 顺序：显式路径（flag / DFO_OATH_GRADES_TABLE）→ 工作目录下的默认位置 →
// **exe 旁的 ../configs**。最后一条是实机必需的：启动器把 gateway 的 cwd 设成
// D:/115us，相对路径 "configs/..." 在那里解析不了，而 exe 一直在 bin/ 下。
//
// 全找不到时**硬失败**。档位表缺失会让每个角色都落回 normal，也就是隐藏 BOSS
// 永远不登场 —— 那是静默的行为变更，比启动时报错难查得多。
func loadOathGradeTable(path string) (*inventory.OathGradeTable, error) {
	candidates := make([]string, 0, 3)
	if path != "" {
		candidates = append(candidates, path)
	} else {
		candidates = append(candidates, oathGradeTableDefaultPath)
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", oathGradeTableDefaultPath))
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return inventory.LoadOathGradeTable(c)
		}
	}
	return nil, fmt.Errorf("no oath grade table (tried %s): pass -oath-grades-table or set DFO_OATH_GRADES_TABLE",
		strings.Join(candidates, ", "))
}

// oathInfoPackets 返回进本时应随加载应答下发的档位通知（noti 2838），
// 外加**诊断注入**的 noti 2859（默认为空 = 不发）。
//
// 2859 与 2838 放在同一时刻下发是有意的：两者写的是同一套「档位」语义的两个入口
// （2859 的第一个 u32 与 2838 的第一个数落在模块的同一个位移），进本时珠子/天平
// 的开场演出就在这之后。见 attunement_reward.go 文件头。
func (w *worldSession) oathInfoPackets() ([]outboundPacket, error) {
	plan, err := w.oathInfoPacketsCore()
	if err != nil {
		return nil, err
	}
	return append(plan, w.attunementRewardPackets()...), nil
}

// attunementRewardPackets 生成 noti 2859「调律之边界奖励档位」，默认空 = 一个字节都不发。
//
// ## 语义（业主 2026-10-08 定调）
//
//	入场砝码给出**保底档**；进本掷骰若更高就用更高那一档；**动画与掉落绑定在同一档上**。
//
// 本仓的档位（primer / oath）本来就是「保底 + 向上随机」掷出来的
// （见 internal/loot/attunement_plan.go 的 PlanRun / pickTierAbove），
// 所以这里只做一件事：**把这一档按客户端演出表的刻度发出去**。
// 刻度换算与实测依据见 loot.AnimationGrade（我们的 40..45 比演出表晚两格）。
//
// ## ⚠️ 三个位移**都必须落在这张表的值域里**（实测 2026-10-08）
//
// 客户端读的是**同一个三元素组**，任何一格越界就整体走「最后一格兜底」：
//
//	42,42,42 -> EpicDrop（业主实测）
//	40,43,43 -> 最低那格（业主实测）
//	42,72,72 -> **PrimevalDrop**（业主实测：本仓 50e223b8 那版把后两格填成模块初值 72）
//	不发（模块 init = -1/72/72）-> PrimevalDrop（这就是「每次必定播放太初动画」的成因）
//
// ⇒ 三格**填同一个演出格**。这也让「三格 = 同一档」与业主口径（动画与掉落绑定）同义；
// 而且无论客户端最终取哪一格，结果都一致 —— 我们还没能静态确认它取的是 `+88` 还是别的，
// 填成同一个值就不需要先知道这一点。**不要再往这两格填 72**：那会让整帧退化成兜底。
//
// 档位落在演出表没有格的位置（normal / rare，值域从「独有」起）时**不发**。
//
// ⚠️ **每次进本重新计算**：档位是这一场掷的。诊断覆盖（`-attunement-reward`）优先，
// 用于继续试值；它对 `@文件` 形式同样是每次进本重读。
func (w *worldSession) attunementRewardPackets() []outboundPacket {
	if w == nil {
		return nil
	}
	if strings.TrimSpace(w.attunementReward) != "" {
		payload, err := attunementRewardSpec(w.attunementReward)
		if err != nil {
			log.Printf("attunement reward (noti 2859): skipped this entry, %v", err)
			return nil
		}
		if len(payload) == 0 {
			return nil
		}
		log.Printf("attunement reward (noti 2859): sending %s (diagnostic override)", describeAttunementReward(payload))
		return []outboundPacket{{"attunement_reward", 0, attunementRewardPacketID, payload}}
	}

	// 门禁：这条包只属于「调律之边界」玩法（包名就是它）。小深渊是**另一套玩法**
	// （`[dungeon type] endkeeper of order`），它的演出未必读这个字段，而它 74% 的场次
	// 档位落在演出表之外（normal/rare）—— 不越过门禁就不会互相影响（业主 2026-10-08 决定）。
	if w.activeDungeon == nil || catalog.DungeonType(w.activeDungeon.Definition) != attunementPlayType {
		return nil
	}

	// 档位取**珠子（primer / 固定池）**：官方口径是「珠子颜色代表对应品质的**装备**」，
	// 誓约线（oath / 附加池）是另一条线的承诺，不参与这一格（业主 2026-10-08 确认）。
	tiers := w.attunementRunTiers
	grade := tiers.Primer
	slot, ok := loot.AnimationGrade(grade)
	if !ok {
		return nil
	}
	payload := make([]byte, attunementRewardPayloadSize)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(payload[i*4:], slot)
	}
	log.Printf("attunement reward (noti 2859): grade=%s(%d) (primer=%s oath=%s) -> %s(%d); payload %s",
		loot.TierForGradeValue(grade), grade,
		loot.TierForGradeValue(tiers.Primer), loot.TierForGradeValue(tiers.Oath),
		loot.AnimationSlot(slot), slot, hex.EncodeToString(payload))
	return []outboundPacket{{"attunement_reward", 0, attunementRewardPacketID, payload}}
}

// attunementPlayType 是「调律之边界」的 [dungeon type] 源值：noti 2859 的发送门禁。
const attunementPlayType = "boundary of attunement"

// attunementRewardPacketID 是 noti 2859 的 id，单列出来给上面那处注入与日志引用。
const attunementRewardPacketID = 2859

func (w *worldSession) oathInfoPacketsCore() ([]outboundPacket, error) {
	// 诊断注入（-oath-grades "primer,oath"）整对覆盖，不参与两条线的预掷。
	// 穿戴装备那条诊断同理。两条诊断都只在启动时显式打开，正常档里 w.oathGrades 是零值。
	if p, o := w.oathGrades[0], w.oathGrades[1]; p != 0 || o != 0 {
		w.attunementRunTiers = loot.RunTiers{Primer: uint32(p), Oath: uint32(o)}
		w.oathTierRun = o
		return []outboundPacket{{"oath_system_grades", 0, 2838, oathInfoPayload(p, o)}}, nil
	}
	if w.oathFromGear {
		p, o := w.wornOathGrades()
		w.attunementRunTiers = loot.RunTiers{Primer: uint32(p), Oath: uint32(o)}
		w.oathTierRun = o
		return []outboundPacket{{"oath_system_grades", 0, 2838, oathInfoPayload(p, o)}}, nil
	}
	// 正常路径：**两条线各自预掷**（见 internal/loot/attunement_plan.go）。
	// 这一步必须在掉落之前完成 —— 掉落发生在这之后（天平死亡），两边必须是同一档。
	tiers, err := w.planAttunementRunTiers()
	if err != nil {
		return nil, err
	}
	// ⚠️ 2026-10-07 撤除「征兆满档 ⇒ 强推 oath=45」：那是把**征兆系统**与
	// **天平的掉落档位**耦合在一起，官方口径里没有这条（业主 2026-10-07 指出）。
	//
	// 两个隐藏 BOSS 的选路**完全在客户端脚本里，只看这两个档位**
	// （`primer_proc.act` 的 `nox_index_checker`，实机/反编译已确认）：
	//
	//	oath_max == 45                  -> 奥尔特尔（OrderChroniclerOrthaire, 109019264）
	//	oath_max < 45 && primer_max == 45 -> 监视者（Watcher）
	//
	// 与官方文案逐字吻合：「消灭奥尔特尔 → 太初级**誓约**，按几率还可获得太初星蕴石」
	//（oath 线）、「消灭监视者 → 太初级**星蕴石**」（primer 线）。所以两条线掷到
	// primeval 时 BOSS 自然登场，**不需要**任何保底，也没有 40.25% 之类的白拿。
	// 概率上也都是官方说的「低几率」：oath=45 ≈ 1.48% × 2.03% ≈ 0.03%，
	// primer=45 ≈ 0.15%。
	w.attunementRunTiers = tiers
	// ⚠️ 档位 0 必须折成 normal(40) 再下发：0 不在客户端那 8 档阶梯（`40..45 / 70 / 71`）里，
	// 而 2838 是**常驻状态**（客户端只在收到它时才覆盖构造器里的兜底）。
	// 没有活动表（非调律副本 / 表未装载 / 本场两条线都没掷到）就走这条路径 ——
	// 代码注释一直写的是「下发 normal/normal」，但实现漏了这一步，
	// `TestOathInfoPacketsFallsBackToNormalWithoutPity` 正是钉它的。
	primer, oath := uint16(tiers.Primer), uint16(tiers.Oath)
	if primer == 0 {
		primer = inventory.OathGradeNormal
	}
	if oath == 0 {
		oath = inventory.OathGradeNormal
	}
	w.oathTierRun = oath
	return []outboundPacket{{"oath_system_grades", 0, 2838,
		oathInfoPayload(primer, oath)}}, nil
}

// planAttunementRunTiers 在**进本时**预掷两条线的档位。
//
// 种子按时间取：这两条线与掉落的 RNG 链无关（掉落另有一条从进本种子开始的链），
// 唯一的要求是「先于掉落定下来」，所以不需要与掉落共种子。
// 没有活动表（非调律副本、或表没装载）时返回零值 ⇒ 2838 下发 normal/normal，
// 与 2026-10-07 之前对非调律副本的行为一致。
func (w *worldSession) planAttunementRunTiers() (loot.RunTiers, error) {
	var tiers loot.RunTiers
	if w == nil || w.loot == nil || w.loot.Attunement == nil || !w.loot.Attunement.Enabled() {
		return tiers, nil
	}
	var dungeon, maze uint32
	if w.activeDungeon != nil {
		dungeon = w.activeDungeon.Definition.ID
		maze = uint32(w.activeDungeon.Maze.Index)
	}
	tiers, _, err := w.loot.Attunement.PlanRun(uint32(time.Now().UnixNano()), dungeon, maze)
	if err != nil {
		return tiers, err
	}
	// 念出来：档位是我们按表掷的，不念出来以后对不上账（与调参层同一条纪律）。
	// `floor=…` 是本场的**保底档**（入场砝码决定的那个下界，业主 2026-10-08 口径：
	// 保底只是下界，掷出来可以更高）。它由副本自己的 fixed 池推出来 ——
	// 实机验收时看这一行就能确认「珠子档位 ≥ 砝码保底」。
	log.Printf("attunement run tiers: dungeon %d maze %d -> floor=%s(%d) primer=%s(%d) oath=%s(%d)",
		dungeon, maze, loot.TierForGradeValue(tiers.Floor), tiers.Floor,
		loot.TierForGradeValue(tiers.Primer), tiers.Primer,
		loot.TierForGradeValue(tiers.Oath), tiers.Oath)
	return tiers, nil
}

// derivedOathGrades 是**没有活动掉落表**时的兜底：只按「这一场该不该召唤隐藏 BOSS」
// 决定，primer 恒 normal。有表时走 planAttunementRunTiers（表就是分布本身）。
func (w *worldSession) derivedOathGrades() (uint16, uint16, error) {
	if w.oathFromGear {
		primer, oath := w.wornOathGrades()
		return primer, oath, nil
	}
	due, err := w.orthaireDue()
	if err != nil {
		return 0, 0, err
	}
	primer, oath := oathGradesForPity(due, rand.New(rand.NewSource(time.Now().UnixNano())))
	return primer, oath, nil
}

// orthaireDue 判断这一场该不该召唤隐藏 BOSS（oath=45，唯一会出奥尔泰尔的档）。
//
// 两条来源，-omen-state 优先：
//
//   - 征兆线（正常路径，业主 2026-09-27 定调 B1）：看角色存档里「上一场刚满档结算过」
//     这个标记。它由 loadOmenRunState 在进本时就读好了（omen_state.go），所以这里
//     不再读库 —— noti 2838 与 noti 2836 必须从**同一份**状态推出来，分两次读库迟早
//     会读到两次不同的快照。
//   - 旧通关保底（-omen-state 关着时的兜底，诊断保留）：-oath-progress-dungeons 里的
//     副本通关 -oath-progress-clears 场后下一场出，见 oath_progress.go。
func (w *worldSession) orthaireDue() (bool, error) {
	if w.omenState {
		return w.omenOrthaierDue, nil
	}
	return w.oathProgressDue(w.oathProgressDungeon())
}

// ⚠️ 2026-10-07 起**不再是正常路径**：星蕴石/珠子的档位改由 CTP 表掷
// （internal/loot/attunement_plan.go 的 PlanRun，分布就是 fixed 表本身）。
// 下面这张权重表保留给诊断与单测，**不要**再拿它当正常档。
// 它记录的是旧模型的六档权重（业主提供的国服 1710 场实测，来源其实是**星蕴石**）。
//
// oathGradesForPity 是旧模型的保底档位纯决策：到期给 oath=45（唯一召唤奥尔泰尔的档），
// 否则两边都是 normal。primer 恒 normal 也意味着第二个隐藏 BOSS「守望者」
// （`oath_max < 45 && primer_max == 45`）暂时不会出现 —— 它要另有一条保底。
// oathGradeWeights 是六档的抽取权重（业主 2026-10-01 拍板：采用国服 1710 场实测爆率）。
//
// 为什么需要这张表：`[ON DAMAGE]` 阶梯能爬到哪一档**完全由 noti 2838 的 `*_max` 决定**
// （§4b：整条链全在客户端本地，PVF 文本里没有服务端参与点），所以「这一场天平是哪一档」
// 只能由服务端定 —— 而 PVF 里**没有**这张概率表（已逐处核对 oathsystemscript.cos 的
// [base rarity section]/[rarity ui infos]/[seasonlevel oath item]、primer_00..07_*_loop.act
// 的 [ON DAMAGE] 阶梯、scale_primer.mob 的 [create var]；唯一出现的概率是
// c:fake_end_prob_prob=30「假结束」，与本表无关）。
//
// 权重取自业主提供的国服实测（1710 次深渊的星蕴石出现率）：
//
//	稀有 27.7% · 神器 32.3% · 传说 5.7% · 史诗 1.9% · 太初 0.35%（其余 32.05% = 不变色/normal）
//
// 按 10000 分整数化以保持确定性。注意 **神器（unique, 3230）略高于稀有（rare, 2770）**
// 是实测形状，不要"顺手修正"成单调递减。
//
// primeval(45) **也在表内**（0.35%）：国服它本来就是概率掉落，不是保底专属；
// 服务端另有「通关 N 场保底」叠加在上面（见 oathGradesForPity 的 due 分支）——
// 到期那场必然 45，其余场按本表掷骰，两者互不覆盖。
var oathGradeWeights = []struct {
	Grade  uint16
	Weight int
}{
	{inventory.OathGradeNormal, 3205}, // 40 normal    其余 32.05%
	{41, 2770},                        // rare         稀有 27.7%
	{42, 3230},                        // unique       神器 32.3%
	{43, 570},                         // legendary    传说  5.7%
	{44, 190},                         // epic         史诗  1.9%
	{oathGradePrimeval, 35},           // 45 primeval  太初 0.35%
}

// rollOathGrade 按 oathGradeWeights 抽一档。rng 为 nil ⇒ normal（零值路径与测试用）。
func rollOathGrade(rng *rand.Rand) uint16 {
	if rng == nil {
		return inventory.OathGradeNormal
	}
	total := 0
	for _, w := range oathGradeWeights {
		total += w.Weight
	}
	pick := rng.Intn(total)
	for _, w := range oathGradeWeights {
		if pick < w.Weight {
			return w.Grade
		}
		pick -= w.Weight
	}
	return inventory.OathGradeNormal
}

// oathGradesForPity 决策 (primer, oath)：
//   - 保底到期 ⇒ oath = primeval（隐藏 BOSS「奥尔泰尔」登场，唯一召唤档）；
//   - 否则按国服实测爆率掷骰 —— 天平因此**每场可能不同颜色**，
//     这正是源里 [rarity ui infos] 给每档配 [color] / symbol 动画的用法。
//
// 两条保底线**互不覆盖**：这里只决定 noti 2838 的档位；征兆的结算走 omen.go 的
// AdvanceOmen / payOmenStages（读 [coupon drop table]），是**另一个独立来源**，
// 同一场里两者可以同时兑现（业主 2026-10-01 明确：变色 + 征兆 = 双份保底）。
//
// `primer` 恒为 normal：第二个隐藏 BOSS「守望者」需要 primer=45 && oath<45，
// 它要另有一条保底（§32.8 已记「暂不出现」），不在本轮范围。
func oathGradesForPity(due bool, rng *rand.Rand) (uint16, uint16) {
	if due {
		return inventory.OathGradeNormal, oathGradePrimeval
	}
	return inventory.OathGradeNormal, rollOathGrade(rng)
}

// wornOathGrades 是**诊断**路径（-oath-grades-from-gear）：按角色实际穿戴的
// 誓约/引子装备算 (primer, oath) 档位。
//
// 一件都没穿（或表未配置）时给 normal。档位映射的取证见
// internal/inventory/oath_grade.go。这条规则退场的理由是客户端脱不下誓约槽：
// 穿上 primeval 就永久 45。
func (w *worldSession) wornOathGrades() (uint16, uint16) {
	if w.oathTable == nil {
		return inventory.OathGradeNormal, inventory.OathGradeNormal
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return inventory.OathGradeNormal, inventory.OathGradeNormal
	}
	return w.oathTable.Grades(bag.Worn)
}
