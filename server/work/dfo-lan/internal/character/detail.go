package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// 当前角色属性采用正常状态100%；基础资料与详细属性必须使用同一状态比例。
// 这不是战斗当前HP/MP，也不通过队伍显示包回复血量或魔法。
const entryBasePercent byte = 100

// EntryAddition uses persisted source attributes and initial skills in native
// wire units. Equipment and advancement-specific skill learning are separate.
func (s *Service) EntryAddition(role Character) ([]byte, error) {
	return s.entryAddition(role, nil, false)
}

// AdventureEliteSkillUsage 只接受该角色已掌握的技能使用偏好。
// 设置不会授予技能或改变等级；实际技能资料仍必须由角色存档生成。
func (s *Service) AdventureEliteSkillUsage(role Character, requested [30]int32) ([30]int32, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return [30]int32{}, err
	}
	known, err := s.knownSkills(role, state, 0)
	if err != nil {
		return [30]int32{}, err
	}
	for _, id := range requested {
		// 零值来自当前客户端未设置的原生记录；-1不指向任何技能。
		if id == 0 || id == -1 {
			continue
		}
		if id < 0 || id > math.MaxUint16 || known[uint16(id)] == 0 {
			return [30]int32{}, fmt.Errorf("精锐角色技能使用设置包含未掌握的技能：%d", id)
		}
	}
	return requested, nil
}

// visualOverrides is used by the Clone reattach sequence. The ordinary entry
// packet keeps its confirmed projection unchanged.
func (s *Service) entryAddition(role Character, visualOverrides map[uint16]uint32, omitResolvedClones bool) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	if state.SourceSHA256 == "" {
		return nil, fmt.Errorf("missing source character data")
	}
	stats, err := entryPackedStats(state)
	if err != nil {
		return nil, err
	}
	return s.entryAdditionWithStats(role, state, stats, visualOverrides, omitResolvedClones)
}

// 玩家和队友使用同一份源属性及单位换算，避免复制成长公式。
func entryPackedStats(state State) (protocol.PackedEntryStats, error) {
	v := state.Attributes
	var failure error
	scaled := func(name string, multiplier, max float64) uint32 {
		n := float64(v[name]) * multiplier
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > max {
			failure = fmt.Errorf("attribute %s out of native bounds", name)
			return 0
		}
		return uint32(math.Round(n))
	}
	signed := func(name string, multiplier float64) int16 {
		n := float64(v[name]) * multiplier
		if math.IsNaN(n) || math.IsInf(n, 0) || n < math.MinInt16 || n > math.MaxInt16 {
			failure = fmt.Errorf("attribute %s out of signed native bounds", name)
			return 0
		}
		return int16(math.Round(n))
	}
	stats := protocol.PackedEntryStats{
		HP: scaled("[hp max]", 10, math.MaxUint32), MP: scaled("[mp max]", 10, math.MaxUint32),
		Core: [4]uint16{uint16(scaled("[physical attack]", 10, math.MaxUint16)), uint16(scaled("[physical defense]", 10, math.MaxUint16)), uint16(scaled("[magical attack]", 10, math.MaxUint16)), uint16(scaled("[magical defense]", 10, math.MaxUint16))},
		// Native 145d2e760 divides these wire values by ten before applying
		// actor attributes. Source parser 147557e90 scales all input by ten,
		// including MP regeneration, whose effective runtime unit differs.
		Element:       [4]int16{signed("[fire resistance]", 10), signed("[water resistance]", 10), signed("[dark resistance]", 10), signed("[light resistance]", 10)},
		Inventory:     int32(scaled("[inventory limit]", 10, math.MaxInt32)),
		Regeneration:  [2]int16{signed("[hp regen speed]", 10), signed("[mp regen speed]", 10)},
		Movement:      scaled("[move speed]", 10, math.MaxUint32),
		AttackCasting: [2]uint16{uint16(scaled("[attack speed]", 10, math.MaxUint16)), uint16(scaled("[cast speed]", 10, math.MaxUint16))},
		RecoveryJump:  [2]int16{signed("[hit recovery]", 10), signed("[jump power]", 10)},
		Weight:        int32(scaled("[weight]", 10, math.MaxInt32)), BasePercent: entryBasePercent,
	}
	return stats, failure
}

func (s *Service) entryAdditionWithStats(role Character, state State, stats protocol.PackedEntryStats, visualOverrides map[uint16]uint32, omitResolvedClones bool) ([]byte, error) {
	var trees [2][]protocol.EntrySkill
	for i := range trees {
		known, e := s.knownSkills(role, state, i)
		if e != nil {
			return nil, e
		}
		ids := skillOrder(state, known)
		for _, id := range ids {
			trees[i] = append(trees[i], protocol.EntrySkill{ID: uint16(id), Level: known[uint16(id)]})
		}
	}
	// The extended-slot unlock byte rides the same saved bag the armoury is
	// rebuilt from, so it is read straight off the raw state here rather than
	// through inventory.ReadBag: a rejected bag must not turn a login into a
	// refusal, and an absent field simply reads as "nothing unlocked".
	var projection struct {
		Inventory struct {
			ExpandEquipFlags byte                    `json:"expand_equip_flags"`
			Worn             []protocol.DetailedWorn `json:"worn"`
		} `json:"inventory"`
	}
	if e := json.Unmarshal(role.State, &projection); e != nil {
		return nil, e
	}
	var worn []protocol.DetailedWorn
	if s.DetailedWornCandidate {
		var wornProjection struct {
			Inventory struct {
				Worn    []inventory.BagEquipment          `json:"worn"`
				Special map[byte][]inventory.BagEquipment `json:"special_equipment"`
			} `json:"inventory"`
		}
		if e := json.Unmarshal(role.State, &wornProjection); e == nil {
			b := inventory.Bag{Worn: wornProjection.Inventory.Worn, Special: wornProjection.Inventory.Special}
			for _, item := range b.WornBaseItems() {
				if omitResolvedClones && item.Group == 0 && visualOverrides[item.Slot] != 0 {
					continue
				}
				// Avatar slots (<= 11) ride the avatar row layout; the creature
				// body slot 26, creature gear slots 27..29 and the pet-skin slot 32
				// ride the plain / creature-extension layouts (protocol.DetailedEquipment
				// pins all of them against native reader sub_1452C1540). Slots 12..25
				// stay excluded exactly as before: their window data already arrives
				// via NOTI 13/14 and their mode-1 projection is a separate,
				// unverified change.
				//
				// 实机 2026-09-27：幻化槽（32）原先被这条过滤挡掉，导致重登后
				// 客户端手上没有槽 32 的物品，F6 幻化框拿不到模板而空白（同一次
				// 登录里槽 26 的本体宠物因为被包含所以正常）。它是 [creature] 物品，
				// 行布局与槽 26 同源。
				if !(item.Slot <= 11 || (item.Slot >= 26 && item.Slot <= 29) || item.Slot == inventory.CreatureSkinSlot) {
					continue
				}
				var dw protocol.DetailedWorn
				dw.Slot = item.Slot
				dw.Template = item.Template
				dw.Durability = item.Durability
				dw.Record = item.Record
				// 头像扩展（孔/徽章）**只有时装行有格子**：槽 ≤ 11 走 avatar 分支，
				// 槽 12..25 不投影，槽 26~29 / 32 走原生 sub_1452C1540 的
				// creature/普通分支，行里根本没有 u32len+blob 两格。
				// 所以这里只给时装槽下传头像块；存档里若混进了这种块（2026-10-07
				// 邮件补孔缺陷就写入过 30 字节全零块），丢掉它而不是让整包被拒 ——
				// 同 ReadBag 的既有口径：一份坏数据不能让角色登不进去。
				if item.Slot <= 11 {
					dw.AvatarOptions = item.AvatarOptions
					if len(dw.AvatarOptions) == 0 && s.Equipment != nil {
						// 老存档时装未带孔扩展：下发视图按 PVF 默认孔就地补上，
						// 客户端才能显示孔（镶嵌路径 UseEmblems 同样会补，这里只
						// 改下发视图、不写回存档）。
						dw.AvatarOptions = s.Equipment.DefaultAvatarSockets(item.Template)
					}
					dw.AvatarSockets = item.AvatarSockets
				}
				dw.Period = item.Period
				if item.Slot <= 11 && item.Group == 0 {
					dw.HeaderTemplateA = b.CloneAvatarLook(item)
					// Coexisting ordinary look: the row's primary template remains
					// the clear avatar. Native sub_1452C1540 treats row+24 as the
					// appearance override for this item category (bit 21). row+28
					// is consumed only when row+24 has bit 25, the random-clear-
					// avatar category, so keep it zero for a normal look.
					for _, other := range b.Worn {
						if dw.HeaderTemplateA == 0 && other.Slot == item.Slot && other.Group == 1 {
							dw.HeaderTemplateA = other.Template
							break
						}
					}
					if dw.HeaderTemplateA == 0 {
						dw.HeaderTemplateA = visualOverrides[item.Slot]
					}
				}
				worn = append(worn, dw)
			}
		}
	}
	fame, err := s.EquipmentFame(role.State)
	if err != nil {
		return nil, err
	}
	var adventureLevel uint32
	if s.Store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		adventureLevel, err = s.Store.AdventureLevel(ctx, role.AccountID, role.ID)
		if err != nil {
			return nil, fmt.Errorf("读取角色属性包的冒险团等级：%w", err)
		}
	}
	return protocol.UserInfoAdditionProbe(protocol.EntryAdditionProbe{Context: s.ChannelContext, ActorServerID: role.WireID, Experience: state.Experience, Stats: stats, SkillTrees: trees, Worn: worn, Fame: fame, ExpandEquipFlags: projection.Inventory.ExpandEquipFlags, AdventureLevel: adventureLevel})
}

// EquipmentFame复用完整穿戴名望投影；明细可通过EquipmentFameBreakdown核对。
func (s *Service) EquipmentFame(raw json.RawMessage) (uint32, error) {
	detail, err := s.EquipmentFameBreakdown(raw)
	return detail.Total, err
}

// The exact .chr loader at 147559d80 stores (ID, first value) in the
// initial-skill vector and (ID, second value) in a separate vector. All 17
// imported professions use second value=1 in their initial section. Preserve
// that supported shape; do not interpret arbitrary growth/PvP conditions.
//
// The [skill] block this reads sits in the profession's top-level [initial
// value] section and is shared by every advancement slot, so an advanced
// character keeps it. The upstream 46d34763 advancement/pilot refusal was
// removed (user ruling 20260918): advanced non-pilot characters are valid
// and were served by the baseline loader.
//
// 单机裁定 20260919（无条件通过）：不可建模的源行——第三格≠1 的未达标授予
// （骑士进阶槽 [4/1/5] [3/1/10] 曾让建号后的自动进场整包被拒）、越界 id/等级
// ——一律跳过（记零），绝不拒绝入场追加包。行语义（等级门槛）未建模，不猜。
func initialSkills(s State) []protocol.EntrySkill {
	cells := s.InitialSkills
	cells = cells[:len(cells)-len(cells)%3]
	var out []protocol.EntrySkill
	for i := 0; i+2 < len(cells); i += 3 {
		id, level, condition := cells[i], cells[i+1], cells[i+2]
		if id <= 0 || id > 65535 || level <= 0 || level > 255 || condition != 1 {
			continue
		}
		out = append(out, protocol.EntrySkill{ID: uint16(id), Level: byte(level)})
	}
	return out
}

func (s *Service) EntrySkills(role Character) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	// Current NOTI19 sends learned rows only. The client owns the catalog of
	// future learnable skills from PVF; the server must not invent zero-rank
	// rows that were absent from the captured initial packet.
	var trees [2]protocol.SkillTree
	for i := range trees {
		rows, e := s.skillRows(role, state, i)
		if e != nil {
			return nil, e
		}
		trees[i] = protocol.SkillTree{SP: state.SkillPoints[i], TP: state.TechniquePoints[i], Skills: rows}
	}
	return protocol.SkillInfoTrees(state.Level, trees)
}
