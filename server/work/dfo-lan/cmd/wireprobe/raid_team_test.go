package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"sync"
	"testing"
)

func TestRaidTeamOwnershipAndRollback(t *testing.T) {
	var r raidTeamRegistry
	team, err := r.create(1, 82, 1, protocol.RaidRecruitment{}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.create(1, 98, 1, protocol.RaidRecruitment{}, database.WorldPosition{}); err == nil {
		t.Fatal("duplicate owner admitted")
	}
	if _, ok := r.get(1, 98); ok {
		t.Fatal("wrong channel admitted")
	}
	r.leave(2, team.Recruitment.ID)
	r.leave(1, team.Recruitment.ID+1)
	if _, ok := r.get(1, 82); !ok {
		t.Fatal("foreign rollback removed owner")
	}
	r.leave(1, team.Recruitment.ID)
	next, err := r.create(1, 82, 1, protocol.RaidRecruitment{}, database.WorldPosition{})
	if err != nil || next.Recruitment.ID <= team.Recruitment.ID {
		t.Fatal("ID was reused")
	}
}
func TestRaidTeamConcurrentDuplicateCreate(t *testing.T) {
	var r raidTeamRegistry
	var wg sync.WaitGroup
	winners := make(chan raidTeam, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			team, err := r.create(1, 82, 1, protocol.RaidRecruitment{}, database.WorldPosition{})
			if err == nil {
				winners <- team
			}
		}()
	}
	wg.Wait()
	close(winners)
	if len(winners) != 1 {
		t.Fatalf("created %d teams for one character", len(winners))
	}
}

func TestRaidAssignmentOwnershipLimitsAndStableIdentity(t *testing.T) {
	var r raidTeamRegistry
	team, err := r.create(1, 82, 1, protocol.RaidRecruitment{Leader: 9, MemberCount: 1}, database.WorldPosition{Town: 152})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		role     int64
		channel  uint32
		actor    uint16
		position uint32
	}{{2, 82, 9, 1}, {1, 98, 9, 1}, {1, 82, 10, 1}, {1, 82, 9, 12}, {1, 82, 9, 256}} {
		if _, err := r.assign(v.role, v.channel, v.actor, v.position, 12); err == nil {
			t.Fatal("invalid assignment accepted", v)
		}
	}
	current, _ := r.get(1, 82)
	if current.Recruitment.MemberPosition != 0 {
		t.Fatal("refused assignment changed member")
	}
	for _, pos := range []uint32{0, 1, 3, 4, 8, 11, 0} {
		got, err := r.assign(1, 82, 9, pos, 12)
		if err != nil || got.Recruitment.MemberPosition != byte(pos) || got.Recruitment.ID != team.Recruitment.ID || got.Origin != team.Origin {
			t.Fatal("assignment lost ownership/identity", got, err)
		}
	}
}
