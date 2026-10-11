package activity

import (
	"dfolan/internal/catalog/pvf"
	"github.com/stretchr/testify/require"
	"math"
	"strings"
	"testing"
	"time"
)

func membershipFixture() MembershipRules {
	reward := Reward{Item: 100, Count: 3, MailTitle: "Bound title", MailMessage: "Bound message"}
	return MembershipRules{SourceChecksum: strings.Repeat("a", 64), Table: "fixture", Dates: Dates{"2026-10-08 06:00:00", "2026-12-10 06:00:00"}, Grades: []MembershipGrade{{0, "Basic", 0, 9, []Reward{reward}}, {1, "Rare", 10, 19, []Reward{reward}}, {2, "Epic", 20, 20, []Reward{reward}}}, RepeatPoints: 5, RepeatRewards: []Reward{reward}, Paybacks: []MembershipPayback{{1001, map[uint32]Reward{1: reward, 2: reward}}}}
}
func TestMembershipBoundariesCarryAndAccountClaimIdentity(t *testing.T) {
	r := membershipFixture()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	var p MembershipProgress
	var e error
	for _, step := range []struct {
		credit uint64
		grade  uint32
		repeat uint64
	}{{9, 0, 0}, {1, 1, 0}, {9, 1, 0}, {1, 2, 0}, {4, 2, 0}, {1, 2, 1}, {7, 2, 2}} {
		p, e = r.Observe(p, now, step.credit)
		require.NoError(t, e)
		require.Equal(t, step.grade, r.Grade(p.Spent))
		require.Equal(t, step.repeat, r.RepeatAvailable(p))
	}
	require.EqualValues(t, 32, p.Spent)
	for index := uint32(1); index <= 3; index++ {
		var rewards []Reward
		p, rewards, e = r.Claim(p, "period", index)
		require.NoError(t, e)
		require.Len(t, rewards, 1)
		_, rewards, e = r.Claim(p, "period", index)
		require.NoError(t, e)
		require.Empty(t, rewards)
	}
	_, _, e = r.Claim(p, "cumulative", 2)
	require.Error(t, e)
	for i := uint32(1); i <= 2; i++ {
		p, _, e = r.Claim(p, "cumulative", i)
		require.NoError(t, e)
	}
	_, _, e = r.Claim(p, "cumulative", 3)
	require.Error(t, e)
	// The threshold and grant change when source values change.
	r.RepeatPoints = 3
	r.RepeatRewards[0].Count = 19
	var rewards []Reward
	p, rewards, e = r.Claim(p, "cumulative", 3)
	require.NoError(t, e)
	require.EqualValues(t, 19, rewards[0].Count)
	_, e = r.Observe(p, now.Add(-time.Second), 1)
	require.Error(t, e)
	_, e = r.Observe(p, now, math.MaxUint64)
	require.Error(t, e)
	_, e = r.Observe(p, time.Date(2026, 12, 10, 6, 0, 0, 0, time.UTC), 0)
	require.ErrorIs(t, e, ErrRoutineInactive)
}
func TestMembershipRejectsCorruptSavesAndMalformedSource(t *testing.T) {
	r := membershipFixture()
	require.NoError(t, r.Validate())
	for _, p := range []MembershipProgress{{Spent: 1}, {Version: 2}, {Version: 1, Spent: 1, Claimed: map[uint32]bool{1: true}}, {Version: 1, RepeatClaimed: 1}, {Version: 1, HighWater: -1}} {
		require.Error(t, r.ValidateProgress(p))
	}
	_, e := ParseMembership([]pvf.Token{{Type: 3, Text: "[local extension controller]"}, {Type: 6, Text: "native-unverified"}})
	require.Error(t, e)
	r.Grades[1].Minimum = 11
	require.Error(t, r.Validate())
}
