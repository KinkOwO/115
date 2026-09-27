package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"testing"
)

// scaleStatusPayload 造一条形状与实测一致的 CMD2329 载荷：
// u32 + char[256] 文本 + u8 + u8 + u32 模板号 + u32 + u32。
func scaleStatusPayload(text string, template uint32) []byte {
	p := make([]byte, 272)
	binary.LittleEndian.PutUint32(p[0:], 1)
	copy(p[scaleStatusTextOffset:scaleStatusTextOffset+scaleStatusTextLen], text)
	p[260] = 1
	p[261] = 1
	binary.LittleEndian.PutUint32(p[262:], template)
	return p
}

const scaleStatusSample = "HP : 94730.00, Action : 20014, triggers : 1, 0, rarities : 71, 72, 44, 72, omen : 1, 40, 0, 0, time_from_max : 0"

// scaleStatusSampleNoEnd 是同一形状但 triggers 第一位为 0 的样本：客户端还没跑收尾。
const scaleStatusSampleNoEnd = "HP : 94730.00, Action : 20014, triggers : 0, 0, rarities : 71, 72, 44, 72, omen : 1, 40, 0, 0, time_from_max : 0"

func TestDecodeScaleStatusReadsTemplateAndHP(t *testing.T) {
	want := map[uint32]bool{scalePrimerTemplate: true}
	r, ok := decodeScaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), want)
	if !ok {
		t.Fatal("real-shaped payload must decode")
	}
	if r.Template != scalePrimerTemplate {
		t.Fatalf("template = %d, want %d", r.Template, scalePrimerTemplate)
	}
	if r.HP != 94730 {
		t.Fatalf("hp = %v, want 94730", r.HP)
	}
}

func TestParseScaleHP(t *testing.T) {
	for _, c := range []struct {
		text string
		want float64
		ok   bool
	}{
		{scaleStatusSample, 94730, true},
		{"HP : 4736540.00, Action : 20014", 4736540, true},
		{"rarities : 71, 72", 0, false},
		{"", 0, false},
	} {
		got, ok := parseScaleHP(c.text)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("parseScaleHP(%q) = %v,%v want %v,%v", c.text, got, ok, c.want, c.ok)
		}
	}
}

// 载荷里的模板号必须是本次运行里关心的那个：别的机器人的日志不能被当成定盘机关。
func TestDecodeScaleStatusIgnoresUnknownTemplate(t *testing.T) {
	want := map[uint32]bool{scalePrimerTemplate: true}
	if _, ok := decodeScaleStatus(scaleStatusPayload(scaleStatusSample, 109019402), want); ok {
		t.Fatal("a payload for another template must not decode")
	}
	if _, ok := decodeScaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), map[uint32]bool{}); ok {
		t.Fatal("an empty want set must not decode")
	}
}

func TestDecodeScaleStatusRejectsTruncatedPayload(t *testing.T) {
	if _, ok := decodeScaleStatus([]byte{1, 2, 3}, map[uint32]bool{scalePrimerTemplate: true}); ok {
		t.Fatal("a short payload must not decode")
	}
}

// 开关默认关闭时，这条命令必须完全不产生任何出站包（行为与改动前一致）。
func TestScaleStatusIsInertWhenDisabled(t *testing.T) {
	w := &worldSession{}
	if plan, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), nil); plan != nil || err != nil {
		t.Fatalf("disabled switch must be inert, got %v %v", plan, err)
	}
	w = &worldSession{scaleDeathFromHP: true}
	if plan, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), nil); plan != nil || err != nil {
		t.Fatalf("no active run must be inert, got %v %v", plan, err)
	}
}

// scaleRunForTest 造一次「两台定盘装置都在场」的最小运行状态。
func scaleRunForTest() *dungeon.Session {
	return &dungeon.Session{
		Loaded: true,
		Monsters: []protocol.DungeonMonster{
			{Entity: 4107, Template: scalePrimerTemplate, Rank: 3},
			{Entity: 4108, Template: scaleOathTemplate},
		},
		Dead:       map[uint16]bool{},
		Unowned:    map[uint16]bool{},
		Visited:    map[uint32][]protocol.DungeonMonster{},
		NextEntity: 4200,
	}
}

func TestParseScaleEndTrigger(t *testing.T) {
	for _, c := range []struct {
		text string
		want bool
	}{
		{scaleStatusSample, true},
		{scaleStatusSampleNoEnd, false},
		{"HP : 1.00, Action : 2", false},
		{"", false},
	} {
		if got := parseScaleEndTrigger(c.text); got != c.want {
			t.Fatalf("parseScaleEndTrigger(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// 2026-09-27 回归：某一运行里 8 个样本全部落在地板 2% 上，本次运行见过的峰值就是地板，
// 于是 rate 恒为 100、血量判据永远不触发。日志本身即「客户端跑过 GO_END」的判据必须独立生效。
func TestScaleStatusKillsOnClientEndWithoutAnyPeak(t *testing.T) {
	w := &worldSession{scaleDeathFromHP: true, role: storage.Character{WireID: 3}, activeDungeon: scaleRunForTest()}
	var kinds []string
	seen := func(e map[string]any) { kinds = append(kinds, e["kind"].(string)) }
	plan, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), seen)
	if err != nil {
		t.Fatalf("forced death errored: %v", err)
	}
	forced := 0
	for _, k := range kinds {
		if k == "scale_death_forced" {
			forced++
		}
	}
	if forced != 1 {
		t.Fatalf("a client-end log must force exactly one death, events = %v", kinds)
	}
	// 两台装置都要退场：同坐标的另一台是 [fixture]，它活着会挡住 roomEnemiesDead()。
	if !w.activeDungeon.Dead[4107] || !w.activeDungeon.Dead[4108] {
		t.Fatalf("both devices must be dead, dead = %v", w.activeDungeon.Dead)
	}
	if len(plan) == 0 {
		t.Fatal("forced death must emit packets")
	}
	// 同一条日志再来一次不能重复判死。
	if _, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), seen); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, k := range kinds {
		if k == "scale_death_forced" {
			total++
		}
	}
	if total != 1 {
		t.Fatalf("the second report must be inert, events = %v", kinds)
	}
}

// 2026-09-27 回归：同一会话里重进副本时 entity 会从 0x1000 重新分配，去重表若不清空，
// 第二场起就再也不会判死（实测「第一场碰一下天平就死、第二场怎么打都不死」）。
func TestScaleStatusRearmsOnANewDungeonRun(t *testing.T) {
	run := scaleRunForTest()
	run.RunID = "run-one"
	w := &worldSession{scaleDeathFromHP: true, role: storage.Character{WireID: 3}, activeDungeon: run}
	forced := 0
	seen := func(e map[string]any) {
		if e["kind"] == "scale_death_forced" {
			forced++
		}
	}
	if _, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), seen); err != nil {
		t.Fatal(err)
	}
	if forced != 1 {
		t.Fatalf("the first run must be killable, forced = %d", forced)
	}
	// 重进副本：RunID 变了、entity 从 0x1000 重新发，Dead 表也是新的。
	run2 := scaleRunForTest()
	run2.RunID = "run-two"
	w.activeDungeon = run2
	if _, err := w.scaleStatus(scaleStatusPayload(scaleStatusSample, scalePrimerTemplate), seen); err != nil {
		t.Fatal(err)
	}
	if forced != 2 {
		t.Fatalf("a new run must be killable again, forced = %d", forced)
	}
	if !run2.Dead[4107] || !run2.Dead[4108] {
		t.Fatalf("both devices must die in the second run too, dead = %v", run2.Dead)
	}
}

// 没有收尾证据、也没有峰值证据时不许判死：光收到日志形状的包不够。
func TestScaleStatusNeedsEvidenceWithoutEndTrigger(t *testing.T) {
	w := &worldSession{scaleDeathFromHP: true, role: storage.Character{WireID: 3}, activeDungeon: scaleRunForTest()}
	var kinds []string
	seen := func(e map[string]any) { kinds = append(kinds, e["kind"].(string)) }
	if _, err := w.scaleStatus(scaleStatusPayload(scaleStatusSampleNoEnd, scalePrimerTemplate), seen); err != nil {
		t.Fatal(err)
	}
	for _, k := range kinds {
		if k == "scale_death_forced" {
			t.Fatalf("no evidence must not kill, events = %v", kinds)
		}
	}
	if len(w.activeDungeon.Dead) != 0 {
		t.Fatalf("nothing may die, dead = %v", w.activeDungeon.Dead)
	}
}
