package protocol

import "fmt"

// TagCharacter 是145962410的flag=1分支使用的新开战角色快照。
// 名字由外层1444FCF90读取；装备和技能不能借用普通玩家登录包的包头。
type TagCharacter struct {
	WireID           uint16
	Name             string
	Level            byte
	Profession       byte
	Advancement      byte
	Stats            PackedEntryStats
	ExpandEquipFlags byte
	Appearance       []EquippedAppearance
	Worn             []DetailedWorn
	Skills           []LearnedSkill
	Intensions       [3]SkillVariation
	Options          [5]SkillVariation
}

// TagTeamInfo对应NOTI1381的1445021A0：u8数量及每个成员的服务器、账号槽位。
// 这里的名单只建立活动编队索引，随后仍须用NOTI1382建立角色对象。
func TagTeamInfo(members []BleedingMineMember) ([]byte, error) {
	if len(members) == 0 || len(members) > 4 {
		return nil, fmt.Errorf("战斗编队人数必须为1至4")
	}
	p := []byte{byte(len(members))}
	seen := map[BleedingMineMember]bool{}
	for _, m := range members {
		if m.Server == 0 || m.Slot < 0 || m.Slot >= 255 || seen[m] {
			return nil, fmt.Errorf("战斗编队成员索引无效或重复")
		}
		seen[m] = true
		p = append(p, m.Server, byte(m.Slot))
	}
	return p, nil
}

// NOTI1382的mode0路径自动从当前玩家复制首位角色；正文只携带其余成员。
// 1444FD120/13A/8E0分别读取玩家数、玩家wireID、额外角色数。
func TagCharacterInfo(owner uint16, companions []TagCharacter) ([]byte, error) {
	slots := make([]byte, len(companions))
	for i := range slots {
		slots[i] = byte(i + 1)
	}
	return tagCharacterInfo(owner, slots, companions)
}

// AdventureEliteCharacterInfo 用账号选角槽位建立类型2的精锐资料容器。
// 1444FDBB3..1444FDC3A按收到的u8槽位扩展列表；1879的142E5B2B9
// 再用1754中的账号槽位查找。不能沿用矿区的连续队内序号，也不发清理类型0的1381。
// 调用方必须确认当前频道会使1444FD014进入精锐分支。
func AdventureEliteCharacterInfo(owner uint16, slots []byte, companions []TagCharacter) ([]byte, error) {
	return tagCharacterInfo(owner, slots, companions)
}

func tagCharacterInfo(owner uint16, slots []byte, companions []TagCharacter) ([]byte, error) {
	if owner == 0 || owner == 65535 || len(companions) > 3 {
		return nil, fmt.Errorf("战斗角色所属玩家或队友数量无效")
	}
	if len(slots) != len(companions) {
		return nil, fmt.Errorf("战斗角色槽位数量与资料不一致")
	}
	p := append(add16([]byte{1}, owner), byte(len(companions)))
	seen := map[uint16]bool{owner: true}
	seenSlots := map[byte]bool{}
	for i, role := range companions {
		if slots[i] == 255 || seenSlots[slots[i]] {
			return nil, fmt.Errorf("战斗角色槽位无效或重复")
		}
		seenSlots[slots[i]] = true
		if role.WireID == 0 || role.WireID == 65535 || seen[role.WireID] || role.Level == 0 {
			return nil, fmt.Errorf("战斗队友身份无效或重复")
		}
		seen[role.WireID] = true
		if _, _, err := parseName(addName(nil, role.Name)); err != nil {
			return nil, err
		}
		stats, err := role.Stats.Bytes()
		if err != nil {
			return nil, err
		}
		appearance, err := equippedAppearanceBlock(role.Appearance)
		if err != nil {
			return nil, err
		}
		equipment, err := TagEquipment(role.Worn)
		if err != nil {
			return nil, err
		}
		// 1444FD93A/958/9C4：队内槽位、名字、角色标识。
		p = add16(addName(append(p, slots[i]), role.Name), role.WireID)
		p = append(p, role.Level, role.Profession, role.Advancement, 0)
		p = append(add32(p, uint32(len(stats))), stats...)
		p = append(p, role.ExpandEquipFlags, 0)
		// 与当前玩家14563EC60分支相同的两项可选扩展，保持未启用初值。
		p = add32(add32(p, 0), 0)
		p = append(p, appearance...)
		p = add32(p, 0)        // 145639460。
		p = append(p, 0, 0, 0) // 14563A080、145639810、145639490。
		p = append(p, 0, 0)    // 单字节扩展及145639B70装扮覆盖列表。
		p = append(p, equipment...)
		p = add16(p, 0) // 145962725/734读取的角色扩展。
		p, err = tagSkills(p, role)
		if err != nil {
			return nil, err
		}
		p = add32(p, 0) // 14563C0B0附加属性列表为空。
		p = append(p, make([]byte, 5*4)...)
		p = append(p, 0) // 14563D3C0可选标志。
		// 144504D40将这四项临时战斗状态初始化为0；仅用于新开战，
		// 不以此覆盖正在进行的战斗、复活或断线恢复状态。
		p = append(p, make([]byte, 4*4)...)
	}
	return p, nil
}

// 14563CFA0：数量、当前树标志、槽位/u16技能/u8等级，随后固定3+5个VP项。
func tagSkills(p []byte, role TagCharacter) ([]byte, error) {
	if len(role.Skills) > 255 {
		return nil, fmt.Errorf("队友技能数量超过客户端容量")
	}
	p = append(p, byte(len(role.Skills)), 0)
	seen := map[uint16]bool{}
	for _, skill := range role.Skills {
		slot := skill.Slot
		if slot == 65535 {
			slot = 255
		}
		if skill.ID == 0 || skill.Level == 0 || slot > 255 || seen[skill.ID] {
			return nil, fmt.Errorf("队友已学技能或槽位无效")
		}
		seen[skill.ID] = true
		p = append(add16(append(p, byte(slot)), skill.ID), skill.Level)
	}
	for _, v := range role.Intensions {
		p = add32(add16(p, v.ID), v.Choice)
	}
	for _, v := range role.Options {
		p = append(add32(add16(p, v.ID), v.Choice), v.Status)
	}
	return add32(p, 0), nil // 14563D386读取后未使用的尾字段。
}
