package world

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"errors"
	"fmt"
	"strings"
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
	// 源地图（如 lemiedia_right.map 89/2 及其他 135 个区域）在 PVF 中定义了负数坐标矩形（r[0] < 0 或 r[1] < 0）。
	// 客户端在这些地图中落点与移动时，通过 16 位补码传输负坐标（如 X=-2 表现为 0xFFFE / uint16(65534)）。
	// 仅当矩形覆盖负坐标区域且输入值处于补码负数范围（>= 0x8000）时，按有符号解释；
	// 正数矩形中保持无符号比较，确保 65535 等越界大数坐标仍被严格拦截。
	if r[0] < 0 && x >= 0x8000 {
		px = int64(int16(x))
	}
	if r[1] < 0 && y >= 0x8000 {
		py = int64(int16(y))
	}
	return r[2] >= 0 && r[3] >= 0 && px >= int64(r[0])-m && py >= int64(r[1])-m && px <= int64(r[0])+int64(r[2])+m && py <= int64(r[1])+int64(r[3])+m
}

// WalkableTolerance 是可行走判定允许的越界像素。
// 客户端经传送门/地图传送落地的坐标会稳定偏出源矩形（实测 9/11/18/33/68 像素），
// 用 0 边距会把合法的门全挡掉；取 128 覆盖这些偏差（如 39/4 月光酒馆地图传送落点偏差 68 像素），
// 同时远小于"任意传送"的量级，权限校验仍然成立。
const WalkableTolerance = 128

func Walkable(a catalog.WorldArea, x, y uint16) bool {
	for _, r := range a.Walkable {
		if Contains(r, x, y, WalkableTolerance) {
			return true
		}
	}
	return false
}

// RequiredLevel is the source level gate of an area. An Arad Odyssey character
// follows the same permission data the client uses: [odyssey enter level] can
// only LOWER the gate, never raise it — the effective Odyssey gate is the
// smaller of the two source values. Live evidence covers both directions:
// Storm Pass (43/*, need 50 / odyssey 45) admits an Odyssey character at 45
// (the client's own DSTR 535 refusal names 45), while West Coast (40/0,
// need 15 / odyssey 35) admits one at 20 — the client sends the move request
// and its refusal text (DSTR 30069) names 15, so a higher Odyssey value is not
// an entry gate (it scopes the area's [phase] variants instead). Ordinary
// characters keep [need level] only, so no existing behaviour changes for them.
func RequiredLevel(a catalog.WorldArea, odyssey bool) uint32 {
	if odyssey && a.OdysseyMinimumLevel > 0 && a.OdysseyMinimumLevel < a.MinimumLevel {
		return a.OdysseyMinimumLevel
	}
	return a.MinimumLevel
}

func (s *Service) ValidatePosition(level byte, odyssey bool, p storage.WorldPosition) error {
	return s.validatePosition(level, p, func(a catalog.WorldArea) uint32 { return RequiredLevel(a, odyssey) })
}

// RestorationLevel is the gate for a position the character is already in (a
// saved login position, a movement report, a gate entered earlier). It never
// tightens what the server already accepted: for an Odyssey character the
// smaller of the two source gates wins, so an upgrade that teaches the server
// about [odyssey enter level] cannot strand a saved character behind a gate it
// passed under the old rule. This matches RequiredLevel's min semantics.
func RestorationLevel(a catalog.WorldArea, odyssey bool) uint32 {
	need := a.MinimumLevel
	if !odyssey || a.OdysseyMinimumLevel == 0 || a.OdysseyMinimumLevel >= need {
		return need
	}
	return a.OdysseyMinimumLevel
}

// ValidateRestoredPosition checks a position the character already occupies.
func (s *Service) ValidateRestoredPosition(level byte, odyssey bool, p storage.WorldPosition) error {
	return s.validatePosition(level, p, func(a catalog.WorldArea) uint32 { return RestorationLevel(a, odyssey) })
}

func (s *Service) validatePosition(level byte, p storage.WorldPosition, gate func(catalog.WorldArea) uint32) error {
	a, ok := s.Catalog.Areas[catalog.AreaKey(p.Town, p.Area)]
	if !ok {
		return errors.New("unknown area")
	}
	if uint32(level) < gate(a) {
		return ErrLevel
	}
	for _, pending := range a.Pending {
		// An unresolved outgoing dynamic portal is never selectable in the
		// catalog's authorized edge set; it does not invalidate known geometry.
		if pending == "dynamic portal destination" {
			continue
		}
		// 服务端没实现的条件（[need quest]、[level acc enter force level]、[event id] 等）
		// 由客户端自己判定；把它们当成"该区域不可进入"会让整片地图彻底打不开。
		if strings.HasPrefix(pending, "unsupported permission") {
			continue
		}
		return fmt.Errorf("area configuration unresolved: %s", pending)
	}
	if !Walkable(a, p.X, p.Y) {
		return errors.New("position outside source walkable rectangles")
	}
	return nil
}
func (s *Service) Transition(level byte, odyssey bool, old storage.WorldPosition, r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	return s.transition(level, odyssey, old, r, false)
}

// TransitionStrict preserves source-edge authorization for progression-gated
// Odyssey travel; ordinary travel keeps the upstream dynamic-portal behavior.
func (s *Service) TransitionStrict(level byte, odyssey bool, old storage.WorldPosition, r protocol.AreaChangeRequest) (storage.WorldPosition, error) {
	return s.transition(level, odyssey, old, r, true)
}

func (s *Service) transition(level byte, odyssey bool, old storage.WorldPosition, r protocol.AreaChangeRequest, strict bool) (storage.WorldPosition, error) {
	next := old
	if r.PreviousTown != old.Town || uint32(r.PreviousArea) != old.Area {
		return next, errors.New("stale source area")
	}
	next.Town, next.Area, next.X, next.Y = r.Town, r.Area, r.X, r.Y
	dest, exists := s.Catalog.Areas[catalog.AreaKey(r.Town, r.Area)]
	if !exists {
		return old, errors.New("unknown destination area")
	}
	if uint32(level) < RequiredLevel(dest, odyssey) {
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
		// 源区域的出边枚举不全时以客户端落点为准，两种情形：
		//   1) "dynamic portal destination"：脚本目的地写成 -1 -1，导入时被丢弃；
		//   2) 该区域一条门户边都没有：玩家用的是 NPC/码头/界面 触发的跨区传送。
		// 实测 694 个区域里 160 个属情形 1、73 个属情形 2。
		permissive := len(src.Portals) == 0 && !src.SeriaReturnWarp
		for _, pending := range src.Pending {
			if pending == "dynamic portal destination" {
				permissive = true
				break
			}
		}
		if strict || !permissive {
			return old, errors.New("no authorized source portal to destination")
		}
	}
	if e := s.ValidatePosition(level, odyssey, next); e != nil {
		return old, e
	}
	if dest.SeriaReturnWarp {
		next.Return = &storage.WorldReturn{Town: old.Town, Area: old.Area, X: old.X, Y: old.Y}
	}
	return next, nil
}
func (s *Service) Enter(ctx context.Context, account, id int64, level byte, odyssey bool, spawn storage.WorldPosition) (storage.WorldState, error) {
	// Re-entering a saved position is a restore, not an entry: the permissive
	// gate keeps a character logged in whichever source gate applies now.
	if e := s.ValidateRestoredPosition(level, odyssey, spawn); e != nil {
		return storage.WorldState{}, e
	}
	state, e := s.Store.LoadWorld(ctx, account, id, spawn, s.Catalog.Source.Checksum)
	if e != nil {
		return state, e
	}
	if state.ConfigVersion != s.Catalog.Source.Checksum {
		return state, errors.New("saved position requires catalog migration")
	}
	return state, s.ValidateRestoredPosition(level, odyssey, state.Position)
}
