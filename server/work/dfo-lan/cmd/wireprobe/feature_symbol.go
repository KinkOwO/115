package main

import (
	"context"
	"dfolan/internal/featurebridge"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"math"
	"sort"
	"time"
)

func (w *worldSession) symbolView(now time.Time) (featurebridge.SymbolView, error) {
	var v featurebridge.SymbolView
	if w == nil || w.items == nil || w.items.SymbolSockets == nil || w.items.EmblemInlay == nil {
		return v, fmt.Errorf("symbol source unavailable")
	}
	layout := w.items.SymbolSockets
	if layout.Source != w.items.Catalog.Source.Checksum || layout.Source != w.items.EmblemInlay.Source {
		return v, fmt.Errorf("symbol source mismatch")
	}
	state, e := inventory.ReadSymbolSockets(w.role.State, layout)
	if e != nil {
		return v, e
	}
	v = featurebridge.SymbolView{Role: uint64(w.role.ID), Sequence: state.Sequence, Source: layout.Source}
	names := make([]string, 0, len(layout.Sockets))
	for name := range layout.Sockets {
		names = append(names, name)
	}
	sort.Strings(names)
	normal, buff := inventory.SymbolAttributes{}, inventory.SymbolAttributes{}
	for _, name := range names {
		g := featurebridge.SymbolGroup{Name: name, Mask: layout.Sockets[name].Mask}
		attributes := inventory.SymbolAttributes{}
		var star *featurebridge.SymbolStar
		var resonance inventory.SymbolOptions
		if typ, ok := inventory.SymbolTypes[name]; ok && layout.Rules != nil {
			progress := state.Progress[name]
			tier := layout.Rules.Tiers[inventory.SymbolTierKey{Type: typ, Grade: progress.Grade}]
			fixed, e := layout.Rules.FixedAt(typ, progress.Grade, progress.Level)
			if e != nil {
				return v, e
			}
			fame, err := inventory.SymbolStarFame(layout, name, progress)
			if err != nil {
				return v, err
			}
			star = &featurebridge.SymbolStar{Grade: progress.Grade, Level: progress.Level, Fame: fame, Fixed: symbolOptionText(fixed)}
			v.Powers = append(v.Powers, layout.FixedTemplates[tier.Levels[progress.Level].OptionIndex])
			for n, grade := range []uint32{2, 3, 7, 8} {
				values, e := layout.Rules.ResonanceAt(progress.Grade, grade)
				if e != nil {
					return v, e
				}
				star.PerEmblem[n] = symbolOptionText(values)
			}
			resonance = inventory.SymbolOptions{Dealer: map[string]float64{}, Buffer: map[string]float64{}}
		}
		for _, i := range state.Groups[name] {
			item := featurebridge.SymbolItem{Template: i.Template, ExpireTime: i.ExpireTime}
			if i.Template > 1 && !protocol.StoredItemExpired(i.ExpireTime, now.Unix()) {
				item.Flags = 1
			}
			if i.Template > 1 {
				script, err := w.items.Catalog.ItemScript(i.Template)
				if err != nil {
					return v, err
				}
				if item.Flags&1 != 0 {
					attributes.Add(script, name == "buff_platinum")
				}
				for n, token := range script.Cells {
					if token.Type == 3 && token.Text == "[grade]" && n+1 < len(script.Cells) && script.Cells[n+1].Type == 0 {
						grade := script.Cells[n+1].Value
						if grade < 0 || grade > 255 {
							return v, fmt.Errorf("invalid emblem grade")
						}
						item.Flags |= uint32(grade) << 16
						if star != nil && item.Flags&1 != 0 {
							key := inventory.SymbolResonanceKey{SymbolGrade: star.Grade, EmblemGrade: uint32(grade)}
							if index, ok := layout.Rules.ResonanceIndices[key]; ok {
								effect := layout.Rules.Resonance[index]
								for k, v := range effect.Dealer {
									resonance.Dealer[k] += v
								}
								for k, v := range effect.Buffer {
									resonance.Buffer[k] += v
								}
								v.Powers = append(v.Powers, layout.ResonanceTemplates[index])
							}
						}
					}
					if token.Type == 3 && token.Text == "[rarity]" && n+1 < len(script.Cells) {
						rarity := script.Cells[n+1]
						if rarity.Type != 0 || rarity.Value < 0 || rarity.Value > 255 {
							return v, fmt.Errorf("invalid emblem rarity")
						}
						item.Flags |= uint32(rarity.Value) << 8
					}
				}
			}
			g.Items = append(g.Items, item)
		}
		g.Attributes = attributes.Text()
		if star != nil {
			star.Resonance = symbolOptionText(resonance)
			g.Star = star
		}
		if name == "buff_platinum" {
			buff.Merge(attributes)
		} else {
			normal.Merge(attributes)
		}
		v.Groups = append(v.Groups, g)
	}
	v.Attributes, v.BuffAttributes = normal.Text(), buff.Text()
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return v, e
	}
	for _, i := range bag.Items {
		mask := w.items.EmblemInlay.Masks[i.Template]
		if mask == 0 || i.Amount == 0 || protocol.StoredItemExpired(i.ExpireTime, now.Unix()) {
			continue
		}
		v.Bag = append(v.Bag, featurebridge.SymbolBagItem{Slot: i.Slot, Mask: mask, Template: i.Template, Amount: i.Amount, ExpireTime: i.ExpireTime})
	}
	sort.Slice(v.Bag, func(i, j int) bool { return v.Bag[i].Slot < v.Bag[j].Slot })
	return v, nil
}

func symbolOptionText(v inventory.SymbolOptions) [2]string {
	format := func(stats map[string]float64) string {
		if n, ok := stats["[equipment damage rate]"]; ok {
			return fmt.Sprintf("攻击强化 +%g%%", math.Round(n*1000)/1000)
		}
		if n, ok := stats["[skill bonus rate]"]; ok {
			return fmt.Sprintf("技能攻击力 +%g%%", math.Round(n*1000)/1000)
		}
		if n, ok := stats["[equipment buff]"]; ok {
			return fmt.Sprintf("增益量 +%g", n)
		}
		return "无"
	}
	return [2]string{format(v.Dealer), format(v.Buffer)}
}

// Executed by the authenticated game connection's serialized event loop.
func (client *gameConnection) symbolFeature(ctx context.Context, request featurebridge.Frame, reply *featurebridge.Frame, now time.Time) {
	w := client.worldState
	if w.items == nil || w.items.SymbolSockets == nil {
		reply.Status = featurebridge.StatusUnsupported
		return
	}
	if request.Kind == featurebridge.SymbolSnapshot {
		if len(request.Body) != 0 {
			reply.Status = featurebridge.StatusRefused
			return
		}
	} else if request.Kind == featurebridge.SymbolRefresh {
		r, e := featurebridge.DecodeSymbolRefresh(request.Body)
		v, ve := w.symbolView(now)
		if e != nil || ve != nil || r.Role != v.Role || r.Sequence != v.Sequence || r.Source != v.Source {
			reply.Status = featurebridge.StatusRefused
			return
		}
		body, e := inventory.WornSpaceUpdate(w.role.State)
		if e != nil || client.output == nil {
			reply.Status = featurebridge.StatusInternal
			return
		}
		// Client publishes the confirmed emblem effects before asking the existing
		// native worn-equipment reader to rebuild the character's attributes.
		if e = client.output.send(0, 14, body); e != nil {
			reply.Status = featurebridge.StatusInternal
			return
		}
	} else {
		r, e := featurebridge.DecodeSymbolRequest(request.Body)
		if e != nil || r.Role != uint64(w.role.ID) || w.activeDungeon != nil || w.state.Position.Town == 0 {
			reply.Status = featurebridge.StatusRefused
			return
		}
		saved, _, e := (&workflow.ItemService{Store: w.store, Items: w.items}).ExchangeSymbolSocket(ctx, w.role, w.items.SymbolSockets, inventory.SymbolSocketRequest{
			Source: r.Source, Sequence: r.Sequence, Group: r.Group, Socket: r.Socket, ExpectedTemplate: r.ExpectedTemplate, BagSlot: r.BagSlot, Template: r.Template,
		}, now)
		if e != nil {
			reply.Status = featurebridge.StatusRefused
			return
		}
		w.role = saved

		bag, e := inventory.ReadBag(saved.State)
		if e != nil {
			reply.Status = featurebridge.StatusInternal
			return
		}
		body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		if e != nil {
			reply.Status = featurebridge.StatusInternal
			return
		}
		// The existing native inventory reader replaces absolute item counts.
		if client.output != nil {
			if e = client.output.send(0, 13, body); e != nil {
				reply.Status = featurebridge.StatusInternal
				return
			}
		}
	}
	if request.Kind != featurebridge.SymbolSnapshot && client.output != nil {
		for _, p := range w.appendFameUpdate(nil, nil) {
			if e := client.output.send(p.Kind, p.ID, p.Payload); e != nil {
				reply.Status = featurebridge.StatusInternal
				return
			}
		}
	}
	v, e := w.symbolView(now)
	if e == nil {
		reply.Body, e = v.Bytes()
	}
	if e != nil {
		reply.Status = featurebridge.StatusInternal
		reply.Body = nil
	}
}
