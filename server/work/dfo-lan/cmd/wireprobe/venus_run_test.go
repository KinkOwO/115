package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// 维纳斯开战流程回归护栏。CMD2043 请求向量取自 2026-10-04 11:11 会话
// （…_111111_716314_next37）events.jsonl 实机帧：玩家点「开始作战」发出
// 24B（13B 信封 + 内容号 106 + 7 个零字节，与伊斯官服 s4 开局同构）。
const venusLiveStartRequestHex = "ffffffffffffffffffffffff006a00000000000000000000"

func venusEnvelopeWithContent(extra ...byte) []byte {
	p := make([]byte, legion.EnvelopeSize)
	for i := range p {
		p[i] = 0xff
	}
	p[legion.EnvelopeSize-1] = 0
	p = binary.LittleEndian.AppendUint32(p, legion.VenusContentID)
	return append(p, extra...)
}

// N2655 固定字节：头 FFFF、Following/field19 FFFFFFFF、记录 +4 u64 全 FF、
// 尾 6B 与标志位零。
func TestVenusWaitingInfoLayout(t *testing.T) {
	got := legion.VenusWaitingInfo()
	if len(got) != 85 {
		t.Fatalf("venus info len=%d, want 85", len(got))
	}
	if got[0] != 0xff || got[1] != 0xff {
		t.Fatalf("head = %x, want ffff", got[0:2])
	}
	if got[2] != 0xff {
		t.Fatalf("choice = %d, want ff (未选择)", got[2])
	}
	if binary.LittleEndian.Uint32(got[3:]) != 2 {
		t.Fatalf("state = %d, want 2 (等待区)", binary.LittleEndian.Uint32(got[3:]))
	}
	if binary.LittleEndian.Uint32(got[7:]) != 0 {
		t.Fatalf("outcome = %d, want 0", binary.LittleEndian.Uint32(got[7:]))
	}
	if binary.LittleEndian.Uint32(got[11:]) != 0 {
		t.Fatalf("stage = %d, want 0", binary.LittleEndian.Uint32(got[11:]))
	}
	if binary.LittleEndian.Uint32(got[15:]) != 0xffffffff || binary.LittleEndian.Uint32(got[19:]) != 0xffffffff {
		t.Fatalf("following/field19 = %x", got[15:23])
	}
	if got[23] != 0 {
		t.Fatalf("record0 target = %d, want 0 (首目标)", got[23])
	}
	for i := 1; i < 4; i++ {
		if got[23+12*i] != 0xff {
			t.Fatalf("record%d target = %d, want ff", i, got[23+12*i])
		}
	}
	for i := 0; i < 4; i++ {
		rec := got[23+12*i:]
		if !bytes.Equal(rec[4:12], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) {
			t.Fatalf("record%d u64 = %x", i, rec[4:12])
		}
	}
	for i, b := range got[71:77] {
		if b != 0 {
			t.Fatalf("tail byte %d = %02x, want 0", 71+i, b)
		}
	}
	if binary.LittleEndian.Uint32(got[77:]) != 0 || binary.LittleEndian.Uint32(got[81:]) != 0 {
		t.Fatalf("relic mask/flag = %x", got[77:])
	}
}

func TestVenusChosenAndAdvancedInfo(t *testing.T) {
	chosen := legion.VenusChosenInfo(2, 0)
	if chosen[2] != 2 || binary.LittleEndian.Uint32(chosen[3:]) != 2 || binary.LittleEndian.Uint32(chosen[11:]) != 0 {
		t.Fatalf("chosen info choice/state/stage = %d/%d/%d", chosen[2], binary.LittleEndian.Uint32(chosen[3:]), binary.LittleEndian.Uint32(chosen[11:]))
	}
	// BUG3 回归：进度保留后 chosen 态携带下一个待进阶段。
	chosen2 := legion.VenusChosenInfo(2, 2)
	if chosen2[2] != 2 || binary.LittleEndian.Uint32(chosen2[11:]) != 2 {
		t.Fatalf("chosen2 info choice/stage = %d/%d", chosen2[2], binary.LittleEndian.Uint32(chosen2[11:]))
	}
	next := legion.VenusStageAdvancedInfo(1, 1)
	if got := binary.LittleEndian.Uint32(next[11:]); got != 1 {
		t.Fatalf("advanced stage = %d, want 1", got)
	}
	if next[23+12] != 1 {
		t.Fatalf("record1 target = %d, want 1", next[23+12])
	}
	if next[23] != 0 {
		t.Fatalf("record0 target = %d, want 0 (保持已发布)", next[23])
	}
}

func TestVenusOperationAckLayout(t *testing.T) {
	deadline := uint32(1770000000)
	got := legion.VenusOperationAck(1, false, deadline, 0)
	if len(got) != 15 {
		t.Fatalf("ack len=%d, want 15", len(got))
	}
	if got[0] != 1 || binary.LittleEndian.Uint32(got[1:]) != 1 || got[5] != 0xff || got[6] != 0 {
		t.Fatalf("open ack = %x", got)
	}
	if binary.LittleEndian.Uint32(got[7:]) != deadline || got[11] != 0 {
		t.Fatalf("deadline/submode = %d/%d", binary.LittleEndian.Uint32(got[7:]), got[11])
	}
	if !bytes.Equal(got[12:], []byte{0, 0, 0}) {
		t.Fatalf("reserved = %x", got[12:])
	}
	closing := legion.VenusOperationAck(4, true, 0, 1)
	if closing[6] != 1 || closing[11] != 1 {
		t.Fatalf("close/submode flags = %d/%d", closing[6], closing[11])
	}
}

func TestDecodeVenusStartLiveRequest(t *testing.T) {
	req, err := hex.DecodeString(venusLiveStartRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := legion.DecodeVenusStart(req); err != nil {
		t.Fatal(err)
	}
	if err := legion.DecodeVenusStart(req[:10]); err == nil {
		t.Fatal("truncated start must be rejected")
	}
	wrong := append([]byte{}, req...)
	wrong[13] = 107 // 末世录内容号
	if err := legion.DecodeVenusStart(wrong); err == nil {
		t.Fatal("content 107 must be rejected")
	}
}

func TestDecodeVenusOperationAndEnter(t *testing.T) {
	// CMD2290 请求没有内容号：13B 信封 + u32 Action @13 + u8 Choice @17。
	op := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 1) // action2 确认，难度 1
	req, err := legion.DecodeVenusOperation(op)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != 2 || req.Choice != 1 {
		t.Fatalf("decoded operation = %+v", req)
	}
	open := append(make([]byte, legion.EnvelopeSize), 1, 0, 0, 0, 255)
	if req, err = legion.DecodeVenusOperation(open); err != nil || req.Action != 1 || req.Choice != 255 {
		t.Fatalf("decoded open = %+v err %v", req, err)
	}
	if _, err = legion.DecodeVenusOperation(append(make([]byte, legion.EnvelopeSize), 1, 0, 0, 0)); err == nil {
		t.Fatal("operation without the choice byte must be rejected")
	}

	enter := venusEnvelopeWithContent(1, 0, 0, 0) // stage 1
	got, err := legion.DecodeVenusEnter(enter)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != 1 {
		t.Fatalf("decoded enter stage = %d, want 1", got.Stage)
	}
	ispinsBody := append(make([]byte, legion.EnvelopeSize), 101, 0, 0, 0, 0, 0, 0, 0)
	if _, err = legion.DecodeVenusEnter(ispinsBody); err == nil {
		t.Fatal("ispins content 101 must be rejected")
	}
}

func TestVenusEndpoint(t *testing.T) {
	for _, c := range []byte{0, 1, 0xff} {
		if got := legion.VenusEndpoint(c); got != 2 {
			t.Fatalf("endpoint(%d) = %d, want 2", c, got)
		}
	}
	if got := legion.VenusEndpoint(2); got != 3 {
		t.Fatalf("endpoint(2) = %d, want 3 (降临第四关)", got)
	}
}

// 流程门禁：未建队不能开战；未选择难度不能进图；CMD2046 推进阶段并在
// 终局后不再发布下一阶段。
func TestVenusRunFlowGuards(t *testing.T) {
	w := &worldSession{
		channelType: 99,
		role:        database.Character{WireID: 7, ID: 7, Name: "VenusCap"},
		characters:  &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
		dungeons:    &catalog.DungeonCatalog{},
	}
	startBody, err := hex.DecodeString(venusLiveStartRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.handleVenusRequest(startBody, legion.CmdStart); err == nil {
		t.Fatal("start before the standby party was created must be refused")
	}
	w.soloPartyReady = true
	plan, notes, err := w.handleVenusRequest(startBody, legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != legion.CmdStart || plan[0].Kind != 1 || plan[1].ID != legion.NotiVenusInfo {
		t.Fatalf("start plan = %+v", plan)
	}
	if binary.LittleEndian.Uint32(plan[1].Payload[3:]) != 2 || plan[1].Payload[2] != 0xff {
		t.Fatal("waiting state must be State2/ChoiceFF")
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_started" {
		t.Fatalf("start notes = %v", notes)
	}

	confirm := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 1) // action2，难度 1
	if _, _, err = w.handleVenusRequest(confirm, legion.CmdVenusOperationSelect); err != nil {
		t.Fatal(err)
	}
	if w.venus == nil || w.venus.choice != 1 {
		t.Fatalf("run choice = %+v", w.venus)
	}

	// 进图守卫（不走真实目录）：跳阶段与越终点都必须拒绝。
	if _, _, err = w.handleVenusRequest(venusEnvelopeWithContent(1, 0, 0, 0), legion.CmdEnterDungeon); err == nil {
		t.Fatal("enter stage 1 before stage 0 must be refused")
	}
	if _, _, err = w.handleVenusRequest(venusEnvelopeWithContent(3, 0, 0, 0), legion.CmdEnterDungeon); err == nil {
		t.Fatal("enter stage 3 with a normal choice must be refused")
	}

	// 阶段推进：CMD2046 权威记录通关并发布下一阶段状态。
	w.venus.stage = 0
	reward := venusEnvelopeWithContent(0, 0, 0, 0)
	plan, _, err = w.handleVenusRequest(reward, legion.CmdRewardEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != legion.CmdRewardEnd || plan[1].ID != legion.NotiVenusInfo {
		t.Fatalf("reward plan = %+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[1].Payload[11:]); got != 1 {
		t.Fatalf("next stage = %d, want 1", got)
	}
	if !w.venus.cleared[0] || w.venus.clearedCount() != 1 {
		t.Fatalf("run after reward end = %+v", w.venus)
	}

	// 终局：普通模式 endpoint=2。翻牌链已在 boss 死亡时记账（cleared[2]=true，
	// completeVenusStage 同步置位），终局 CMD2046 = 翻牌结束信号：回 ACK +
	// **N2655 final 态**（State3/Outcome1，家族约定镜像伊斯 N2255 final——伊斯
	// 通关视频由它触发），会话保持、finalDone 置位（客户端留在副本播视频）。
	w.venus.cleared[1] = true
	w.venus.cleared[2] = true
	w.venus.stage = 2
	reward = venusEnvelopeWithContent(2, 0, 0, 0)
	plan, _, err = w.handleVenusRequest(reward, legion.CmdRewardEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != legion.CmdRewardEnd || plan[1].Name != "venus_info_final" || plan[1].ID != legion.NotiVenusInfo {
		t.Fatalf("terminal reward plan = %+v", plan)
	}
	if !w.venus.finalDone {
		t.Fatal("terminal reward end must arm the finale movie flow")
	}
	if state := binary.LittleEndian.Uint32(plan[1].Payload[3:]); state != 3 {
		t.Fatalf("final state = %d, want 3", state)
	}
	// 通关视频的 CMD191：暂停（state=0）答 N170；恢复（state=1）= 视频播完——
	// 发 leave 态（N2655 State5）关右上角面板与遗物显示，玩家随后点返回城镇。
	pause := make([]byte, 16)
	pause[1] = 1
	pausePlan, err := w.venusStoryPause(pause)
	if err != nil {
		t.Fatal(err)
	}
	if len(pausePlan) != 1 || pausePlan[0].ID != 170 {
		t.Fatalf("story pause plan = %+v", pausePlan)
	}
	resume := append([]byte(nil), pause...)
	resume[0] = 1
	resumePlan, err := w.venusStoryPause(resume)
	if err != nil {
		t.Fatal(err)
	}
	hasLeave := false
	for _, p := range resumePlan {
		if p.Name == "venus_info_leave" && p.ID == legion.NotiVenusInfo {
			hasLeave = true
			if state := binary.LittleEndian.Uint32(p.Payload[3:]); state != 5 {
				t.Fatalf("leave state = %d, want 5", state)
			}
		}
		if p.Name == "dungeon_leave_ack" {
			t.Fatal("story resume must not exit the dungeon (返回城镇走 CMD72)")
		}
	}
	if !hasLeave {
		t.Fatalf("story resume must publish the leave state: %+v", resumePlan)
	}
	// 返回城镇（F12 → CMD72 state=1）：退场 + run 作废 + 关面板收尾（State0）。
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[2]}}
	exit := make([]byte, 16)
	exit[0] = 1
	_, exitPlan, err := w.settlementExit(exit)
	if err != nil {
		t.Fatal(err)
	}
	if len(exitPlan) == 0 || exitPlan[0].Name != "settlement_exit_ack" {
		t.Fatalf("final exit plan = %+v", exitPlan)
	}
	hasClosed := false
	for _, p := range exitPlan {
		if p.Name == "venus_info_closed" {
			hasClosed = true
		}
	}
	if !hasClosed {
		t.Fatal("final exit must carry the panel-close state")
	}
	if w.venus != nil {
		t.Fatal("run must be retired after the final town exit")
	}
	// 通关后终局补发拒绝（run 已作废）。
	if _, _, err = w.handleVenusRequest(reward, legion.CmdRewardEnd); err == nil {
		t.Fatal("reward end after the finale must be refused (run retired)")
	}
	// 副本进行中禁止开局与作战选择。
	w.activeDungeon = &dungeon.Session{}
	if _, _, err := w.handleVenusRequest(startBody, legion.CmdStart); err == nil {
		t.Fatal("start inside an active dungeon must be refused")
	}
	if _, _, err := w.handleVenusRequest(confirm, legion.CmdVenusOperationSelect); err == nil {
		t.Fatal("operation inside an active dungeon must be refused")
	}
}

// 第一阶段动态圣物怪准备：源图 venus_1phase.map 无 [monster] 段（三只是
// 事件怪），N29 固定行按 SourceIndex 解析不到源行、客户端不建怪（2026-10-04
// 13:52 实机 N29 带三行、画面无怪）。现在花名册照常登记三只、坐标取源图
// [event monster position] 槽，返回的待注册行由 C37 后的 N2194 下发。
func TestVenusPhase1CarrierStaging(t *testing.T) {
	c := &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{
		100011986: {Cells: []pvf.Token{
			{Type: 3, Text: "[event monster position]"},
			{Type: 0, Value: 373}, {Type: 0, Value: 372}, {Type: 0, Value: 0},
			{Type: 0, Value: 373}, {Type: 0, Value: 372}, {Type: 0, Value: 0},
			{Type: 3, Text: "[/event monster position]"},
		}},
	}}
	s := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0], BasisLevel: 145},
		Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011986},
		Maze:       catalog.DungeonMaze{Start: [2]byte{1, 0}},
		NextEntity: 4096,
		Visited:    map[uint32][]protocol.DungeonMonster{},
	}
	s.Visited[s.Room.Map] = s.Monsters
	pending := stageVenusPhaseCarriers(*c, s, 0)
	if len(pending) != 3 {
		t.Fatalf("pending carriers = %d, want 3", len(pending))
	}
	if len(s.Monsters) != 3 {
		t.Fatalf("roster = %d, want 3", len(s.Monsters))
	}
	for i, row := range pending {
		if row.Template != legion.VenusPhase1Carriers[i] {
			t.Fatalf("carrier %d template = %d, want %d", i, row.Template, legion.VenusPhase1Carriers[i])
		}
		if row.Entity != uint16(4096+i) || row.Grid != [2]byte{1, 0} || row.X != 373 || row.Y != 372 {
			t.Fatalf("carrier %d row = %+v", i, row)
		}
		m := s.Monsters[i]
		if m.Entity != row.Entity || m.Team != 100 || m.Level != 145 || m.Rank != 0 {
			t.Fatalf("carrier %d roster row = %+v", i, m)
		}
	}
	if s.NextEntity != 4099 {
		t.Fatalf("next entity = %d, want 4099", s.NextEntity)
	}
	visited := s.Visited[s.Room.Map]
	if len(visited) != 3 || visited[0].Template != legion.VenusPhase1Carriers[0] {
		t.Fatalf("visited roster = %+v", visited)
	}
	// 幂等与门禁：已有投放单、非首关、非维纳斯副本都不动。
	if again := stageVenusPhaseCarriers(*c, s, 0); again != nil {
		t.Fatal("staging must be idempotent")
	}
	if rows := stageVenusPhaseCarriers(*c, s, 1); rows != nil {
		t.Fatal("stage 1 must keep the source roster")
	}
	foreign := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100003126}, NextEntity: 4096, Visited: map[uint32][]protocol.DungeonMonster{}}
	if rows := stageVenusPhaseCarriers(*c, foreign, 0); rows != nil || len(foreign.Monsters) != 0 {
		t.Fatal("non-venus dungeon must not gain carriers")
	}
	// 源图缺失事件位：不登记花名册（退化为可通行空房），不产出待注册行。
	bare := &catalog.DungeonCatalog{}
	empty := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0], BasisLevel: 145},
		Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011986},
		NextEntity: 4096,
		Visited:    map[uint32][]protocol.DungeonMonster{},
	}
	if rows := stageVenusPhaseCarriers(*bare, empty, 0); rows != nil || len(empty.Monsters) != 0 {
		t.Fatal("missing event positions must stage nothing")
	}
}

// CMD37 加载完成后用 N2194 注册待注册的动态圣物怪（月湖先例：动态怪不能进
// N29 固定行，真实 C37 后逐只注册），坐标取源图事件位槽；注册即清 pending。
func TestVenusDynamicCarrierSpawnOnLoading(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011986},
		},
	}
	if out := w.venusDynamicSpawnPackets(); out != nil {
		t.Fatalf("no pending carriers must emit nothing: %+v", out)
	}
	w.venus.pending = []protocol.UnassignedMonster115{
		{Grid: [2]byte{1, 0}, Entity: 4096, Template: 109016980, X: 373, Y: 372},
		{Grid: [2]byte{1, 0}, Entity: 4097, Template: 109016981, X: 373, Y: 372},
		{Grid: [2]byte{1, 0}, Entity: 4098, Template: 109016982, X: 373, Y: 372},
	}
	out := w.venusDynamicSpawnPackets()
	if len(out) != 1 || out[0].ID != 2194 || out[0].Kind != 0 {
		t.Fatalf("spawn packets = %+v", out)
	}
	body := out[0].Payload
	if len(body) != 1+33*3 || body[0] != 3 {
		t.Fatalf("N2194 body = %x", body)
	}
	for i, want := range legion.VenusPhase1Carriers {
		row := body[1+33*i:]
		if binary.LittleEndian.Uint16(row[2:]) != uint16(4096+i) {
			t.Fatalf("row %d entity = %d", i, binary.LittleEndian.Uint16(row[2:]))
		}
		if binary.LittleEndian.Uint32(row[6:]) != want {
			t.Fatalf("row %d template = %d, want %d", i, binary.LittleEndian.Uint32(row[6:]), want)
		}
		if binary.LittleEndian.Uint32(row[17:]) != 373 || binary.LittleEndian.Uint32(row[21:]) != 372 {
			t.Fatalf("row %d coords = %d,%d", i, binary.LittleEndian.Uint32(row[17:]), binary.LittleEndian.Uint32(row[21:]))
		}
	}
	if len(w.venus.pending) != 0 {
		t.Fatalf("pending after spawn = %+v", w.venus.pending)
	}
	// 异房间的待注册行不注册、不清除。
	w.venus.pending = []protocol.UnassignedMonster115{{Grid: [2]byte{0, 0}, Entity: 4099, Template: 109016983}}
	if out := w.venusDynamicSpawnPackets(); out != nil || len(w.venus.pending) != 1 {
		t.Fatalf("mismatched grid must be kept: out=%v pending=%v", out, w.venus.pending)
	}
	// 非维纳斯副本不注册。
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100003126}, Room: catalog.DungeonRoom{X: 1, Y: 0}}
	w.venus.pending = []protocol.UnassignedMonster115{{Grid: [2]byte{1, 0}, Entity: 4096, Template: 109016980, X: 373, Y: 372}}
	if out := w.venusDynamicSpawnPackets(); out != nil {
		t.Fatalf("non-venus dungeon must not register carriers: %+v", out)
	}
}

// N1474（DUNGEON_TIMEOUT_TIME）阶段倒计时：正文 8B [时限秒, 阶段开始秒]，
// 频道 99 走秒分支（毫秒特例是矿区频道 106）。第 1..3 关 600 秒、降临第 4 关
// 900 秒；开始时间在该阶段第一次加载完成时冻结，同阶段重载复用原值（规格
// 文档「已推翻」条：用当前发送时间会错误续时）；resetRun 清时钟；非维纳斯
// 副本/无 run/副本外不发包。
func TestVenusStageTimerSync(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011986},
		},
	}
	start := time.Unix(1789000000, 0)
	out := w.venusStageTimer(start)
	if len(out) != 1 || out[0].ID != legion.NotiDungeonTimeoutTime || out[0].Kind != 0 || out[0].Name != "venus_stage_timer_sync" {
		t.Fatalf("timer packets = %+v", out)
	}
	body := out[0].Payload
	if len(body) != 8 || binary.LittleEndian.Uint32(body) != 600 || binary.LittleEndian.Uint32(body[4:]) != uint32(start.Unix()) {
		t.Fatalf("stage 0 N1474 = %x", body)
	}
	// 同阶段重载：开始时间冻结，不随当前发送时间续时。
	if reload := w.venusStageTimer(start.Add(123 * time.Second)); len(reload) != 1 || !bytes.Equal(reload[0].Payload, body) {
		t.Fatalf("reload must reuse the frozen start: %+v", reload)
	}
	// 降临第 4 关 900 秒，开始时间按该阶段自己的首次加载冻结；第 1 关时钟不受影响。
	later := start.Add(123 * time.Second)
	w.activeDungeon.Definition.ID = legion.VenusStageDungeons[3]
	out = w.venusStageTimer(later)
	if len(out) != 1 {
		t.Fatalf("stage 3 packets = %+v", out)
	}
	if binary.LittleEndian.Uint32(out[0].Payload) != 900 || binary.LittleEndian.Uint32(out[0].Payload[4:]) != uint32(later.Unix()) {
		t.Fatalf("stage 3 N1474 = %x", out[0].Payload)
	}
	if !w.venus.stageClock[0].Equal(start) {
		t.Fatalf("stage 0 clock mutated: %v", w.venus.stageClock[0])
	}
	// resetRun 清空阶段时钟：重新加载后按当前时间重新冻结。
	w.venus.resetRun()
	if out := w.venusStageTimer(start.Add(200 * time.Second)); len(out) != 1 ||
		binary.LittleEndian.Uint32(out[0].Payload[4:]) != uint32(start.Add(200*time.Second).Unix()) {
		t.Fatalf("post-reset refreeze = %+v", out)
	}
	// 非维纳斯副本、无 run、副本外都不发包。
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100003126}}
	if out := w.venusStageTimer(start); out != nil {
		t.Fatalf("non-venus dungeon must not sync a timer: %+v", out)
	}
	w.venus = nil
	if out := w.venusStageTimer(start); out != nil {
		t.Fatalf("no run must not sync a timer: %+v", out)
	}
	w.activeDungeon = nil
	if out := w.venusStageTimer(start); out != nil {
		t.Fatalf("outside a dungeon must not sync a timer: %+v", out)
	}
}

// finishDungeonLoading（CMD37 加载完成）应答在 N30 之前携带 N1474 阶段倒计时
// （官服巴卡尔段 20261002 抓包 21:42:19 实序：CMD37 ack → N1474 → N30；伊斯
// 官服 timer_sync 回放同序）。非维纳斯副本的同一条应答不含 1474。
func TestVenusStageTimerOnLoading(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		state: database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
		venus: &venusRun{},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0},
		},
	}
	plan, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	loaded, timer := -1, -1
	for i, p := range plan {
		switch p.ID {
		case 30:
			loaded = i
		case legion.NotiDungeonTimeoutTime:
			timer = i
		}
	}
	if loaded < 0 || timer < 0 || timer > loaded || timer < 2 {
		t.Fatalf("N1474 must sit between the CMD37 ack and N30: 37ack@0 1474@%d 30@%d", timer, loaded)
	}
	if body := plan[timer].Payload; len(body) != 8 || binary.LittleEndian.Uint32(body) != 600 {
		t.Fatalf("stage 1 N1474 = %x", body)
	}
	// 非维纳斯副本：同一条加载应答不含 1474。
	plain := &worldSession{role: database.Character{ID: 5, WireID: 503},
		state:         database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}},
		activeDungeon: &dungeon.Session{},
	}
	plan, err = plain.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plan {
		if p.ID == legion.NotiDungeonTimeoutTime {
			t.Fatalf("non-venus loading plan carries N1474: %+v", p)
		}
	}
}

// 倒计时到期判定挑战失败：完整回城序列（dungeon_leave_ack 打头）+ 末尾等待态
// N2655，run 复位、会话清理（ticker 路径无 dispatch 的 dungeon_leave_ack 监视
// 器，照 moonReturn 自行清理）。未到期不触发；结算期（Completed/completionSent/
// resultSent）不拽人；重复触发、非维纳斯副本、副本外都是空操作。
func TestVenusStageTimeout(t *testing.T) {
	w := &worldSession{channelType: 99,
		role:  database.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
		venus: &venusRun{choice: 2, stage: 2},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[2]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0},
		},
	}
	started := time.Unix(1789000000, 0)
	w.venus.stageClock[2] = started
	// 结算中的终点关不触发（翻牌期间到时不拽人）。
	w.activeDungeon.MarkCompleted()
	w.completionSent = true
	if out := w.venusStageTimeout(started.Add(601*time.Second), nil); out != nil {
		t.Fatalf("completed stage must not time out: %+v", out)
	}
	// 未通关的终点关：差 1 秒不触发，到点判定失败（stage 2 = 第 3 关，600 秒）。
	// 带进度与已选难度（超时不清进度：第二十二轮口径，难度第三十三轮起锁定）。
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[2]}}
	w.completionSent = false
	w.venus = &venusRun{choice: 2, stage: 2, relicMask: 1 << 3, entered: true,
		cleared: [4]bool{true, true, false, false}}
	w.venus.stageClock[2] = started
	if out := w.venusStageTimeout(started.Add(599*time.Second), nil); out != nil {
		t.Fatalf("unexpired stage must not time out: %+v", out)
	}
	var notes []map[string]any
	plan := w.venusStageTimeout(started.Add(600*time.Second), func(n map[string]any) { notes = append(notes, n) })
	// N33 原生超时失败开路（reason=100），**不带 dungeon_leave_ack**（42-ack 是
	// 对从未发生的客户端 GIVEUP 请求的伪造应答——直接推 42-ack+城镇外观包会让
	// 角色在退场前 ~1 秒裸体渲染，2026-10-05 实机）。
	if len(plan) == 0 || plan[0].Name != "venus_time_limit_failed" || plan[0].ID != 33 || len(plan[0].Payload) != 1 || plan[0].Payload[0] != 100 {
		t.Fatalf("timeout plan must start with the N33 timeout fail: %+v", plan)
	}
	for _, p := range plan {
		if p.Name == "dungeon_leave_ack" || p.ID == 42 {
			t.Fatalf("timeout plan must not carry the fabricated 42-ack: %+v", p)
		}
	}
	hasTownState, hasReturnArea, hasRevive := false, false, false
	for _, p := range plan {
		switch p.Name {
		case "town_actor_state":
			hasTownState = true
		case "dungeon_return_area":
			hasReturnArea = true
		case "venus_timeout_actor_revived":
			hasRevive = true
			if p.ID != 32 || len(p.Payload) < 3 || p.Payload[2] != 1 {
				t.Fatalf("revive must carry the alive state: %+v", p)
			}
		}
	}
	if !hasTownState || !hasReturnArea || !hasRevive {
		t.Fatalf("timeout plan missing town route/revive: state=%v area=%v revive=%v", hasTownState, hasReturnArea, hasRevive)
	}
	// 序列末尾 N2655 带权威已选难度（Choice=2，Stage=保留进度 2）。
	if last := plan[len(plan)-1]; last.Name != "venus_info_timeout_waiting" || last.ID != legion.NotiVenusInfo ||
		last.Payload[2] != 2 || binary.LittleEndian.Uint32(last.Payload[3:]) != 2 || binary.LittleEndian.Uint32(last.Payload[11:]) != 2 {
		t.Fatalf("timeout plan must end with the locked waiting N2655: %+v payload %x", last, last.Payload)
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_stage_timeout" || notes[0]["stage"] != 2 || notes[0]["limit"] != 600 {
		t.Fatalf("timeout notes = %+v", notes)
	}
	if w.venus.choice != 2 || w.venus.relicMask != 1<<3 || !w.venus.entered || w.venus.clearedCount() != 2 || !w.venus.stageClock[2].IsZero() {
		t.Fatalf("timeout must keep the locked progress, got %+v", w.venus)
	}
	if w.activeDungeon != nil || w.deathSent != nil || w.completionSent || w.resultSent || w.selectingDungeon || w.approvedDungeonGate != 0 {
		t.Fatalf("session not cleaned after timeout: dungeon=%v death=%v completion=%v result=%v selecting=%v gate=%d",
			w.activeDungeon, w.deathSent, w.completionSent, w.resultSent, w.selectingDungeon, w.approvedDungeonGate)
	}
	// 重复触发是空操作。
	if out := w.venusStageTimeout(started.Add(901*time.Second), nil); out != nil {
		t.Fatalf("repeat timeout must be a no-op: %+v", out)
	}
	// 非维纳斯副本与副本外不触发。
	w.venus = &venusRun{choice: 0, stage: 0}
	w.venus.stageClock[0] = started
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100003126}}
	if out := w.venusStageTimeout(started.Add(901*time.Second), nil); out != nil {
		t.Fatalf("non-venus dungeon must not time out: %+v", out)
	}
	w.activeDungeon = nil
	if out := w.venusStageTimeout(started.Add(901*time.Second), nil); out != nil {
		t.Fatalf("outside a dungeon must not time out: %+v", out)
	}
}

// 难度选择窗倒计时归 0：推原生 close ACK（2290 ACK @5 close=1，action=1）自动
// 关窗——客户端自己对 0 不做任何事（2026-10-05 实测图4），残留的开口状态会让
// 超时/撤退回待机区后难度窗自动重弹。截止前不推；确认难度（choice≠FF）后不推；
// 在副本中不推；resetRun 清截止时刻；重复触发空操作。
func TestVenusOperationWindowClose(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 0xff},
	}
	open := append(make([]byte, legion.EnvelopeSize), 1, 0, 0, 0, 255)
	if _, _, err := w.venusOperation(open); err != nil {
		t.Fatal(err)
	}
	if w.venus.windowDeadline.IsZero() {
		t.Fatal("open must record the window deadline")
	}
	deadline := w.venus.windowDeadline
	// 截止前不关。
	if out := w.venusOperationClose(deadline.Add(-time.Second), nil); out != nil {
		t.Fatalf("window must stay open before the deadline: %+v", out)
	}
	// 归 0：原生 close ACK（action=1, close=1）。
	out := w.venusOperationClose(deadline.Add(time.Second), nil)
	if len(out) != 1 || out[0].ID != legion.CmdVenusOperationSelect || out[0].Kind != 1 || out[0].Name != "venus_operation_close" {
		t.Fatalf("close packets = %+v", out)
	}
	body := out[0].Payload
	if len(body) != 15 || body[0] != 1 || binary.LittleEndian.Uint32(body[1:]) != 1 || body[6] != 1 {
		t.Fatalf("close ack body = %x", body)
	}
	// 关一次即清零：重复触发空操作。
	if out := w.venusOperationClose(deadline.Add(2*time.Second), nil); out != nil {
		t.Fatalf("repeat close must be a no-op: %+v", out)
	}
	// 确认难度后不推 close（窗已原生关闭）。
	if _, _, err := w.venusOperation(open); err != nil {
		t.Fatal(err)
	}
	confirm := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 1)
	if _, _, err := w.venusOperation(confirm); err != nil {
		t.Fatal(err)
	}
	if out := w.venusOperationClose(deadline.Add(3*time.Second), nil); out != nil {
		t.Fatalf("confirmed run must not push close: %+v", out)
	}
	if !w.venus.windowDeadline.IsZero() {
		t.Fatal("confirm must clear the window deadline")
	}
	// 在副本中不推 close（防御）。
	if _, _, err := w.venusOperation(open); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]}}
	if out := w.venusOperationClose(deadline.Add(4*time.Second), nil); out != nil {
		t.Fatalf("inside a dungeon must not push close: %+v", out)
	}
	// resetRun 清截止时刻。
	w.activeDungeon = nil
	w.venus.resetRun()
	if !w.venus.windowDeadline.IsZero() {
		t.Fatal("resetRun must clear the window deadline")
	}
}

// CMD2044（LEGION_FAIL 内容 106）= 副本内放弃：ACK 1+8B、run 复位为全新
// 未选状态（不是作废）、回城序列携带 dungeon_leave_ack（主循环据此清
// activeDungeon）、序列末尾垫等待态 N2655。
func TestVenusRetreatCmd2044(t *testing.T) {
	w := &worldSession{channelType: 99,
		role:  database.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
		venus: &venusRun{choice: 2, stage: 1, relicMask: 1 << 3, cleared: [4]bool{true, false, false, false},
			pending: []protocol.UnassignedMonster115{{Grid: [2]byte{1, 0}, Entity: 4096, Template: 109016980}}},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]},
			Loaded:     true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}},
		},
	}
	body := venusEnvelopeWithContent(0, 0, 0, 0)
	outside := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7}}
	if _, _, err := outside.handleVenusRequest(body, legion.CmdFail); err == nil {
		t.Fatal("fail outside a venus dungeon must be refused")
	}
	plan, notes, err := w.handleVenusRequest(body, legion.CmdFail)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) < 5 {
		t.Fatalf("retreat plan = %+v", plan)
	}
	ack := plan[0]
	if ack.Name != "venus_fail_ack" || ack.Kind != 1 || ack.ID != legion.CmdFail {
		t.Fatalf("fail ack = %+v", ack)
	}
	if len(ack.Payload) != legion.FailAckSize || ack.Payload[0] != 1 {
		t.Fatalf("fail ack payload = %x, want 1+8B success", ack.Payload)
	}
	if plan[1].Name != "dungeon_leave_ack" || plan[1].ID != 42 {
		t.Fatalf("route must carry dungeon_leave_ack: %+v", plan[1])
	}
	waiting := plan[len(plan)-1]
	if waiting.Name != "venus_info_waiting" || waiting.ID != legion.NotiVenusInfo || waiting.Payload[2] != 0xff || binary.LittleEndian.Uint32(waiting.Payload[3:]) != 2 {
		t.Fatalf("trailing N2655 = %+v payload %x", waiting, waiting.Payload)
	}
	run := w.venus
	if run == nil || run.choice != 0xff || run.stage != 0 || run.clearedCount() != 0 || run.relicMask != 0 || len(run.pending) != 0 {
		t.Fatalf("retreat must reset the run, got %+v", run)
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_retreated" {
		t.Fatalf("retreat notes = %v", notes)
	}
	// 命令分发按内容号 106 识别 CMD2044。
	if !isVenusRequest(legion.CmdFail, body) {
		t.Fatal("CMD2044 with content 106 must be recognized as a venus request")
	}
}

// CMD72（EPLP_COMMAND）= 副本内右上角「撤退」：focus（State2）只回执不清场，
// 真正退出（State1）ACK 后走回城序列；进度保留（第二十二轮），第三十三轮起
// 难度与遗物一并保留——序列末尾的 N2655 带权威已选难度（Choice≠FF），客户端
// 点 Open 直接进「已选择X。确定要进入吗？」变更提示，不再弹三卡片自由重选。
// 请求体取自 2026-10-04 12:27 实机帧 01020100…。
func TestVenusSettlementExitRetreatCmd72(t *testing.T) {
	exit := make([]byte, 16)
	exit[0], exit[1], exit[2] = 1, 2, 1
	if _, err := protocol.DecodeSettlementExit(exit); err != nil {
		t.Fatal(err)
	}
	focus := append([]byte(nil), exit...)
	focus[0] = 2
	w := &worldSession{channelType: 99, completionSent: true,
		role:  database.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
		venus: &venusRun{choice: 2, stage: 0, relicMask: 1, entered: true,
			pending: []protocol.UnassignedMonster115{{Grid: [2]byte{1, 0}, Entity: 4096, Template: 109016980}}},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
			Loaded:     true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{
				{Entity: 4096, Rank: 0, Team: 100},
				{Entity: 4097, Rank: 0, Team: 100},
				{Entity: 4098, Rank: 0, Team: 100},
			},
		},
	}
	if _, plan, err := w.settlementExit(focus); err != nil || len(plan) != 1 || plan[0].Name != "settlement_focus_ack" || w.venus == nil || w.venus.choice != 2 {
		t.Fatalf("focus must keep the run: plan=%v err=%v", plan, err)
	}
	pending, plan, err := w.settlementExit(exit)
	if err != nil || pending != nil || len(plan) < 4 {
		t.Fatalf("retreat exit: pending=%v plan=%v err=%v", pending, plan, err)
	}
	if plan[0].Name != "settlement_exit_ack" || plan[0].Kind != 1 || plan[0].ID != 72 || !bytes.Equal(plan[0].Payload, []byte{1, 1, 2}) {
		t.Fatalf("retreat ACK = %+v", plan[0])
	}
	for i, want := range []uint16{72, 3, 23, 24} {
		if plan[i].ID != want {
			t.Fatalf("route[%d]=%d, want %d", i, plan[i].ID, want)
		}
	}
	waiting := plan[len(plan)-1]
	if waiting.Name != "venus_info_waiting" || waiting.ID != legion.NotiVenusInfo || waiting.Payload[2] != 2 || binary.LittleEndian.Uint32(waiting.Payload[3:]) != 2 {
		t.Fatalf("trailing N2655 = %+v payload %x", waiting, waiting.Payload)
	}
	run := w.venus
	if run == nil || run.choice != 2 || run.stage != 0 || run.clearedCount() != 0 || run.relicMask != 1 || !run.entered || len(run.pending) != 0 {
		t.Fatalf("retreat must keep the locked run, got %+v", run)
	}
}

// CMD2290 Action4（更改难度重选）：先回权威未选择 N2655（ChoiceFF）再回
// action4 ACK（带新截止时间）；run 的选择被清掉，后续 action2 重新确认。
// 副本内 / 未开战必须拒绝——13:33 会话实机：此前的静默拒绝让客户端作战
// 窗口状态机卡死（卡片点不动、后续点 Open 无上行）。
func TestVenusOperationReopenAction4(t *testing.T) {
	w := &worldSession{channelType: 99,
		role:     database.Character{WireID: 7, ID: 7, Name: "VenusCap"},
		dungeons: &catalog.DungeonCatalog{},
		venus:    &venusRun{choice: 2, stage: 0},
	}
	reopen := append(make([]byte, legion.EnvelopeSize), 4, 0, 0, 0, 255)
	inside := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus:         &venusRun{choice: 2},
		activeDungeon: &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]}},
	}
	if _, _, err := inside.handleVenusRequest(reopen, legion.CmdVenusOperationSelect); err == nil {
		t.Fatal("reopen inside a dungeon must be refused")
	}
	noRun := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7}}
	if _, _, err := noRun.handleVenusRequest(reopen, legion.CmdVenusOperationSelect); err == nil {
		t.Fatal("reopen before start must be refused")
	}
	plan, notes, err := w.handleVenusRequest(reopen, legion.CmdVenusOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != legion.NotiVenusInfo || plan[1].ID != legion.CmdVenusOperationSelect {
		t.Fatalf("reopen plan = %+v", plan)
	}
	if plan[0].Payload[2] != 0xff || binary.LittleEndian.Uint32(plan[0].Payload[3:]) != 2 {
		t.Fatalf("reopened N2655 choice/state = %d/%d", plan[0].Payload[2], binary.LittleEndian.Uint32(plan[0].Payload[3:]))
	}
	ack := plan[1].Payload
	if len(ack) != 15 || ack[0] != 1 || binary.LittleEndian.Uint32(ack[1:]) != 4 || ack[5] != 0xff || ack[6] != 0 || binary.LittleEndian.Uint32(ack[7:]) == 0 {
		t.Fatalf("reopen ack = %x", ack)
	}
	if w.venus.choice != 0xff {
		t.Fatalf("run choice after reopen = %d, want ff", w.venus.choice)
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_operation_reopened" {
		t.Fatalf("reopen notes = %v", notes)
	}
	// 重选后重新确认难度，进图链路恢复。
	confirm := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 1)
	if _, _, err = w.handleVenusRequest(confirm, legion.CmdVenusOperationSelect); err != nil {
		t.Fatal(err)
	}
	if w.venus.choice != 1 {
		t.Fatalf("reconfirmed choice = %d, want 1", w.venus.choice)
	}
	// 第三十三轮：进过图（entered）的挑战难度锁定——action4 重选回原生负包
	//（公共 00 + 错误码 4），不回 N2655、不清锁、不重开三卡窗。
	w.venus.entered = true
	plan, notes, err = w.handleVenusRequest(reopen, legion.CmdVenusOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].Name != "venus_operation_locked_refused" || plan[0].Kind != 1 ||
		plan[0].ID != legion.CmdVenusOperationSelect || !bytes.Equal(plan[0].Payload, []byte{0, 4, 0}) {
		t.Fatalf("locked reopen must refuse natively: %+v payload %x", plan, plan[0].Payload)
	}
	if w.venus.choice != 1 || !w.venus.entered {
		t.Fatalf("locked reopen must keep the run, got %+v", w.venus)
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_operation_locked" {
		t.Fatalf("locked reopen notes = %v", notes)
	}
	// 锁定期间 action1 开窗与换档 action2 同样拒绝；同档重复确认放行。
	open := append(make([]byte, legion.EnvelopeSize), 1, 0, 0, 0, 255)
	plan, _, err = w.handleVenusRequest(open, legion.CmdVenusOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].Name != "venus_operation_locked_refused" {
		t.Fatalf("locked open must refuse: %+v", plan)
	}
	if !w.venus.windowDeadline.IsZero() {
		t.Fatal("locked open must not arm the window deadline")
	}
	switchChoice := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 0)
	plan, _, err = w.handleVenusRequest(switchChoice, legion.CmdVenusOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].Name != "venus_operation_locked_refused" || w.venus.choice != 1 {
		t.Fatalf("locked switch must refuse: %+v choice=%d", plan, w.venus.choice)
	}
	sameChoice := append(make([]byte, legion.EnvelopeSize), 2, 0, 0, 0, 1)
	if _, _, err = w.handleVenusRequest(sameChoice, legion.CmdVenusOperationSelect); err != nil {
		t.Fatal(err)
	}
	if w.venus.choice != 1 {
		t.Fatal("same-choice reconfirm must pass")
	}
}

// 第三十三轮 BUG3：撤退/超时保留的锁定 run 重新开战（CMD2043）——waiting 向
// 量带权威已选难度（Choice≠FF、State2、Stage=下一个待进阶段），run 的选择与
// 遗物掩码原样保留；未选过难度的重开维持未选等待态。
func TestVenusRunRestartKeepsLockedChoice(t *testing.T) {
	startBody, err := hex.DecodeString(venusLiveStartRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{channelType: 99,
		role:       database.Character{WireID: 7, ID: 7, Name: "VenusCap"},
		characters: &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
		dungeons:   &catalog.DungeonCatalog{},
		venus: &venusRun{choice: 2, stage: 1, relicMask: 1 << 3, entered: true,
			cleared: [4]bool{true, false, false, false}},
	}
	w.soloPartyReady = true
	plan, _, err := w.handleVenusRequest(startBody, legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[1].ID != legion.NotiVenusInfo {
		t.Fatalf("restart plan = %+v", plan)
	}
	body := plan[1].Payload
	if body[2] != 2 || binary.LittleEndian.Uint32(body[3:]) != 2 || binary.LittleEndian.Uint32(body[11:]) != 1 {
		t.Fatalf("restart waiting must carry the locked choice: %x", body)
	}
	if w.venus.choice != 2 || w.venus.relicMask != 1<<3 || w.venus.clearedCount() != 1 {
		t.Fatalf("restart must keep the locked run: %+v", w.venus)
	}
	// 撤退自第 1 关（未清任何进度）的 run 同样锁定：chosen 态 Stage=0。
	w.venus = &venusRun{choice: 0, entered: true}
	plan, _, err = w.handleVenusRequest(startBody, legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	if body = plan[1].Payload; body[2] != 0 || binary.LittleEndian.Uint32(body[3:]) != 2 {
		t.Fatalf("kept zero-progress run must stay chosen: %x", body)
	}
	// 未选过难度的全新开战维持未选等待态（Choice FF）。
	w.venus = &venusRun{choice: 0xff}
	plan, _, err = w.handleVenusRequest(startBody, legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	if body = plan[1].Payload; body[2] != 0xff {
		t.Fatalf("fresh restart must stay unchosen: %x", body)
	}
}

// 清怪投影（2655 文档 boss death projects to the next Stage/Target）：阶段
// 最后一只战斗怪确认死亡后，权威 N2655 推进到下一阶段。2026-10-04 14:41
// 实机：客户端清完三只圣物怪后按 N2655 阶段记录直进（CMD2062），没有这份
// 投影就永远重选第 1 关，形成无限进图循环。
func TestVenusStageProjectionOnLastDeath(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 1, stage: 0},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011986},
			Loaded:     true,
			Monsters: []protocol.DungeonMonster{
				{Entity: 4096, Team: 100},
				{Entity: 4097, Team: 100},
			},
			Dead: map[uint16]bool{4096: true},
		},
	}
	death := func(entity uint16) []byte {
		p := make([]byte, 64)
		binary.LittleEndian.PutUint32(p, uint32(entity))
		binary.LittleEndian.PutUint16(p[4:], 7) // killer = 本角色 WireID
		return p
	}
	notes := 0
	plan, err := w.monsterDeath(death(4097), func(map[string]any) { notes++ })
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pkt := range plan {
		if pkt.Name == "venus_info_stage_cleared" {
			found = true
			if pkt.ID != legion.NotiVenusInfo {
				t.Fatalf("stage cleared noti id = %d", pkt.ID)
			}
			if binary.LittleEndian.Uint32(pkt.Payload[11:]) != 1 {
				t.Fatalf("projected stage = %d, want 1", binary.LittleEndian.Uint32(pkt.Payload[11:]))
			}
			if pkt.Payload[23+12] != 1 {
				t.Fatalf("record1 target = %d, want 1", pkt.Payload[23+12])
			}
		}
	}
	if !found {
		t.Fatalf("last death must publish the advanced N2655: %d packets", len(plan))
	}
	if !w.venus.cleared[0] || w.venus.stage != 1 {
		t.Fatalf("run after projection = cleared:%v stage:%d", w.venus.cleared[0], w.venus.stage)
	}
	// 幂等：阶段已推进后不再发布。
	if out := w.venusStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("projection must be one-shot, got %+v", out)
	}
	// 非维纳斯副本不投影。
	foreign := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 1},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: 100003126},
			Monsters:   []protocol.DungeonMonster{{Entity: 4096, Team: 100}},
			Dead:       map[uint16]bool{},
		},
	}
	if out := foreign.venusStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("non-venus dungeon must not project: %+v", out)
	}
	// 终点阶段（choice 2 降临 = 第 3 关）：标记通关、不发布下一阶段，也不直接
	// 发 N31——终局翻牌链（N31→N2252→N2253）由随后的 completeDungeon →
	// completeVenusStage 发出（仿伊斯家族形状，军团共用 N2252/N2253）。
	// 绝不能在这里发通用 N31，那会触发 CMD46 通用结算链（N34/N35/N261+8张牌），
	// 团本里不存在那种结算面板。
	end := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 2, stage: 3, cleared: [4]bool{true, true, true, false}},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[3]},
			Monsters:   []protocol.DungeonMonster{{Entity: 4096, Team: 100}},
			Dead:       map[uint16]bool{4096: true},
		},
	}
	endMarked := false
	out := end.venusStageProjection(func(n map[string]any) { endMarked = n["endpoint"] == true })
	if out != nil {
		t.Fatalf("endpoint projection must not publish packets (chain lives in completeVenusStage): %+v", out)
	}
	if !end.venus.cleared[3] || !endMarked {
		t.Fatalf("endpoint stage must be marked cleared with the endpoint note")
	}
	if !end.activeDungeon.Completed() {
		t.Fatal("endpoint projection must mark the session completed for the settlement flow")
	}
}

// 转阶段直进（CMD2062）：客户端清怪后按 N2655 推进后的阶段记录直进下一阶段
// 副本；应答带选择 UI 头（gate_ack15 + N27）。终点关已通关后的重选用回城
// 序列终止（终局结算分支未实现）。合成目录驱动真实 Select。
func TestVenusDirectMoveStageTransition(t *testing.T) {
	c := &catalog.DungeonCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)},
		Dungeons: map[uint32]catalog.DungeonDefinition{
			legion.VenusStageDungeons[0]: {ID: legion.VenusStageDungeons[0], MinimumLevel: 1,
				Mazes: []catalog.DungeonMaze{{Index: 0, Quest: 0, Start: [2]byte{1, 0}, Boss: [2]byte{0, 0},
					Rooms: []catalog.DungeonRoom{{X: 1, Y: 0, Map: 100011986}, {X: 0, Y: 0, Map: 100011985}}}}},
			legion.VenusStageDungeons[1]: {ID: legion.VenusStageDungeons[1], MinimumLevel: 1,
				Mazes: []catalog.DungeonMaze{{Index: 0, Quest: 0, Start: [2]byte{1, 0}, Boss: [2]byte{0, 0},
					Rooms: []catalog.DungeonRoom{{X: 1, Y: 0, Map: 100011988}, {X: 0, Y: 0, Map: 100011985}}}}},
		},
		Maps: map[uint32]catalog.ScriptRecord{
			100011986: {Cells: []pvf.Token{{Type: 3, Text: "[monster]"}}},
			100011985: {Cells: []pvf.Token{{Type: 3, Text: "[monster]"}}},
			100011988: {Cells: []pvf.Token{{Type: 3, Text: "[monster]"}}},
		},
	}
	directMove := func(id uint32) protocol.DungeonDirectMove {
		var r protocol.DungeonDirectMove
		r.ID = id
		return r
	}
	// 门禁：无 run / 非维纳斯目标 / 跳阶段。
	noRun := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7}, dungeons: c,
		activeDungeon: &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]}}}
	if _, _, err := noRun.enterVenusStageDirectMove(directMove(legion.VenusStageDungeons[1])); err == nil {
		t.Fatal("direct move without a run must be refused")
	}
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7}, level: 115, dungeons: c,
		venus: &venusRun{choice: 1, stage: 0}}
	if _, _, err := w.enterVenusStageDirectMove(directMove(legion.VenusStageDungeons[1])); err == nil {
		t.Fatal("direct move to stage 1 before stage 0 cleared must be refused")
	}
	// 同关守卫（2026-10-04 17:00/17:42 实机）：未清 boss 时玩家走进门矩形，
	// 客户端 procCheckDirectMove 进 ForceMove 并发 2062 直进当前关。静默拒绝
	// 会让 ForceMove 每帧持续拉拽玩家（抽搐、吞技能/跳跃输入）直到 boss 死亡；
	// 改回通用拒绝应答（Result=00 + 错误码 4）让客户端放弃过门。
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]}}
	w.venus = &venusRun{choice: 1, stage: 1}
	pending, plan, err := w.enterVenusStageDirectMove(directMove(legion.VenusStageDungeons[1]))
	if err != nil || pending != nil {
		t.Fatalf("same-stage refusal: pending=%v err=%v", pending, err)
	}
	if len(plan) != 1 || plan[0].ID != 2062 || plan[0].Kind != 1 || !bytes.Equal(plan[0].Payload, protocol.VenusDirectMoveAck()) {
		t.Fatalf("same-stage refusal plan = %+v payload %x", plan, plan[0].Payload)
	}
	if w.venus.stage != 1 || w.activeDungeon.Definition.ID != legion.VenusStageDungeons[1] {
		t.Fatal("refused direct move must leave the current session untouched")
	}
	// 清怪投影推进后：直进下一阶段成功，应答带选择 UI 头与进图序列。
	w.venus.cleared[0] = true
	w.venus.stage = 1
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]}}
	s, plan, err := w.enterVenusStageDirectMove(directMove(legion.VenusStageDungeons[1]))
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.Definition.ID != legion.VenusStageDungeons[1] {
		t.Fatalf("direct move session = %+v", s)
	}
	if len(plan) < 4 {
		t.Fatalf("direct move plan = %d packets", len(plan))
	}
	if plan[0].ID != 15 || plan[0].Name != "dungeon_gate_ack" || plan[1].ID != 27 {
		t.Fatalf("selection head = %+v / %+v", plan[0], plan[1])
	}
	hasSelectAck, hasStartMap := false, false
	for _, pkt := range plan {
		if pkt.Name == "dungeon_select_ack" && pkt.ID == 16 {
			hasSelectAck = true
		}
		if pkt.Name == "dungeon_start_map_sent" && pkt.ID == 29 {
			hasStartMap = true
		}
	}
	if !hasSelectAck || !hasStartMap {
		t.Fatalf("direct move plan must carry the select ack and the start map")
	}
	if w.venus.stage != 1 {
		t.Fatal("direct move must advance nothing; the dispatch adopts the returned session")
	}
	// 终点关已通关后的重选：回城序列终止（dungeon_leave_ack 驱动主循环清理）。
	end := &worldSession{channelType: 99,
		role:          database.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		state:         database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
		dungeons:      c,
		venus:         &venusRun{choice: 2, stage: 3, cleared: [4]bool{true, true, true, true}},
		activeDungeon: &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[3]}},
	}
	pending, plan, err = end.enterVenusStageDirectMove(directMove(legion.VenusStageDungeons[3]))
	if err != nil || pending != nil {
		t.Fatalf("terminal re-entry: pending=%v err=%v", pending, err)
	}
	if plan[0].Name != "dungeon_leave_ack" || plan[0].ID != 42 {
		t.Fatalf("terminal plan must start with the leave ack: %+v", plan[0])
	}
	waiting := plan[len(plan)-1]
	if waiting.Name != "venus_info_terminal_waiting" || waiting.ID != legion.NotiVenusInfo || waiting.Payload[2] != 0xff {
		t.Fatalf("terminal plan must end with the waiting N2655: %+v", waiting)
	}
	if end.venus == nil || end.venus.choice != 0xff || end.venus.clearedCount() != 0 {
		t.Fatalf("terminal re-entry must reset the run: %+v", end.venus)
	}
}

// CMD2329（MONSTER_HISTORY_LOG）维纳斯分支：`Venus_N_Phase_Shift` 是 boss
// 行为脚本的技能日志（圈圈/抓取/变身预备），**不是击杀信号**——玩家未攻击时
// 也会上报（2026-10-04 16:17 实机进图 4-5 秒即有）。此前把它当击杀处理，boss
// 每次放技能就被误杀：N38/N2655 打断客户端演出（抓取弹到半空、抽搐、技能进
// 冷却不释放）、二阶段变身永不发生。现在只记诊断日志，死亡仍由二阶段打空后
// 的真实 CMD39 驱动。
func TestVenusPhaseShiftLogIsNotAKill(t *testing.T) {
	live := "Venus_2_Phase_Shift, Monster HP : 0.00, Dungeon Time : 4"
	payload := func(text string) []byte {
		p := make([]byte, 4+256+32)
		binary.LittleEndian.PutUint32(p, 2)
		copy(p[4:], text)
		binary.LittleEndian.PutUint32(p[4+256:], 109016983)
		return p
	}
	noted := 0
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 2, stage: 1},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]},
			Room:       catalog.DungeonRoom{X: 1, Y: 0, Map: 100011987},
			Loaded:     true,
			Monsters:   []protocol.DungeonMonster{{Entity: 4096, Template: 109016983, Rank: 3, Team: 100}},
			Dead:       map[uint16]bool{},
		},
	}
	plan, err := w.scaleStatus(payload(live), func(map[string]any) { noted++ })
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 0 {
		t.Fatalf("phase shift log must not answer anything: %+v", plan)
	}
	if noted != 1 || w.activeDungeon.Dead[4096] {
		t.Fatalf("log must be recorded without killing the boss: noted=%d dead=%v", noted, w.activeDungeon.Dead[4096])
	}
	if w.venus.cleared[1] || w.venus.stage != 1 {
		t.Fatalf("run must be untouched: %+v", w.venus)
	}
	// 非 Phase_Shift 文本连日志都不记。
	noted = 0
	if plan, err = w.scaleStatus(payload("SomeOther_Log, Monster HP : 57.81"), func(map[string]any) { noted++ }); err != nil || len(plan) != 0 || noted != 0 {
		t.Fatalf("unrelated log must be ignored: plan=%v noted=%d err=%v", plan, noted, err)
	}
}

// CMD2059（PLAYER_REVIVE_WHEN_PHASE_CHANGE）：boss 形态转换技能把玩家
// phase-change 击杀后的免费复活请求（DGN [player revive when phase change
// tag max count]）。此前静默，客户端复活流程挂起——角色卡在死亡/复活中间态
// （半空站起来、抽搐、技能进冷却不释放、不能跳跃，2026-10-04 17:00 会话 9 次
// 击杀全部挂起）。现在回放伊斯官服 ACK2059 的 16B 常量形状。
func TestVenusPhaseReviveAck(t *testing.T) {
	w := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 2, stage: 1},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]},
			Loaded:     true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: 109016983, Rank: 3, Team: 100}},
		},
	}
	plan := w.venusPhaseRevive(make([]byte, 37))
	if len(plan) != 1 || plan[0].ID != legion.CmdVenusPhaseRevive || plan[0].Kind != 1 {
		t.Fatalf("revive plan = %+v", plan)
	}
	ack := plan[0].Payload
	if len(ack) != 16 || ack[0] != 1 || ack[6] != 0xed || !bytes.Equal(ack[8:13], []byte{0x94, 0xf5, 0xc2, 0x8b, 0x3c}) {
		t.Fatalf("revive ack = %x", ack)
	}
	// 非维纳斯副本与副本外保持静默（该命令只在维纳斯语境出现过）。
	foreign := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7},
		activeDungeon: &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100005014}, Loaded: true}}
	if plan = foreign.venusPhaseRevive(make([]byte, 37)); plan != nil {
		t.Fatalf("non-venus revive must stay silent: %+v", plan)
	}
	outside := &worldSession{channelType: 99, role: database.Character{ID: 7, WireID: 7}}
	if plan = outside.venusPhaseRevive(make([]byte, 37)); plan != nil {
		t.Fatalf("revive outside a dungeon must stay silent: %+v", plan)
	}
}

// 终局翻牌按 2026-10-04 用户方案（官服维纳斯口径）：第一排 8 位 = 5 随机
// 装备（池子 roll，各 1）+ 3 难度材料（深渊券 10/15/25、飘散丝绸 60/100/150、
// 碎裂丝绸 16/20/25）；第二排 5 件固定材料各 1（融合石礼盒/竞拍券/眼泪×2/
// 花瓣）。N2252 @1600 起 8 条 8B 条目、@7760 阶段 token；N2253 40B 记录
// 步长 5 条全展示位（flag=01）。
func TestVenusTerminalFlipRewardsByChoice(t *testing.T) {
	gear := []uint32{115000001, 115000002, 115000003, 115000004, 115000005}
	for _, c := range []struct {
		choice                        byte
		abyss, looseSilk, crackedSilk uint32
	}{
		{0, 10, 60, 16},
		{1, 15, 100, 20},
		{2, 25, 150, 25},
	} {
		got := venusRewardItems(c.choice, gear)
		if len(got) != 13 {
			t.Fatalf("choice %d rewards len=%d, want 13", c.choice, len(got))
		}
		for i := 0; i < legion.VenusFlipGearCount; i++ {
			if got[i] != (loot.Award{Template: gear[i], Amount: 1}) {
				t.Fatalf("choice %d gear slot %d = %+v", c.choice, i, got[i])
			}
		}
		if got[5] != (loot.Award{Template: 10362429, Amount: c.abyss}) {
			t.Fatalf("choice %d abyss ticket = %+v", c.choice, got[5])
		}
		if got[6] != (loot.Award{Template: 10404330, Amount: c.looseSilk}) {
			t.Fatalf("choice %d loose silk = %+v", c.choice, got[6])
		}
		if got[7] != (loot.Award{Template: 10404667, Amount: c.crackedSilk}) {
			t.Fatalf("choice %d cracked silk = %+v", c.choice, got[7])
		}
		wantRow2 := []loot.Award{
			{Template: 10404730, Amount: 1}, // 维纳斯神器套装融合石自选礼盒
			{Template: 10403546, Amount: 1}, // 维纳斯竞拍参与券
			{Template: 10404807, Amount: 1}, // 天帷巨兽的眼泪
			{Template: 10404679, Amount: 1}, // 天帷巨兽的眼泪（可交易1次）
			{Template: 10404346, Amount: 1}, // 凋零的纯洁花瓣
		}
		for i, want := range wantRow2 {
			if got[8+i] != want {
				t.Fatalf("choice %d row-2 item %d = %+v, want %+v", c.choice, i, got[8+i], want)
			}
		}

		// N2252：行布局与 protocol.LegionBasicRewards115 一致——行0-1 @0/@40（40B），
		// 行2+ @1600+44*(i-2)（44B）；每行 @0 template u32、@4 value u32。
		// 前5行是装备（value=1），后3行是材料（value=数量）。
		// @7760 低2B为阶段token。
		basic, err := legion.VenusBasicClearReward(c.choice, 2, gear)
		if err != nil {
			t.Fatal(err)
		}
		if len(basic) != 7772 {
			t.Fatalf("N2252 len=%d, want 7772", len(basic))
		}
		n2252Offset := func(i int) int {
			if i < 2 {
				return 40 * i
			}
			return 1600 + 44*(i-2)
		}
		for i := 0; i < 8; i++ {
			off := n2252Offset(i)
			if tpl := binary.LittleEndian.Uint32(basic[off:]); tpl != got[i].Template {
				t.Fatalf("choice %d N2252 entry %d template=%d, want %d", c.choice, i, tpl, got[i].Template)
			}
			wantVal := got[i].Amount
			if v := binary.LittleEndian.Uint32(basic[off+4:]); v != wantVal {
				t.Fatalf("choice %d N2252 entry %d value=%d, want %d", c.choice, i, v, wantVal)
			}
		}
		// 第8行（索引8）起始位置之后应保持零（第一排只有8项）。
		nextOff := n2252Offset(8)
		if basic[nextOff] != 0 {
			t.Fatalf("N2252 byte after the last entry must stay zero, got %d", basic[nextOff])
		}

		// N2253：5 条 40B 记录全展示位（flag=01、count=1、const 03 @9）。
		additional, err := legion.VenusAdditionalClearReward()
		if err != nil {
			t.Fatal(err)
		}
		if len(additional) != 2405 {
			t.Fatalf("N2253 len=%d, want 2405", len(additional))
		}
		for i, want := range wantRow2 {
			off := 40 * i
			if additional[off] != 1 {
				t.Fatalf("N2253 record %d flag=%d, want 1", i, additional[off])
			}
			if tpl := binary.LittleEndian.Uint32(additional[off+1:]); tpl != want.Template {
				t.Fatalf("N2253 record %d item=%d, want %d", i, tpl, want.Template)
			}
			if additional[off+5] != 1 {
				t.Fatalf("N2253 record %d count=%d, want 1", i, additional[off+5])
			}
			if additional[off+9] != 3 {
				t.Fatalf("N2253 record %d const=%d, want 3", i, additional[off+9])
			}
		}
	}

	// 越界与超量防御：stage 4 拒绝；装备 roll 超过 5 位拒绝。
	if _, err := legion.VenusBasicClearReward(0, 4, gear); err == nil {
		t.Fatal("stage 4 must be rejected")
	}
	if _, err := legion.VenusBasicClearReward(0, 0, append(append([]uint32{}, gear...), 115000006)); err == nil {
		t.Fatal("gear roll beyond 5 slots must be rejected")
	}

	// 池子 roll：结果不重复且来自池子；n 超过池子大小时收窄；空池子返回 nil。
	pool := []uint32{1, 2, 3, 4, 5, 6, 7}
	poolSet := make(map[uint32]bool, len(pool))
	for _, v := range pool {
		poolSet[v] = true
	}
	rolled := legion.VenusRollFlipGear(pool, 5)
	if len(rolled) != 5 {
		t.Fatalf("roll len=%d, want 5", len(rolled))
	}
	seen := map[uint32]bool{}
	for _, v := range rolled {
		if seen[v] || !poolSet[v] {
			t.Fatalf("roll produced %d (dup=%v inPool=%v)", v, seen[v], poolSet[v])
		}
		seen[v] = true
	}
	if got := legion.VenusRollFlipGear(nil, 5); got != nil {
		t.Fatalf("empty pool roll = %v, want nil", got)
	}
	if got := legion.VenusRollFlipGear([]uint32{1, 2}, 5); len(got) != 2 {
		t.Fatalf("roll beyond pool size = %v, want 2 items", got)
	}

	// cardPlan：Items[8] 正好是第一排（5 装备 + 3 难度材料）；N35 卡组与
	// CMD71 领取同源（venusCardPlan 冻结进 store，pickFrozenCard 校验
	// stored==plan）。
	plan := venusCardPlan(2, gear, "run-1", "src", 145)
	if plan.Run != "run-1" || plan.Source != "src" || plan.Level != 145 || plan.Model != "venus-terminal-v2" {
		t.Fatalf("plan header = %+v", plan)
	}
	for i := 0; i < legion.VenusFlipGearCount; i++ {
		if plan.Items[i] != (loot.Award{Template: gear[i], Amount: 1}) {
			t.Fatalf("plan gear slot %d = %+v", i, plan.Items[i])
		}
	}
	if plan.Items[5] != (loot.Award{Template: 10362429, Amount: 25}) ||
		plan.Items[6] != (loot.Award{Template: 10404330, Amount: 150}) ||
		plan.Items[7] != (loot.Award{Template: 10404667, Amount: 25}) {
		t.Fatalf("plan materials = %+v", plan.Items[5:8])
	}
	// JSON 往返相等（pickFrozenCard 的 stored==plan 校验依赖它）。
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var round loot.CardPlan
	if err = json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if round != plan {
		t.Fatalf("plan JSON round-trip mismatch: %+v vs %+v", round, plan)
	}
}

// CMD2291（GET_VENUS_RELIC）：源配对（模板,ID）+ 本房登记校验；首次领取
// 先发权威 N2655（带七位掩码）再回 ACK；重复领取/配对错误按 00+u16 0 拒绝
// （拒绝也应答，客户端待答登记靠它清除）。
func TestVenusRelicReport(t *testing.T) {
	w := &worldSession{channelType: 99,
		role:  database.Character{ID: 7, WireID: 7},
		venus: &venusRun{choice: 1, stage: 0},
		activeDungeon: &dungeon.Session{
			Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
			Loaded:     true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{
				{Entity: 4096, Template: 109016980, Team: 100},
				{Entity: 4097, Template: 109016981, Team: 100},
				{Entity: 4098, Template: 109016982, Team: 100},
			},
		},
	}
	relic := func(template uint32, id byte) []byte {
		p := make([]byte, legion.EnvelopeSize)
		for i := range p {
			p[i] = 0xff
		}
		p[legion.EnvelopeSize-1] = 0
		p = binary.LittleEndian.AppendUint32(p, template)
		return append(p, id)
	}
	plan, notes, err := w.handleVenusRequest(relic(109016980, 5), legion.CmdVenusRelic)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != legion.NotiVenusInfo || plan[1].ID != legion.CmdVenusRelic {
		t.Fatalf("relic plan = %+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[0].Payload[77:]); got != 1<<5 {
		t.Fatalf("published relic mask = %x, want %x", got, 1<<5)
	}
	if len(plan[1].Payload) != 6 || plan[1].Payload[0] != 1 || binary.LittleEndian.Uint32(plan[1].Payload[1:]) != 109016980 || plan[1].Payload[5] != 5 {
		t.Fatalf("relic ack = %x", plan[1].Payload)
	}
	if len(notes) != 1 || notes[0]["kind"] != "venus_relic_collected" {
		t.Fatalf("relic notes = %v", notes)
	}
	// 配对错误（剪刀模板报成香水 ID）与本房未登记的模板都按 3B 拒绝。
	for _, bad := range [][]byte{relic(109016980, 6), relic(109016985, 3)} {
		plan, _, err = w.handleVenusRequest(bad, legion.CmdVenusRelic)
		if err != nil || len(plan) != 1 || !bytes.Equal(plan[0].Payload, legion.VenusRelicRefused()) {
			t.Fatalf("bad relic report must answer the 3B refusal: plan=%v err=%v", plan, err)
		}
	}
	// 已获得的位再报一次：拒绝但不改掩码。
	plan, _, err = w.handleVenusRequest(relic(109016980, 5), legion.CmdVenusRelic)
	if err != nil || len(plan) != 1 || !bytes.Equal(plan[0].Payload, legion.VenusRelicRefused()) {
		t.Fatalf("duplicate relic report must be refused: plan=%v err=%v", plan, err)
	}
	if w.venus.relicMask != 1<<5 {
		t.Fatalf("relic mask = %x, want %x", w.venus.relicMask, 1<<5)
	}
}

// 第三十四轮（用户口径）：退出队伍 = 放弃攻坚——返回选择角色（CMD7）时
// 先于菜单应答补发 State0 关闭态（客户端在过场里按 manager 最后状态重建
// VENUS_MAIN_INFO_WINDOW，171948 会话 trace；关闭态让重建渲染为空、选角
// 界面不再残留面板），同时 run 整体作废（难度/进度/遗物全清零，重进走
// CMD12+CMD2043 全新开团）；关闭态用 ChoiceFF/掩码0，客户端遗物显示一并
// 清掉。保留进度只属于「撤退回待机区」（CMD72，队伍还在）。
// 1566 退出爆发期间的 N2655 补发已删除：拆屏期到达只会把窗口重建打开
// （172342 会话 trace Close 3789 → RECV N2655 → Open 3789）。
func TestVenusExitToSelectAbandonsRun(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	events := make(chan map[string]any, 8)
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys: make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 99,
			role: database.Character{ID: 7, WireID: 7},
			venus: &venusRun{choice: 2, stage: 1, relicMask: 1 << 3, entered: true,
				cleared: [4]bool{true, false, false, false}}},
		event: func(e map[string]any) { events <- e }}
	c.output = newConnectionOutput(server, c.keys, "test", c.event)
	go func() {
		if got := c.dispatchVenus(&clientRequest{frame: wire.Frame{Type: 1, ID: 7}, verified: true}); got != dispatchNext {
			t.Error(got)
		}
	}()
	peer.SetReadDeadline(time.Now().Add(4 * time.Second))
	h := make([]byte, 16)
	if _, e := io.ReadFull(peer, h); e != nil {
		t.Fatal(e)
	}
	size := int(binary.LittleEndian.Uint32(h[3:7]))
	if size < 16 || size > wire.MaxPacketSize {
		t.Fatal(size)
	}
	b := make([]byte, size-16)
	if _, e := io.ReadFull(peer, b); e != nil {
		t.Fatal(e)
	}
	if id := binary.LittleEndian.Uint16(h[1:3]); id != legion.NotiVenusInfo {
		t.Fatal("exit close must be N2655, got", id)
	}
	plain, e := wire.DecryptPayload(c.keys, legion.NotiVenusInfo, b)
	if e != nil {
		t.Fatal(e)
	}
	// 关闭态：State0 + ChoiceFF + 掩码清零（run 已作废，权威状态归零）。
	if len(plain) < 85 || plain[2] != 0xff || binary.LittleEndian.Uint32(plain[3:]) != 0 ||
		binary.LittleEndian.Uint32(plain[77:]) != 0 {
		t.Fatalf("exit close state = %x", plain)
	}
	select {
	case e := <-events:
		if e["kind"] != "venus_exit_to_select" || e["abandoned"] != true {
			t.Fatal("unexpected event", e)
		}
	case <-time.After(time.Second):
		t.Fatal("exit not logged")
	}
	if c.worldState.venus != nil {
		t.Fatalf("leaving the party must abandon the run: %+v", c.worldState.venus)
	}
	// 终局已完成的 run 同样作废（重复 State0 是无害收尾）。
	done := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys: make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 99,
			venus: &venusRun{choice: 2, finalDone: true}},
		event: func(e map[string]any) { events <- e }}
	done.output = newConnectionOutput(server, done.keys, "test", done.event)
	go func() {
		if got := done.dispatchVenus(&clientRequest{frame: wire.Frame{Type: 1, ID: 7}, verified: true}); got != dispatchNext {
			t.Error(got)
		}
	}()
	// 读走关闭帧（net.Pipe 同步写，不读会卡住发送方）。
	peer.SetReadDeadline(time.Now().Add(4 * time.Second))
	if _, e := io.ReadFull(peer, h); e != nil {
		t.Fatal(e)
	}
	dsize := int(binary.LittleEndian.Uint32(h[3:7]))
	db := make([]byte, dsize-16)
	if _, e := io.ReadFull(peer, db); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-events:
		if e["kind"] != "venus_exit_to_select" {
			t.Fatal("finished run must be abandoned on exit too", e)
		}
	case <-time.After(time.Second):
		t.Fatal("finished run exit not logged")
	}
	if done.worldState.venus != nil {
		t.Fatal("finished run must be retired on exit")
	}
	// 非维纳斯频道不触发。
	town := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys:       make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 1, venus: &venusRun{choice: 2}},
		event:      func(e map[string]any) { events <- e }}
	town.output = newConnectionOutput(server, town.keys, "test", town.event)
	if got := town.dispatchVenus(&clientRequest{frame: wire.Frame{Type: 1, ID: 7}, verified: true}); got != dispatchNext {
		t.Fatal(got)
	}
	select {
	case e := <-events:
		if e["kind"] == "venus_exit_to_select" {
			t.Fatal("non-venus channel must not trigger the exit hook")
		}
	case <-time.After(200 * time.Millisecond):
	}
}

// 第三十五轮：BUG4 遗物重置钩子的重置包必须是 State0 关闭态——客户端在
// 城镇里周期性发 CMD35（位置同步），通关 CMD72 回城后第一条周期 CMD35 就
// 会命中本钩子（191905 会话 11:26:23 实证）；旧实现发 State2 等待态把右上
// 角面板重新顶起来。State0 掩码照样归零且面板保持关闭。
func TestVenusRelicResetKeepsPanelClosed(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	events := make(chan map[string]any, 8)
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys:       make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 99, pendingRelicReset: true},
		event:      func(e map[string]any) { events <- e }}
	c.output = newConnectionOutput(server, c.keys, "test", c.event)
	go func() {
		if got := c.dispatchVenus(&clientRequest{frame: wire.Frame{Type: 1, ID: 35}, verified: true}); got != dispatchNext {
			t.Error(got)
		}
	}()
	peer.SetReadDeadline(time.Now().Add(4 * time.Second))
	h := make([]byte, 16)
	if _, e := io.ReadFull(peer, h); e != nil {
		t.Fatal(e)
	}
	size := int(binary.LittleEndian.Uint32(h[3:7]))
	b := make([]byte, size-16)
	if _, e := io.ReadFull(peer, b); e != nil {
		t.Fatal(e)
	}
	if id := binary.LittleEndian.Uint16(h[1:3]); id != legion.NotiVenusInfo {
		t.Fatal("relic reset must be N2655, got", id)
	}
	plain, e := wire.DecryptPayload(c.keys, legion.NotiVenusInfo, b)
	if e != nil {
		t.Fatal(e)
	}
	if len(plain) < 85 || plain[2] != 0xff || binary.LittleEndian.Uint32(plain[3:]) != 0 ||
		binary.LittleEndian.Uint32(plain[77:]) != 0 {
		t.Fatalf("relic reset state = %x (want State0 closed, mask 0)", plain)
	}
	select {
	case e := <-events:
		if e["kind"] != "venus_relic_ui_reset" {
			t.Fatal("unexpected event", e)
		}
	case <-time.After(time.Second):
		t.Fatal("reset not logged")
	}
	if c.worldState.pendingRelicReset || c.worldState.lastVenusResetCharacter != 7 {
		t.Fatalf("reset flags = %v/%v", c.worldState.pendingRelicReset, c.worldState.lastVenusResetCharacter)
	}
	// 消费一次后不再重复发（下一条周期 CMD35 应无包）。
	go func() {
		_ = c.dispatchVenus(&clientRequest{frame: wire.Frame{Type: 1, ID: 35}, verified: true})
	}()
	peer.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, e := io.ReadFull(peer, h); e == nil {
		t.Fatal("second CMD35 must not push another state")
	}
}
