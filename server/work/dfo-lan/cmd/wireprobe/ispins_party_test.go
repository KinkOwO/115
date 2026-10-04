package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// next79 §21 回归护栏：伊斯待机区 CMD12（PARTY_CREATE）应答。
//
// 官服 10-02 s4 抓包（tshark 包时间戳对齐）：op=12（帧 125，body 48B，
// 队名「111」容量 4）之后 0.5s 内的唯一下行是单帧 NOTI9（帧 354，body
// 208B）；客户端收到即进已建队状态，随后直接 CMD2043 开战。私服客户端
// 实测所发 CMD12 与官服帧 125 逐字节一致（05:54 会话两次，48B 完整体），
// 此前服务端无应答，点「确定」UI 毫无反应。

// ispinsStandbyPartyRequestHex 是官服 s4 帧 125 的完整 CMD12 body；私服
// 客户端发出的帧与它逐字节相同，解码器必须接受它。
const ispinsStandbyPartyRequestHex = "000003000000313131040000000000000000" +
	"0b01000101020407070707ffffffff000000000000000000000000000000"

// ispinsStandbyPartyReplyHex 是官服 s4 帧 354 的完整 NOTI9 body（队名
// 「111」）。它只作语义参照：官服 208B 是新版客户端布局，其 p[4:6]=105
// 落在 2.38.2 读取器的成员数槽位，verbatim 回放两次实测闪退（06:32/06:41，
// 后者 actor 已本地化仍崩），故应答改用黑鸦族原生语法（见
// TestIspinsStandbyPartyReplyNativeGrammar）。
const ispinsStandbyPartyReplyHex = "01000f2769000356150001000100030000003131310100040000000000000000" +
	"0b01000000000000000000040000000000010102040707070700ffffffffff00" +
	"000000003c000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"000000000100ed000001000000000100010075e5000000010000000000000b00" +
	"75e50000000100000010009c0015010000000410009c00000000000000000000" +
	"000000397fbe37450000000000000000"

// 2.38.2 原生语法护栏：应答必须是黑鸦族布局（1452F2620 写入器），成员数
// 槽 p[4:6]=1；伊斯语义字段（容量 4 / 类型 0x0b / 模式 1 / 本地 actor）各
// 就各位。官服 208B 模板已证伪，不得回归。
func TestIspinsStandbyPartyReplyNativeGrammar(t *testing.T) {
	got, err := protocol.IspinsStandbyPartyReply([]byte("111"), 7, [2]byte{0x03, 0x56}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 110 {
		t.Fatalf("reply len=%d, want 110 (107+name)", len(got))
	}
	if binary.LittleEndian.Uint16(got[0:2]) != 1 || binary.LittleEndian.Uint16(got[2:4]) != 9999 {
		t.Fatalf("flag/party = %x", got[0:4])
	}
	if m := binary.LittleEndian.Uint16(got[4:6]); m != 1 {
		t.Fatalf("member count slot p[4:6]=%d, want 1 (official 208B puts 105 here; it crashes 2.38.2)", m)
	}
	if !bytes.Equal(got[6:8], []byte{0x03, 0x56}) {
		t.Fatalf("channel context = %x", got[6:8])
	}
	if n := binary.LittleEndian.Uint32(got[14:18]); n != 3 {
		t.Fatalf("name len=%d, want 3", n)
	}
	if string(got[18:21]) != "111" {
		t.Fatalf("name = %x", got[18:21])
	}
	q := got[3:] // q = p[len(name):]，黑鸦同构
	if q[20] != 4 {
		t.Fatalf("capacity = %d, want 4", q[20])
	}
	if q[25] != 5 {
		t.Fatalf("difficulty = %d, want 5", q[25])
	}
	if q[29] != 0x0b || q[95] != 0x0b {
		t.Fatalf("party type = %d/%d, want 11/11 (军团普通队伍)", q[29], q[95])
	}
	if binary.LittleEndian.Uint16(q[30:32]) != 1 {
		t.Fatalf("mode = %d, want 1", binary.LittleEndian.Uint16(q[30:32]))
	}
	if !bytes.Equal(q[46:54], []byte{1, 1, 2, 4, 7, 7, 7, 7}) {
		t.Fatalf("attr block = %x", q[46:54])
	}
	if q[69] != 1 {
		t.Fatalf("q[69] = %d, want 1", q[69])
	}
	if a := binary.LittleEndian.Uint16(q[71:73]); a != 7 {
		t.Fatalf("captain actor = %d, want 7", a)
	}
	// 尾段扩展类型 0：q[95] 之后全零（不借用沉月湖 41）。
	for i, b := range q[96:] {
		if b != 0 {
			t.Fatalf("tail byte q[%d]=%02x, want 0", 96+i, b)
		}
	}
	// 拒绝：空名 / 超长名 / actor 0 / 65535 / 零频道。
	if _, err := protocol.IspinsStandbyPartyReply(nil, 7, [2]byte{3, 86}, 4); err == nil {
		t.Fatal("empty name must be rejected")
	}
	if _, err := protocol.IspinsStandbyPartyReply(bytes.Repeat([]byte{0x31}, 64), 7, [2]byte{3, 86}, 4); err == nil {
		t.Fatal("overlong name must be rejected")
	}
	if _, err := protocol.IspinsStandbyPartyReply([]byte("111"), 0, [2]byte{3, 86}, 4); err == nil {
		t.Fatal("actor 0 must be rejected")
	}
	if _, err := protocol.IspinsStandbyPartyReply([]byte("111"), 65535, [2]byte{3, 86}, 4); err == nil {
		t.Fatal("actor 65535 must be rejected")
	}
	if _, err := protocol.IspinsStandbyPartyReply([]byte("111"), 7, [2]byte{}, 4); err == nil {
		t.Fatal("zero channel context must be rejected")
	}
}

func TestDecodeIspinsStandbyPartyOfficialRequest(t *testing.T) {
	req, err := hex.DecodeString(ispinsStandbyPartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	name, err := protocol.DecodeIspinsStandbyParty(req)
	if err != nil {
		t.Fatal(err)
	}
	if string(name.Name) != "111" {
		t.Fatalf("decoded name %q, want 111", name.Name)
	}
	// Invalid sizes and other party types remain outside this handler.
	if _, err := protocol.DecodeIspinsStandbyParty(req[:16]); err == nil {
		t.Fatal("truncated request must be rejected")
	}
	black := append([]byte{}, req...)
	black[9] = 0 // 非法容量 0
	if _, err := protocol.DecodeIspinsStandbyParty(black); err == nil {
		t.Fatal("capacity 0 must be rejected")
	}
	wrongType := append([]byte{}, req...)
	wrongType[18] = 7 // 队伍类型 7（黑鸦）
	if _, err := protocol.DecodeIspinsStandbyParty(wrongType); err == nil {
		t.Fatal("party type 7 must be rejected")
	}
	nul := append([]byte{}, req...)
	nul[6] = 0
	if _, err := protocol.DecodeIspinsStandbyParty(nul); err == nil {
		t.Fatal("NUL in name must be rejected")
	}
}

func TestIspinsStandbyPartyPreservesRequestedCapacity(t *testing.T) {
	for capacity := byte(1); capacity <= 4; capacity++ {
		request, _ := hex.DecodeString(ispinsStandbyPartyRequestHex)
		binary.LittleEndian.PutUint32(request[9:], uint32(capacity))
		w := &worldSession{
			channelType: 81,
			role:        database.Character{WireID: 7, ID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
			characters:  &character.Service{ChannelContext: [2]byte{3, 86}},
		}
		handled, packets, err := w.ispinsStandbyPartyHandle(12, request)
		if !handled || err != nil || len(packets) != 3 || !w.soloPartyReady {
			t.Fatalf("capacity%d create failed: %v", capacity, err)
		}
		p := packets[2].Payload
		if binary.LittleEndian.Uint16(p[4:]) != 1 || p[23] != capacity {
			t.Fatalf("capacity%d confused members with party limit: %x", capacity, p)
		}
	}
	for _, capacity := range []uint32{0, 5, 256, 0xffffffff} {
		request, _ := hex.DecodeString(ispinsStandbyPartyRequestHex)
		binary.LittleEndian.PutUint32(request[9:], capacity)
		if _, err := protocol.DecodeIspinsStandbyParty(request); err == nil {
			t.Fatalf("invalid capacity%d accepted", capacity)
		}
	}
}

// 处理器门禁：仅 Type 81 待机区连接、已选角、CMD12 触发；其它频道/命令
// 一律不接手（黑鸦频道 op12 仍走 blackPurgatoryHandle）。
func TestIspinsStandbyPartyHandlerGate(t *testing.T) {
	req, err := hex.DecodeString(ispinsStandbyPartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	town := &worldSession{channelType: 1, role: database.Character{WireID: 7, ID: 7}}
	if handled, _, _ := town.ispinsStandbyPartyHandle(12, req); handled {
		t.Fatal("town channel must not be handled by the ispins standby party handler")
	}
	unselected := &worldSession{channelType: 81}
	if handled, _, _ := unselected.ispinsStandbyPartyHandle(12, req); handled {
		t.Fatal("CMD12 before character selection must be ignored")
	}
	// 无角色服务：接手但拒绝，不能静默吞掉建队请求。
	noService := &worldSession{channelType: 81, role: database.Character{WireID: 7, ID: 7}}
	if handled, _, err := noService.ispinsStandbyPartyHandle(12, req); !handled || err == nil {
		t.Fatal("standby CMD12 without a character service must be handled with an error")
	}
	// 成功路径：黑鸦族原生 NOTI9 + 队长资料两个 op=2 先行（黑鸦建队先例）。
	standby := &worldSession{
		channelType: 81,
		role:        database.Character{WireID: 7, ID: 7, Name: "IspinsCap", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		characters:  &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
	}
	handled, plan, err := standby.ispinsStandbyPartyHandle(12, req)
	if !handled || err != nil {
		t.Fatal(handled, err)
	}
	if len(plan) != 3 {
		t.Fatalf("plan len=%d, want 3 (captain basic/addition/NOTI9)", len(plan))
	}
	want := []struct {
		name string
		id   uint16
	}{{"伊斯队长资料", 2}, {"伊斯队长详细资料", 2}, {"伊斯待机区队伍创建", 9}}
	for i, w := range want {
		if plan[i].Name != w.name || plan[i].ID != w.id {
			t.Fatalf("plan[%d] = %s/%d, want %s/%d", i, plan[i].Name, plan[i].ID, w.name, w.id)
		}
	}
	// NOTI9 必须是 2.38.2 原生黑鸦族布局（110B），官服 208B 布局已证闪退。
	if len(plan[2].Payload) != 110 {
		t.Fatalf("NOTI9 body len=%d, want 110", len(plan[2].Payload))
	}
	if !standby.soloPartyReady {
		t.Fatal("standby party creation must arm soloPartyReady")
	}
	if handled, _, _ := standby.ispinsStandbyPartyHandle(36, req); handled {
		t.Fatal("CMD36 must not be handled by the party handler")
	}
}
