package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func delezieRewardFixture() []pvf.Token {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 5, Text: s} }
	text := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	number := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	out := []pvf.Token{tag("[RAID]"), number(13), tag("[PHASE]"), tag("[/PHASE]"), tag("[/RAID]"), tag("[RAID]"), number(14), tag("[REPRESENTATIVE DUNGEON INDEX]"), number(123)}
	for d := DelezieNormal; d <= DelezieMatch; d++ {
		out = append(out, tag("[PHASE DIFFICULTY]"), text(d.Name()), tag("[CLEAR REWARD ITEM]"), tag("[STATE REWARD]"), text("party_card"), number(-1), number(17), number(100+int32(d)), number(0), number(0), tag("[/CLEAR REWARD ITEM]"))
		if d == DelezieNormal || d == DelezieHard {
			out = append(out, tag("[SPECIAL COMMON REWARD]"), text("squad_item"), number(19), number(-1), number(0), text("squad_item"), number(3), number(200+int32(d)), number(1), tag("[/SPECIAL COMMON REWARD]"))
		}
		out = append(out, tag("[/PHASE DIFFICULTY]"))
	}
	return append(out, tag("[/RAID]"))
}

func TestDelezieRewardSourceModesAndCommonRows(t *testing.T) {
	tokens := delezieRewardFixture()
	rules, err := parseDelezieRewards(tokens)
	if err != nil || len(rules.Modes) != 4 || rules.RepresentativeDungeon != 123 {
		t.Fatal(rules, err)
	}
	for d, mode := range rules.Modes {
		if len(mode.Rewards) != 1 || mode.Rewards[0].Template != 100+int32(d) || mode.Rewards[0].Weight != 17 {
			t.Fatal(d, mode)
		}
		if (d == DelezieNormal || d == DelezieHard) && (len(mode.Common) != 2 || mode.Common[0].Template != -1 || mode.Common[1].Weight != 3) {
			t.Fatal("common reward format lost", mode)
		}
	}
	row := rules.Modes[DelezieMatch].Rewards[0].Row
	tokens[row+4].Value = 999
	changed, err := parseDelezieRewards(tokens)
	if err != nil || changed.Modes[DelezieMatch].Rewards[0].Template != 999 || changed.Modes[DelezieNormal].Rewards[0].Template != 100 {
		t.Fatal("mode rewards did not follow their source", changed, err)
	}
	mode := &DelezieModeRules{Difficulty: DelezieMatch}
	owner := &DelezieRaidRules{Rewards: rules, Modes: map[DelezieDifficulty]*DelezieModeRules{DelezieMatch: mode}}
	program, err := owner.RewardProgram(mode)
	if err != nil {
		t.Fatal(err)
	}
	program.Rewards[0].Template = 777
	if rules.Modes[DelezieMatch].Rewards[0].Template != 103 {
		t.Fatal("reward snapshot exposed source slice")
	}
	foreign := *mode
	if _, err := owner.RewardProgram(&foreign); err == nil {
		t.Fatal("unowned mode accepted")
	}
}

func TestDelezieRewardsRejectBrokenScopesAndFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]pvf.Token) []pvf.Token
	}{
		{"unknown-mode", func(ts []pvf.Token) []pvf.Token { ts[10].Text = "unknown"; return ts }},
		{"duplicate-mode", func(ts []pvf.Token) []pvf.Token {
			for i, token := range ts {
				if token.Type == 6 && token.Text == "hard" {
					ts[i].Text = "normal"
				}
			}
			return ts
		}},
		{"no-raid-end", func(ts []pvf.Token) []pvf.Token { return ts[:len(ts)-1] }},
		{"missing-mode-end", func(ts []pvf.Token) []pvf.Token { return append(ts[:len(ts)-2], ts[len(ts)-1]) }},
		{"zero-weight", func(ts []pvf.Token) []pvf.Token { ts[15].Value = 0; return ts }},
		{"invalid-grade", func(ts []pvf.Token) []pvf.Token { ts[17].Value = 3; return ts }},
		{"invalid-template", func(ts []pvf.Token) []pvf.Token { ts[16].Value = -2; return ts }},
		{"noninteger", func(ts []pvf.Token) []pvf.Token { ts[15].Type = 6; return ts }},
		{"orphan-reward", func(ts []pvf.Token) []pvf.Token { ts[11].Text = "[STATE REWARD]"; return ts }},
		{"broken-common", func(ts []pvf.Token) []pvf.Token {
			for i, token := range ts {
				if token.Text == "[/SPECIAL COMMON REWARD]" {
					return append(ts[:i], ts[i+1:]...)
				}
			}
			return ts
		}},
		{"duplicate-raid", func(ts []pvf.Token) []pvf.Token { return append(ts, ts[5:]...) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseDelezieRewards(tc.edit(delezieRewardFixture())); err == nil {
				t.Fatal("malformed source accepted")
			}
		})
	}
}

func TestDelezieRewardLimitsSource(t *testing.T) {
	table := &pvf.CTPTable{Path: "fixture.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[WEEKLY CLEAR COUNT]", Cells: []pvf.CTPCell{{Kind: "float", Float: 2}}},
		{Index: 1, Parent: -1, Name: "[WEEKLY REWARD COUNT]", Cells: []pvf.CTPCell{{Kind: "float", Float: 3}}},
	}}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	rules := &DelezieRaidRules{Header: tree}
	a, b, err := rules.RewardLimits()
	if err != nil || a != 2 || b != 3 {
		t.Fatal(a, b, err)
	}
	table.Records[1].Cells[0].Float = 4
	_, b, err = rules.RewardLimits()
	if err != nil || b != 4 {
		t.Fatal("weekly cap did not follow source", b, err)
	}
}

func TestDelezieRewardsNativePVF(t *testing.T) {
	file := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if file == "" {
		t.Skip("native archive required")
	}
	a, err := OpenTestArchiveCached(file, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ImportRaidEntrances(a, &dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := entries[112].Delezie
	if rules.Source.Checksum != a.Snapshot().Checksum || rules.Rewards.Kind != 14 {
		t.Fatal("reward source identity missing")
	}
	clears, rewards, err := rules.RewardLimits()
	if err != nil || clears != 1 || rewards != 1 {
		t.Fatal(clears, rewards, err)
	}
	for _, tc := range []struct {
		mode         DelezieDifficulty
		rows, common int
	}{{DelezieNormal, 9, 2}, {DelezieHard, 10, 2}, {DelezieSingle, 5, 0}, {DelezieMatch, 5, 0}} {
		program, err := rules.RewardProgram(rules.Modes[tc.mode])
		if err != nil || len(program.Rewards) != tc.rows || len(program.Common) != tc.common {
			t.Fatal(tc.mode, program, err)
		}
		t.Logf("source=%s mode=%s personal-and-auction-rows=%d shared-rows=%d weekly-clear=%d weekly-reward=%d live-settlement=false", rules.Source.Checksum, tc.mode.Name(), len(program.Rewards), len(program.Common), clears, rewards)
	}
}
