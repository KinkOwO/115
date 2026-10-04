package workflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand"
	"time"
)

// UnsealService settles CMD393 magic-seal unsealing durably. It mirrors the
// consume/pickup character-event pattern: the roll, the gold charge and the
// receipt commit in one PostgreSQL character transaction, so a retried
// right-click replays the original outcome instead of rolling twice.
//
// The bag transform stays in inventory (Bag.UnsealRandomOption); this workflow
// owns the transaction and the persisted receipt.
type UnsealService struct {
	Store         *database.Store
	Equipment     *inventory.EquipmentCatalog
	RandomOptions *inventory.RandomOptionCatalog
	Model         string
}

type UnsealReceipt struct {
	Slot       uint16                    `json:"slot"`
	Template   uint32                    `json:"template"`
	OptionType byte                      `json:"option_type"`
	Options    [3]inventory.RolledOption `json:"options"`
	Count      byte                      `json:"count"`
	Gold       uint32                    `json:"gold"`
	Record     []byte                    `json:"record"`
	Source     string                    `json:"source"`
}

func newUnsealRand() (*mrand.Rand, error) {
	seed, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 63))
	if e != nil {
		return nil, e
	}
	return mrand.New(mrand.NewSource(seed.Int64() ^ time.Now().UnixNano())), nil
}

// Unseal breaks the magic seal on the bag equipment at the requested slot.
// The scroll slot must be the native no-scroll sentinel; scroll-assisted
// unsealing is not proven against the current client and stays refused.
func (s *UnsealService) Unseal(ctx context.Context, role database.Character, version string, r protocol.UnsealRequest) (database.Character, UnsealReceipt, bool, error) {
	var out UnsealReceipt
	fail := func(e error) (database.Character, UnsealReceipt, bool, error) {
		return role, out, false, e
	}
	if s == nil || s.Store == nil || s.Equipment == nil || s.RandomOptions == nil {
		return fail(fmt.Errorf("unseal service unavailable"))
	}
	if r.ScrollSlot != protocol.UnsealNoScrollSlot {
		return fail(fmt.Errorf("unseal scroll slot %d is not proven", r.ScrollSlot))
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return fail(e)
	}
	var sealed [protocol.CurrentItemRecordSize]byte
	found := false
	for _, item := range b.Equipment {
		if item.Slot == r.TargetSlot {
			sealed, found = inventory.EquipmentRow(item), true
			break
		}
	}
	if !found {
		return fail(fmt.Errorf("no equipment at slot %d", r.TargetSlot))
	}
	// The key binds the sealed pre-image: replaying the identical request
	// returns the original roll, while a different item at the same slot
	// hashes differently and gets its own event.
	digest := sha256.Sum256(sealed[:])
	key := fmt.Sprintf("unseal:%d:%s", r.TargetSlot, hex.EncodeToString(digest[:8]))
	rng, e := newUnsealRand()
	if e != nil {
		return fail(e)
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, version, key, s.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			bag, row, roll, e := bag.UnsealRandomOption(s.Equipment, s.RandomOptions, r.TargetSlot, rng)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, nil, e
			}
			out = UnsealReceipt{
				Slot:       r.TargetSlot,
				Template:   binary.LittleEndian.Uint32(row[2:]),
				OptionType: roll.OptionType,
				Options:    roll.Options,
				Count:      roll.Count,
				Gold:       roll.Gold,
				Record:     append([]byte(nil), row[:]...),
				Source:     version,
			}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != version || out.Slot != r.TargetSlot || len(out.Record) != protocol.CurrentItemRecordSize {
		return fail(fmt.Errorf("unseal receipt conflict"))
	}
	// A replay must describe the item the bag actually holds now; a stale key
	// from a different sealed pre-image at the same slot fails loudly rather
	// than acknowledging an unseal that never happened.
	current, e := inventory.ReadBag(saved.State)
	if e != nil {
		return fail(e)
	}
	verified := false
	for _, item := range current.Equipment {
		if item.Slot == r.TargetSlot {
			row := inventory.EquipmentRow(item)
			verified = string(row[:]) == string(out.Record)
			break
		}
	}
	if !verified {
		return fail(fmt.Errorf("unseal receipt does not match slot %d", r.TargetSlot))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
