package protocol

import "fmt"

// NOTI1668 / 145307070只读取u32任务编号，通过144F3A780同步完成关联，
// 再由144F65B10移除已接任务并刷新界面，不创建奖励窗口。
// 调用方必须先完成存档事务，且确认无需展示或应用任何奖励。
func QuestFinishedWithoutRewardWindow(qid uint16) ([]byte, error) {
	if qid == 0 || qid == 65535 {
		return nil, fmt.Errorf("任务完成通知的任务编号无效")
	}
	return add32(nil, uint32(qid)), nil
}

// This is the no-inline-item terminal reply. Inventory rewards, if any, are
// restored from the committed bag in a separate current NOTI13 notification.
func QuestFinishedNoItems(qid uint16, gain uint32) ([]byte, error) {
	if qid == 0 || qid == 65535 {
		return nil, fmt.Errorf("invalid finished quest")
	}
	p := add16([]byte{1}, qid)
	p = add32(append(p, 0), gain)
	return append(p, 0, 0, 0), nil
}

func CompletedQuests(ids []uint32) ([]byte, error) {
	// Current NOTI3421452c9b50 rebuilds a40000-entry completion bitmap.
	if len(ids) > 40000 {
		return nil, fmt.Errorf("completed quest count out of bounds")
	}
	p := add32(nil, uint32(len(ids)))
	seen := map[uint32]bool{}
	for _, id := range ids {
		if id == 0 || id >= 40000 || seen[id] {
			return nil, fmt.Errorf("invalid completed quest identity")
		}
		seen[id] = true
		p = add32(p, id)
	}
	return p, nil
}
