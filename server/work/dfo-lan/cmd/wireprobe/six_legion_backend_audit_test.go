package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"encoding/binary"
	"os"
	"testing"
)

// A diagnostic audit reports the current result without making missing
// transitions a permanent regression expectation. Native gameplay remains
// unaccepted even if the old UI handler returns a successful command reply.
func TestRequestedLegionBackendAuditNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range contents.Contents {
		if content.Recruiting.Town == 204 || content.Recruiting.Town == 239 {
			if !content.Complete {
				t.Fatalf("requested legion source incomplete: %s %v", name, content.Missing)
			}
			t.Logf("source=%s content=%s recruiting=%d/%d event=%d dungeons=%d", contents.Path, name, content.Recruiting.Town, content.Recruiting.Area, content.UIInts["ENABLE_CHANNEL_TAB_EVENT_ID"], len(content.Dungeons))
		}
	}
	operations, err := catalog.ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations.Doors) != 4 {
		t.Fatalf("native shortcut source bindings: %v", operations.Doors)
	}
	for _, choice := range []byte{0, 1, 2, 4} {
		plan, err := legion.PlanForChoice(operations, clock, choice)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range plan.Gates.Schedule {
			for _, door := range operations.Doors {
				path, err := plan.Gates.PhysicalDoorPath(row.FlowKeys, door)
				if row.FlowKeys[door.Slot] == 0 {
					if err == nil {
						t.Fatal("closed native door accepted")
					}
				} else if err != nil || len(path) == 0 || path[0] != door.Target {
					t.Fatal(door, path, err)
				}
			}
		}
	}
	for _, door := range operations.Doors {
		t.Logf("native door object=%d slot=%d destination=%d dungeon=%d object_source=%s action_source=%s", door.Object, door.Slot, door.Target, door.Dungeon, door.ObjectPath, door.ActionPath)
	}
	for _, stage := range []uint32{0, 1} {
		s := &legionSession{catalog: operations, clock: clock, channelType: 119}
		w := townSession(1)
		w.channelType = 119
		if _, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart); err != nil {
			t.Fatal(err)
		}
		selectBody := make([]byte, legion.EnvelopeSize+9)
		binary.LittleEndian.PutUint32(selectBody[13:], 2)
		binary.LittleEndian.PutUint32(selectBody[18:], uint32(legion.OperationChannelCode))
		if _, err := s.handle(w, selectBody, legion.CmdOperationSelect); err != nil {
			t.Fatal(err)
		}
		if stage == 0 {
			for _, request := range []struct {
				id   uint16
				body []byte
			}{
				{legion.CmdFail, failPayload(uint32(legion.ApocalypseContent))},
				{legion.CmdRewardEnd, rewardEndPayload(uint32(legion.ApocalypseContent), 0, 0)},
			} {
				before, plan := *s.session, s.plan
				result, endErr := s.handle(w, request.body, request.id)
				t.Logf("Apocalypse CMD%d error=%v response_packets=%d owned_run=false durable_reward=false", request.id, endErr, len(result.Packets))
				if endErr == nil || len(result.Packets) != 0 || len(result.Events) != 0 || *s.session != before || s.plan != plan {
					t.Fatal("unowned completion acknowledged or choice changed")
				}
			}
		}
		if stage == 0 {
			limits, err := operations.CompileRoleLimits()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("native role quotas source=%s field=[role per member limit]/[member] quotas=%v", operations.Source, limits)
			w.activeDungeon = &dungeon.Session{}
			for _, role := range []uint32{0, 1, 2} {
				result, roleErr := s.handle(w, rolePayload(role, legion.OperationChannelCode), legion.CmdRoleSelect)
				t.Logf("Apocalypse CMD2355 role=%d error=%v response_packets=%d owned_navigation_room=false", role, roleErr, len(result.Packets))
				if roleErr == nil || len(result.Packets) != 0 {
					t.Fatal("unowned dungeon accepted role selection")
				}
			}
			w.activeDungeon = nil
		}
		enterBody := make([]byte, legion.EnvelopeSize+8)
		binary.LittleEndian.PutUint32(enterBody[13:], uint32(legion.OperationChannelCode))
		binary.LittleEndian.PutUint32(enterBody[17:], stage)
		result, err := s.handle(w, enterBody, legion.CmdEnterDungeon)
		t.Logf("Apocalypse CMD2045 stage=%d error=%v response_packets=%d real_dungeon_started=%v", stage, err, len(result.Packets), w.activeDungeon != nil)
	}
}
