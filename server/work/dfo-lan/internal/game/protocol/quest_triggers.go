package protocol

import "fmt"

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
