package protocol

import "fmt"

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
