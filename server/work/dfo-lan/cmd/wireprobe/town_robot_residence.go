package main

import (
	"dfolan/internal/catalog"
	"sort"
	"strings"
)

// Operator ambience policy: distribute the bounded roster round-robin across
// source-public towns. A career has one stable home, not one copy per scene.
// Representatives and walkability come from the current world catalog.
func robotResidenceAreas(c catalog.WorldCatalog) []catalog.WorldArea {
	representatives := map[uint32]catalog.WorldArea{}
	for _, a := range c.Areas {
		if a.SeriaReturnWarp || a.MinimumLevel > 115 || (a.Kind != "[normal]" && !(a.Kind == "" && c.Source.Checksum == "")) {
			continue
		}
		if _, ok := robotRectangle(a); !ok {
			continue
		}
		// Operator-requested ambience scope: ordinary town asset namespaces,
		// excluding event/raid lobbies. IDs and geometry remain source-derived.
		if c.Source.Checksum != "" {
			path := strings.ToLower(a.MapPath)
			ordinary := strings.HasPrefix(path, "commonmap/town/") || strings.HasPrefix(path, "map/town/") || strings.HasPrefix(path, "map/cataclysm/town/")
			if !ordinary || strings.Contains(path, "/event/") || strings.Contains(path, "_raid/") || strings.Contains(path, "/raid/") || strings.Contains(path, "raid.map") || strings.Contains(path, "/waitroom.") {
				continue
			}
		}
		previous, exists := representatives[a.Town]
		if !exists || a.Area < previous.Area {
			representatives[a.Town] = a
		}
	}
	out := make([]catalog.WorldArea, 0, len(representatives))
	for _, a := range representatives {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Town > out[j].Town })
	// Twelve sparse home towns admit 5–6 of the 69 careers each. Prefer the
	// higher source town ordinals so the current high-level region is populated.
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func robotResidentIDs(c catalog.WorldCatalog, count int, town, area uint32) []int {
	homes := robotResidenceAreas(c)
	out := []int{}
	if len(homes) == 0 {
		return out
	}
	for id := 1; id <= count; id++ {
		home := homes[(id-1)%len(homes)]
		if home.Town == town {
			// A home belongs to a town, not a single street. Only one live actor
			// per record is admitted by the fleet even with multiple town viewers.
			current, ok := c.Areas[catalog.AreaKey(town, area)]
			if !ok || current.SeriaReturnWarp {
				continue
			}
			if _, ok = robotRectangle(current); !ok {
				continue
			}
			out = append(out, id)
		}
	}
	return out
}

// Called after the owner is placed, before population pruning and scene lists.
// This moves the same peers without sending a new party or resetting its UDP
// transport. Private rooms hide the models while retaining the party.
func (w *worldSession) moveTownRobotParty() {
	if w == nil || w.hub == nil || w.peer == nil || w.hub.townRobots == nil {
		return
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[w.peer]
	if p == nil || p.dungeon {
		return
	}
	pos := w.state.Position
	private := w.privateArea(pos)
	var rect [4]int32
	walkable := false
	if w.service != nil {
		if a, ok := w.service.Catalog.Areas[catalog.AreaKey(pos.Town, pos.Area)]; ok {
			rect, walkable = robotRectangle(a)
		}
	}
	for i, b := range p.bots {
		f.hub.mu.Lock()
		alreadyHere := b.peer.placed && b.peer.channel == w.peer.channel && b.peer.town == pos.Town && b.peer.area == pos.Area
		f.hub.mu.Unlock()
		// A repeated entry/settlement publish needs no new placement. Retain an
		// already source-accepted pose even when this route lacks geometry.
		if !private && alreadyHere {
			continue
		}
		if private || !walkable {
			f.hub.retire(b.peer)
			b.suspended = true
			continue
		}
		x, y := robotPoint(rect, uint32(b.peer.actorID)+uint32(i*137))
		b.rect = rect
		b.walking = false
		b.announced = false
		b.suspended = false
		b.idleUntil = 0
		f.hub.publish(b.peer, w.peer.channel, pos.Town, pos.Area, x, y, false)
	}
}
