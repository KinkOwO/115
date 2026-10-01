package main

import (
	"dfolan/internal/inventory"
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

// 中间四档的随机：业主 2026-10-01 拍板「按稀有度递减」。
// 守四件事：① 不越界；② primeval(45) 只能来自保底分支；③ 大样本下占比与权重对齐；
// ④ 权重确实严格递减。
func TestRollOathGradeFollowsTheDecliningWeights(t *testing.T) {
	for _, w := range oathGradeWeights {
		if _, ok := oathGradeTiers[w.Grade]; !ok {
			t.Fatalf("weight table names %d, outside the eight tiers the script accepts", w.Grade)
		}
		if w.Grade == oathGradePrimeval {
			t.Fatal("primeval must stay pity-only and never join the random table")
		}
	}
	for i := 1; i < len(oathGradeWeights); i++ {
		if oathGradeWeights[i-1].Weight <= oathGradeWeights[i].Weight {
			t.Fatalf("weights are not strictly declining at index %d", i)
		}
	}
	if got := rollOathGrade(nil); got != inventory.OathGradeNormal {
		t.Fatalf("nil rng = %d, want normal(%d)", got, inventory.OathGradeNormal)
	}
	const draws = 200000
	rng := rand.New(rand.NewSource(20261001))
	count := map[uint16]int{}
	total := 0
	for _, w := range oathGradeWeights {
		total += w.Weight
	}
	for i := 0; i < draws; i++ {
		g := rollOathGrade(rng)
		if _, ok := oathGradeTiers[g]; !ok {
			t.Fatalf("draw %d produced %d, outside the eight tiers", i, g)
		}
		if g == oathGradePrimeval {
			t.Fatalf("draw %d produced primeval(%d) from the random table", i, g)
		}
		count[g]++
	}
	for _, w := range oathGradeWeights {
		want := float64(w.Weight) / float64(total)
		got := float64(count[w.Grade]) / float64(draws)
		if math.Abs(got-want) > want*0.15 {
			t.Fatalf("grade %d share = %.4f, want %.4f (within 15%%)", w.Grade, got, want)
		}
	}
}

// 保底档位的纯决策：到期给 45（唯一召唤奥尔泰尔的档），否则两边 normal。
func TestOathGradesForPity(t *testing.T) {
	primer, oath := oathGradesForPity(true, nil)
	if _, ok := oathGradeTiers[oath]; !ok {
		t.Fatalf("oath %d is outside the eight tiers the script accepts", oath)
	}
	// 45 是第四档「太初」，也是 nox_index_checker 里唯一选奥尔泰尔的值
	// （docs/protocol/endkeeper-of-order-primer-20260926.md §32.1）。
	if oath != 45 {
		t.Fatalf("the hidden boss needs oath == 45 exactly, got %d", oath)
	}
	if primer != 40 {
		t.Fatalf("primer = %d, want 40 (the watcher branch needs primer 45; we never trigger it)", primer)
	}

	primer, oath = oathGradesForPity(false, nil)
	if primer != 40 || oath != 40 {
		t.Fatalf("not due = %d/%d, want 40/40", primer, oath)
	}
}

// 保底只在配置的副本上生效；保底关闭（或没有存储服务）时一律不到期，也就不碰数据库。
func TestOathProgressScope(t *testing.T) {
	w := &worldSession{}
	if w.oathProgressEnabled(100005014) {
		t.Fatal("a zero session must not enable any dungeon")
	}
	if due, err := w.oathProgressDue(100005014); err != nil || due {
		t.Fatalf("no pity configured: due=%v err=%v", due, err)
	}
	if w.oathProgressDungeon() != 0 {
		t.Fatal("no active dungeon must report 0")
	}

	w = &worldSession{oathProgressClears: 5, oathProgressDungeons: map[uint32]bool{100005014: true}}
	if !w.oathProgressEnabled(100005014) {
		t.Fatal("the configured dungeon must be enabled")
	}
	if w.oathProgressEnabled(100005066) {
		t.Fatal("only the configured dungeon counts")
	}
	// service 为 nil 时读不到计数 —— 这条路必须退化成「不到期」而不是崩。
	// 真机装配后 service 恒非 nil，这里只是把 nil 路径钉住。
	if due, err := w.oathProgressDue(100005014); err != nil || due {
		t.Fatalf("nil service must degrade to not-due, got due=%v err=%v", due, err)
	}
}

// 副本号解析：逗号分隔、容忍空白与空项、拒绝 0 与非数字。
func TestParseOathProgressDungeons(t *testing.T) {
	got, err := parseOathProgressDungeons(" 100005014 , 100005066 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[100005014] || !got[100005066] {
		t.Fatalf("got %v", got)
	}
	// 尾随逗号只是空项，不该让启动失败。
	if one, err := parseOathProgressDungeons("100005014,"); err != nil || len(one) != 1 {
		t.Fatalf("trailing comma = %v err=%v, want one dungeon", one, err)
	}
	if empty, err := parseOathProgressDungeons(""); err != nil || len(empty) != 0 {
		t.Fatalf("empty spec = %v err=%v, want an empty set", empty, err)
	}
	// 0 是「没有副本」，不是合法键；发出去等于把保底挂到不存在的地图上。
	for _, bad := range []string{"0", "100005014,0", "x", "100005014,x", "-1"} {
		if _, err := parseOathProgressDungeons(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

// 默认值必须自洽：阈值 > 0，默认副本就是小深渊，且 45 在脚本接受的八档里。
func TestOathProgressDefaults(t *testing.T) {
	if oathDefaultProgressClears <= 0 {
		t.Fatalf("default pity = %d, must be positive", oathDefaultProgressClears)
	}
	set, err := parseOathProgressDungeons(oathDefaultProgressDungeons)
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 1 || !set[100005014] {
		t.Fatalf("default dungeons = %v, want exactly the small abyss 100005014", set)
	}
	if _, ok := oathGradeTiers[oathGradePrimeval]; !ok {
		t.Fatalf("pity tier %d is outside the eight tiers", oathGradePrimeval)
	}
}

// 端到端：保底到期时，2838 载荷的 [4:8) 必须正好是 45。
func TestOathInfoPayloadCarriesThePityTier(t *testing.T) {
	primer, oath := oathGradesForPity(true, nil)
	p := oathInfoPayload(primer, oath)
	if len(p) != 8 {
		t.Fatalf("payload = %d bytes, want 8", len(p))
	}
	if got := binary.LittleEndian.Uint32(p[0:4]); got != 40 {
		t.Fatalf("primer field = %d, want 40", got)
	}
	if got := binary.LittleEndian.Uint32(p[4:8]); got != 45 {
		t.Fatalf("oath field = %d, want 45 (the only tier that summons Orthaire)", got)
	}
}
