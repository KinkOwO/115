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

// CompletedQuests encodes the native NOTI342 CLEAR_QUEST_LIST body.
//
// FORMAT IS STILL UNRESOLVED for the private client (2.38.2.34) - next79 §17:
//   - The official 2026 capture (client 2.38.3.25) sends a zlib stream that
//     inflates to count(u32)+ids. Sending that zlib body to the private
//     client makes it fast-exit ~2s after town entry (session 20261003_005404):
//     2.38.2 parses the body raw, so the zlib magic 789c reads as count
//     0x9c78=40056 and the id loop runs off the buffer.
//   - The raw count+ids body (this encoding) also fails to populate the
//     client's completed set: with it, the client still requests accepting
//     already-completed quest 3145 (quest_rejected in session logs) and the
//     Ispins channel gate keeps demanding its chain quests.
// The reader sites the unicorn oracle captured (0x1452c9bd3 count,
// 0x1452c9bea ids) sit ~0x83 bytes into handler 0x1452c9b50; the missing
// transform between body start and those reads is the open question.
func CompletedQuests(ids []uint32) ([]byte, error) {
	if len(ids) > 40000 {
		return nil, fmt.Errorf("completed quest count out of bounds")
	}
	seen := map[uint32]bool{}
	p := add32(nil, uint32(len(ids)))
	for _, id := range ids {
		if id == 0 || id >= 40000 || seen[id] {
			return nil, fmt.Errorf("invalid completed quest identity")
		}
		seen[id] = true
		p = add32(p, id)
	}
	return p, nil
}
