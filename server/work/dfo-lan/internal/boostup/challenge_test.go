package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBoostChallengeSourceAndLifecycle(t *testing.T) {
	dir := os.Getenv("US115_TEST_BOOST_CHALLENGE")
	if dir == "" {
		t.Skip("explicit read-only challenge source required")
	}
	b, err := os.ReadFile(filepath.Join(dir, "00-boostupspecupchallenge.evt.tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cells []pvf.Token
	if err = json.Unmarshal(b, &cells); err != nil {
		t.Fatal(err)
	}
	rows, err := ParseChallenges(cells)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatal("source rows", len(rows))
	}
	c := &Catalog{Challenges: rows}
	for i, threshold := range []uint32{115, 55950, 63257, 71179} {
		d, e := c.Challenge(byte(i))
		if e != nil || d.UnlockValue != threshold || d.Repeat != 1 || !d.UnlockMail || !d.ClearMail {
			t.Fatalf("row %d: %+v %v", i, d, e)
		}
	}
	s, changed, err := c.UnlockChallenges(nil, 115, 55950)
	if err != nil || !changed || !s.Rows[0].Unlocked || !s.Rows[1].Unlocked || s.Rows[2].Unlocked || s.Rows[3].Unlocked {
		t.Fatal(s, err)
	}
	// Go 按钮的目标区域/落点必须是源自己写的 `[go contents town area]`：
	// 行 0 = 241/1（千年苍穹 最终调律者入口）@143,173，与实机 CMD36 逐字节相符。
	for i, want := range [4][4]uint32{{241, 1, 143, 173}, {214, 1, 500, 255}, {228, 1, 143, 173}, {220, 1, 1663, 238}} {
		d, e := c.Challenge(byte(i))
		if e != nil || !d.HasGoTarget || d.GoTarget != want {
			t.Fatalf("row %d go target %+v (want %+v): %v", i, d.GoTarget, want, e)
		}
	}
	if s.Rows[0].UnlockClaimed || s.Rows[0].Progress != 0 {
		t.Fatal("unlock awarded without claim")
	}
	n, rewards, err := c.ClaimChallenge(s, 0, 0)
	if err != nil || len(rewards) != 1 || rewards[0] != (Reward{590015966, 1000}) || !n.Rows[0].UnlockClaimed || s.Rows[0].UnlockClaimed {
		t.Fatal("unlock claim/clone", err)
	}
	if _, _, err = c.ClaimChallenge(n, 0, 0); err == nil {
		t.Fatal("duplicate unlock")
	}
	if _, _, err = c.ClaimChallenge(n, 0, 1); err == nil {
		t.Fatal("clear claim before clear")
	}
	for i := 0; i < 12; i++ {
		n, _, err = c.ApplyChallengeClear(n, "clear endkeeper of order", 0, 100005014)
		if err != nil {
			t.Fatal(err)
		}
	}
	if n.Rows[0].Progress != 10 || n.Rows[1].Progress != 0 {
		t.Fatal("count/cap", n.Rows)
	}
	n, _, err = c.ClaimChallenge(n, 0, 1)
	if err != nil || n.Rows[0].Claims != 1 {
		t.Fatal(err, n)
	}
	if _, _, err = c.ClaimChallenge(n, 0, 1); err == nil {
		t.Fatal("duplicate clear reward")
	}
	n, _, err = c.ApplyChallengeClear(n, "clear higher or legion", 31, 100004520)
	if err != nil || n.Rows[1].Progress != 1 || n.Rows[3].Progress != 0 {
		t.Fatal("locked fame row counted", err, n)
	}
	n, _, err = c.UnlockChallenges(n, 115, 71179)
	if err != nil || !n.Rows[3].Unlocked || n.Rows[3].Progress != 0 {
		t.Fatal("unlock must not invent historical clears", err)
	}
	n, _, err = c.ApplyChallengeClear(n, "clear higher or legion", 26, 100004134)
	if err != nil || n.Rows[3].Progress != 0 {
		t.Fatal("wrong content counted", err)
	}
	n, _, err = c.ApplyChallengeClear(n, "clear higher or legion", 36, 100003630)
	if err != nil || n.Rows[3].Progress != 1 {
		t.Fatal(err)
	}
	for _, bad := range []*ChallengeState{
		{Version: 1, Rows: map[byte]ChallengeProgress{0: {Unlocked: true}}},
		{Version: 1, Enrolled: true, Rows: map[byte]ChallengeProgress{0: {Progress: 1}}},
		{Version: 1, Enrolled: true, Rows: map[byte]ChallengeProgress{0: {Unlocked: true, Progress: 11}}},
		{Version: 1, Enrolled: true, Rows: map[byte]ChallengeProgress{31: {Unlocked: true}}},
	} {
		if c.ValidateChallengeState(bad) == nil {
			t.Fatal("invalid state accepted", bad)
		}
	}
}
