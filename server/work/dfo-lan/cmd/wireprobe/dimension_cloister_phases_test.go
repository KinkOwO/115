package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
)

// 次元回廊三阶段帧组的契约测试。
//
//	CMD2043（开始作战）→ 只回 CMD2043 应答（倒计时/横幅/UI 由客户端演出）
//	CMD2080（点右上角 UI 选难度）→ 回同族操作窗应答（见 dimension_cloister_window_test.go）
//	CMD2045（带关卡号）→ Entry 组：整套进图帧列（N28 = 该关副本号）
//
// 这一组主要防「把加载挂回 CMD2043」（那样客户端会跳过倒计时与难度选择直接进图）。

func vectorIDs(vs []legion.DimCloisterVector) []uint16 {
	out := make([]uint16, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.ID)
	}
	return out
}

// TestDimCloisterWindowGroupHasNoDungeonLoad Window 组（官服窗口批次）里不许出现
// 进图帧列的锚点。
func TestDimCloisterWindowGroupHasNoDungeonLoad(t *testing.T) {
	window := legion.GetDimCloisterWindowVectors()
	if len(window) == 0 {
		t.Fatal("Window 组为空")
	}
	for _, v := range window {
		switch {
		case v.Kind == 0 && (v.ID == 27 || v.ID == 28 || v.ID == 29 || v.ID == 1584),
			v.Kind == 1 && v.ID == legion.CmdEnterDungeon:
			t.Fatalf("Window 组里出现了进图帧（op=%d kind=%d）", v.ID, v.Kind)
		}
	}
}

// TestDimCloisterSelectGroupCarriesCandidates 钉住抓包语料仍在（供对照分析用）。
func TestDimCloisterSelectGroupCarriesCandidates(t *testing.T) {
	sel := legion.GetDimCloisterSelectVectors()
	if len(sel) == 0 {
		t.Fatal("Select 组为空")
	}
	got := vectorIDs(sel)
	if !contains(got, 2080) {
		t.Fatalf("Select 组缺少官服 N2080 语料: %v", got)
	}
	if !contains(got, 1539) {
		t.Fatalf("Select 组缺少官服 N1539 语料: %v", got)
	}
}

// TestDimCloisterEntryGroupIsTheFullBatch 钉住 Entry 组的顺序与内容（官服进图批次）。
func TestDimCloisterEntryGroupIsTheFullBatch(t *testing.T) {
	want := []uint16{26, 781, 782, 27, 476, 1584, 28, 629, 29, 465, 475, 2045, 3, 390, 37, 1474}
	vectors, err := legion.GetDimCloisterEntryVectorsForStage(0)
	if err != nil {
		t.Fatal(err)
	}
	got := vectorIDs(vectors)
	if len(got) != len(want) {
		t.Fatalf("Entry 组帧数 %d, want %d\ngot=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Entry 组第 %d 帧 op=%d, want %d\ngot=%v", i, got[i], want[i], got)
		}
	}
}

// TestDimCloisterEntryVectorsUseTheStageOwnColumn 钉住**按副本号取整列**。
//
// ★ 旧测试（TestDimCloisterEntryVectorsPatchOnlyDungeonID）钉的是"三关只差 N28 首 4 字节"，
// 那个假设**被业主 2026-10-10 逐屏对照证伪**：只改 N28 ⇒ 名字/横幅变第4界，
// 而地图与 BOSS 仍是第1界（决定地图号与怪物的是 N29 等帧）。
// 现在要求：换副本号 = 换**那一轮的整列**，且除了 N28 自带的副本号外，
// 至少 N29（START_MAP，含地图号）在不同界之间**必须不同**。
func TestDimCloisterEntryVectorsUseTheStageOwnColumn(t *testing.T) {
	first, err := legion.GetDimCloisterEntryVectorsForDungeon(legion.DimCloisterStageDungeons[0])
	if err != nil {
		t.Fatal(err)
	}
	fourth, err := legion.GetDimCloisterEntryVectorsForDungeon(legion.DimCloisterStageDungeons[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(fourth) {
		t.Fatalf("两界帧数不一致: %d vs %d", len(first), len(fourth))
	}
	var n28, n29 []byte
	for i := range first {
		if first[i].ID != fourth[i].ID || first[i].Kind != fourth[i].Kind {
			t.Fatalf("第 %d 帧 op/kind 不一致: %d/%d vs %d/%d",
				i, first[i].ID, first[i].Kind, fourth[i].ID, fourth[i].Kind)
		}
		switch {
		case first[i].Kind == 0 && first[i].ID == 28:
			n28 = first[i].Body
			got := uint32(fourth[i].Body[0]) | uint32(fourth[i].Body[1])<<8 |
				uint32(fourth[i].Body[2])<<16 | uint32(fourth[i].Body[3])<<24
			if got != legion.DimCloisterStageDungeons[1] {
				t.Fatalf("第4界那列的 N28 副本号 %d, want %d", got, legion.DimCloisterStageDungeons[1])
			}
		case first[i].Kind == 0 && first[i].ID == 29:
			n29 = first[i].Body
			if string(first[i].Body) == string(fourth[i].Body) {
				t.Fatal("两界的 N29（START_MAP，含地图号）逐字节相同 —— 那就会重演" +
					"名字第4界、地图与 BOSS 第1界" + "的错位")
			}
		}
	}
	if len(n28) < 4 || len(n29) == 0 {
		t.Fatalf("没找到 N28/N29（len28=%d len29=%d）", len(n28), len(n29))
	}
	// 兜底分支（未知副本号）仍应可用：只改 N28 首 4 字节。
	fallback, err := legion.GetDimCloisterEntryVectorsForDungeon(100003180)
	if err != nil {
		t.Fatal(err)
	}
	if len(fallback) == 0 {
		t.Fatal("未知副本号没有兜底帧列")
	}
}

// TestDimCloisterStageDungeonsMatchCapture 三关副本号与抓包 N28 一致。
func TestDimCloisterStageDungeonsMatchCapture(t *testing.T) {
	want := []uint32{100003195, 100003190, 100003185}
	if len(legion.DimCloisterStageDungeons) != len(want) {
		t.Fatalf("阶段数 %d, want %d", len(legion.DimCloisterStageDungeons), len(want))
	}
	for i, id := range want {
		if legion.DimCloisterStageDungeons[i] != id {
			t.Fatalf("第 %d 关副本号 %d, want %d", i, legion.DimCloisterStageDungeons[i], id)
		}
	}
}

// TestDimCloisterStageGuard 越界关卡必须拒绝。
func TestDimCloisterStageGuard(t *testing.T) {
	if _, err := legion.GetDimCloisterEntryVectorsForStage(3); err == nil {
		t.Fatal("stage=3 未报错")
	}
	if _, err := legion.GetDimCloisterEntryVectorsForStage(-1); err == nil {
		t.Fatal("stage=-1 未报错")
	}
	for stage := 0; stage < len(legion.DimCloisterStageDungeons); stage++ {
		if _, err := legion.GetDimCloisterEntryVectorsForStage(stage); err != nil {
			t.Fatalf("stage=%d 合法却报错: %v", stage, err)
		}
	}
}

func contains(ids []uint16, want uint16) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestDimCloisterRewardEndPushesNextStageWindow 钉住清关后的重开窗：
// 官服 s30 第 1 界打完是 #821 N2316 → #824 N2314 @3=02 → #833 N2314 @3=06
// → 客户端再开一次难度窗 → CMD2045 选第 2 界。所以要回 ACK2046，且在其后
// 补「本界记录态 + 开窗列表态」两帧。
func TestDimCloisterRewardEndPushesNextStageWindow(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType, dimCloisterStage: 0}
	w := &worldSession{}
	w.role.ID = 9
	result, err := s.dimCloisterRewardEnd(w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 3 {
		t.Fatalf("帧数 %d, want 3（ACK2046 + 本界记录态 + 开窗态）", len(result.Packets))
	}
	if ack := result.Packets[0]; ack.Kind != 1 || ack.ID != legion.CmdRewardEnd {
		t.Fatalf("第 1 帧 = op%d/kind%d, want op%d/kind1", ack.ID, ack.Kind, legion.CmdRewardEnd)
	}
	if p := result.Packets[1]; p.Kind != 0 || p.ID != legion.NotiDimCloisterInfo || p.Payload[3] != 0x02 {
		t.Fatalf("第 2 帧 = op%d/kind%d/@3=%02x, want op%d/kind0/@3=02", p.ID, p.Kind, p.Payload[3], legion.NotiDimCloisterInfo)
	}
	if p := result.Packets[2]; p.Kind != 0 || p.ID != legion.NotiDimCloisterInfo || p.Payload[3] != 0x06 {
		t.Fatalf("第 3 帧 = op%d/kind%d/@3=%02x, want op%d/kind0/@3=06", p.ID, p.Kind, p.Payload[3], legion.NotiDimCloisterInfo)
	}
	if s.dimCloisterCleared != 1 {
		t.Fatalf("已清界数 %d, want 1", s.dimCloisterCleared)
	}
}

// TestDimCloisterFinalStageSendsVideoThenClose 钉住最后一界收尾：
// 官服 s30 第三界打完 #985 N2314 @3=03（放视频）→ #993 N2314 @3=05（关 UI）。
func TestDimCloisterFinalStageSendsVideoThenClose(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType, dimCloisterStage: 2}
	w := &worldSession{}
	w.role.ID = 9
	result, err := s.dimCloisterRewardEnd(w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 3 {
		t.Fatalf("帧数 %d, want 3（ACK2046 + 终局态 + 关闭态）", len(result.Packets))
	}
	if p := result.Packets[1]; p.Payload[3] != 0x03 {
		t.Fatalf("终局态 @3=%02x, want 03（客户端据此放视频）", p.Payload[3])
	}
	if p := result.Packets[2]; p.Payload[3] != 0x05 {
		t.Fatalf("收尾态 @3=%02x, want 05（右上角 UI 关闭）", p.Payload[3])
	}
}

// TestDimCloisterInfoBodiesAreOfficialBytes 钉住「整帧照抄」：状态帧必须与生成
// 文件里的官服原文逐字节相同，且长度是官服的 112B。
func TestDimCloisterInfoBodiesAreOfficialBytes(t *testing.T) {
	names := []string{
		legion.DimCloisterInfoHallInitial,
		legion.DimCloisterInfoHallOptions,
		legion.DimCloisterInfoHallPicked,
		legion.DimCloisterInfoFinale,
	}
	for _, name := range names {
		body, err := legion.DimCloisterInfoBody(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(body) != 112 {
			t.Fatalf("%s 长度 %d, want 112", name, len(body))
		}
		again, err := legion.DimCloisterInfoBody(name)
		if err != nil {
			t.Fatal(err)
		}
		if &body[0] == &again[0] {
			t.Fatalf("%s 返回的是共享切片，调用方会改到模板", name)
		}
	}
	if _, err := legion.DimCloisterInfoBody("noSuchState"); err == nil {
		t.Fatal("未知状态名没有报错")
	}
	if _, _, err := legion.DimCloisterDungeonInfoBody(3); err == nil {
		t.Fatal("第 4 界没有官服变体，却没有报错")
	}
}

// TestDimCloisterEntryLedgerStampsFreshTime 钉住账本里的时间戳按当前时间写：
// 官服九帧 N2254 里只有 @256..260 与每周标记位会变，时间戳照抄抓包值会过期。
func TestDimCloisterEntryLedgerStampsFreshTime(t *testing.T) {
	body, err := legion.DimCloisterEntryLedgerBody(0x0123456789)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 272 {
		t.Fatalf("账本长度 %d, want 272", len(body))
	}
	want := []byte{0x89, 0x67, 0x45, 0x23, 0x01}
	for i, b := range want {
		if body[256+i] != b {
			t.Fatalf("时间戳第 %d 字节 %02x, want %02x", i, body[256+i], b)
		}
	}
	other, err := legion.DimCloisterEntryLedgerBody(0)
	if err != nil {
		t.Fatal(err)
	}
	if other[256] != 0 || other[257] != 0 {
		t.Fatal("时间戳没有按参数覆盖")
	}
}

// TestDimCloisterLedgerMarksMatchOfficialShapes 钉住内容行旗标只用官服实测过的形态。
//
// 官服九帧 N2254 按 (行 65, 66, 67, 69 的 +5 字节) 归并只出现三种组合：
//
//	#246 (7f 7f 00 7f) / #752 (00 7f 00 00) / #822 等 (00 00 00 00)
//
// 2026-10-10 实机之所以要在意它：本仓先前把伊斯那一族的 N2254（@117=7f 的形态）
// 发给了次元回廊频道，客户端在开难度窗时 exit=0xC0000005。
func TestDimCloisterLedgerMarksMatchOfficialShapes(t *testing.T) {
	cases := []struct {
		name  string
		marks dimCloisterLedgerMarks
		want  [4]byte
	}{
		{"login #246", dimCloisterLedgerLogin, [4]byte{0x7f, 0x7f, 0x00, 0x7f}},
		{"start #752", dimCloisterLedgerStart, [4]byte{0x00, 0x7f, 0x00, 0x00}},
	}
	for _, tc := range cases {
		body, err := dimCloisterEntryLedgerAt(0, tc.marks)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := [4]byte{body[21], body[45], body[69], body[117]}
		if got != tc.want {
			t.Fatalf("%s 旗标 = %02x, want %02x", tc.name, got, tc.want)
		}
		if body[256] != 0 || body[257] != 0 {
			t.Fatalf("%s 时间戳没有被覆盖", tc.name)
		}
	}
}

// TestDimCloisterLoginLedgerOnlyForCloisterChannels 钉住登录期的账本分流：
// 只有次元回廊那两行频道（50/84）拿本内容的账本，其它频道一律不是（返回 false）。
//
// 2026-10-10 实机证据：崩溃会话的 events.jsonl 里次元回廊频道收到的两条都是
// `ispins_login_entry_character_info_sent` —— 也就是伊斯那一族的形态。
func TestDimCloisterLoginLedgerOnlyForCloisterChannels(t *testing.T) {
	for _, tc := range []struct {
		channelType uint32
		want        bool
	}{
		{legion.DimCloisterHallChannelType, true},
		{legion.DimCloisterChannelType, true},
		{81, false},  // 伊斯
		{119, false}, // 末世录
		{86, false},  // 苏醒之森
		{1, false},   // 普通城镇
	} {
		body, isCloister, err := dimCloisterLoginLedger(tc.channelType, 0)
		if err != nil {
			t.Fatalf("频道 %d: %v", tc.channelType, err)
		}
		if isCloister != tc.want {
			t.Fatalf("频道 %d: isCloister=%v, want %v", tc.channelType, isCloister, tc.want)
		}
		if !tc.want {
			if body != nil {
				t.Fatalf("频道 %d 不该发账本却给了 %d 字节", tc.channelType, len(body))
			}
			continue
		}
		if len(body) != 272 {
			t.Fatalf("频道 %d 账本长度 %d, want 272", tc.channelType, len(body))
		}
		// 登录形态 = 官服 #246 那组旗标。
		if got := [4]byte{body[21], body[45], body[69], body[117]}; got != [4]byte{dimCloisterLedgerLogin[0], dimCloisterLedgerLogin[1], dimCloisterLedgerLogin[2], dimCloisterLedgerLogin[3]} {
			t.Fatalf("频道 %d 登录旗标 = %02x, want %02x", tc.channelType, got, dimCloisterLedgerLogin)
		}
	}
}

// TestDimCloisterOfficialPartyInfoPatchesName 钉住官服 #759 那帧 NOTI9 的取用：
// 正文长度必须是官服的 176B，队名按单字节写在 @18，容量/队伍类型保持官服值。
func TestDimCloisterOfficialPartyInfoPatchesName(t *testing.T) {
	body, err := legion.DimCloisterOfficialPartyInfo("55555")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 176 {
		t.Fatalf("官服 N9 正文长度 %d, want 176", len(body))
	}
	if got := string(body[18:22]); got != "5555" {
		t.Fatalf("队名 @18..21 = %q, want \"5555\"（只放得下 4 个单字节字符）", got)
	}
	// @22 起是容量 u32 = 4；@31 是官服那帧原文里就带的 0x0d（次元回廊队伍类型）。
	if capacity := binary.LittleEndian.Uint32(body[22:]); capacity != 4 {
		t.Fatalf("容量 @22 = %d, want 4", capacity)
	}
	if body[31] != 0x0d {
		t.Fatalf("队伍类型 @31 = %02x, want 0d（官服 #759 原文值）", body[31])
	}
	// 不得与共享向量共用底层数组：改一次不能污染下一次。
	body[18] = 'X'
	again, err := legion.DimCloisterOfficialPartyInfo("55555")
	if err != nil {
		t.Fatal(err)
	}
	if again[18] != '5' {
		t.Fatal("官服 N9 向量被调用方改到了（没有拷贝）")
	}
}

// TestDimCloisterLoginSendsOwnLedgerNotIspins 端到端钉住登录期分流：
// 在次元回廊频道（84）调用 ispinsPostSelection，必须发出**本内容的 N2254**
// （272B、旗标 = #246 形态），不得再走伊斯那一条。
//
// 2026-10-10 实机证据（这是「点开 UI 就 exit=0xC0000005」的根因）：
// 崩溃会话 events.jsonl 里 18:21:08 / 18:21:13 各有一条
// `ispins_login_entry_character_info_sent`，也就是把伊斯的账本发给了次元回廊频道。
func TestDimCloisterLoginSendsOwnLedgerNotIspins(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	var (
		mu     sync.Mutex
		kinds  []string
		record = func(e map[string]any) {
			mu.Lock()
			defer mu.Unlock()
			if k, ok := e["kind"].(string); ok {
				kinds = append(kinds, k)
			}
		}
	)
	c := &gameConnection{
		gatewayRuntime:      &gatewayRuntime{},
		bootstrapped:        true,
		selectedCharacterID: 7,
		keys:                make([]byte, wire.SessionKeyBytes),
		worldState:          &worldSession{channelType: legion.DimCloisterHallChannelType, role: database.Character{ID: 7, WireID: 7}},
		event:               record,
	}
	c.output = newConnectionOutput(server, c.keys, "test", record)

	done := make(chan struct{})
	go func() {
		c.ispinsPostSelection(7)
		close(done)
	}()

	// 1.1 秒的登录延迟之后应有两帧：N2254 与 N781（周本无限难度）。
	readFrame := func() (uint16, []byte) {
		t.Helper()
		peer.SetReadDeadline(time.Now().Add(6 * time.Second))
		header := make([]byte, 16)
		if _, err := io.ReadFull(peer, header); err != nil {
			t.Fatalf("没有收到登录期帧: %v", err)
		}
		size := int(binary.LittleEndian.Uint32(header[3:7]))
		body := make([]byte, size-16)
		if _, err := io.ReadFull(peer, body); err != nil {
			t.Fatal(err)
		}
		id := binary.LittleEndian.Uint16(header[1:3])
		plain, err := wire.DecryptPayload(c.keys, id, body)
		if err != nil {
			t.Fatal(err)
		}
		return id, plain
	}

	id, plain := readFrame()
	if id != legion.NotiEntryCharacterInfo {
		t.Fatalf("登录期第一帧 op=%d, want %d（N2254）", id, legion.NotiEntryCharacterInfo)
	}
	if len(plain) != 272 {
		t.Fatalf("账本长度 %d, want 272", len(plain))
	}
	// 登录形态 = 官服 #246 那组旗标；伊斯形态会带 @117=7f 之外的值。
	got := [4]byte{plain[21], plain[45], plain[69], plain[117]}
	want := [4]byte{dimCloisterLedgerLogin[0], dimCloisterLedgerLogin[1], dimCloisterLedgerLogin[2], dimCloisterLedgerLogin[3]}
	if got != want {
		t.Fatalf("登录账本旗标 = %02x, want %02x（伊斯形态的标志是 @21/@117 不同）", got, want)
	}
	// 第二帧是周本「无限难度」（N781），与伊斯同窗；第三帧是 N782。
	if id2, _ := readFrame(); id2 != 781 {
		t.Fatalf("登录期第二帧 op=%d, want 781", id2)
	}
	if id3, _ := readFrame(); id3 != 782 {
		t.Fatalf("登录期第三帧 op=%d, want 782", id3)
	}
	// 三帧读完就解除读超时，别再让后续读超时打断（连接会在 defer 里关）。
	peer.SetReadDeadline(time.Time{})

	<-done
	mu.Lock()
	collected := append([]string(nil), kinds...)
	mu.Unlock()
	for _, k := range collected {
		if k == "ispins_login_entry_character_info_sent" {
			t.Fatalf("次元回廊频道仍然发了伊斯的入场账本: %v", collected)
		}
	}
	found := false
	for _, k := range collected {
		if k == "dim_cloister_entry_ledger_sent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("没有记录 dim_cloister_entry_ledger_sent: %v", collected)
	}
}

// TestDimCloisterLoginFallsThroughForIspinsChannel 反向护栏：
// 伊斯频道（81）仍走原来的分支（挂 pending，不在这里发账本）。
func TestDimCloisterLoginFallsThroughForIspinsChannel(t *testing.T) {
	c := &gameConnection{
		gatewayRuntime:      &gatewayRuntime{},
		bootstrapped:        true,
		selectedCharacterID: 7,
		keys:                make([]byte, wire.SessionKeyBytes),
		worldState:          &worldSession{channelType: 81, role: database.Character{ID: 7, WireID: 7}},
		event:               func(map[string]any) {},
	}
	c.ispinsPostSelection(7)
	if !c.worldState.pendingLegionEntryInfo {
		t.Fatal("伊斯频道没有挂起 pending 账本")
	}
}

// TestDimCloisterLoopWalksEveryStage 走一遍三界的循环状态机：
// 前两界打完推「本界记录态 + 开窗态」，已清界数逐界 +1，而且每一界的记录态
// 必须是**官服对应那一帧**（不是复用同一帧）；最后一界换成终局链。
func TestDimCloisterLoopWalksEveryStage(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 77

	last := len(legion.DimCloisterStageDungeons) - 1
	bodies := make([][]byte, len(legion.DimCloisterStageDungeons))
	for stage := 0; stage <= last; stage++ {
		s.dimCloisterStage = stage
		result, err := s.dimCloisterRewardEnd(w, nil)
		if err != nil {
			t.Fatalf("第 %d 界结算: %v", stage, err)
		}
		if s.dimCloisterCleared != stage+1 {
			t.Fatalf("第 %d 界打完，已清界数 %d, want %d", stage, s.dimCloisterCleared, stage+1)
		}
		// 三帧：ACK2046 + 记录/终局态 + 开窗/关闭态。
		if len(result.Packets) != 3 {
			t.Fatalf("第 %d 界帧数 %d, want 3", stage, len(result.Packets))
		}
		if result.Packets[0].ID != legion.CmdRewardEnd || result.Packets[0].Kind != 1 {
			t.Fatalf("第 %d 界首帧不是 ACK2046: %+v", stage, result.Packets[0])
		}
		record := result.Packets[1]
		if record.Kind != 0 || record.ID != legion.NotiDimCloisterInfo {
			t.Fatalf("第 %d 界记录帧 op/kind = %d/%d", stage, record.ID, record.Kind)
		}
		bodies[stage] = append([]byte(nil), record.Payload...)
		if stage < last {
			// 非终局：记录态必须是官服那一界的原文，**记忆记录区一律清零**
			// —— 客户端 `MyresDimensionCloister.etc` 的 `[string data]` 全空，
			// 任何非零记录都会让客户端崩在文本构造里（第五/六轮实机，exit=0xC0000005）。
			//
			// ★ 第二十二轮：第 N 界清关后发的是"已清界数"当下标的那一帧
			// （官服：第1界清完发 dungeon0/@11=01，第2界清完发 dungeon1/@11=02；
			//  本仓下标 = 已清界数，第 0 界清完 cleared=1 ⇒ dungeon1）。
			// 传 stage 本身等于把进度原地重写一遍，客户端会认为本界没打过 ⇒ 选不了下一界。
			infoStage := stage + 1
			if infoStage >= len(legion.DimCloisterStageDungeons) {
				infoStage = len(legion.DimCloisterStageDungeons) - 1
			}
			want, _, err := legion.DimCloisterDungeonInfoBody(infoStage)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(record.Payload, dimCloisterApplyMemoryRecords(want, nil)) {
				t.Fatalf("第 %d 界记录态不是官服原文（记忆记录清零后）", stage)
			}
			for i := dimCloisterMemoryRecordFrom; i < dimCloisterMemoryRecordTo; i++ {
				if record.Payload[i] != 0 {
					t.Fatalf("第 %d 界记录态 @%d = %#x，客户端数据缺口未补前必须全 0",
						stage, i, record.Payload[i])
				}
			}
			if options := result.Packets[2]; options.Payload[3] != 0x06 {
				t.Fatalf("第 %d 界开窗态 @3=%02x, want 06", stage, options.Payload[3])
			}
			if stage > 0 && bytes.Equal(bodies[stage], bodies[stage-1]) {
				t.Fatalf("第 %d 界记录态与上一界逐字节相同（应各用官服那一帧）", stage)
			}
			continue
		}
		// 终局：@3=03 放视频 → @3=05 关闭右上角 UI。
		if record.Payload[3] != 0x03 {
			t.Fatalf("终局态 @3=%02x, want 03", record.Payload[3])
		}
		if closing := result.Packets[2]; closing.Payload[3] != 0x05 {
			t.Fatalf("收尾态 @3=%02x, want 05", closing.Payload[3])
		}
		if bytes.Equal(bodies[last], bodies[0]) {
			t.Fatal("终局态与第 1 界记录态逐字节相同（应各用官服那一帧）")
		}
	}
}

// TestDimCloisterStartRefusesWhileInDungeon 已有副本会话时开战必须明确拒绝，
// 不能静默吞掉（否则玩家会觉得「点了没反应」）。
func TestDimCloisterStartRefusesWhileInDungeon(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 5
	w.activeDungeon = &dungeon.Session{}
	_, _, err := s.dimCloisterStart(w, mustHex(t, officialCmd2043))
	if err == nil {
		t.Fatal("副本内开战没有被拒绝")
	}
}

// TestDimCloisterLoadStageRefusesTwice 连续两次 CMD2045 必须拒第二次
// （否则会在已有会话上再建一个副本）。
func TestDimCloisterLoadStageRefusesTwice(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 5
	w.activeDungeon = &dungeon.Session{}
	if _, err := s.dimCloisterLoadStage(w, mustHex(t, officialCmd2045)); err == nil {
		t.Fatal("已有副本会话时进图没有被拒绝")
	}
}
