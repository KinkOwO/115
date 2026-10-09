package main

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
)

func forestHardEnterRequest(content uint32, stage uint32) []byte {
	body := make([]byte, legion.EnvelopeSize+8)
	binary.LittleEndian.PutUint32(body[legion.EnvelopeSize:], content)
	binary.LittleEndian.PutUint32(body[legion.EnvelopeSize+4:], stage)
	return body
}

// Extreme 开战：官服 16B ACK + **N2565 等待态**（不是 Normal 的 N2563），
// 且 run 必须落成 hard。
func TestForestHardStartUsesOfficialWaitingVector(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 7, WireID: 7}, soloPartyReady: true}
	plan, notes, err := w.startForest(forestHardEnterRequest(legion.ForestHardContentID, 0), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("hard start plan = %d frames, want ACK + waiting", len(plan))
	}
	if plan[0].ID != legion.CmdStart || !bytes.Equal(plan[0].Payload, legion.ForestHardStartAck()) {
		t.Fatalf("hard start ack = id %d %x", plan[0].ID, plan[0].Payload)
	}
	if plan[1].ID != legion.NotiForestHardInfo || !bytes.Equal(plan[1].Payload, legion.ForestHardWaitingInfo()) {
		t.Fatalf("hard waiting state = id %d %x", plan[1].ID, plan[1].Payload)
	}
	if w.forest == nil || !w.forest.hard {
		t.Fatal("hard start did not arm a hard run")
	}
	if len(notes) != 1 || notes[0]["hard"] != true {
		t.Fatalf("hard start notes = %v", notes)
	}
	// Normal 开战仍然是 N2563 等待态（回归护栏）。
	normal := &worldSession{role: database.Character{ID: 7, WireID: 7}, soloPartyReady: true}
	plan, _, err = normal.startForest(forestHardEnterRequest(legion.ForestContentID, 0), false)
	if err != nil {
		t.Fatal(err)
	}
	if plan[1].ID != legion.NotiForestInfo || !bytes.Equal(plan[1].Payload, legion.ForestWaitingInfo()) {
		t.Fatalf("normal waiting state = id %d %x", plan[1].ID, plan[1].Payload)
	}
}

// 内容 105 的 CMD2045/2046 必须被硬模式家族接手，内容 104 只走 Normal。
func TestForestHardFamilyRequestRouting(t *testing.T) {
	hard := &worldSession{forest: &forestRun{hard: true}}
	if !hard.isForestHardFamilyRequest(legion.CmdEnterDungeon, forestHardEnterRequest(legion.ForestHardContentID, 0)) {
		t.Fatal("content 105 CMD2045 must be claimed by the hard run")
	}
	if hard.isForestHardFamilyRequest(legion.CmdEnterDungeon, forestHardEnterRequest(legion.ForestContentID, 0)) {
		t.Fatal("content 104 must not be claimed by the hard family matcher")
	}
	if hard.isForestHardFamilyRequest(legion.CmdStart, forestHardEnterRequest(legion.ForestHardContentID, 0)) {
		t.Fatal("CMD2043 has its own routing branch")
	}
	normal := &worldSession{forest: &forestRun{}}
	if normal.isForestHardFamilyRequest(legion.CmdRewardEnd, forestHardEnterRequest(legion.ForestHardContentID, 2)) {
		t.Fatal("a Normal run must not claim content 105")
	}
}

// 非终点关清关：官服**有** N31 横幅（22:06:06.828），没有翻牌链。
func TestForestHardNonTerminalClearSendsBannerOnly(t *testing.T) {
	s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.ForestHardStageDungeons[0]}}
	s.MarkCompleted()
	w := &worldSession{
		role:          database.Character{ID: 7, WireID: 7},
		activeDungeon: s,
		forest:        &forestRun{hard: true},
	}
	plan, err := w.completeForestStage()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 31 {
		t.Fatalf("hard stage 0 clear plan = %+v, want the N31 banner only", plan)
	}
	banner, err := legion.ForestHardStageClearEnabled(0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plan[0].Payload, banner) {
		t.Fatalf("banner = %x, want the Extreme stage 0 vector %x", plan[0].Payload, banner)
	}
	if !w.forest.cleared[0] {
		t.Fatal("stage 0 was not marked cleared")
	}
}

// 终点关清关：官服顺序 N2566 → N31 → N2252 → N2253 → N9；N2252 用首关 token？
// 不 —— 用**本关（第三关）**的 Extreme token。
func TestForestHardTerminalClearSendsFullChain(t *testing.T) {
	s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.ForestHardStageDungeons[2]}}
	s.MarkCompleted()
	w := &worldSession{
		role:          database.Character{ID: 7, WireID: 7},
		activeDungeon: s,
		forest:        &forestRun{hard: true, cleared: [3]bool{true, true, false}},
	}
	plan, err := w.completeForestStage()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 5 {
		t.Fatalf("terminal clear plan = %d frames: %+v", len(plan), plan)
	}
	if plan[0].ID != legion.NotiForestHardPhaseTick {
		t.Fatalf("first frame = id %d, want N2566", plan[0].ID)
	}
	if plan[1].ID != 31 {
		t.Fatalf("second frame = id %d, want the N31 banner", plan[1].ID)
	}
	if plan[2].ID != legion.NotiIspinsBasicClearReward || len(plan[2].Payload) != 7772 {
		t.Fatalf("third frame = id %d %dB, want the 7772B N2252", plan[2].ID, len(plan[2].Payload))
	}
	if plan[3].ID != legion.NotiIspinsAdditionalClearReward || len(plan[3].Payload) != 2405 {
		t.Fatalf("fourth frame = id %d %dB, want the 2405B N2253", plan[3].ID, len(plan[3].Payload))
	}
	token, err := legion.ForestHardStageToken(2)
	if err != nil {
		t.Fatal(err)
	}
	if plan[2].Payload[7760] != token[0] || plan[2].Payload[7761] != token[1] {
		t.Fatalf("N2252 token = %x %x, want %x", plan[2].Payload[7760], plan[2].Payload[7761], token)
	}
	if !w.forest.cleared[2] {
		t.Fatal("terminal stage was not marked cleared")
	}
}

// CMD46 应答：Extreme = N2565 清关 tick +（非终点关）下一关作战窗；
// 终点关只有 tick。
func TestForestHardResultSendsTickAndNextWindow(t *testing.T) {
	w := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true, cleared: [3]bool{true, false, false}},
	}
	plan, err := w.forestResult(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("stage 0 result plan = %d frames: %+v", len(plan), plan)
	}
	wantTick, _ := legion.ForestHardClearTickInfo(0)
	wantWindow, _ := legion.ForestHardWindowInfo(1)
	if plan[0].ID != legion.NotiForestHardInfo || !bytes.Equal(plan[0].Payload, wantTick) {
		t.Fatalf("tick frame = id %d %x", plan[0].ID, plan[0].Payload)
	}
	if plan[1].ID != legion.NotiForestHardInfo || !bytes.Equal(plan[1].Payload, wantWindow) {
		t.Fatalf("window frame = id %d %x", plan[1].ID, plan[1].Payload)
	}

	terminal := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true, cleared: [3]bool{true, true, true}},
	}
	plan, err = terminal.forestResult(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != legion.NotiForestHardInfo {
		t.Fatalf("terminal result plan = %+v, want the tick only", plan)
	}
	wantTick, _ = legion.ForestHardClearTickInfo(2)
	if !bytes.Equal(plan[0].Payload, wantTick) {
		t.Fatalf("terminal tick = %x, want %x", plan[0].Payload, wantTick)
	}
}

// CMD2046 终局：N2565 state3 + 官服 32B ACK（内容 105）。
func TestForestHardRewardEndSendsFinaleState(t *testing.T) {
	w := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true, cleared: [3]bool{true, true, true}},
	}
	plan, notes, err := w.forestRewardEnd(forestHardEnterRequest(legion.ForestHardContentID, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("reward end plan = %d frames: %+v", len(plan), plan)
	}
	if plan[0].ID != legion.NotiForestHardInfo || !bytes.Equal(plan[0].Payload, legion.ForestHardFinalInfo()) {
		t.Fatalf("finale state = id %d %x", plan[0].ID, plan[0].Payload)
	}
	if plan[1].ID != legion.CmdRewardEnd || !bytes.Equal(plan[1].Payload, legion.ForestHardRewardEndAck()) {
		t.Fatalf("reward end ack = id %d %x", plan[1].ID, plan[1].Payload)
	}
	if len(notes) != 1 || notes[0]["hard"] != true {
		t.Fatalf("reward end notes = %v", notes)
	}
	// 未清关时拒绝。
	early := &worldSession{role: database.Character{ID: 7, WireID: 7}, forest: &forestRun{hard: true}}
	if _, _, err := early.forestRewardEnd(forestHardEnterRequest(legion.ForestHardContentID, 2)); err == nil {
		t.Fatal("reward end before the terminal stage was cleared must be refused")
	}
}

// 派发层：hard 队伍的内容 105 开战必须由苏醒之森接手并进入 hard run。
func TestForestHardStartIsClaimedByForestDispatch(t *testing.T) {
	client, _, events := newDispatchTestClient()
	client.worldState = &worldSession{
		channelType:     96,
		role:            database.Character{ID: 2, WireID: 2},
		soloPartyReady:  true,
		forestPartyHard: true,
	}
	client.selectedCharacterID = 2

	body := make([]byte, legion.EnvelopeSize+4)
	for i := 0; i < legion.EnvelopeSize; i++ {
		body[i] = 0xff
	}
	binary.LittleEndian.PutUint32(body[legion.EnvelopeSize:], legion.ForestHardContentID)

	result := client.dispatch(&clientRequest{
		frame:     wire.Frame{Type: 1, ID: legion.CmdStart},
		plaintext: body,
		verified:  true,
	})
	if result != dispatchHandled {
		t.Fatalf("dispatch result = %v, want handled by the forest layer", result)
	}
	if client.worldState.forest == nil || !client.worldState.forest.hard {
		t.Fatal("content 105 start with a 0x19 party did not arm a hard run")
	}
	found := false
	for _, e := range *events {
		if e["kind"] == "forest_started" && e["hard"] == true {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a hard forest_started event: %v", *events)
	}
}

// 待机区建队：Extreme 应答必须是 0x19 + q[55]=1（官服两份 208B 应答的
// 全部差异就是这两处），Normal 应答保持 0x18 且 q[55]=0。
func TestForestStandbyPartyReplyModes(t *testing.T) {
	name := []byte("11")
	normal, err := protocol.ForestStandbyPartyReply(name, 7, [2]byte{1, 2}, false)
	if err != nil {
		t.Fatal(err)
	}
	hard, err := protocol.ForestStandbyPartyReply(name, 7, [2]byte{1, 2}, true)
	if err != nil {
		t.Fatal(err)
	}
	if normal[len(name)+29] != protocol.ForestPartyTypeNormal {
		t.Fatalf("normal party type = %#x", normal[len(name)+29])
	}
	if hard[len(name)+29] != protocol.ForestPartyTypeHard {
		t.Fatalf("extreme party type = %#x, want %#x", hard[len(name)+29], protocol.ForestPartyTypeHard)
	}
	if normal[len(name)+55] != 0 {
		t.Fatalf("normal q[55] = %#x, want 0", normal[len(name)+55])
	}
	if hard[len(name)+55] != 1 {
		t.Fatalf("extreme q[55] = %#x, want 1", hard[len(name)+55])
	}
}

// Extreme 进图是**两段式**（业主 2026-10-09 实机校准）：
//
//	CMD2045 → **N2568 净化开始横幅 + 演出** →（官服 4.000s）→ 进图帧列
//
// 客户端拿到 N2568 会打 `[SEAMLESS LOADING] PREPARE_LEGION_ENTER_DUNGEON start
// type[1] delay[4]`：进图帧列**必须落在它自己声明的 4 秒窗口内**。此前用
// 「4.1s 常数 + 1 秒 tick」实际是 4.87s，客户端退回非无缝路径、界面态卡在
// 「无缝加载中」—— 实机症状就是进图后屏幕 UI 全丢。现在由 connection_session
// 的精确定时器按 4.000s 触发（force=true），这里把整条链锁住。
func TestForestHardBannerDefersEntryFrames(t *testing.T) {
	if forestPurifyBannerSeconds != 4*time.Second {
		t.Fatalf("purify banner delay = %v, want the official 4s window", forestPurifyBannerSeconds)
	}
	// N2568 正文 @8..11 就是客户端读到的 delay（秒）；两个数必须一致，
	// 否则我们把窗口写错、客户端按自己的值等。
	vector := legion.ForestHardPrepareEnterInfo()
	if len(vector) != 24 {
		t.Fatalf("N2568 body = %d bytes, want 24", len(vector))
	}
	if got := binary.LittleEndian.Uint32(vector[8:12]); got != uint32(forestPurifyBannerSeconds/time.Second) {
		t.Fatalf("N2568 delay field = %d, banner constant = %d", got, forestPurifyBannerSeconds/time.Second)
	}
	s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.ForestHardStageDungeons[0]}}
	now := time.Now()
	w := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true},
		forestEntryPending: &forestStageEntryPending{
			session: s,
			stage:   0,
			at:      now.Add(forestPurifyBannerFallback),
			packets: []outboundPacket{{"forest_dungeon_info", 0, 28, []byte{1}}},
		},
	}
	// 1 秒 tick 的兜底路径在 fallback 期限前不发（避免早于客户端的 4 秒窗口）。
	if packets, events := w.forestEntryDue(now, false); len(packets) != 0 || len(events) != 0 {
		t.Fatalf("the ticker fallback must wait for the banner: %v %v", packets, events)
	}
	if w.activeDungeon != nil {
		t.Fatal("the dungeon session must not go live before the banner finishes")
	}
	// 精确定时器（force）到期即发。
	packets, events := w.forestEntryDue(now.Add(forestPurifyBannerSeconds), true)
	if len(packets) != 1 || packets[0].ID != 28 {
		t.Fatalf("deferred entry packets = %+v", packets)
	}
	if w.activeDungeon != s {
		t.Fatal("the dungeon session must go live when the entry frames are sent")
	}
	if w.forestEntryPending != nil {
		t.Fatal("the pending entry must be consumed")
	}
	if len(events) != 1 || events[0]["kind"] != "forest_stage_entered" {
		t.Fatalf("entry events = %v", events)
	}
	// 兜底路径也会发（定时器万一失效）。
	late := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true},
		forestEntryPending: &forestStageEntryPending{
			session: s, stage: 0, at: now.Add(-time.Second), packets: []outboundPacket{{"x", 0, 28, nil}},
		},
	}
	if packets, _ := late.forestEntryDue(now, false); len(packets) != 1 {
		t.Fatalf("the ticker fallback must still deliver past the deadline: %+v", packets)
	}
	// run 已作废（撤退/死亡）：宁可停在待机区，也不能把进图帧砸进城市场景。
	gone := &worldSession{
		role: database.Character{ID: 7, WireID: 7},
		forestEntryPending: &forestStageEntryPending{
			session: s, stage: 0, at: now.Add(-time.Second), packets: []outboundPacket{{"x", 0, 28, nil}},
		},
	}
	packets, events = gone.forestEntryDue(now, true)
	if len(packets) != 0 || len(events) != 1 || events[0]["kind"] != "forest_purify_entry_aborted" {
		t.Fatalf("aborted entry = %v %v", packets, events)
	}
	if gone.activeDungeon != nil {
		t.Fatal("an aborted entry must not arm a dungeon session")
	}
}

// 官服军团副本进图的「军团帧列」必须按锚点注入（与伊斯 enterIspinsStage 同一套）：
// N26/N781/N782 在 N27 之前、N476 在 N1584 之前、N629 在 N28 之后、N465 在 N29 之后；
// 锚点缺失必须**报错**（静默少发正是 2026-10-09 三轮「进图后 HUD 全丢」的成因）。
func TestForestLegionEntryFramesInjection(t *testing.T) {
	index := func(plan []outboundPacket, id uint16) int {
		for i, p := range plan {
			if p.Kind == 0 && p.ID == id {
				return i
			}
		}
		return -1
	}
	base := []outboundPacket{
		{"forest_hard_info_window", 0, legion.NotiForestHardInfo, nil},
		{"forest_enter_ack", 1, legion.CmdEnterDungeon, nil},
		{"forest_dungeon_selection", 0, 27, nil},
		{"forest_dungeon_info", 0, 28, nil},
		{"forest_start_map", 0, 29, nil},
		{"forest_stackable_dungeon_limit", 0, 1584, nil},
	}
	out, err := injectForestLegionFrames(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(base)+6 {
		t.Fatalf("injected plan = %d frames, want %d (26/781/782/476/629/465)", len(out), len(base)+6)
	}
	for _, id := range []uint16{26, 781, 782} {
		if at, anchor := index(out, id), index(out, 27); at < 0 || at > anchor {
			t.Fatalf("NOTI%d at %d must precede NOTI27 at %d", id, at, anchor)
		}
	}
	if at, anchor := index(out, 476), index(out, 1584); at < 0 || at > anchor {
		t.Fatalf("NOTI476 at %d must precede NOTI1584 at %d", at, anchor)
	}
	if at, anchor := index(out, 629), index(out, 28); at != anchor+1 {
		t.Fatalf("NOTI629 at %d must directly follow NOTI28 at %d", at, anchor)
	}
	if at, anchor := index(out, 465), index(out, 29); at != anchor+1 {
		t.Fatalf("NOTI465 at %d must directly follow NOTI29 at %d", at, anchor)
	}
	// 字节必须与官服抓包表（伊斯回放表，两处抓包同串）一致。
	for _, want := range []struct {
		name string
		id   uint16
	}{
		{"fatigue_acceleration", 476},
		{"linked_dungeon_info", 629},
		{"monster_move_system", 465},
		{"udp_host", 26},
	} {
		frames, err := legion.IspinsReplayFrames(want.name)
		if err != nil {
			t.Fatal(err)
		}
		at := index(out, want.id)
		if at < 0 {
			t.Fatalf("NOTI%d missing from the injected plan", want.id)
		}
		if !bytes.Equal(out[at].Payload, frames[0].Body) {
			t.Fatalf("NOTI%d body = %x, want the official %x", want.id, out[at].Payload, frames[0].Body)
		}
	}
	// 锚点缺失必须报错，不能静默少发。
	if _, err := injectForestLegionFrames([]outboundPacket{{"n27", 0, 27, nil}}); err == nil {
		t.Fatal("a plan without NOTI28/NOTI29/NOTI1584 must be refused")
	}
}

// 官服进图的**第一帧**是 N23（USER_AREA）且 area=0xff —— Normal 抓包
// （21:42:18.422 `c401 c6000000 ff000000 …`）与 Extreme 抓包（22:05:45.515
// `4400 c6000000 ff000000 …`）都是。本仓此前任何副本进图都不发它；无缝加载
// （N2568）之后缺了它，客户端就停在城镇区域态 ⇒ 屏幕 UI 全丢。
func TestForestDungeonAreaNoticeUsesDungeonArea(t *testing.T) {
	if forestDungeonArea != 0xff {
		t.Fatalf("dungeon area = %#x, want the official 0xff", forestDungeonArea)
	}
	w := &worldSession{
		role:  database.Character{ID: 7, WireID: 0x0044},
		state: database.WorldState{Position: database.WorldPosition{Town: 198, Area: 3, X: 494, Y: 287}},
		flags: [3]byte{5, 0x21, 0},
	}
	body, err := w.forestDungeonAreaNotice()
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 16 {
		t.Fatalf("N23 body = %d bytes, want the repo's 16B NOTI23 shape", len(body))
	}
	if actor := binary.LittleEndian.Uint16(body[0:]); actor != 0x0044 {
		t.Fatalf("actor = %#x", actor)
	}
	if town := binary.LittleEndian.Uint32(body[2:]); town != 198 {
		t.Fatalf("town = %d, want the current town 198", town)
	}
	if area := binary.LittleEndian.Uint32(body[6:]); area != forestDungeonArea {
		t.Fatalf("area = %d, want %d", area, forestDungeonArea)
	}
	if x, y := binary.LittleEndian.Uint16(body[10:]), binary.LittleEndian.Uint16(body[12:]); x != 494 || y != 287 {
		t.Fatalf("position = %d,%d want the pre-entry landing 494,287", x, y)
	}
}

// 业主 2026-10-09「改成能一直开始」：通关后客户端在本地把本周账本标成已用完，
// 「开始作战」不再发 CMD2043（三轮实机都是零包，把 N2254 + N781/N782 重推回去也
// 没解开）。而**客户端一收到 N2565 作战窗就自己发 CMD2045 进图**
// （官服 22:05:41.244 推窗 → .255 c2s CMD2045；本机 09:50:01 推窗 → 同一秒 CMD2045）。
// 所以把「走进极难度集结区（town198/area3）」当作开战意图，由服务端主动推等待态 +

// 撤退/死亡出本后右上角倒计时必须重置：清掉冻结的起算时刻 + 补一帧满额 N1474
// （业主 2026-10-09：「点击撤退出去或者死亡强制退出，右上角的倒计时没有刷新」）。
func TestForestStageTimerReset(t *testing.T) {
	started := time.Now().Add(-20 * time.Minute)
	w := &worldSession{
		role:   database.Character{ID: 7, WireID: 7},
		forest: &forestRun{hard: true, stageClock: [3]time.Time{started, {}, {}}, potionUsed: [3]int{5, 0, 0}},
	}
	now := time.Now()
	plan := w.forestStageTimerReset(now, 0, "retreat")
	if len(plan) != 1 || plan[0].ID != legion.NotiDungeonTimeoutTime {
		t.Fatalf("timer reset plan = %+v", plan)
	}
	if !w.forest.stageClock[0].IsZero() {
		t.Fatal("the frozen stage clock must be cleared so the next entry starts at full limit")
	}
	if w.forest.potionUsed[0] != 0 {
		t.Fatal("the per-stage potion counter must be cleared with the clock")
	}
	if limit := binary.LittleEndian.Uint32(plan[0].Payload[0:4]); limit != legion.ForestStageLimits[0] {
		t.Fatalf("reset limit = %d, want %d", limit, legion.ForestStageLimits[0])
	}
	if start := binary.LittleEndian.Uint32(plan[0].Payload[4:8]); int64(start) != now.Unix() {
		t.Fatalf("reset start = %d, want %d", start, now.Unix())
	}
	if out := w.forestStageTimerReset(now, 9, "retreat"); out != nil {
		t.Fatal("an out-of-range stage must not produce a reset")
	}
}
