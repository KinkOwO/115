package protocol

import (
	"encoding/binary"
	"testing"
)

// These are synthetic bodies with the confirmed native sender layout, not
// captured Delezie combat vectors. Other option bytes must not select mode.
func TestDelezieCreateDifficultyUsesFifthOption(t *testing.T) {
	for _, difficulty := range []byte{0, 1, 2, 3} {
		body := []byte{14}
		body = addName(body, "test")
		body = append(body, 3, 2, 1, 7, difficulty, 0)
		body = add32(body, 43210)
		request, err := DecodeRaidCreateRequest(body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := request.DelezieMode115()
		if err != nil || got != difficulty || request.Limit != 43210 {
			t.Fatalf("mode/limit shifted: %+v; mode=%d err=%v", request, got, err)
		}
	}
	for _, request := range []RaidCreateRequest{
		{Kind: 8}, {Kind: 13}, {Kind: 14, Options: [6]byte{0, 0, 0, 0, 4}},
		{Kind: 14, Options: [6]byte{0, 0, 0, 0, 255}},
	} {
		if _, err := request.DelezieMode115(); err == nil {
			t.Fatalf("unsupported kind/difficulty accepted: %+v", request)
		}
	}
}

// Consume the independent native 144CDBF80 layout up to outer+129. Checking
// only server state/Options would miss the old response's constant zero.
func TestDelezieRecruitmentDifficultyReachesNativeConsumer(t *testing.T) {
	for difficulty := byte(0); difficulty < 4; difficulty++ {
		recruitment := RaidRecruitment{ID: 7, Create: RaidCreateRequest{Kind: 14, Title: "中文队伍", Options: [6]byte{0, 0, 0, 7, difficulty}}, Leader: 9, LeaderName: "test", MemberCount: 1}
		owned, err := RaidOwnedDetails(recruitment)
		if err != nil {
			t.Fatal(err)
		}
		listed, err := RaidRecruitmentList([]RaidRecruitment{recruitment})
		if err != nil {
			t.Fatal(err)
		}
		full, err := RaidFullInfoSuccess(recruitment)
		if err != nil {
			t.Fatal(err)
		}
		for _, snapshot := range []struct {
			name   string
			body   []byte
			prefix int
		}{
			{"owned", owned, 8}, {"list", listed, 4}, {"full", full, 6},
		} {
			at := snapshot.prefix
			take := func(n int) []byte {
				t.Helper()
				if n < 0 || at+n > len(snapshot.body) {
					t.Fatalf("%s native reader overrun at %d", snapshot.name, at)
				}
				out := snapshot.body[at : at+n]
				at += n
				return out
			}
			if binary.LittleEndian.Uint32(take(4)) != recruitment.ID {
				t.Fatal("record ID shifted")
			}
			titleBytes := int(binary.LittleEndian.Uint32(take(4)))
			if string(take(titleBytes)) != recruitment.Create.Title {
				t.Fatal("title shifted")
			}
			if take(3)[0] != 14 {
				t.Fatal("kind shifted")
			} // outer+28,+2C,+34
			if binary.LittleEndian.Uint32(take(4)) != uint32(recruitment.Leader) {
				t.Fatal("leader shifted")
			}
			take(3) // outer+120,+122,+123
			take(5) // outer+124 through +128, each native u8
			if got := take(1)[0]; got != difficulty {
				t.Fatalf("%s mode%d reached native outer+129 as %d", snapshot.name, difficulty, got)
			}
			if binary.LittleEndian.Uint32(take(4)) != recruitment.Create.Limit {
				t.Fatal("mode fix shifted limit")
			}
		}
	}
}
