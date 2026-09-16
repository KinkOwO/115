package protocol

import (
	"encoding/binary"
	"fmt"
)

type ActiveQuest struct {
	ID       uint16
	Progress uint32
}
type QuestSubmitRequest struct {
	ID              uint16
	RewardSelection uint16
	Option          uint16
}

// Native14519ff60 writes four u16s; live34: 2200490cffff0100.
func DecodeQuestSubmit(p []byte) (QuestSubmitRequest, error) {
	var q QuestSubmitRequest
	if len(p) != 8 || binary.LittleEndian.Uint16(p) != 34 {
		return q, fmt.Errorf("invalid quest submit packet")
	}
	q = QuestSubmitRequest{binary.LittleEndian.Uint16(p[2:]), binary.LittleEndian.Uint16(p[4:]), binary.LittleEndian.Uint16(p[6:])}
	if q.ID == 0 || q.ID == 65535 {
		return q, fmt.Errorf("invalid quest ID")
	}
	return q, nil
}

// Native14527d217 recognizes code19 and unwinds the submit UI without reading
// any reward fields. Its official business label is not recovered. This is a
// refusal only; it must never be interpreted as a successful completion.
func QuestSubmitRefused() []byte { return add16([]byte{0}, 19) }

// Current 14519fe80 emits an inner command marker followed by a u16 quest ID.
// Observed live: 1f00490c => accept 3145, with a validated cipher/checksum.
func DecodeQuestRequest(id uint16, p []byte) (uint16, error) {
	if id != 31 && id != 32 {
		return 0, fmt.Errorf("unsupported quest operation")
	}
	if len(p) < 4 || binary.LittleEndian.Uint16(p) != id {
		return 0, fmt.Errorf("invalid quest command marker")
	}
	alignment := 16
	if e := padding(p[4:], alignment); e != nil {
		return 0, e
	}
	q := binary.LittleEndian.Uint16(p[2:])
	if q == 0 {
		return 0, fmt.Errorf("invalid quest ID")
	}
	return q, nil
}

// Native 145261c2a..145261d49 reads quest ID, progress and party-row count.
func QuestAccepted(id uint16, progress uint32) []byte {
	return append(add32(add16([]byte{1}, id), progress), 0)
}

// Native 14527ddb6 reads only the abandoned quest ID on success.
func QuestAbandoned(id uint16) []byte { return add16([]byte{1}, id) }
