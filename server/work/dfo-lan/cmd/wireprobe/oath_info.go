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
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/inventory"
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

func (w *worldSession) oathInfoPackets() ([]outboundPacket, error) {
	primer, oath := w.oathGrades[0], w.oathGrades[1]
	if primer == 0 && oath == 0 {
		var err error
		if primer, oath, err = w.derivedOathGrades(); err != nil {
			return nil, err
		}
	}
	return []outboundPacket{{"oath_system_grades", 0, 2838, oathInfoPayload(primer, oath)}}, nil
}

// derivedOathGrades 是正常路径：按「这一场该不该召唤隐藏 BOSS」算档位。
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

// oathGradesForPity 是保底档位的纯决策：到期给 oath=45（唯一召唤奥尔泰尔的档），
// 否则两边都是 normal。primer 恒 normal 也意味着第二个隐藏 BOSS「守望者」
// （`oath_max < 45 && primer_max == 45`）暂时不会出现 —— 它要另有一条保底。
// oathGradeWeights 是**中间五档**的抽取权重（业主 2026-10-01 拍板：按稀有度递减）。
//
// 为什么需要这张表：`[ON DAMAGE]` 阶梯能爬到哪一档**完全由 noti 2838 的 `*_max` 决定**
// （§4b：整条链全在客户端本地，PVF 文本里没有服务端参与点），所以「这一场天平是哪一档」
// 只能由服务端定 —— 而 PVF 里**没有**这张概率表（已逐处核对 oathsystemscript.cos 的
// [base rarity section]/[rarity ui infos]/[seasonlevel oath item]、primer_00..07_*_loop.act
// 的 [ON DAMAGE] 阶梯、scale_primer.mob 的 [create var]；唯一出现的概率是
// c:fake_end_prob_prob=30「假结束」，与本表无关）。
//
// 形状：normal 仍占大头（普通场次不该每场都变色），rare/unique/legendary/epic 依次变少。
// **primeval(45) 刻意不在表内** —— 它由「通关 N 场保底」独占（见 oathGradesForPity 的 due 分支），
// 混进随机会让隐藏 BOSS 从「保底」退化成「随机」。
var oathGradeWeights = []struct {
	Grade  uint16
	Weight int
}{
	{inventory.OathGradeNormal, 55}, // 40 normal
	{41, 22},                        // rare
	{42, 13},                        // unique
	{43, 7},                         // legendary
	{44, 3},                         // epic
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
//   - 否则按稀有度递减随机抽中间四档 —— 天平因此**每场可能不同颜色**，
//     这正是源里 [rarity ui infos] 给每档配 [color] / symbol 动画的用法。
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
