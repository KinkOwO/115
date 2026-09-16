package world

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"errors"
	"fmt"
)

type Rules struct {
	RequirePortalProximity bool   `json:"require_portal_proximity"`
	PortalMargin           uint16 `json:"portal_margin"`
}
type Service struct {
	Store   *storage.Store
	Catalog catalog.WorldCatalog
	Rules   Rules
}

var ErrLevel = errors.New("destination level requirement not met")

func Contains(r [4]int32, x, y uint16, margin uint16) bool {
	px, py, m := int64(x), int64(y), int64(margin)
	return r[2] >= 0 && r[3] >= 0 && px >= int64(r[0])-m && py >= int64(r[1])-m && px <= int64(r[0])+int64(r[2])+m && py <= int64(r[1])+int64(r[3])+m
}
func Walkable(a catalog.WorldArea, x, y uint16) bool {
	for _, r := range a.Walkable {
		if Contains(r, x, y, 0) {
			return true
		}
	}
	return false
}

func (s *Service) ValidatePosition(level byte, p storage.WorldPosition) error {
	a, ok := s.Catalog.Areas[catalog.AreaKey(p.Town, p.Area)]
	if !ok {
		return errors.New("unknown area")
	}
	if uint32(level) < a.MinimumLevel {
		return ErrLevel
	}
	for _, pending := range a.Pending {
		// An unresolved outgoing dynamic portal is never selectable in the
		// catalog's authorized edge set; it does not invalidate known geometry.
		if pending != "dynamic portal destination" {
			return fmt.Errorf("area configuration unresolved: %s", pending)
		}
	}
	if !Walkable(a, p.X, p.Y) {
		return errors.New("position outside source walkable rectangles")
	}
	return nil
}
func (s *Service) Transition(level byte, old storage.WorldPosition, r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	next := old
	if r.PreviousTown != old.Town || uint32(r.PreviousArea) != old.Area {
		return next, errors.New("stale source area")
	}
	next.Town, next.Area, next.X, next.Y = r.Town, r.Area, r.X, r.Y
	dest, exists := s.Catalog.Areas[catalog.AreaKey(r.Town, r.Area)]
	if !exists {
		return old, errors.New("unknown destination area")
	}
	if uint32(level) < dest.MinimumLevel {
		return old, ErrLevel
	}
	src, ok := s.Catalog.Areas[catalog.AreaKey(old.Town, old.Area)]
	if !ok {
		return old, errors.New("unknown source area")
	}
	adjacent := false
	for _, p := range src.Portals {
		if p.Town == r.Town && p.Area == r.Area && (!s.Rules.RequirePortalProximity || Contains(p.Bounds, old.X, old.Y, s.Rules.PortalMargin)) {
			adjacent = true
		}
	}
	// A source Seria return warp targets the saved origin, never an arbitrary
	// client-selected map. Its animated portal condition remains explicit.
	if old.Return != nil && src.SeriaReturnWarp && old.Return.Town == r.Town && old.Return.Area == r.Area {
		if !s.Rules.RequirePortalProximity {
			adjacent = true
		} else {
			for _, rect := range src.ReturnWarpBounds {
				if Contains(rect, old.X, old.Y, s.Rules.PortalMargin) {
					adjacent = true
				}
			}
		}
		if adjacent {
			// The Seria map selector may supply its generic animation landing
			// point (live17: 746,157), which is outside this source town's
			// walkable geometry. Restore this character's validated saved
			// origin instead. Destination/portal ownership is still checked.
			next.X, next.Y = old.Return.X, old.Return.Y
			next.Return = nil
		}
	}
	if !adjacent {
		return old, errors.New("no authorized source portal to destination")
	}
	if e := s.ValidatePosition(level, next); e != nil {
		return old, e
	}
	if dest.SeriaReturnWarp {
		next.Return = &storage.WorldReturn{Town: old.Town, Area: old.Area, X: old.X, Y: old.Y}
	}
	return next, nil
}
func (s *Service) Enter(ctx context.Context, account, id int64, level byte, spawn storage.WorldPosition) (storage.WorldState, error) {
	if e := s.ValidatePosition(level, spawn); e != nil {
		return storage.WorldState{}, e
	}
	state, e := s.Store.LoadWorld(ctx, account, id, spawn, s.Catalog.Source.Checksum)
	if e != nil {
		return state, e
	}
	if state.ConfigVersion != s.Catalog.Source.Checksum {
		return state, errors.New("saved position requires catalog migration")
	}
	return state, s.ValidatePosition(level, state.Position)
}
