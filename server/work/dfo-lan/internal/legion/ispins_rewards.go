package legion

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Current 115 readers 1424fdc30/1424fdb60 copy fixed bodies; consumers
// 14250f330/14250edf0 read quantities as u32 and skip owner rows with flag 1.
// The basic tail is elapsed milliseconds (i64), followed by four skip bytes.
type IspinsRewardItem struct{ Template, Amount uint32 }

func IspinsSourceRewardPayloads(basic, additional []IspinsRewardItem, elapsed time.Duration, eligible bool) ([]byte, []byte, error) {
	if elapsed < 0 || len(basic) > 35 || len(additional) > 15 || (!eligible && (len(basic) > 0 || len(additional) > 0)) {
		return nil, nil, fmt.Errorf("invalid Ispins source settlement projection")
	}
	b, a := make([]byte, 7772), make([]byte, 2405)
	binary.LittleEndian.PutUint64(b[7760:], uint64(elapsed/time.Millisecond))
	for owner := 1; owner < 4; owner++ {
		b[7768+owner] = 1
		a[2401+owner] = 1
	}
	if eligible {
		a[0] = 1
	}
	for i, item := range basic {
		if item.Template == 0 || item.Amount == 0 {
			return nil, nil, fmt.Errorf("invalid basic reward item")
		}
		off := 1600 + i*44
		binary.LittleEndian.PutUint32(b[off:], item.Template)
		binary.LittleEndian.PutUint32(b[off+4:], item.Amount)
	}
	for i, item := range additional {
		if item.Template == 0 || item.Amount == 0 {
			return nil, nil, fmt.Errorf("invalid additional reward item")
		}
		off := 1 + i*40
		binary.LittleEndian.PutUint32(a[off:], item.Template)
		binary.LittleEndian.PutUint32(a[off+4:], item.Amount)
	}
	return b, a, nil
}
