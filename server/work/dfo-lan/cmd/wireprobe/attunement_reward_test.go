package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
)

// noti 2859 的三个字段在源里**还没有语义结论**（见 attunement_reward.go 文件头），
// 所以这一组测试钉的不是「值该是多少」，而是**注入通道本身的形状**：
// 长度必须正好 12 字节、三种写法等价、默认一个字节都不发、开口后严格贴在 2838 后面。

func TestParseAttunementRewardAcceptsHexAndDecimals(t *testing.T) {
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint32(raw[0:4], 44)
	binary.LittleEndian.PutUint32(raw[4:8], 43)
	binary.LittleEndian.PutUint32(raw[8:12], 0)

	for name, spec := range map[string]string{
		"紧凑十六进制":  hex.EncodeToString(raw),
		"带空格十六进制": "2c000000 2b000000 00000000",
		"带逗号十六进制": "2c000000,2b000000,00000000",
		"带 0x 前缀": "0x2c0000002b00000000000000",
		"三个十进制":   "44,43,0",
		"十进制带空格":  "44, 43, 0",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseAttunementReward(spec)
			if err != nil {
				t.Fatalf("parse(%q): %v", spec, err)
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("parse(%q) = %x, want %x", spec, got, raw)
			}
		})
	}
}

// 空值 = 不发。默认一个字节都不发是有意的：三个字段的语义没定，
// 写错档位会让客户端的珠子演出与掉落实质性不符，比不发更糟。
// 模块 init 里 `+88` 的初值是 -1（全 F）—— 试值时要能照着写。
func TestParseAttunementRewardAcceptsTheNegativeSentinel(t *testing.T) {
	got, err := parseAttunementReward("72,72,-1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []byte{72, 0, 0, 0, 72, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(got, want) {
		t.Fatalf("= %x, want %x（-1 按二补数写成全 F）", got, want)
	}
	// 超出 u32 的值必须拒掉，而不是静默截断。
	for _, spec := range []string{"4294967296,0,0", "-2147483649,0,0"} {
		if _, err := parseAttunementReward(spec); err == nil {
			t.Fatalf("parse(%q) 应当报错", spec)
		}
	}
}

func TestParseAttunementRewardEmptyMeansNoInjection(t *testing.T) {
	for _, spec := range []string{"", "   ", "\t"} {
		got, err := parseAttunementReward(spec)
		if err != nil {
			t.Fatalf("parse(%q) 不该报错：%v", spec, err)
		}
		if got != nil {
			t.Fatalf("parse(%q) = %x, want nil（留空 = 不发）", spec, got)
		}
	}
}

// 长度必须是 12：解析器是 `sub_146EA0BE0(&v1, 12)` 一次定长读满
// （2838 是同一个形状的 8 字节，那条长度护栏 2026-09-27 用 2839 撞过）。
// 写成 8 / 16 / 只有两个字段一律拒绝，不给客户端留错位的半包。
func TestParseAttunementRewardRejectsWrongLength(t *testing.T) {
	for name, spec := range map[string]string{
		"两个字段":      "44,43",
		"四个字段":      "44,43,0,1",
		"8 字节十六进制":  "2c0000002b000000",
		"16 字节十六进制": "2c0000002b0000000000000000000000",
		"非数字":       "a,b,c",
		"溢出 u32":    "4294967296,0,0",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAttunementReward(spec); err == nil {
				t.Fatalf("parse(%q) 应当报错", spec)
			}
		})
	}
}

// 日志里要把三个数按位移念出来，并标注它是不是客户端认识的八档之一 ——
// 实机试值时这张「值 → 名字」的对照就是唯一的现场线索。
func TestDescribeAttunementRewardNamesTheTiers(t *testing.T) {
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint32(raw[0:4], 44)
	binary.LittleEndian.PutUint32(raw[4:8], 43)
	binary.LittleEndian.PutUint32(raw[8:12], 0)
	text := describeAttunementReward(raw)
	for _, want := range []string{"[0]=44(epic)", "[1]=43(legendary)", "[2]=0(not-a-client-tier)", hex.EncodeToString(raw)} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Fatalf("日志 %q 缺少 %q", text, want)
		}
	}
}

// 注入必须严格贴在 2838 后面，而且**只在显式给值时**出现。
// 2838 本身是常驻状态（非调律副本也要下发 normal/normal），不能被这条诊断顶掉。
func TestOathInfoPacketsAppendsTheAttunementRewardOnlyWhenInjected(t *testing.T) {
	w := &worldSession{}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("默认计划 = %+v，应当只有 2838", plan)
	}

	want := make([]byte, 12)
	binary.LittleEndian.PutUint32(want[0:4], 45)
	binary.LittleEndian.PutUint32(want[4:8], 45)
	binary.LittleEndian.PutUint32(want[8:12], 45)
	w.attunementReward = "45,45,45"
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("注入后计划 = %d 项，want 2（2838 + 2859）", len(plan))
	}
	if plan[0].ID != 2838 || plan[1].ID != attunementRewardPacketID {
		t.Fatalf("顺序 = %d,%d，want 2838 在前、2859 在后", plan[0].ID, plan[1].ID)
	}
	if !bytes.Equal(plan[1].Payload, want) {
		t.Fatalf("2859 载荷 = %x, want %x（必须逐字节原样，不做任何加工）", plan[1].Payload, want)
	}
	if plan[1].Name != "attunement_reward" {
		t.Fatalf("name = %q，日志里要能一眼认出这是注入的那一帧", plan[1].Name)
	}
}

// 试值要靠 `@文件` 才能不重启换值：**每次进本重新解析**，改完存盘下一次进本就生效。
// 这条测试直接钉住「两次解析之间文件被改了，第二次必须拿到新值」。
func TestAttunementRewardSpecReadsTheFileOnEveryEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(path, []byte("44,43,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := "@" + path

	w := &worldSession{attunementReward: spec}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || !bytes.Equal(plan[1].Payload, []byte{44, 0, 0, 0, 43, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("第一次进本载荷 = %+v", plan)
	}

	// 存盘换值 —— 不重启服务端，下一次进本必须拿到新值。
	if err := os.WriteFile(path, []byte("# 试第二个值\n45,45,45\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || !bytes.Equal(plan[1].Payload, []byte{45, 0, 0, 0, 45, 0, 0, 0, 45, 0, 0, 0}) {
		t.Fatalf("改文件后载荷 = %+v（没有重读？）", plan)
	}

	// 空文件 = 不发：这样「关掉注入」也不用重启。
	if err := os.WriteFile(path, []byte("\n# 只留注释\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("空文件时应当只剩 2838：%+v", plan)
	}
}

// 试值期间把文件改坏，代价不该是「这局进不去」：进本时解析失败只记一笔并跳过。
func TestAttunementRewardSkipsABrokenTrialFileInsteadOfFailingTheEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(path, []byte("这不是载荷\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &worldSession{attunementReward: "@" + path}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatalf("进本不该因为试值文件写坏而失败：%v", err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("坏文件时应当只发 2838：%+v", plan)
	}
}

// 但**启动期**必须挡住写错的路径/内容（1 秒内失败，而不是跑完 48 秒 PVF 准备）——
// 这条同时钉住「缺文件也会早期失败」，否则试值时会以为注入生效了。
func TestAttunementRewardSpecRejectsAMissingTrialFile(t *testing.T) {
	cfg := Config{AttunementReward: "@" + filepath.Join(t.TempDir(), "nope.txt")}
	err := cfg.validate()
	if err == nil {
		t.Fatal("指向不存在的文件必须在 validate 就被拒")
	}
	if !strings.Contains(err.Error(), "attunement-reward") {
		t.Fatalf("错误要写明是哪个开关：%v", err)
	}
}

// 正常接线（没有诊断覆盖）时，2859 必须按**本场掷出的档位**发，
// 而且**动画与掉落同档**：payload[0] = 该档在客户端演出表的格。
//
// 档位取两条线的**高**者（业主口径：保底之上掷到更高就用更高那档，
// 「至少出对应动画」对两条线都要成立），并记进日志便于对着掉落验收。
//
// ⚠️ 直接测 attunementRewardPackets：oathInfoPacketsCore 会用**真实奖励表**
// 重掷档位并覆盖 w.attunementRunTiers（见 oath_info.go），零值世界下会把这里
// 注入的档位抹成 0。生产路径的顺序是 core 先算、这里后发，所以这条单测对准的是后者。
func TestAttunementRewardFollowsTheRunGrade(t *testing.T) {
	cases := []struct {
		name         string
		primer, oath uint32
		wantSlot     uint32
		wantNone     bool
	}{
		{"传说砝码掷到传说", 43, 40, 41, false},        // LegendaryDrop
		{"誓约更高也不改演出（只按珠子）", 43, 44, 41, false}, // 珠子=传说 → LegendaryDrop
		{"史诗砝码掷到史诗", 44, 40, 42, false},        // EpicDrop
		{"掷到太初", 45, 45, 43, false},            // PrimevalDrop
		{"小深渊掷到普通", 40, 40, 0, true},           // 演出表没有 normal 的格 ⇒ 不发
		{"没有档位（非调律副本）", 0, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &worldSession{activeDungeon: boundaryDungeon(), attunementRunTiers: loot.RunTiers{Primer: c.primer, Oath: c.oath}}
			plan := w.attunementRewardPackets()
			if c.wantNone {
				if len(plan) != 0 {
					t.Fatalf("应当不发：%+v", plan)
				}
				return
			}
			if len(plan) != 1 || plan[0].ID != attunementRewardPacketID {
				t.Fatalf("应当发一帧 2859：%+v", plan)
			}
			p := plan[0].Payload
			if len(p) != attunementRewardPayloadSize {
				t.Fatalf("载荷长度 = %d，want %d", len(p), attunementRewardPayloadSize)
			}
			if got := binary.LittleEndian.Uint32(p[0:4]); got != c.wantSlot {
				t.Fatalf("演出格 = %d(%s)，want %d", got, loot.AnimationSlot(got), c.wantSlot)
			}
			// 三格必须**同值且都在演出表的域内**：实测任何一格越界（例如填模块初值 72）
			// 都会让整帧退化成「最后一格兜底」（业主看到太初）。
			// 填同一个值也让「取哪一格」这个问题不再重要。
			for i := 1; i < 3; i++ {
				if got := binary.LittleEndian.Uint32(p[i*4:]); got != c.wantSlot {
					t.Fatalf("位移 %d = %d，want 与第 1 格同值 %d（越界会让整帧兜底）", i, got, c.wantSlot)
				}
			}
		})
	}
}

// 诊断覆盖必须**整帧优先**（继续试值用），不能被正常接线顶掉。
func TestDiagnosticOverrideWinsOverTheRunGrade(t *testing.T) {
	w := &worldSession{
		attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43},
		attunementReward:   "45,45,45",
	}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[1].Payload[0:4]); got != 45 {
		t.Fatalf("诊断值必须原样发出，得到 %d", got)
	}
}

// boundaryDungeon 造一个「调律之边界」的假副本（只需要 [dungeon type]）。
// 门禁按源声明分流，所以测试也必须给出源声明，而不是省掉它。
func boundaryDungeon() *dungeon.Session {
	return &dungeon.Session{Definition: catalog.DungeonDefinition{
		Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[dungeon type]"},
			{Type: 8, Reference: "boundary of attunement"},
		}},
	}}
}

// smallAbyssDungeon 是**另一套玩法**（终末之边界）：2859 不许越界发过去。
func smallAbyssDungeon() *dungeon.Session {
	return &dungeon.Session{Definition: catalog.DungeonDefinition{
		Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[dungeon type]"},
			{Type: 8, Reference: "endkeeper of order"},
		}},
	}}
}

// 门禁：另一套玩法（终末之边界 / 小深渊）**不许**收到这条包 —— 它的档位 74% 落在
// 演出表之外（normal/rare），越界发送只会把它的表现改成「兜底太初」。
// 判据取副本自己的 [dungeon type]，不按副本号硬编。
func TestAttunementRewardIsGatedToTheBoundaryOfAttunementMode(t *testing.T) {
	small := &worldSession{activeDungeon: smallAbyssDungeon(), attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43}}
	if plan := small.attunementRewardPackets(); len(plan) != 0 {
		t.Fatalf("小深渊不该收到 2859：%+v", plan)
	}
	// 没有活动副本（不在副本上下文里）同样不发。
	none := &worldSession{attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43}}
	if plan := none.attunementRewardPackets(); len(plan) != 0 {
		t.Fatalf("没有活动副本时不该发：%+v", plan)
	}
	// 诊断覆盖**不受门禁限制**（试值要在任何副本里都能做）。
	probe := &worldSession{activeDungeon: smallAbyssDungeon(), attunementReward: "42,42,42"}
	plan := probe.attunementRewardPackets()
	if len(plan) != 1 || plan[0].ID != attunementRewardPacketID {
		t.Fatalf("诊断覆盖应当不受门禁限制：%+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[0].Payload[0:4]); got != 42 {
		t.Fatalf("诊断载荷被改动了：%d", got)
	}
}
