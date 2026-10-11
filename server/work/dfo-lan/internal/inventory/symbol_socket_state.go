package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// SymbolSocketLayout is read from the same source as the emblem definitions.
// Counts come from equipemblemwindow.xui, not from a second gameplay table.
// They describe the visual slots, not proof that all slots are unlocked.
// The names and masks are UI/execution identifiers, matching the native
// avatar socket parser (14716AA90). Platinum effects require separate passive
// and buff execution; a compatible socket alone does not activate effects.
type SymbolSocketLayout struct {
	Source                             string
	Sockets                            map[string]SymbolSocketGroup
	Rules                              *SymbolEmblemRules
	FixedTemplates, ResonanceTemplates map[uint32][2]uint32
}
type SymbolSocketGroup struct {
	Count uint16
	Mask  uint16
}
type symbolXUINode struct {
	XMLName  xml.Name
	ID       string          `xml:"ID,attr"`
	Children []symbolXUINode `xml:",any"`
}

func ParseSymbolSocketLayout(source string, raw []byte) (*SymbolSocketLayout, error) {
	digest, err := hex.DecodeString(source)
	if err != nil || len(digest) != 32 || len(raw) == 0 {
		return nil, fmt.Errorf("missing symbol socket source")
	}
	// PVF kind 3 stores UTF-16LE despite the XML declaration saying UTF-8.
	if len(raw) > 1 && (raw[1] == 0 || raw[0] == 0xff && raw[1] == 0xfe) {
		if len(raw)%2 != 0 {
			return nil, fmt.Errorf("odd symbol XUI length")
		}
		units := make([]uint16, len(raw)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
		raw = []byte(strings.Trim(string(utf16.Decode(units)), "\ufeff\x00"))
	}
	var root symbolXUINode
	if e := xml.Unmarshal(raw, &root); e != nil {
		return nil, e
	}
	masks := map[string]uint16{"red_symbol": 1, "yellow_symbol": 2, "green_symbol": 4, "blue_symbol": 8, "multi_symbol": avatarMultiSocket, "platinum": avatarPlatinumSocket, "buff_platinum": avatarPlatinumSocket}
	r := &SymbolSocketLayout{Source: source, Sockets: map[string]SymbolSocketGroup{}}
	for _, control := range root.Children {
		if control.XMLName.Local != "CONTROL" {
			continue
		}
		for _, group := range control.Children {
			name := strings.TrimPrefix(group.ID, "STR:")
			mask, ok := masks[name]
			if !ok {
				continue
			}
			if _, exists := r.Sockets[name]; exists {
				return nil, fmt.Errorf("duplicate symbol group %s", name)
			}
			seen := map[int]bool{}
			for _, slot := range group.Children {
				id := strings.TrimPrefix(slot.ID, "STR:")
				if !strings.HasPrefix(id, "slot_") {
					continue
				}
				n, e := strconv.Atoi(strings.TrimPrefix(id, "slot_"))
				if e != nil || n < 0 || n > 255 || seen[n] {
					return nil, fmt.Errorf("invalid symbol slot %s", id)
				}
				seen[n] = true
			}
			if len(seen) == 0 {
				return nil, fmt.Errorf("empty symbol group %s", name)
			}
			for i := 0; i < len(seen); i++ {
				if !seen[i] {
					return nil, fmt.Errorf("noncontiguous symbol group %s", name)
				}
			}
			r.Sockets[name] = SymbolSocketGroup{Count: uint16(len(seen)), Mask: mask}
		}
	}
	if len(r.Sockets) != len(masks) {
		return nil, fmt.Errorf("incomplete symbol socket layout")
	}
	return r, nil
}

// The new system owns one top-level state field. Legacy inventory/character
// fields remain lossless, including fields written by other source revisions.
type SymbolSocketState struct {
	Version  uint32                        `json:"version"`
	Sequence uint64                        `json:"sequence"`
	Groups   map[string][]SymbolSocketItem `json:"groups"`
	Progress map[string]SymbolProgress     `json:"progress,omitempty"`
}
type SymbolProgress struct{ Grade, Level uint32 }

var SymbolTypes = map[string]uint32{"red_symbol": 0, "yellow_symbol": 1, "green_symbol": 2, "blue_symbol": 3, "multi_symbol": 4}

type SymbolSocketItem struct {
	Template   uint32 `json:"template,omitempty"`
	ExpireTime uint32 `json:"expire_time,omitempty"`
}
type SymbolSocketRequest struct {
	Source           string `json:"source"`
	Sequence         uint64 `json:"sequence"`
	Group            string `json:"group"`
	Socket           uint16 `json:"socket"`
	ExpectedTemplate uint32 `json:"expected_template"`
	BagSlot          uint16 `json:"bag_slot"`
	Template         uint32 `json:"template"` // Zero requests removal, never destruction.
}

func ReadSymbolSockets(state json.RawMessage, layout *SymbolSocketLayout) (SymbolSocketState, error) {
	var fields map[string]json.RawMessage
	var out SymbolSocketState
	if layout == nil || len(layout.Sockets) == 0 {
		return out, fmt.Errorf("symbol layout unavailable")
	}
	if e := json.Unmarshal(state, &fields); e != nil {
		return out, e
	}
	if fields == nil {
		return out, fmt.Errorf("character state is not an object")
	}
	if raw, ok := fields["symbol_emblem_sockets"]; ok {
		if e := json.Unmarshal(raw, &out); e != nil {
			return out, e
		}
		if out.Version != 1 || out.Groups == nil {
			return out, fmt.Errorf("unsupported symbol socket state")
		}
	} else {
		out = SymbolSocketState{Version: 1, Groups: map[string][]SymbolSocketItem{}}
	}
	for name, rows := range out.Groups {
		group, ok := layout.Sockets[name]
		if !ok || len(rows) != int(group.Count) {
			return out, fmt.Errorf("saved symbol group does not match source")
		}
		for _, item := range rows {
			if item.Template == 0 && item.ExpireTime != 0 {
				return out, fmt.Errorf("invalid empty symbol item")
			}
		}
	}
	for name, group := range layout.Sockets {
		if _, ok := out.Groups[name]; !ok {
			out.Groups[name] = make([]SymbolSocketItem, group.Count)
		}
	}
	if layout.Rules != nil {
		if out.Progress == nil {
			out.Progress = map[string]SymbolProgress{}
		}
		for name, typ := range SymbolTypes {
			if name == "multi_symbol" {
				continue
			}
			p, ok := out.Progress[name]
			if !ok {
				p = SymbolProgress{Grade: math.MaxUint32, Level: math.MaxUint32}
				for key, tier := range layout.Rules.Tiers {
					if key.Type == typ && key.Grade < p.Grade {
						p.Grade = key.Grade
						p.Level = math.MaxUint32
						for level := range tier.Levels {
							if level < p.Level {
								p.Level = level
							}
						}
					}
				}
				out.Progress[name] = p
			}
			if _, e := layout.Rules.FixedAt(typ, p.Grade, p.Level); e != nil {
				return out, e
			}
		}
		// The CN guide defines the multi-color star as the lowest grown color.
		lowest := SymbolProgress{Grade: math.MaxUint32, Level: math.MaxUint32}
		for name, p := range out.Progress {
			if name == "multi_symbol" {
				continue
			}
			if _, ok := SymbolTypes[name]; !ok {
				return out, fmt.Errorf("unknown saved star")
			}
			if p.Grade < lowest.Grade || p.Grade == lowest.Grade && p.Level < lowest.Level {
				lowest = p
			}
		}
		out.Progress["multi_symbol"] = lowest
	}
	return out, nil
}

// PrepareSymbolSocketExchange is pure: validate, consume the new stack,
// return the old item, then serialize the two owned fields. The caller commits
// this result and the request receipt in one existing character transaction.
// This is an unwired transaction candidate, not a complete native executor.
// Do not expose it to clients until native slot unlocking, platinum skill
// eligibility, replacement semantics, and attribute consumers are verified.
func PrepareSymbolSocketExchange(state json.RawMessage, c catalog.LootCatalog, bagRules BagRules, layout *SymbolSocketLayout, masks *EmblemInlayRules, req SymbolSocketRequest, now int64) (json.RawMessage, error) {
	if layout == nil || masks == nil || req.Source != c.Source.Checksum || layout.Source != req.Source || masks.Source != req.Source || bagRules.Source != req.Source {
		return nil, fmt.Errorf("symbol exchange source mismatch")
	}
	saved, e := ReadSymbolSockets(state, layout)
	if e != nil {
		return nil, e
	}
	if saved.Sequence != req.Sequence || saved.Sequence == math.MaxUint64 {
		return nil, fmt.Errorf("stale symbol exchange sequence")
	}
	group, ok := layout.Sockets[req.Group]
	if !ok || req.Socket >= group.Count {
		return nil, fmt.Errorf("unknown symbol socket")
	}
	old := saved.Groups[req.Group][req.Socket]
	if old.Template != req.ExpectedTemplate {
		return nil, fmt.Errorf("stale equipped emblem")
	}
	if req.Template == 0 && (old.Template == 0 || req.BagSlot != 0) {
		return nil, fmt.Errorf("empty or invalid removal")
	}
	if req.Template != 0 && req.Template == old.Template {
		return nil, fmt.Errorf("same emblem already equipped")
	}
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	// Reject ambiguous bag rows before any consumption.
	slots := map[uint16]bool{}
	for _, row := range b.Items {
		if slots[row.Slot] {
			return nil, fmt.Errorf("duplicate inventory slot")
		}
		slots[row.Slot] = true
	}
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	incoming := SymbolSocketItem{}
	if req.Template != 0 {
		definition, ok := c.Items[req.Template]
		if !ok || definition.Kind != "stackable" || definition.StackableType != "[avatar emblem]" || masks.Masks[req.Template]&group.Mask == 0 {
			return nil, fmt.Errorf("emblem does not match symbol color")
		}
		found := -1
		for i, row := range next.Items {
			if row.Slot == req.BagSlot {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("emblem stack missing")
		}
		row := next.Items[found]
		if row.Template != req.Template || row.Amount == 0 || protocol.StoredItemExpired(row.ExpireTime, now) {
			return nil, fmt.Errorf("stale or expired emblem stack")
		}
		incoming = SymbolSocketItem{Template: row.Template, ExpireTime: row.ExpireTime}
		if row.Amount == 1 {
			next.Items = append(next.Items[:found], next.Items[found+1:]...)
		} else {
			next.Items[found].Amount--
		}
	}
	if old.Template != 0 {
		// Returning a saved emblem must never merge distinct expiration identities.
		// Bag.Add's legacy expiry inheritance is inappropriate for removable items.
		next, e = returnSymbolEmblem(next, c, bagRules, old)
		if e != nil {
			return nil, e
		}
	}
	saved.Groups[req.Group][req.Socket] = incoming
	saved.Sequence++
	var fields, inventory map[string]json.RawMessage
	if e = json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if e = json.Unmarshal(fields["inventory"], &inventory); e != nil {
		return nil, e
	}
	if inventory == nil {
		return nil, fmt.Errorf("missing inventory state")
	}
	inventory["items"], e = json.Marshal(next.Items)
	if e != nil {
		return nil, e
	}
	fields["inventory"], e = json.Marshal(inventory)
	if e != nil {
		return nil, e
	}
	fields["symbol_emblem_sockets"], e = json.Marshal(saved)
	if e != nil {
		return nil, e
	}
	return json.Marshal(fields)
}

func returnSymbolEmblem(b Bag, c catalog.LootCatalog, r BagRules, item SymbolSocketItem) (Bag, error) {
	definition, ok := c.Items[item.Template]
	if !ok || definition.Kind != "stackable" || definition.StackableType != "[avatar emblem]" {
		return b, fmt.Errorf("saved emblem definition missing")
	}
	home := stackableSlotRange(r, definition.StackableType)
	limit := stackLimitFor(r, definition.StackableType, definition.StackLimit)
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	occupied := map[uint16]bool{}
	for _, v := range b.Equipment {
		occupied[v.Slot] = true
	}
	for i, v := range next.Items {
		occupied[v.Slot] = true
		if v.Slot >= home[0] && v.Slot <= home[1] && b.OrdinarySlotUnlocked(v.Slot) && v.Template == item.Template && v.ExpireTime == item.ExpireTime && v.Amount < limit {
			next.Items[i].Amount++
			return next, nil
		}
	}
	if limit == 0 {
		return b, fmt.Errorf("emblem stack limit missing")
	}
	for n := uint32(home[0]); n <= uint32(home[1]); n++ {
		slot := uint16(n)
		if !occupied[slot] && b.OrdinarySlotUnlocked(slot) {
			next.Items = append(next.Items, BagItem{Slot: slot, Template: item.Template, Amount: 1, ExpireTime: item.ExpireTime})
			return next, nil
		}
	}
	return b, fmt.Errorf("emblem inventory is full; exchange unchanged")
}
