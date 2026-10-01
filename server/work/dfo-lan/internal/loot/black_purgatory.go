package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

const BlackPurgatorySquadDungeon uint32 = 100000527
const blackPurgatoryCardModel = "black-purgatory-card-v1"
const blackPurgatoryBossModel = "local-black-purgatory-boss-v1"

var blackPurgatoryBossTiers = [3]string{"epic", "mythic", "corrupt_product"}

type blackPurgatoryBossGroup struct {
	SourceGroup uint32 `json:"source_group"`
	Candidates  []struct {
		Template     uint32 `json:"template"`
		MinimumLevel int32  `json:"minimum_level"`
		Rarity       int32  `json:"rarity"`
		Path         string `json:"path"`
		SHA256       string `json:"sha256"`
	} `json:"candidates"`
}

type blackPurgatoryBossRules struct {
	GroupScript   string                             `json:"group_script"`
	GroupHash     string                             `json:"group_script_sha256"`
	RoutingScript string                             `json:"routing_script"`
	RoutingHash   string                             `json:"routing_script_sha256"`
	Model         string                             `json:"model"`
	Denominator   uint32                             `json:"denominator"`
	Rates         map[string]uint32                  `json:"rates"`
	Groups        map[string]blackPurgatoryBossGroup `json:"groups"`
}

type BlackPurgatoryRewards struct {
	VIPSourceOnly  []RewardBoxCandidate    `json:"vip_source_only"`
	Model          string                  `json:"model"`
	Source         string                  `json:"source"`
	ClientSource   string                  `json:"client_pvf_sha256"`
	Script         string                  `json:"script"`
	ScriptHash     string                  `json:"script_sha256"`
	Dungeon        uint32                  `json:"dungeon"`
	Cards          []RewardBoxCandidate    `json:"cards"`
	Boss           blackPurgatoryBossRules `json:"boss_equipment"`
	boxes          RewardBoxSource
	items          map[uint32]catalog.LootItem
	bossDurability map[uint32]uint16
}

// 装备范围取自当前PVF；顶层概率和组内等概率为用户确认的本服规则。
func (r *BlackPurgatoryRewards) ValidateBossEquipment(equipment *inventory.EquipmentCatalog) error {
	if r == nil || equipment == nil || equipment.Source.Checksum != r.Source ||
		r.Boss.Model != blackPurgatoryBossModel || r.Boss.Denominator != 1000000 ||
		len(r.Boss.Rates) != 3 || len(r.Boss.Groups) != 3 {
		return fmt.Errorf("黑鸦本服领主掉落规则或装备目录不完整")
	}
	durabilities := map[uint32]uint16{}
	for i, tier := range blackPurgatoryBossTiers {
		group, ok := r.Boss.Groups[tier]
		rate, rateOK := r.Boss.Rates[tier]
		if !ok || !rateOK || rate > r.Boss.Denominator || len(group.Candidates) == 0 ||
			group.SourceGroup != [3]uint32{1010001, 1010500, 1010007}[i] {
			return fmt.Errorf("黑鸦领主奖励组无效：%s", tier)
		}
		for _, candidate := range group.Candidates {
			definition, err := equipment.Definition(candidate.Template)
			if err != nil {
				return fmt.Errorf("黑鸦装备未导入：%d：%w", candidate.Template, err)
			}
			level, rarity := definition.Fields["[minimum level]"], definition.Fields["[rarity]"]
			if candidate.Template == 0 || definition.Path != candidate.Path || definition.SHA256 != candidate.SHA256 ||
				candidate.MinimumLevel <= 0 || len(level) != 1 || level[0].Type != 0 || level[0].Value != candidate.MinimumLevel ||
				len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value != candidate.Rarity || candidate.Rarity != [3]int32{4, 7, 4}[i] {
				return fmt.Errorf("黑鸦装备源数据不一致：%d", candidate.Template)
			}
			if _, exists := durabilities[candidate.Template]; exists {
				return fmt.Errorf("黑鸦领主装备组重复：%d", candidate.Template)
			}
			durability, err := equipment.Reward(candidate.Template)
			if err != nil {
				return fmt.Errorf("黑鸦装备不能发放：%d：%w", candidate.Template, err)
			}
			durabilities[candidate.Template] = durability
		}
	}
	r.bossDurability = durabilities
	return nil
}

func blackPurgatoryFinalBossDead(d *dungeon.Session) bool {
	if d == nil || !d.Loaded || d.Definition.ID != BlackPurgatorySquadDungeon ||
		(d.Room.Map != 100003073 && d.Room.Map != 100003089) {
		return false
	}
	for _, monster := range d.Monsters {
		if monster.Template == 109012760 && monster.Rank == 3 && !monster.NonCombat &&
			!monster.APC && d.Dead[monster.Entity] {
			return true
		}
	}
	return false
}

// 只匹配源小队最终两张地图的阿斯特罗斯，不把中途精英或同名剧情怪当作奖励目标。
func BlackPurgatoryBossDeath(d *dungeon.Session, entity uint16) bool {
	if !blackPurgatoryFinalBossDead(d) {
		return false
	}
	for _, monster := range d.Monsters {
		if monster.Entity == entity && monster.Template == 109012760 && monster.Rank == 3 &&
			!monster.NonCombat && !monster.APC && d.Dead[entity] {
			return true
		}
	}
	return false
}

// 1473D51F0读取奖励类型；1473D5232处理扩展条件；随后读取权重、显示类型和数量。
// 小队五档条件的普通翻牌表相同，类型1为VIP奖励，不能混入普通奖励池。
func LoadBlackPurgatoryRewards(path string, boxes RewardBoxSource, itemLookup func(uint32) (catalog.LootItem, bool)) (*BlackPurgatoryRewards, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r BlackPurgatoryRewards
	if err = json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return NewBlackPurgatoryRewards(r, boxes, itemLookup)
}

// NewBlackPurgatoryRewards binds validated source tables to the existing reward
// expansion and storage projection, regardless of their input format.
func NewBlackPurgatoryRewards(r BlackPurgatoryRewards, boxes RewardBoxSource, itemLookup func(uint32) (catalog.LootItem, bool)) (*BlackPurgatoryRewards, error) {
	if r.Model != blackPurgatoryCardModel || r.Dungeon != BlackPurgatorySquadDungeon ||
		r.Script != "etc/dungeonspecialreward.etc" || len(r.Cards) != 5 || boxes == nil || itemLookup == nil {
		return nil, fmt.Errorf("黑鸦奖励配置或礼包目录不完整")
	}
	for _, hash := range []string{r.Source, r.ClientSource, r.ScriptHash} {
		if data, err := hex.DecodeString(hash); err != nil || len(data) != 32 {
			return nil, fmt.Errorf("黑鸦奖励来源校验值无效")
		}
	}
	r.boxes, r.items = boxes, map[uint32]catalog.LootItem{}
	// 全图校验先于入场，缺少任一分支都不能在抽中后静默吞奖。
	visiting := map[uint32]bool{}
	var visit func(uint32, int) (uint32, error)
	visit = func(id uint32, depth int) (uint32, error) {
		if depth > rewardBoxMaxDepth || visiting[id] {
			return 0, fmt.Errorf("黑鸦奖励容器循环或层数超限：%d", id)
		}
		if box, ok := boxes.RewardBox(id); ok {
			if len(box.Pools) == 0 {
				return 0, fmt.Errorf("黑鸦奖励容器为空：%d", id)
			}
			visiting[id] = true
			defer delete(visiting, id)
			var count uint32
			for _, pool := range box.Pools {
				if len(pool.Candidates) == 0 || pool.draws() > 8 {
					return 0, fmt.Errorf("黑鸦奖励抽取组无效：%d", id)
				}
				var total uint64
				var largest uint32
				for _, c := range pool.Candidates {
					if c.Weight == 0 || c.Count == 0 {
						return 0, fmt.Errorf("黑鸦奖励权重或数量无效：%d", id)
					}
					total += uint64(c.Weight)
					n, err := visit(c.Template, depth+1)
					if err != nil {
						return 0, err
					}
					if _, nested := boxes.RewardBox(c.Template); nested {
						if c.Count > 8 {
							return 0, fmt.Errorf("黑鸦奖励包装数量超限")
						}
						n *= c.Count
					}
					largest = max(largest, n)
				}
				if total == 0 || total > uint64(^uint32(0)) {
					return 0, fmt.Errorf("黑鸦奖励权重溢出")
				}
				count += largest * pool.draws()
				if count > 8 {
					return 0, fmt.Errorf("黑鸦翻牌物品数量超出通知边界")
				}
			}
			return count, nil
		}
		item, ok := itemLookup(id)
		if !ok || item.ID != id || item.Kind != "stackable" || boxes.Container(id) || !boxes.Item(id) {
			return 0, fmt.Errorf("黑鸦奖励物品未完整导入：%d", id)
		}
		r.items[id] = item
		return 1, nil
	}
	var total uint32
	seen := map[uint32]bool{}
	for _, c := range r.Cards {
		if c.Weight == 0 || c.Weight > 10000 || c.Count != 1 || seen[c.Template] {
			return nil, fmt.Errorf("黑鸦翻牌分支无效")
		}
		seen[c.Template] = true
		total += c.Weight
		if _, ok := boxes.RewardBox(c.Template); !ok {
			return nil, fmt.Errorf("黑鸦翻牌缺少奖励包装：%d", c.Template)
		}
		if _, err := visit(c.Template, 0); err != nil {
			return nil, err
		}
	}
	if total != 10000 {
		return nil, fmt.Errorf("黑鸦翻牌权重之和必须为10000")
	}
	return &r, nil
}

func (r *BlackPurgatoryRewards) StorageCatalog(c catalog.LootCatalog) (catalog.LootCatalog, error) {
	if r == nil || r.Source != c.Source.Checksum {
		return c, fmt.Errorf("黑鸦奖励与物品目录来源不一致")
	}
	items := make(map[uint32]catalog.LootItem, len(c.Items)+len(r.items))
	for id, item := range c.Items {
		items[id] = item
	}
	for id, item := range r.items {
		if _, exists := items[id]; !exists {
			items[id] = item
		}
	}
	c.Items = items
	return c, nil
}

func (s *Service) PlanBlackPurgatoryCards(role Role, d *dungeon.Session, seed uint32) (CardPlan, error) {
	p := CardPlan{}
	if s == nil || s.BlackPurgatory == nil || len(s.BlackPurgatory.bossDurability) == 0 || !blackPurgatoryFinalBossDead(d) ||
		d.Definition.ID != BlackPurgatorySquadDungeon || role.ConfigVersion != savecontract.Identity() {
		return p, fmt.Errorf("黑鸦奖励尚未加载或挑战未通关")
	}
	r := s.BlackPurgatory
	p = CardPlan{Run: d.RunID, Source: r.Source, Model: blackPurgatoryCardModel}
	return p, nil
}
func (s *Service) PrepareBlackPurgatoryCards(p CardPlan, seed uint32) (json.RawMessage, error) {
	r := s.BlackPurgatory
	rng := RNG{Seed: seed}
	choice, ok := (RewardBoxPool{Candidates: r.Cards}).pick(&rng)
	if !ok {
		return nil, fmt.Errorf("黑鸦翻牌奖励池为空")
	}
	items, next, skipped := OpenRewardBoxes(rng.Seed, r.boxes, []Award{{Template: choice.Template, Amount: 1}})
	if len(items) == 0 || len(items) > len(p.Items) || len(skipped) != 0 {
		return nil, fmt.Errorf("黑鸦翻牌展开失败：%v", skipped)
	}
	copy(p.Items[:], items)
	rng.Seed = next
	p.BossModel = r.Boss.Model
	for i, tier := range blackPurgatoryBossTiers {
		if rng.Next(r.Boss.Denominator) < r.Boss.Rates[tier] {
			candidates := r.Boss.Groups[tier].Candidates
			p.BossItems[i] = Award{Template: candidates[rng.Next(uint32(len(candidates)))].Template, Amount: 1}
		}
	}
	return json.Marshal(p)
}
func (s *Service) ValidateBlackPurgatoryCards(p CardPlan, d *dungeon.Session) error {
	r := s.BlackPurgatory
	if p.Run != d.RunID || p.Source != r.Source || p.Model != blackPurgatoryCardModel || p.Items[0].Amount == 0 || p.Gold != 0 {
		return fmt.Errorf("黑鸦冻结奖单不匹配")
	}
	return nil
}

type BlackPurgatoryBossReceipt struct {
	Run         string
	Index       byte
	Award       Award
	Destination uint16
}

func (s *Service) ValidateBlackPurgatoryBossOwner(role Role, index byte) error {
	if s == nil || index < 1 || index > 3 || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fmt.Errorf("黑鸦领主奖励归属无效")
	}
	return nil
}
func (s *Service) ValidateBlackPurgatoryBossPlan(role Role, plan CardPlan, run string, index byte, expected Award) error {
	if plan.Run != run || plan.Source != role.ConfigVersion || plan.Model != blackPurgatoryCardModel ||
		plan.BossModel != blackPurgatoryBossModel || plan.BossItems[index-1] != expected || expected.Template == 0 || expected.Amount != 1 {
		return fmt.Errorf("黑鸦领主奖励与冻结奖单不一致")
	}
	return nil
}
func (s *Service) PrepareBlackPurgatoryBoss(current Role, run string, index byte, expected Award) (json.RawMessage, json.RawMessage, error) {
	bag, err := inventory.ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	bag, slots, err := bag.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, expected.Template, 1)
	if err != nil {
		return nil, nil, err
	}
	state, err := inventory.SaveBag(current.State, bag)
	if err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(BlackPurgatoryBossReceipt{run, index, expected, slots[0]})
	return state, data, err
}

const BlackPurgatoryCardModel = blackPurgatoryCardModel
