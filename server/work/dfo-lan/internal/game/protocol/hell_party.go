package protocol

import "fmt"

type HellPartyAPC struct{ Template, Level uint32 }

// NOTI666, native1452aff70: u32 count followed by (u32 AIC,u32 level).
// This prepares AIC resources; ownership IDs live in NOTI29, not this packet.
func HellPartyMonsterInfo(rows []HellPartyAPC) ([]byte, error) {
	if len(rows) == 0 || len(rows) > 255 {
		return nil, fmt.Errorf("invalid Hell APC preload count")
	}
	p := add32(nil, uint32(len(rows)))
	for _, row := range rows {
		if row.Template == 0 || row.Level == 0 || row.Level > 255 {
			return nil, fmt.Errorf("invalid Hell APC preload")
		}
		p = add32(add32(p, row.Template), row.Level)
	}
	return p, nil
}

// NOTI777, native1452aff30: empty-body quest/event notification.
func HellPartyClear() []byte { return []byte{} }
