package inventory

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
)

// 装备实例行里的增幅字段（取证：docs/进度-20260917-副本可进性与GM工具.md 第四节，
// 并用真实存档复核过——太刀 record[10] & 0x1f = 16 与当时的强化等级一致）：
//
//	offset 10 : ★ 等级字节 —— bit0-4 = 等级 0..31，bit5-7 = 再封装次数
//	            强化等级与**增幅等级共用这一个字节**：装备一旦有次元属性（offset 19 != 0），
//	            客户端就把这个等级渲染成「增幅 +N」；没有次元属性时渲染成「强化 +N」。
//	            佐证：黄金增幅书会「删除当前强化等级」（清空低五位），正因为两者是同一个字段。
//	offset 19 : amplify_type  —— 次元属性类型（红字），0 = 没有
//	offset 20 : amplify_value —— 次元属性**数值**，打上红字时由增幅书的加权表摇出
//	            ⚠️ 这不是增幅等级。早期实现把增幅等级写进了这里，结果是：红字数值被等级覆盖，
//	            而等级字节仍是 0 —— 装备上看不到任何「+N」，这就是「增幅成功但没显示」的原因。
//
// 锻造在记录尾部（0..8，仅武器），本次未动。
const (
	amplifyTypeOffset  = 19
	amplifyValueOffset = 20
	// 强化等级写在装备实例行偏移 10 的低五位（bit0-4），高三位（bit5-7）保留为再封装次数；
	// 黄金增幅书会「删除当前强化等级」，即清掉低五位（reinforcement_gold.go:97 / reinforcement.go:208 同约定）。
	amplifyReinforceOffset = 10
	reinforceLevelMask     = 0x1f
	// 纯书 / 黄金书走「纯净」行为（见 amplifyGrimoireOutcome）：只落「增幅 +0」、**不写红字数值**。
	// 服主要求「只需要增幅 +0，不需要力量 +6」，所以 offset 20 恒为 0（客户端把 0 当作
	// 「没有次元属性数值」→ 异次元属性那一行不显示）。类型 offset 19 仍写入，等级仍渲染成「增幅 +0」。
)

// amplifyLevel 读装备行里的增幅/强化共用的等级字节。
func amplifyLevel(row []byte) int { return int(row[amplifyReinforceOffset] & reinforceLevelMask) }

// setAmplifyLevel 写等级字节，保留高三位的再封装次数。
func setAmplifyLevel(row []byte, level byte) {
	row[amplifyReinforceOffset] = row[amplifyReinforceOffset]&^byte(reinforceLevelMask) | (level & byte(reinforceLevelMask))
}

// 客户端 dstr 21306..21309 = 体力 / 精神 / 力量 / 智力，按 21305 + type 排列，
// 所以次元属性类型取值是 1..4（不是 3..6 —— 那本是 [amplification random value]
// 的数值表，早期误读成了类型表，见 2026-09-27 CHANGELOG）。
const (
	amplifyTypeVitality     = 1
	amplifyTypeSpirit       = 2
	amplifyTypeStrength     = 3
	amplifyTypeIntelligence = 4
)

// 增幅书：物品脚本带 [amplification random value] 段。
// 段内是 (次元属性数值, 权重) 加权表 —— 服务端按它摇出打红字时的初始值：
// 普通增幅书 3..6，强烈的增幅书（super_amplification_grimoire / s_amp）8..13。
// 类型不是摇出来的，由玩家在窗口里选，随 CMD205 请求一起上来。
type amplifyValueWeight struct {
	Value  int `json:"value"`
	Weight int `json:"weight"`
}

type amplifyGrimoireRules struct {
	Version int `json:"version"`
	Source  string
	// PureTemplates 是**没有 [amplification random value] 段**、却由客户端当作增幅书
	// （对其发 CMD205）的纯净增幅书（Pure Amplification Scroll）模板清单。
	// 例：本服（115US）的纯净增幅书被重模板为 1286（stackable/consumption_1286.stk，
	// 一个 [etc] 消耗品，脚本里没有任何增幅段），客户端仍走 CMD205。这类书无法靠
	// 「随机表值恒 0」自动识别，只能显式登记。导出脚本重导出时会保留本字段。
	PureTemplates []uint32 `json:"pure_templates"`
	Grimoires     []struct {
		Template uint32               `json:"template"`
		Path     string               `json:"path"`
		Random   []amplifyValueWeight `json:"random"`
		Expires  bool                 `json:"expires"`
		// Golden 由脚本路径识别：纯净的黄金增幅书（amplification_book_golden 一类）会
		// 「扭转已有红字 + 删除当前强化等级」，与普通/超级增幅书（只打/只加红字）不同。
		// PVF 里没有专门的标记字段，只能用路径里的 golden 区分（见 IsGoldenGrimoire）。
		Golden bool `json:"golden"`
		// Pure 由脚本 [amplification random value] 段识别：段内加权表的值恒为 0（[(0,100)]），
		// 即「只打红字、增幅等级恒 0」的纯净增幅书（Pure Amplification Scroll）。
		Pure bool `json:"pure"`
	} `json:"grimoires"`
}

var amplifyGrimoires *amplifyGrimoireRules

// pureGrimoireTemplates 是 PureTemplates 的去重集合，供 IsPureGrimoire 快速查表。
var pureGrimoireTemplates = map[uint32]bool{}

// amplifyRandomInt 是可替换的随机数源，测试里换成固定序列。
var amplifyRandomInt = func(n int) (int, error) {
	if n <= 1 {
		return 0, nil
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(b[:]) % uint32(n)), nil
}

// LoadAmplifyGrimoires 读取增幅书清单；文件缺失时该功能整体拒绝（不影响强化）。
func LoadAmplifyGrimoires(path string) error {
	var rules amplifyGrimoireRules
	if loaded, err := loadOptionalJSON(path, &rules); !loaded || err != nil {
		return err
	}

	if rules.Version != 1 || len(rules.Source) != 64 || len(rules.Grimoires) == 0 {
		return fmt.Errorf("增幅书规则源定义不完整")
	}
	for i := range rules.Grimoires {
		g := &rules.Grimoires[i]
		// 路径含 golden 即视为黄金增幅书（纯净的黄金增幅书 / 活动金书），会删除强化等级。
		g.Golden = strings.Contains(strings.ToLower(g.Path), "golden")
		// 随机表的值恒为 0（[(0,100)]）即纯净增幅书：只打红字、增幅等级恒 0。
		pure := len(g.Random) > 0
		for _, w := range g.Random {
			if w.Value != 0 {
				pure = false
				break
			}
		}
		g.Pure = pure
	}
	amplifyGrimoires = &rules
	pureGrimoireTemplates = make(map[uint32]bool, len(rules.PureTemplates))
	for _, t := range rules.PureTemplates {
		pureGrimoireTemplates[t] = true
	}
	return nil
}

// AmplifyGrimoiresLoaded 报告增幅书清单是否已装载。
func AmplifyGrimoiresLoaded() bool { return amplifyGrimoires != nil }

// IsGoldenGrimoire 报告模板是否为黄金增幅书（扭转红字 + 删除强化等级）。
func IsGoldenGrimoire(template uint32) bool {
	if amplifyGrimoires == nil {
		return false
	}
	for _, g := range amplifyGrimoires.Grimoires {
		if g.Template == template {
			return g.Golden
		}
	}
	return false
}

// IsPureGrimoire 报告模板是否为纯净增幅书（Pure Amplification Scroll）：只打红字、
// 增幅等级恒 0。识别口径有二：
//  1. 脚本 [amplification random value] 段的值恒为 0（[(0,100)]）—— 见 LoadAmplifyGrimoires；
//  2. 显式登记在 PureTemplates 里的模板（那些没有增幅段、却被客户端当增幅书发 CMD205 的书，
//     如本服的 1286）。
func IsPureGrimoire(template uint32) bool {
	if amplifyGrimoires == nil {
		return false
	}
	if pureGrimoireTemplates[template] {
		return true
	}
	for _, g := range amplifyGrimoires.Grimoires {
		if g.Template == template {
			return g.Pure
		}
	}
	return false
}

// ClassifyAmplifyBook 判定 CMD205 请求里的增幅书属于哪一类，并给出普通书摇出的红字数值。
//
//	golden=true → 黄金增幅书（脚本路径含 golden）：**仍走摇值即等级**（8..12），
//	              与纯净增幅书是两个不同产物，不并入；
//	pure=true   → 纯净增幅书（只落「增幅 +0」、不写红字数值）；
//	否则         → 普通 / 白银 / 强烈书，value = 按加权表摇出的数值（3..6 / 8..13）。
//
// ★ 兜底：客户端只会对「增幅书」发 CMD205。凡不在 433 加权表里（脚本没有
// [amplification random value] 段）的模板，都当作**纯净增幅书**（而不是拒绝）——
// 本服除 1286 外，实测玩家用过的还有 stackable/.../pure.stk(590704000)、
// 10356261、10354248、10360807、10000605。以前这里直接拒绝「窗口里的物品不是增幅书」，
// 导致玩家背包里除 1286 外的纯书全都用不了。
func ClassifyAmplifyBook(template uint32) (golden, pure bool, value byte) {
	golden = IsGoldenGrimoire(template)
	pure = IsPureGrimoire(template)
	if pure {
		return golden, true, 0
	}
	// 黄金增幅书**不并入纯净**：它的随机表在普通档（值非 0），所以 IsPureGrimoire 对它
	// 返回 false，会走到下面的摇值分支 —— 这正是本仓库既有的「摇值即等级」行为。
	v, ok := AmplifyGrimoireRollValue(template)
	if !ok {
		// 兜底：不在 433 加权表里、脚本也没有 [amplification random value] 段的模板，
		// 一律当纯净书（不再拒绝「窗口里的物品不是增幅书」）。
		return golden, true, 0
	}
	return golden, false, v
}

// AmplifyGrimoireValueTable 返回该增幅书的 (数值, 权重) 加权表。
func AmplifyGrimoireValueTable(template uint32) ([]amplifyValueWeight, bool) {
	if amplifyGrimoires == nil {
		return nil, false
	}
	for _, g := range amplifyGrimoires.Grimoires {
		if g.Template != template {
			continue
		}
		table := make([]amplifyValueWeight, 0, len(g.Random))
		for _, row := range g.Random {
			if row.Weight > 0 {
				table = append(table, row)
			}
		}
		return table, len(table) > 0
	}
	return nil, false
}

// AmplifyGrimoireRollValue 按增幅书脚本的加权表摇出打红字时的次元属性初始值。
// 摇不出（不是增幅书 / 表为空 / 随机源失败）时返回 false，调用方应拒绝而不是写 0 ——
// 数值 0 在客户端等于「没有次元属性」，写了就等于白打一本书。
func AmplifyGrimoireRollValue(template uint32) (byte, bool) {
	table, ok := AmplifyGrimoireValueTable(template)
	if !ok {
		return 0, false
	}
	total := 0
	for _, row := range table {
		total += row.Weight
	}
	if total <= 0 {
		return 0, false
	}
	roll, err := amplifyRandomInt(total)
	if err != nil {
		return 0, false
	}
	for _, row := range table {
		if roll < row.Weight {
			if row.Value <= 0 || row.Value > 255 {
				return 0, false
			}
			return byte(row.Value), true
		}
		roll -= row.Weight
	}
	return 0, false
}

// AmplifyTypeName 把次元属性类型译成中文，仅用于日志。
// 取值 1..4 对应客户端 dstr 21306..21309（体力 / 精神 / 力量 / 智力）。
func AmplifyTypeName(t int) string {
	switch t {
	case amplifyTypeVitality:
		return "体力"
	case amplifyTypeSpirit:
		return "精神"
	case amplifyTypeStrength:
		return "力量"
	case amplifyTypeIntelligence:
		return "智力"
	}
	return fmt.Sprintf("未知(%d)", t)
}

// applyGrimoireLevel 把打完红字后的**增幅等级**落到 offset 10 低五位，返回落完的等级。
//
// 增幅书摇出的那个数值，既是次元属性数值（offset 20，红字那一行），也是**增幅等级**
// —— 客户端把 offset 10 渲染成「增幅 +N」，只写红字不写等级的话装备上永远是「增幅 +0」，
// 玩家看到的就是「红字打上了、等级没动」。
//
//	白银 / 普通增幅书：随机 3..6  → 打完直接 +3~+6
//	强烈增幅书      ：随机 8..13 → 打完直接 +8~+13
//	黄金增幅书      ：随机 8..12 → 打完直接 +8~+12（与白银/强烈书同一套规则）；
//	                ⚠️ **不并入纯净**（两个不同产物）
//	                纯净增幅书：只落「增幅 +0」、不写红字数值，见 amplifyGrimoireOutcome 的 pure 分支
//
// ⚠️ 官方的黄金增幅书是「扭转红字 + 删除当前强化等级」（等级清 0），这里**不按官方**，
// 按服主要求统一成「摇值即等级」。副作用：对已有高增幅的装备用黄金书会把等级顶回 8~12。
//
// bit5-7 的再封装次数一律保留。
func applyGrimoireLevel(row []byte, value byte) byte {
	setAmplifyLevel(row, value)
	return row[amplifyReinforceOffset] & reinforceLevelMask
}

// amplifyGrimoireOutcome 决定打完红字后红字数值（offset 20）与增幅等级（offset 10 低五位）。
//
//	**纯净增幅书**：只落「增幅 +0」、**不写红字数值**（"纯净"行为，服主要求
//	  「只需要增幅 +0、不需要力量 +6」）。
//	  - redValue = 0：客户端把 offset 20 = 0 当作「没有次元属性数值」→「力量 +N」那一行不显示。
//	  - 类型 offset 19 仍由调用方写入，客户端据此把等级渲染成「增幅 +0」（而不是「强化 +0」）。
//	普通 / 白银 / 强烈书 / **黄金增幅书**：红字数值 = 摇出值，增幅等级 = 摇出值
//	  （红字与等级一起落库，客户端才显示「增幅 +N」）。
//
// ⚠️ **黄金增幅书不并入纯净**：它与纯净增幅书是两个不同产物（前者路径含 golden、
// 随机表在普通档 8..12，后者只打 +0 且不写红字数值）。服主明确规定要区分，别合并。
//
// 再封装次数（offset 10 高三位）一律保留，由 applyGrimoireLevel 负责。
func amplifyGrimoireOutcome(row []byte, value byte, pure bool) (redValue, level byte) {
	if pure {
		// 不写红字数值：offset 20 清成 0 → 客户端不显示「力量 +N」，但仍是「增幅 +0」。
		redValue = 0
		level = applyGrimoireLevel(row, 0)
		return redValue, level
	}
	redValue = value
	level = applyGrimoireLevel(row, value)
	return redValue, level
}

// AmplifyGrimoireReceipt 是一次「打红字」的结果。
type AmplifyGrimoireReceipt struct {
	Request         protocol.AmplifyOptionRequest `json:"request"`
	BookTemplate    uint32                        `json:"book_template"`
	BookSlot        uint16                        `json:"book_slot"`
	BookRemaining   uint32                        `json:"book_remaining"`
	Equipment       BagEquipment                  `json:"equipment"`
	EquipmentSpace  byte                          `json:"equipment_space"`
	EquipmentSlot   uint16                        `json:"equipment_slot"`
	AmplifyType     byte                          `json:"amplify_type"`
	AmplifyTypeName string                        `json:"amplify_type_name"`
	AmplifyValue    byte                          `json:"amplify_value"`
	ReAmplified     bool                          `json:"re_amplified"`
	// Golden 由增幅书脚本路径识别（.../amplification_book_golden.stk）。
	// 它摇的是 8..12；按服主要求与白银书同一套规则：摇值即增幅等级（不按官方清 0）。
	Golden             bool `json:"golden"`
	Pure               bool `json:"pure"`
	PrevReinforceLevel byte `json:"prev_reinforce_level"`
	// AmplifyLevel 是打完红字后装备行 offset 10 的等级 = 增幅书摇出的数值。
	// 客户端就按这个字节渲染「增幅 +N」，所以红字与等级必须一起落库。
	AmplifyLevel byte `json:"amplify_level"`
	// PrevAmplifyType 是覆盖前装备原有的次元属性类型（重增幅时才有，首打为 0）。
	// 放进回包的结果段，对齐强化回包「旧等级」字段，客户端据此播放扭转动画。
	PrevAmplifyType byte `json:"prev_amplify_type"`
}

const amplifyGrimoireModel = "amplify-grimoire-v1"

// ApplyAmplifyGrimoire 处理 CMD 205：校验增幅书与目标装备，写入次元属性类型，扣掉一本书。

func (s *WearService) ApplyAmplifyGrimoire(role Role, r protocol.AmplifyOptionRequest, value byte, golden, pure bool) (json.RawMessage, AmplifyGrimoireReceipt, error) {
	var out AmplifyGrimoireReceipt
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	// 目标装备：先按背包装备区找，再按已穿戴空间找（窗口两种都能点）。
	space, items, index := bag.findEquipment(0, r.EquipmentSlot, r.EquipmentTemplate)
	if index < 0 {
		return nil, out, fmt.Errorf("目标装备不在背包或已穿戴槽位里")
	}
	gear := items[index]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	if _, ok := d.Fields["[equipment type]"]; !ok {
		return nil, out, fmt.Errorf("目标不是装备")
	}
	row := EquipmentRow(gear)
	// 增幅书允许「重打 / 扭转红字」：装备已有红字时覆盖类型与数值，没有红字时则是首次打红字。
	// 黄金增幅书本就是用来对已带红字的装备重增幅（扭转类型或数值）的，绝不能因为「已有红字」就拒绝；
	// 数值 0 的半成品红字也一并覆盖，玩家不会因早期把数值写成 0 的 bug 被永久锁死。
	// 真正要拦的是「装备行本身损坏读不出次元属性区」这种上游错误，已在 ValidateRecord 处处理。
	reAmplified := row[amplifyTypeOffset] != 0                             // 类型位非空即视为「原本就带红字」
	prevType := row[amplifyTypeOffset]                                     // 覆盖前的原类型（扭转动画用）
	prevReinforceLevel := row[amplifyReinforceOffset] & reinforceLevelMask // 强化等级（低五位）
	// 纯净增幅书（Pure Amplification Scroll）：只能用于「无红字 + 0 强化」的装备。
	// 官方描述：Cannot be used on equipment that has been Reinforced；Adds a Dimensional
	// stat to equipment with no Dimensional properties。覆写旧等级/旧红字一律拒绝。
	if pure && (reAmplified || prevReinforceLevel != 0) {
		return nil, out, fmt.Errorf("纯净增幅书只能用于无红字且 0 强化的装备")
	}
	// 黄金增幅书：扭转已有红字时，玩家不能选与之前相同的属性（描述：「无法选择与之前相同的属性」）。
	if golden && reAmplified && r.Type == row[amplifyTypeOffset] {
		return nil, out, fmt.Errorf("黄金增幅书无法选择与之前相同的异次元属性")
	}

	// 扣书：背包里那个槽位必须就是这本增幅书。
	remaining := uint32(0)
	rows := append([]BagItem(nil), bag.Items...)
	found := false
	for i, item := range rows {
		if item.Slot != r.BookSlot {
			continue
		}
		if item.Template != r.BookTemplate || item.Amount == 0 {
			return nil, out, fmt.Errorf("增幅书槽位与模板不符")
		}
		remaining = item.Amount - 1
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		found = true
		break
	}
	if !found {
		return nil, out, fmt.Errorf("增幅书不在背包里")
	}

	// 写红字：类型 @19。数值 @20 与等级 @10 的处理因书而异（纯书 / 黄金书 / 普通·白银·强烈书）。
	row[amplifyTypeOffset] = r.Type
	redValue, level := amplifyGrimoireOutcome(row[:], value, pure)
	row[amplifyValueOffset] = redValue
	gear.Record = append([]byte(nil), row[:]...)
	items = append([]BagEquipment(nil), items...)
	items[index] = gear
	if space == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}
	bag.Items = rows

	out = AmplifyGrimoireReceipt{
		Request: r, BookTemplate: r.BookTemplate, BookSlot: r.BookSlot, BookRemaining: remaining,
		Equipment: gear, EquipmentSpace: space, EquipmentSlot: r.EquipmentSlot,
		AmplifyType: r.Type, AmplifyTypeName: AmplifyTypeName(int(r.Type)), AmplifyValue: redValue,
		ReAmplified: reAmplified, Golden: golden, Pure: pure,
		PrevReinforceLevel: prevReinforceLevel,
		PrevAmplifyType:    prevType,
		AmplifyLevel:       level,
	}
	out.Equipment.Record = append([]byte(nil), gear.Record...)

	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, out, err
	}
	return next, out, nil
}
