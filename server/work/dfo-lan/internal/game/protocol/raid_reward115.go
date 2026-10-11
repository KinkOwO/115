package protocol

import (
	"fmt"
	"unicode/utf8"
)

type RaidCardSelection115 struct{ Kind, Index byte }

// 143d4a350 and146ab8ffb send category then index, each through the
// one-byte writer146d75cc0. Unlike ordinary CMD71, the native raid sender
// submits categories1 and0 together and optionally2 for the PC-room card.
func DecodeRaidCardSelection115(p []byte) (RaidCardSelection115, error) {
	if err := raidBodySize(p, 2); err != nil {
		return RaidCardSelection115{}, err
	}
	if p[0] > 2 {
		return RaidCardSelection115{}, fmt.Errorf("invalid native raid card category")
	}
	return RaidCardSelection115{p[0], p[1]}, nil
}

type RaidRewardItem115 struct {
	Template         uint32
	Amount           uint16
	Kind             byte
	Special          bool
	Attribute, Value uint32
	Metadata         [21]byte
}

// N587 reader144ce6990: category/u32/count, then actor/u8/byte-name/u8/
// item-count and37-byte item rows. The signed short quantity shares the
// native item constructor with other reward displays; refuse overflow.
func RaidRewardList115(kind byte, actor uint16, name string, items []RaidRewardItem115) ([]byte, error) {
	if kind > 2 || actor == 0 || actor == 65535 || name == "" || len(name) > 255 || !utf8.ValidString(name) || len(items) > 255 {
		return nil, fmt.Errorf("invalid native raid reward owner/list")
	}
	p := add32([]byte{kind}, ^uint32(0))
	p = append(p, 1)
	p = add16(p, actor)
	p = append(p, 0)
	p = addName(p, name)
	p = append(p, 0, byte(len(items)))
	for _, item := range items {
		if item.Template == 0 || item.Amount == 0 || item.Amount > 32767 {
			return nil, fmt.Errorf("invalid native raid reward item/quantity")
		}
		special := byte(0)
		if item.Special {
			special = 1
		}
		p = append(p, item.Kind)
		p = add16(add32(p, item.Template), item.Amount)
		p = append(p, special)
		p = add32(add32(p, item.Attribute), item.Value)
		p = append(p, item.Metadata[:]...)
	}
	return p, nil
}

// N2302 reader144ce68b0 consumes one byte and then releases the reward
// decision queue. The byte is not passed to the consumer.
func RaidRewardDecision115() []byte { return []byte{0} }
