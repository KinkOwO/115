package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Current1452AE370 (registered at1452BB718) reads two u32s. It converts both to ms
// for the scene and stores [startSeconds,startSeconds+durationSeconds] in
// the mode clock pair. This is NOT two absolute timestamps or a u64 value.
func LegionDungeonTimeout115(start time.Time, limit time.Duration) ([]byte, error) {
	seconds := start.Unix()
	if start.IsZero() || seconds <= 0 || seconds > 0x7fffffff || limit <= 0 || limit%time.Second != 0 || limit > time.Hour || seconds+int64(limit/time.Second) > 0x7fffffff {
		return nil, fmt.Errorf("invalid legion owned clock range")
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p, uint32(limit/time.Second))
	binary.LittleEndian.PutUint32(p[4:], uint32(seconds))
	return p, nil
}
