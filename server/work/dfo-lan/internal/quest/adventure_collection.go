package quest

import "dfolan/internal/catalog"

const RegisterAdventureEquipment = "adventure-equipment-registration-v1"

// 当前 PVF 的 guide_quest_231 要求登记前一步奖励的装备。
// 仅开放已接入扣物和存档的引导目标；不能让其它未支持的登记任务被接受。
func AdventureCollectionObjective(d catalog.QuestDefinition) (uint32, bool) {
	if d.ID != 21651 || d.Kind != "[register adventure collection equip item]" || len(d.Pending) != 0 || len(d.ObjectiveCells) != 1 {
		return 0, false
	}
	t := d.ObjectiveCells[0]
	return uint32(t.Value), t.Type == 0 && t.Value == 100261068
}
