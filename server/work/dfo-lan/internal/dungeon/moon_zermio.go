package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"math"
	"time"
)

// The client plays its sourced escape action, reports state, then removes
// that actor through the native world/UDP object lifecycle. This is NOT a kill.
func (r *MoonSoloOwner) ReportMoonZermio(stamp MoonSoloStamp, member uint16, report protocol.MoonZermioReport115, now time.Time) (bool, MoonProgress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return false, MoonProgress{}, e
	}
	s := r.session
	if r.phase != moonSoloRunning || member != r.leader || s.Definition.ID != 100004137 || !s.Loaded || !s.MoonGridReady || s.MoonZermioDefeated || s.Completed() {
		return false, MoonProgress{}, fmt.Errorf("Zermio report outside active controlled encounter")
	}
	if s.MoonZermioReportRoom == stamp.Room {
		if report.Health != s.MoonZermioHealth || report.MeterBits != s.MoonZermioMeter {
			return false, MoonProgress{}, fmt.Errorf("Zermio duplicate changes state")
		}
		return false, s.MoonProgress(), nil
	}
	if s.MoonZermioSpawnedAt.IsZero() || now.Before(s.MoonZermioSpawnedAt.Add(50*time.Second)) {
		return false, MoonProgress{}, fmt.Errorf("Zermio source escape interval not reached")
	}
	grid := [2]int32{int32(s.Room.X), int32(s.Room.Y)}
	if grid != s.MoonZermioGrid {
		return false, MoonProgress{}, fmt.Errorf("Zermio report from wrong grid")
	}
	var destination [2]int32
	switch grid {
	case [2]int32{3, 0}:
		destination = [2]int32{1, 0}
	case [2]int32{1, 0}:
		destination = [2]int32{2, 1}
	default:
		return false, MoonProgress{}, fmt.Errorf("Zermio last source grid cannot escape")
	}
	present := false
	for _, room := range s.Maze.Rooms {
		present = present || [2]int32{int32(room.X), int32(room.Y)} == destination
	}
	if !present {
		return false, MoonProgress{}, fmt.Errorf("Zermio next source grid absent")
	}
	f := float64(math.Float32frombits(report.MeterBits))
	if report.Health <= 1 || report.Health > math.MaxInt64 || math.IsNaN(f) || math.IsInf(f, 0) || s.MoonZermioHealth != 0 && report.Health > s.MoonZermioHealth {
		return false, MoonProgress{}, fmt.Errorf("Zermio report has invalid or increased health")
	}
	entity := uint16(0)
	for _, m := range s.Monsters {
		if m.Template == 109017557 && !s.Dead[m.Entity] {
			if entity != 0 {
				return false, MoonProgress{}, fmt.Errorf("multiple active Zermio actors")
			}
			entity = m.Entity
		}
	}
	if entity == 0 {
		return false, MoonProgress{}, fmt.Errorf("no current Zermio actor")
	}
	if s.MoonRetired == nil {
		s.MoonRetired = map[uint16]bool{}
	}
	s.MoonRetired[entity] = true
	kept := make([]protocol.DungeonMonster, 0, len(s.Monsters)-1)
	for _, m := range s.Monsters {
		if m.Entity != entity {
			kept = append(kept, m)
		}
	}
	s.Monsters = kept
	s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), kept...)
	delete(s.MoonDynamic, entity)
	s.MoonZermioGrid = destination
	s.MoonZermioHealth, s.MoonZermioMeter = report.Health, report.MeterBits
	s.MoonZermioReportRoom = stamp.Room
	return true, s.MoonProgress(), nil
}
