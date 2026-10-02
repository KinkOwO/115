package inventory

import (
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
)

const EmblemCompoundModel = "emblem-compound-v1"

type EmblemCompoundReceipt struct {
	Inputs    []protocol.CompoundEmblemInput  `json:"inputs"`
	Mode      byte                            `json:"mode"`
	Sequence  uint64                          `json:"sequence"`
	Rewards   []protocol.CompoundEmblemReward `json:"rewards"`
	Source    string                          `json:"source"`
	Ack       []byte                          `json:"-"`
	Inventory []byte                          `json:"-"`
}

func emblemCompoundDraw(limit uint32) (uint32, error) {
	if limit == 0 {
		return 0, fmt.Errorf("empty emblem compound random range")
	}
	n, e := rand.Int(rand.Reader, new(big.Int).SetUint64(uint64(limit)))
	if e != nil {
		return 0, e
	}
	return uint32(n.Uint64()), nil
}

func (r *EmblemCompoundReceipt) prepare(state json.RawMessage) error {
	var e error
	r.Ack, e = protocol.CompoundEmblemSuccess(r.Rewards)
	if e != nil {
		return e
	}
	b, e := ReadBag(state)
	if e != nil {
		return e
	}
	r.Inventory, e = protocol.InventoryRestore(b.Rows(), b.Expansion)
	return e
}

func (s *ItemService) EmblemCompoundSequence(role Role) (uint64, error) {
	if s == nil || s.EmblemCompound == nil {
		return 0, fmt.Errorf("emblem compound service unavailable")
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return 0, fmt.Errorf("emblem compound inventory source mismatch")
	}
	b, err := ReadBag(role.State)
	if err != nil {
		return 0, err
	}
	if b.EmblemCompoundSeq == math.MaxUint64 {
		return 0, fmt.Errorf("emblem compound sequence exhausted")
	}
	return b.EmblemCompoundSeq, nil
}

func EmblemCompoundEventKey(sequence uint64, req protocol.CompoundEmblemRequest) (string, error) {
	identity, err := json.Marshal(struct {
		Inputs []protocol.CompoundEmblemInput
		Mode   byte
	}{req.Inputs, req.Mode})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("emblem-compound:%d:%x", sequence, sha256.Sum256(identity)), nil
}

// PrepareEmblemCompound applies the loot-domain state transition. The workflow
// owns persistence and receipt replay.
func (s *ItemService) PrepareEmblemCompound(current Role, req protocol.CompoundEmblemRequest, sequence uint64) (json.RawMessage, json.RawMessage, error) {
	b, err := ReadBag(current.State)
	if err != nil {
		return nil, nil, err
	}
	if b.EmblemCompoundSeq != sequence {
		return nil, nil, fmt.Errorf("stale emblem compound sequence")
	}
	b, rewards, err := b.CompoundEmblems(s.Catalog, s.BagRules, s.EmblemCompound, req, emblemCompoundDraw)
	if err != nil {
		return nil, nil, err
	}
	b.EmblemCompoundSeq++
	updated, err := SaveBag(current.State, b)
	if err != nil {
		return nil, nil, err
	}
	receipt := EmblemCompoundReceipt{Inputs: append([]protocol.CompoundEmblemInput(nil), req.Inputs...), Mode: req.Mode, Sequence: sequence, Rewards: rewards, Source: s.Catalog.Source.SaveIdentity()}
	if err := receipt.prepare(updated); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(receipt)
	return updated, data, err
}

func (s *ItemService) PrepareEmblemCompoundReceipt(receipt *EmblemCompoundReceipt, state json.RawMessage) error {
	return receipt.prepare(state)
}

func (b Bag) CompoundEmblems(c catalog.LootCatalog, bagRules BagRules, rules *EmblemCompoundRules, r protocol.CompoundEmblemRequest, draw func(uint32) (uint32, error)) (Bag, []protocol.CompoundEmblemReward, error) {
	fail := func(e error) (Bag, []protocol.CompoundEmblemReward, error) { return b, nil, e }
	if rules == nil || draw == nil || rules.Source != c.Source.Checksum || bagRules.Source != c.Source.Checksum {
		return fail(fmt.Errorf("emblem compound source unavailable or mismatched"))
	}
	// Directed selection modes need their own source groups and native contract.
	if r.Mode != 0 {
		return fail(fmt.Errorf("directed emblem compound mode unsupported"))
	}
	if len(r.Inputs) < 2 || len(r.Inputs) > 5 {
		return fail(fmt.Errorf("invalid emblem compound count"))
	}
	seen := map[uint16]bool{}
	var grade int32
	for _, in := range r.Inputs {
		if seen[in.Slot] {
			return fail(fmt.Errorf("duplicate emblem compound slot"))
		}
		seen[in.Slot] = true
		g, ok := rules.Grades[in.Template]
		item, present := c.Items[in.Template]
		if !ok || !present || item.Kind != "stackable" || item.StackableType != "[avatar emblem]" {
			return fail(fmt.Errorf("unsupported source emblem compound input"))
		}
		if grade != 0 && grade != g {
			return fail(fmt.Errorf("mixed emblem compound grades"))
		}
		grade = g
		found := false
		for _, row := range b.Items {
			if row.Slot == in.Slot {
				if row.Template != in.Template || row.Amount == 0 || row.ExpireTime != 0 {
					return fail(fmt.Errorf("stale or limited-period emblem compound input"))
				}
				found = true
				break
			}
		}
		if !found {
			return fail(fmt.Errorf("emblem compound input missing"))
		}
	}
	prob, ok := rules.Rolls[EmblemCompoundKey{grade, len(r.Inputs)}]
	if !ok {
		return fail(fmt.Errorf("unsupported source emblem compound combination"))
	}
	n, e := draw(100)
	if e != nil {
		return fail(e)
	}
	if n >= 100 {
		return fail(fmt.Errorf("invalid emblem compound random draw"))
	}
	col := 0
	for col < 4 && n >= prob[col] {
		n -= prob[col]
		col++
	}
	if col == 4 {
		return fail(fmt.Errorf("invalid emblem compound probability row"))
	}
	pool := rules.Pools[int32(col+1)]
	if len(pool) == 0 {
		return fail(fmt.Errorf("empty source emblem compound pool"))
	}
	n, e = draw(uint32(len(pool)))
	if e != nil {
		return fail(e)
	}
	if n >= uint32(len(pool)) {
		return fail(fmt.Errorf("invalid emblem compound pool draw"))
	}
	id := pool[n]
	if rules.Grades[id] != int32(col+1) {
		return fail(fmt.Errorf("invalid source emblem compound reward"))
	}
	next := b
	next.Items = make([]BagItem, 0, len(b.Items))
	for _, row := range b.Items {
		if seen[row.Slot] {
			row.Amount--
		}
		if row.Amount > 0 {
			next.Items = append(next.Items, row)
		}
	}
	next, _, e = next.Add(c, bagRules, id, 1)
	if e != nil {
		return fail(e)
	}
	return next, []protocol.CompoundEmblemReward{{Template: id, Count: 1}}, nil
}
