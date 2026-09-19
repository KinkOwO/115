package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

const MaxExpireTime = math.MaxInt32

type BoosterRewardCandidate struct {
	Template uint32 `json:"template"`
	Weight   uint32 `json:"weight"`
	Count    uint32 `json:"count"`
}

type BoosterRewardPool struct {
	DrawCount  uint32                   `json:"draw_count"`
	Candidates []BoosterRewardCandidate `json:"candidates"`
}

func (p BoosterRewardPool) Pick(r *rand.Rand) []BoosterRewardCandidate {
	if len(p.Candidates) == 0 {
		return nil
	}
	if len(p.Candidates) == 1 {
		return []BoosterRewardCandidate{p.Candidates[0]}
	}
	var totalWeight uint32
	for _, c := range p.Candidates {
		totalWeight += c.Weight
	}
	if totalWeight == 0 {
		totalWeight = uint32(len(p.Candidates))
	}
	draws := p.DrawCount
	if draws == 0 {
		draws = 1
	}
	var results []BoosterRewardCandidate
	for d := uint32(0); d < draws; d++ {
		roll := r.Uint32() % totalWeight
		var acc uint32
		picked := false
		for _, c := range p.Candidates {
			w := c.Weight
			if c.Weight == 0 {
				w = 1
			}
			acc += w
			if roll < acc {
				results = append(results, c)
				picked = true
				break
			}
		}
		if !picked {
			results = append(results, p.Candidates[len(p.Candidates)-1])
		}
	}
	return results
}

type BoosterDefinition struct {
	Template uint32              `json:"template"`
	Type     string              `json:"type"`
	Pools    []BoosterRewardPool `json:"pools,omitempty"`
}

type ItemIndexInfo struct {
	ID            uint32 `json:"id"`
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	StackableType string `json:"stackable_type"`
	StackLimit    uint32 `json:"stack_limit"`
}

type BoosterCatalog struct {
	Definitions map[uint32]BoosterDefinition
	Items       map[uint32]ItemIndexInfo
}

func LoadBoosterCatalog(catPath, indexPath string) (*BoosterCatalog, error) {
	cat := &BoosterCatalog{
		Definitions: make(map[uint32]BoosterDefinition),
		Items:       make(map[uint32]ItemIndexInfo),
	}

	if catPath != "" {
		data, err := os.ReadFile(catPath)
		if err == nil {
			var raw map[string]BoosterDefinition
			if err = json.Unmarshal(data, &raw); err == nil {
				for _, def := range raw {
					cat.Definitions[def.Template] = def
				}
			}
		}
	}

	if indexPath != "" {
		data, err := os.ReadFile(indexPath)
		if err == nil {
			var raw struct {
				Items map[string]ItemIndexInfo `json:"items"`
			}
			if err = json.Unmarshal(data, &raw); err == nil {
				for _, it := range raw.Items {
					cat.Items[it.ID] = it
				}
			}
		}
	}

	return cat, nil
}

type boosterEventStore interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
	CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error)
}

func (w *worldSession) openBoosterItem(
	ctx context.Context,
	store boosterEventStore,
	wear *inventory.WearService,
	lootSvc *loot.Service,
	boosterCat *BoosterCatalog,
	choices odysseyWeaponChoices,
	plaintext, raw []byte,
) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("booster open requires selected character in town")
	}
	if w.activeDungeon != nil {
		return nil, fmt.Errorf("booster open requires character in town")
	}

	req, err := protocol.DecodeBoosterUseRequest(plaintext)
	if err != nil {
		return nil, err
	}

	// 1. Check current inventory to see what box is in slot
	curBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	var boxItem *inventory.BagItem
	for i := range curBag.Items {
		if curBag.Items[i].Slot == req.Slot {
			boxItem = &curBag.Items[i]
			break
		}
	}
	if boxItem == nil {
		return nil, fmt.Errorf("booster box not found at slot %d", req.Slot)
	}

	// If this is the Odyssey weapon creation box and Odyssey rewards are enabled, delegate to existing handler
	if boxItem.Template == 10417789 && len(req.Selections) > 0 && odysseyRewardsEnabled() {
		sel := protocol.WeaponBoxSelection{
			Slot:     req.Slot,
			Category: [2]byte{byte(req.Category), byte(req.Category >> 8)},
			Template: req.Selections[0],
		}
		if choices.allows(sel) {
			if realStore, ok := store.(*storage.Store); ok {
				saved, plan, e := selectOdysseyWeapon(ctx, realStore, wear, w.role, choices, sel)
				if e == nil {
					w.role = saved
				}
				return plan, e
			}
		}
	}

	// 2. Transactional execution of booster opening
	eventKey := fmt.Sprintf("booster-open:%d:%x", w.role.ID, sha256.Sum256(raw))
	type outcome struct {
		BoxTemplate  uint32
		Granted      []protocol.BoosterGrantedItem
		HasAvatars   bool
		HasCreatures bool
	}
	var res outcome

	saved, applied, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, eventKey, "booster-open-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		b, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		boxIdx := -1
		for i, it := range b.Items {
			if it.Slot == req.Slot {
				boxIdx = i
				break
			}
		}
		if boxIdx < 0 {
			return nil, nil, fmt.Errorf("box at slot %d not found", req.Slot)
		}
		box := b.Items[boxIdx]
		if box.Amount < req.Amount {
			return nil, nil, fmt.Errorf("insufficient box amount")
		}

		boxTemplate := box.Template
		boxHasExp := box.ExpireTime != 0

		// Determine items to grant
		var toGrant []protocol.BoosterGrantedItem
		if len(req.Selections) > 0 {
			for _, selTpl := range req.Selections {
				toGrant = append(toGrant, protocol.BoosterGrantedItem{
					Template: selTpl,
					Count:    req.Amount,
				})
			}
		} else if boosterCat != nil {
			def, ok := boosterCat.Definitions[boxTemplate]
			if !ok || len(def.Pools) == 0 {
				return nil, nil, fmt.Errorf("booster %d has no reward pool defined", boxTemplate)
			}
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			for _, pool := range def.Pools {
				picks := pool.Pick(r)
				for _, p := range picks {
					toGrant = append(toGrant, protocol.BoosterGrantedItem{
						Template: p.Template,
						Count:    p.Count * req.Amount,
					})
				}
			}
		}
		if len(toGrant) == 0 {
			return nil, nil, fmt.Errorf("no items granted from booster %d", boxTemplate)
		}

		// Consume the box
		if box.Amount > req.Amount {
			b.Items[boxIdx].Amount -= req.Amount
		} else {
			b.Items = append(b.Items[:boxIdx], b.Items[boxIdx+1:]...)
		}

		hasAvatars := false
		hasCreatures := false

		// Avatar option mapping if selections had options
		optMap := make(map[uint32][]byte)
		for _, ao := range req.AvatarOptions {
			optMap[ao.Template] = append(optMap[ao.Template], ao.Option)
		}

		// Award items
		for _, g := range toGrant {
			var (
				kind     string
				itemPath string
			)
			if boosterCat != nil {
				if info, ok := boosterCat.Items[g.Template]; ok {
					kind = info.Kind
					itemPath = info.Path
				}
			}
			if kind == "" && lootSvc != nil {
				if info, ok := lootSvc.Catalog.Items[g.Template]; ok {
					kind = info.Kind
				}
			}
			if kind == "" && wear != nil && wear.Catalog != nil {
				if def, err := wear.Catalog.Definition(g.Template); err == nil {
					if strings.Contains(def.Path, "avatar") {
						kind = "avatar"
					} else if strings.Contains(def.Path, "creature") {
						kind = "equipment"
					} else {
						kind = "equipment"
					}
					itemPath = def.Path
				}
			}

			// Destination 1: Avatar (kind == "avatar" or path contains "/avatar/" or option specified)
			isAvatar := kind == "avatar" || strings.Contains(itemPath, "/avatar/")
			if !isAvatar && len(req.AvatarOptions) > 0 {
				for _, ao := range req.AvatarOptions {
					if ao.Template == g.Template {
						isAvatar = true
						break
					}
				}
			}
			if isAvatar {
				hasAvatars = true
				occupied := map[uint16]bool{}
				if b.Special != nil {
					for _, av := range b.Special[1] {
						occupied[av.Slot] = true
					}
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(0); s < 210; s++ {
						if !occupied[s] {
							occupied[s] = true
							if b.Special == nil {
								b.Special = map[byte][]inventory.BagEquipment{}
							}
							var per uint32
							if boxHasExp {
								per = MaxExpireTime
							}
							dur := uint16(0)
							if opts, ok := optMap[g.Template]; ok && len(opts) > 0 {
								dur = uint16(opts[0])
								optMap[g.Template] = opts[1:]
							}
							b.Special[1] = append(b.Special[1], inventory.BagEquipment{
								Slot:       s,
								Template:   g.Template,
								Durability: dur,
								Period:     per,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("avatar inventory full")
					}
				}
				continue
			}

			// Destination 2: Creature (path contains "equipment/creature/")
			if strings.Contains(itemPath, "equipment/creature/") {
				hasCreatures = true
				occupied := map[uint16]bool{}
				if b.Special != nil {
					for _, cr := range b.Special[7] {
						occupied[cr.Slot] = true
					}
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(0); s < 140; s++ {
						if !occupied[s] {
							occupied[s] = true
							if b.Special == nil {
								b.Special = map[byte][]inventory.BagEquipment{}
							}
							b.Special[7] = append(b.Special[7], inventory.BagEquipment{
								Slot:     s,
								Template: g.Template,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("creature inventory full")
					}
				}
				continue
			}

			// Destination 3: Regular Equipment (kind == "equipment")
			if kind == "equipment" {
				occupied := map[uint16]bool{}
				for _, eq := range b.Equipment {
					occupied[eq.Slot] = true
				}
				for _, it := range b.Items {
					occupied[it.Slot] = true
				}
				for cnt := uint32(0); cnt < g.Count; cnt++ {
					foundSlot := false
					for s := uint16(9); s <= 64; s++ {
						if !occupied[s] {
							occupied[s] = true
							b.Equipment = append(b.Equipment, inventory.BagEquipment{
								Slot:       s,
								Template:   g.Template,
								Durability: 0,
							})
							foundSlot = true
							break
						}
					}
					if !foundSlot {
						return nil, nil, fmt.Errorf("equipment inventory full")
					}
				}
				continue
			}

			// Destination 4: Stackable (kind == "stackable")
			if kind != "stackable" {
				return nil, nil, fmt.Errorf("item %d kind %s cannot be delivered as stackable", g.Template, kind)
			}
			var exp uint32
			if boxHasExp {
				exp = MaxExpireTime
			}
			if lootSvc != nil {
				// Ensure item is registered in catalog if missing
				if _, ok := lootSvc.Catalog.Items[g.Template]; !ok {
					stType := "[etc]"
					if boosterCat != nil {
						if info, ok := boosterCat.Items[g.Template]; ok && info.StackableType != "" {
							stType = info.StackableType
						}
					}
					lootSvc.Catalog.Items[g.Template] = catalog.LootItem{
						ID:            g.Template,
						Kind:          "stackable",
						StackableType: stType,
						StackLimit:    1000,
					}
				}
				var addErr error
				b, _, addErr = b.Add(lootSvc.Catalog, lootSvc.BagRules, g.Template, g.Count, exp)
				if addErr != nil {
					return nil, nil, addErr
				}
			}
		}

		receiptData := outcome{
			BoxTemplate:  boxTemplate,
			Granted:      toGrant,
			HasAvatars:   hasAvatars,
			HasCreatures: hasCreatures,
		}
		receiptBytes, err := json.Marshal(receiptData)
		if err != nil {
			return nil, nil, err
		}

		rawBag, err := inventory.SaveBag(current.State, b)
		if err != nil {
			return nil, nil, err
		}
		return rawBag, receiptBytes, nil
	})
	if err != nil {
		return nil, err
	}

	receiptBytes, err := store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, eventKey)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(receiptBytes, &res); err != nil {
		return nil, err
	}

	w.role = saved
	finalBag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}

	// 3. Build response packets
	ack := protocol.BoosterOpenSuccess(res.BoxTemplate, req.Slot, res.Granted)

	var plan []outboundPacket

	mainRows := finalBag.Rows()
	slotStillOccupied := false
	for _, it := range finalBag.Items {
		if it.Slot == req.Slot {
			slotStillOccupied = true
			break
		}
	}
	if !slotStillOccupied {
		mainRows = append([][protocol.CurrentItemRecordSize]byte{protocol.OrdinaryItem(req.Slot, 0, 0)}, mainRows...)
	}

	mainUpdate, err := protocol.InventoryUpdate(mainRows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"booster_main_inventory_updated", 0, 14, mainUpdate})

	if res.HasAvatars && len(finalBag.Special[1]) > 0 {
		avatarPayload, err := inventory.EquipmentPayload(1, finalBag.Special[1], false)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"booster_avatar_inventory_updated", 0, 14, avatarPayload})
	}
	if res.HasCreatures && len(finalBag.Special[7]) > 0 {
		creaturePayload, err := inventory.EquipmentPayload(7, finalBag.Special[7], false)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"booster_creature_inventory_updated", 0, 14, creaturePayload})
	}

	plan = append(plan, outboundPacket{"booster_open_ack", 1, 160, ack})

	_ = applied
	_ = strconv.Itoa
	return plan, nil
}
