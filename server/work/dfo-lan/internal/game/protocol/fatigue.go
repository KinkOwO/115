package protocol

import "fmt"

// NOTI_FATIGUE 36 / 1452d0360 reads five u16 values. The two trailing
// optional systems are disabled (zero) in this compatibility bootstrap.
func Fatigue(used, limit, usedMax uint16) ([]byte, error) {
	if limit == 0 || used > limit {
		return nil, fmt.Errorf("invalid fatigue state")
	}
	p := add16(add16(add16(nil, used), limit), usedMax)
	return add16(add16(p, 0), 0), nil
}
