package protocol

import "fmt"

// CMD33 的成功响应由145296130读取u16任务ID、u32剩余进度，
// 经144F45ED0查找已接任务，再由144F69770原位更新。
// 不使用NOTI291清空重建任务列表，避免重走接任务时的自动寻路。
func QuestTriggerUpdated(q ActiveQuest) ([]byte, error) {
	if q.ID == 0 || q.ID == 65535 {
		return nil, fmt.Errorf("任务进度更新的任务编号无效")
	}
	return add32(add16([]byte{1}, q.ID), q.Progress), nil
}

// NOTI291 clears/restores accepted quest triggers. Native1452de670 reads
// u16 count + (u16 quest,u32 remaining), then two optional u32 counts.
// This refresh does not complete or reward quests; zero means ready to submit.
func QuestTriggers(active []ActiveQuest) ([]byte, error) {
	if len(active) > 4096 {
		return nil, fmt.Errorf("too many active quest triggers")
	}
	p := add16(nil, uint16(len(active)))
	seen := map[uint16]bool{}
	for _, q := range active {
		if q.ID == 0 || q.ID == 65535 || seen[q.ID] {
			return nil, fmt.Errorf("invalid or duplicate quest trigger")
		}
		seen[q.ID] = true
		p = add32(add16(p, q.ID), q.Progress)
	}
	return add32(add32(p, 0), 0), nil
}
