package catalog

import (
	"dfolan/internal/catalog/pvf"
)

// The ETC [WAITING ROOM] pair is the area the start flow gathers in. The
// handover binary created the Bakal raid standing in town 152 area 2 (the
// ETC pair, 12:14:06 raid_created_waiting), every other raid type that
// session likewise created in its ETC waiting area, and the live channel-92
// raid's N578 member record (2026-10-05 01:55:15) carries the waiting area
// 6. The waiting hall (area 1, waitroom.map) still hosts the create dialog
// of the verified client flow — the user confirmed the raid is created in
// the left hall and the formation is edited in the right camp before
// starting — so both raid-town rooms accept CMD656.
func bindBakalWaitingRoom(a *pvf.Archive, r *BakalRaidRules) error {
	bound, err := ImportTownArea(a, uint32(r.WaitingRoomTown), uint32(r.WaitingRoomArea))
	if err != nil {
		return err
	}
	r.WaitingRoom = &bound
	return nil
}

func (r *BakalRaidRules) OwnsWaitingRoom(town, area uint32) bool {
	if r == nil || r.WaitingRoom == nil || r.WaitingRoom.TownID != town {
		return false
	}
	// Area 1 is the raid-town waiting hall (waitroom.map, the create room
	// of the verified flow); the ETC waiting area (camp1.map) also accepts
	// creates per the captured official 152/2 create position.
	return area == 1 || area == r.WaitingRoom.AreaID
}
