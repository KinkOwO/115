package legion

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"
)

// NewApocalypseRunID mints a run identifier for one 攻坚. It is the basis of the
// terminal reward's idempotency key ("apocalypse-terminal:<id>"), so it must be
// unique per run and never reused across runs on the same connection.
func NewApocalypseRunID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		binary.LittleEndian.PutUint64(b[:8], uint64(time.Now().UnixNano()))
	}
	return hex.EncodeToString(b[:])
}

// 末世录终局翻牌奖励（N2252 / N2253）。形状与维纳斯/伊斯完全同族（军团家族
// 共用 7772B N2252 + 2405B N2253），布局说明见 venus.go 的 VenusBasicClearReward，
// 本文件只放末世录自己的数值与两个构造器。
//
// 数值来源（两条独立证据，互相印证）：
//
//  1. `configs/legion-contents.generated.json`（PVF
//     contents/system/legionsystem/legionsystem.cos 的 [reward data] 段）：
//     Apocalypse/basic 一行一件、数量 1，模板按难度分流
//     normal=10421368、expert/master=10421370、match=10421366。
//     这就是抓包里 N2252 头部那件（normal 难度抓到 10421367）。
//  2. 抓包 D:\zhuabao\captures\20261008-105227 的 N2252（难度1/难度2 各一场
//     全清）：@1600 起以 44B 步长逐行给出模板与数量，@7760 是阶段 token。
//
// 抓包解析结果（行号 = @1600 之后的第几条 44B 记录）：
//
//	难度1（choice 0）：10421367x1, 10421371x20, 10421373x96, 10362429x240,
//	                   10415196x5,  117020241x1, 然后 8 条随机装备 1004/1001/1003xxxxx
//	难度2（choice 1）：10421369x1, 10421371x30, 10421372x10, 10421373x96,
//	                   10421374x24, 10362429x260, 10415196x15, 10421377x1,
//	                   然后 7 条随机装备
//
// 也就是：第 1 件 = 表里那件 basic 奖励（本次通关证明），后面是固定材料，
// 再后面是随机装备掉落。本文件的 ShowItems 覆盖**已经证实的固定部分**；
// 随机装备位留空（不编造模板号），并在事件里记明。
//
// 注意：素材模板号（`10362429`）与维纳斯的「深渊入场 : 终末之启示」是同一个
// 模板号——这是源数据里同一件材料的复用，不是抄错；两支军团本共用深渊票池
// 是官服口径（见 D:\115US-001\rz\001 修复日志第十五轮的奖励池说明）。

// ApocalypseRewardItem 是一条末世录终局奖励（模板 + 数量）。
type ApocalypseRewardItem struct {
	Template uint32
	Amount   uint32
}

// 末世录终局奖励模板。
const (
	// ApocalypseRewardProofNormal / Expert / Master / Match 是
	// legionsystem.cos [reward data] 里 Apocalypse 各难度 basic 那一件。
	// 抓包在 normal/expert 两个难度上验证到的是「表值 - 1」的有效模板
	// （10421368→10421367、10421370→10421369），说明源模板号有一格偏移；
	// 这里按**抓包实证的有效模板**写，表值只作溯源注释。
	ApocalypseRewardProofNormal uint32 = 10421367 // 表 10421368
	ApocalypseRewardProofExpert uint32 = 10421369 // 表 10421370
	ApocalypseRewardProofMaster uint32 = 10421370 // 表 master 10421370
	ApocalypseRewardProofMatch  uint32 = 10421366 // 表 10421366

	// 固定材料（抓包在两个难度上都出现，数量随难度递增）。
	ApocalypseRewardAbyssTicket uint32 = 10362429
	// ApocalypseRewardMaterialA 是 10421371 / 10421372 / 10421374 这一族，
	// 数量随难度递增；难度1/2 的样本分别是 20 / 30（同一个 10421371）。
	ApocalypseRewardMaterialA uint32 = 10421371
	// ApocalypseRewardUnknownFlag 是难度2抓包固定段最后一行 10421377x1。
	ApocalypseRewardUnknownFlag uint32 = 10421377
	// apocalypseRewardFlagRow1 是难度1 固定段倒数第二行（10415196x5）。
	// 客户端 tooltip 未取证，暂按材料记名。
	apocalypseRewardFlagRow1 uint32 = 10415196
	// apocalypseRewardMaterialExpertA / ExpertB 只在难度2 的固定段出现
	// （10421372x10 / 10421374x24）。
	apocalypseRewardMaterialExpertA uint32 = 10421372
	apocalypseRewardMaterialExpertB uint32 = 10421374
	// ApocalypseRewardMaterialB 是 10421373（两个难度的固定段都是 x96）。
	ApocalypseRewardMaterialB uint32 = 10421373
	// ApocalypseRewardMaterialExpertC 是难度2 固定段最后一件 10421377x1。
	ApocalypseRewardMaterialExpertC uint32 = 10421377
)

// ApocalypseRewardTable 是一个难度（按 CMD2354 的难度键）的固定奖励。
//
// keys 是难度键（0 = 难度1 … 4 = 难度5「匹配」），与 legion.PlanForChoice
// 的入参同一个值域；表里没声明的键由 ApocalypseRewardFor 明确报错。
type ApocalypseRewardTable struct {
	Choice byte
	Label  string
	Show   []ApocalypseRewardItem
	Grant  []ApocalypseRewardItem
	// GrantIsAuthoritative 为 true 表示 Grant 里的**固定项**已按抓包/源表逐项
	// 落实；随机装备行是另一条 roll 链（ApocalypseFlipGearRoll，按品级 15/35/50），
	// 不在 Grant 里，由调用方单独记录 roll 结果。
	GrantIsAuthoritative bool
	// RandomGearSlots 是抓包里观察到的随机装备条数，仅用于日志与后续任务。
	RandomGearSlots int
}

// apocalypseRewardTables 是四个已声明难度的固定奖励表。
//
// Show 是抓包 N2252 的逐行复刻（@1600 起，44B 步长），**包含**本轮未建模的
// 随机装备行（用捕获到的模板号占位，便于对照，不参与发放）；
// Grant 只含**有实证、可入库**的固定项：源表那件通关证明 + 固定材料。
// Show 与 Grant 目前逐项相同；保留两个字段是因为展示（客户端界面）与发放
// （背包入库）将来可能分叉——例如某个难度只展示不发。
var apocalypseRewardTables = []ApocalypseRewardTable{
	{
		Choice: 0, Label: "normal",
		// 抓包 20261008-105227 难度1 行序（@1600 起 44B 步长）：
		// @0 通关证明(=源表 basic, 10421367)；@1600=10421371x20、@1644=10421373x96、
		// @1688=10362429x240、@1732=10415196x5；@1776 起 8 行随机装备。
		Show: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofNormal, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 20},
			{Template: ApocalypseRewardMaterialB, Amount: 96},
			{Template: ApocalypseRewardAbyssTicket, Amount: 240},
			{Template: apocalypseRewardFlagRow1, Amount: 5},
		},
		Grant: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofNormal, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 20},
			{Template: ApocalypseRewardMaterialB, Amount: 96},
			{Template: ApocalypseRewardAbyssTicket, Amount: 240},
			{Template: apocalypseRewardFlagRow1, Amount: 5},
		},
		GrantIsAuthoritative: true,
		RandomGearSlots:      8,
	},
	{
		Choice: 1, Label: "expert",
		// 抓包难度2 行序：
		// @0 通关证明(=源表 basic, 10421369)；@1600=10421371x30、@1644=10421372x10、
		// @1688=10421373x96、@1732=10421374x24、@1776=10362429x260、
		// @1820=10415196x15、@1864=10421377x1；@1908 起 7 行随机装备。
		Show: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofExpert, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 30},
			{Template: apocalypseRewardMaterialExpertA, Amount: 10},
			{Template: ApocalypseRewardMaterialB, Amount: 96},
			{Template: apocalypseRewardMaterialExpertB, Amount: 24},
			{Template: ApocalypseRewardAbyssTicket, Amount: 260},
			{Template: apocalypseRewardFlagRow1, Amount: 15},
			{Template: ApocalypseRewardUnknownFlag, Amount: 1},
		},
		Grant: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofExpert, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 30},
			{Template: apocalypseRewardMaterialExpertA, Amount: 10},
			{Template: ApocalypseRewardMaterialB, Amount: 96},
			{Template: apocalypseRewardMaterialExpertB, Amount: 24},
			{Template: ApocalypseRewardAbyssTicket, Amount: 260},
			{Template: apocalypseRewardFlagRow1, Amount: 15},
			{Template: ApocalypseRewardUnknownFlag, Amount: 1},
		},
		GrantIsAuthoritative: true,
		RandomGearSlots:      7,
	},
	{
		Choice: 2, Label: "master",
		// 抓包只覆盖难度1/难度2。master 的**通关证明**来自源表（basic=10421370），
		// 材料按同族口径外推但**没有实证**：master 的通关证明来自源表，
		// 材料数量是按 expert 同族外推的，日志里要能看出这一点。
		Show: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofMaster, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 40},
			{Template: apocalypseRewardMaterialExpertA, Amount: 15},
			{Template: ApocalypseRewardMaterialB, Amount: 120},
			{Template: apocalypseRewardMaterialExpertB, Amount: 30},
			{Template: ApocalypseRewardAbyssTicket, Amount: 300},
			{Template: apocalypseRewardFlagRow1, Amount: 20},
			{Template: ApocalypseRewardUnknownFlag, Amount: 2},
		},
		Grant: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofMaster, Amount: 1},
			{Template: ApocalypseRewardMaterialA, Amount: 40},
			{Template: apocalypseRewardMaterialExpertA, Amount: 15},
			{Template: ApocalypseRewardMaterialB, Amount: 120},
			{Template: apocalypseRewardMaterialExpertB, Amount: 30},
			{Template: ApocalypseRewardAbyssTicket, Amount: 300},
			{Template: apocalypseRewardFlagRow1, Amount: 20},
			{Template: ApocalypseRewardUnknownFlag, Amount: 2},
		},
		GrantIsAuthoritative: true,
	},
	{
		Choice: 4, Label: "match",
		// match 难度没有任何材料行样本，只发源表那件 basic。
		Show: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofMatch, Amount: 1},
		},
		Grant: []ApocalypseRewardItem{
			{Template: ApocalypseRewardProofMatch, Amount: 1},
		},
		GrantIsAuthoritative: true,
	},
}

// ApocalypseRewardFor 返回一个难度的固定奖励表。match 没有材料行样本，
// 只带源表那件 basic；master 的材料是按 expert 同族外推的。
func ApocalypseRewardFor(choice byte) (ApocalypseRewardTable, error) {
	for _, t := range apocalypseRewardTables {
		if t.Choice == choice {
			return t, nil
		}
	}
	return ApocalypseRewardTable{}, fmt.Errorf("apocalypse reward for difficulty key %#x is not sourced", choice)
}

// ApocalypseRewardLabels 列出已取证的难度标签，供日志与测试引用。
func ApocalypseRewardLabels() []string {
	out := make([]string, 0, len(apocalypseRewardTables))
	for _, t := range apocalypseRewardTables {
		out = append(out, t.Label)
	}
	return out
}

// ApocalypseStageTokens 是 N31 通关横幅与 N2252 尾 @7760 必须一致的阶段 token。
//
// 抓包实证（2026-10-08，两个难度各一场全清；两处必须相等是客户端的硬契约，
// 见 venus.go 的 VenusDungeonClearEnabled 注释）：
//
//	难度1（choice 0）@96.39s 的 N2252 尾 = d0 d7
//	难度2（choice 1）@212.28s 的 N2252 尾 = c3 43
//
// 两个样本对应的 stage 都是 6（五关全清）。此前这张表是「按阶段确定性生成」的
// 占位值（6b01..6b06）—— 那只是自洽，不是源值，已按实证改正。
// 其余四格仍无样本，按同族确定性派生，仅供两端对齐用。
var ApocalypseStageTokens = [6][2]byte{
	{0x6b, 0x01}, {0x6b, 0x02}, {0x6b, 0x03},
	{0x6b, 0x04}, {0xd0, 0xd7}, {0xc3, 0x43},
}

// ApocalypseStageToken 返回某一阶段的 token（stage 是「已到达的最大阶段」，
// 1 = 刚进攻坚房间，6 = 五关全清）。
func ApocalypseStageToken(stage int) ([2]byte, error) {
	if stage < 1 || stage > len(ApocalypseStageTokens) {
		return [2]byte{}, fmt.Errorf("apocalypse stage %d has no clear token", stage)
	}
	return ApocalypseStageTokens[stage-1], nil
}

// ApocalypseDungeonClearEnabled 构造 16B N31 通关横幅，格式与伊斯/维纳斯同款：
// @0 = 阶段 token（低 2B），@4.. 5B 运行期 nonce。nonce 只用于去重，客户端不解析。
func ApocalypseDungeonClearEnabled(stage int, nonce [5]byte) ([]byte, error) {
	token, err := ApocalypseStageToken(stage)
	if err != nil {
		return nil, err
	}
	p := make([]byte, 16)
	copy(p[0:], token[:])
	copy(p[4:], nonce[:])
	return p, nil
}

// ApocalypseFlipGearSlots 是一个难度的随机装备行数。抓包实测：
// 难度1（choice 0）= 8 行、难度2（choice 1）= 7 行（固定材料之后的那些
// 1001/1002/1003xxxxx 行）。master/match 没有样本，按 expert 取 7。
func ApocalypseFlipGearSlots(choice byte) int {
	if choice == 0 {
		return 8
	}
	return 7
}

// apocalypseRandIntn 返回 [0,n) 的均匀随机整数，用 crypto/rand（与
// VenusRollFlipGear 同源），并对取模偏差做拒绝采样。
//
// 存在的理由：本文件用的是 `crypto/rand`（不是 `math/rand`），它没有 Intn；
// 直接 `% n` 会有模偏差，对 15/35/50 这种权重已经足够小，但既然池子大小可变、
// 拒绝采样的代价又极低，就做对。
func apocalypseRandIntn(n int) int {
	if n <= 1 {
		return 0
	}
	max := big.NewInt(int64(n))
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

// ApocalypseFlipGearTier 是翻牌装备池里的一档：品级 + 权重 + 模板表。
type ApocalypseFlipGearTier struct {
	// Rarity 是品级序号（与 inventory.equipmentRarityNames 同序）：
	// 2 = rare（魔法）、3 = unique（神器）、4 = epic（史诗）。
	Rarity int `json:"rarity"`
	// Label 是给人看的档位名（魔法/神器/史诗），只用于日志。
	Label string `json:"label,omitempty"`
	// Weight 是这一档的相对权重（业主口径 15/35/50）。
	Weight int `json:"weight"`
	// Templates 是这一档的装备模板号。
	Templates []uint32 `json:"templates"`
}

// ApocalypseFlipGearPool 是**末世录翻牌专用装备池**（业主 2026-10-08 方案 A）。
//
// 为什么要新建而不是复用维纳斯那个池子（`venus-flip-gear.generated.json`）：
// 那个池子是 `rarity=[2,3]`（魔法/神器）**没有 SS(史诗)**，凑不出业主的
// 15/35/50 三档。本池子按 rarity 2/3/4 三档分别列模板，语义与概率一一对应。
//
// 生成方式与 `tmp_enum_gear`（维纳斯池当年的脚本）同源：读 PVF 的
// `list/equipment.lst` 全量清单，逐条解析 `[rarity]` / `[minimum level]` /
// `[equipment type]`，筛出 115 级可发放装备（见 cmd/dfo-tool 的 flippool 子命令）。
type ApocalypseFlipGearPool struct {
	Source struct {
		Format   string `json:"format"`
		Checksum string `json:"checksum"`
		Origin   string `json:"origin"`
	} `json:"source"`
	Level int                      `json:"level"`
	Tiers []ApocalypseFlipGearTier `json:"tiers"`
}

// LoadApocalypseFlipGearPool 读已导出的翻牌装备池。
func LoadApocalypseFlipGearPool(path string) (ApocalypseFlipGearPool, error) {
	var p ApocalypseFlipGearPool
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	total := 0
	for _, t := range p.Tiers {
		total += len(t.Templates)
	}
	if total == 0 {
		return p, fmt.Errorf("apocalypse flip gear pool %s has no templates", path)
	}
	return p, nil
}

// ApocalypseFlipGearPoolStats 报告池子里**每一档**的件数与配置权重。
func ApocalypseFlipGearPoolStats(p ApocalypseFlipGearPool) string {
	parts := make([]string, 0, len(p.Tiers))
	for _, t := range p.Tiers {
		label := t.Label
		if label == "" {
			label = fmt.Sprintf("rarity%d", t.Rarity)
		}
		parts = append(parts, fmt.Sprintf("%s(%d):%d件 %d%%", label, t.Rarity, len(t.Templates), t.Weight))
	}
	return strings.Join(parts, "  ")
}

// ApocalypseFlipGearTierWeights 是默认的三档权重（业主口径 15/35/50）。
//
// 池子文件里每档自带 `weight`；这里只是**兜底默认**（池子没写 weight 时用）。
//
//	rarity 2 = rare（魔法）  15%
//	rarity 3 = unique（神器）35%
//	rarity 4 = epic（史诗）  50%
var ApocalypseFlipGearTierWeights = map[int]int{2: 15, 3: 35, 4: 50}

// ApocalypseRollFlipGearTiers 从分档池里按权重抽 n 件（业主口径 15/35/50）。
//
// 实现要点：
//   - **逐件独立掷档位**，所以每一件的档位分布严格等于配置权重；
//   - 某档被取空时把它从候选里摘掉，权重按条件概率分摊给剩余档位；
//   - 同一场内模板去重；
//   - 池子为空 / n<=0 时返回 nil，奖励链降级为只发固定项。
func ApocalypseRollFlipGearTiers(pool ApocalypseFlipGearPool, n int) []uint32 {
	if n <= 0 || len(pool.Tiers) == 0 {
		return nil
	}
	type tier struct {
		weight    int
		templates []uint32
	}
	var tiers []tier
	total := 0
	for _, t := range pool.Tiers {
		if len(t.Templates) == 0 {
			continue
		}
		w := t.Weight
		if w <= 0 {
			w = ApocalypseFlipGearTierWeights[t.Rarity]
		}
		if w <= 0 {
			continue
		}
		// 去掉 0 与非本档的重复（防御：池子文件可能带脏数据）。
		ids := make([]uint32, 0, len(t.Templates))
		for _, id := range t.Templates {
			if id != 0 {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			continue
		}
		tiers = append(tiers, tier{weight: w, templates: ids})
		total += w
	}
	if total <= 0 {
		return nil
	}
	used := map[uint32]bool{}
	out := make([]uint32, 0, n)
	for len(out) < n {
		pick := apocalypseRandIntn(total)
		chosen := 0
		acc := 0
		for i, t := range tiers {
			acc += t.weight
			if pick < acc {
				chosen = i
				break
			}
		}
		candidates := tiers[chosen].templates
		start := apocalypseRandIntn(len(candidates))
		found := false
		for i := 0; i < len(candidates); i++ {
			id := candidates[(start+i)%len(candidates)]
			if used[id] {
				continue
			}
			used[id] = true
			out = append(out, id)
			found = true
			break
		}
		if !found {
			// 这一档取空了：摘掉它，权重分摊给剩余档位。
			total -= tiers[chosen].weight
			tiers = append(tiers[:chosen], tiers[chosen+1:]...)
			if total <= 0 || len(tiers) == 0 {
				break
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApocalypseBasicClearReward 构造 7772B N2252 终局翻牌第一排。
//
// 行布局（**按抓包逐字节对齐，2026-10-08 修正**）：
//
//	行 0        @0            40B 记录（@0 = u32 模板、@4 = u32 数量）
//	@40         **必须保持全零**（抓包两个难度的 @40 都是 0x0；这里曾经写入
//	            「第 2 行」，导致客户端翻牌界面整片显示 EMPTY）
//	行 1..n     @1600 + 44*i  44B 记录（i 从 0 起）
//	@7760       **通关耗时毫秒（u64 LE）**
//
// 抓包实测（D:\zhuabao\captures\20261008-105227）：
//
//	难度1 @96.39s ：@0=10421367x1、@1600=10421371x20、@1644=10421373x96、…
//	                 @7760 = d0d7 = 53463 ms ≈ 53.5 秒
//	难度2 @212.28s：@0=10421369x1、@1600=10421371x30、@1644=10421372x10、…
//	                 @7760 = c343 = 49987 ms ≈ 50 秒
//	两者的 @40 都是 0。
//
// ★ @7760 的语义（2026-10-08 修正）：此前实现把它当作「阶段 token」（一张
// 自造的 6b01..6b06 表），**错了**。两个难度的实测值与「nav 载入 → 终局清关」
// 的墙钟耗时吻合到秒级（55s / 50s），而且规格
// 2252-LEGIONBASICREWARD.md 明写「@7760 8B：耗时（毫秒）—— 显示本次耗时」
// 「14250FAF0 读取后转换秒数」，2895 规格也把 @152..199 称作「六个目标索引 u64
// 耗时」。客户端用它渲染结算面板的「通关时间」，写错值就是截图里的
// **「通关时间 EMPTY」**，并连带让奖励行不被渲染（业主 2026-10-08 反馈
// 「翻牌还是空的，但是却给出了物品」）。
//
// elapsedMS 为 0 时写 0（不编造）。
//
// clearTemplate 是本次通关证明那一件（取值见 ApocalypseRewardFor），items 是
// 固定材料，gear 是本场 roll 到的随机装备（各占一行、数量 1，与抓包里
// 1001/1002/1003xxxxx 那批行同形；gear 为空时这一段不占位）。
func ApocalypseBasicClearReward(clearTemplate uint32, items []ApocalypseRewardItem, gear []uint32, elapsedMS uint64) ([]byte, error) {
	if clearTemplate == 0 {
		return nil, fmt.Errorf("apocalypse clear reward template is missing")
	}
	type row struct {
		template uint32
		value    uint32
	}
	rows := make([]row, 0, len(items)+len(gear)+1)
	rows = append(rows, row{template: clearTemplate, value: 1})
	for _, it := range items {
		if it.Template == 0 {
			return nil, fmt.Errorf("apocalypse reward row %d has no template", len(rows))
		}
		if it.Template == clearTemplate {
			continue // 同一件不重复占位
		}
		rows = append(rows, row{template: it.Template, value: it.Amount})
	}
	for _, g := range gear {
		if g == 0 {
			return nil, fmt.Errorf("apocalypse gear row %d has no template", len(rows))
		}
		rows = append(rows, row{template: g, value: 1})
	}
	if len(rows) > 16 {
		return nil, fmt.Errorf("apocalypse basic reward carries %d rows, native layout holds 16", len(rows))
	}
	raw := make([]byte, 7772)
	for i, r := range rows {
		var off int
		if i == 0 {
			off = 0
		} else {
			off = 1600 + 44*(i-1)
		}
		if off+8 > len(raw) {
			return nil, fmt.Errorf("apocalypse reward row %d overflows the native layout", i)
		}
		binary.LittleEndian.PutUint32(raw[off:], r.template)
		binary.LittleEndian.PutUint32(raw[off+4:], r.value)
	}
	binary.LittleEndian.PutUint64(raw[7760:], elapsedMS)
	return raw, nil
}

// ApocalypseAdditionalClearReward 构造 2405B N2253 第二排。
//
// 抓包实测：末世录两场全清的 N2253 都是 **2408 字节全零**（逐字节核对，
// 非零字节数 0），也就是这一排不需要任何展示行——追加奖励走别处。
// 本函数因此返回全零体，只作为链序占位，不编造展示行。
func ApocalypseAdditionalClearReward() ([]byte, error) {
	return make([]byte, 2405), nil
}
