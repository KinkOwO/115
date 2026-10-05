package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// 维纳斯待机区（频道 Type 99）「创建队伍」回归护栏。
//
// 实机 2026-10-04 10:03 会话（…_100335_903957_next37）events.jsonl：频道99
// 待机区点「确定」后客户端发出 CMD12（02:16:25.839 / 02:16:59.508 两次同
// 文），此前服务端无任何处理器应答，点「确定」UI 毫无反应。同一会话里伊
// 斯频道（Type 81）的 CMD12 类型字节 0x0b、末世录频道（Type 119）的为
// 0x26（=38，与 protocol.ApocalypsePartyMode115 一致），维纳斯为 0x22
//（34），三包其余字节完全同构。

// venusLivePartyRequestHex 是上述实机 CMD12 body（48B，队名「1」容量 4）：
// [u16 0][u32 名长][队名][u32 容量=4][5 零][u8 类型 0x22][u16 模式 1]
// [1,1,2,4,7,7,7,7][ff×4][零尾]。
const venusLivePartyRequestHex = "00000100000031040000000000000000" +
	"2201000101020407070707ffffffff0000000000000000000000000000000000"

func TestDecodeVenusStandbyPartyLiveRequest(t *testing.T) {
	req, err := hex.DecodeString(venusLivePartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(req) != 48 {
		t.Fatalf("live request len=%d, want 48", len(req))
	}
	name, err := protocol.DecodeVenusStandbyParty(req)
	if err != nil {
		t.Fatal(err)
	}
	if string(name) != "1" {
		t.Fatalf("decoded name %q, want 1", name)
	}
	// 长度不足、伊斯类型 0x0b、末世录类型 0x26、模式非 1、容量非 4 与
	// 队名含 NUL 都必须拒绝——本处理器只认维纳斯 0x22 的 4 人军团队。
	if _, err := protocol.DecodeVenusStandbyParty(req[:16]); err == nil {
		t.Fatal("truncated request must be rejected")
	}
	ispins := append([]byte{}, req...)
	ispins[16] = 0x0b // 类型 11（伊斯军团普通队伍）
	if _, err := protocol.DecodeVenusStandbyParty(ispins); err == nil {
		t.Fatal("party type 0x0b must be rejected")
	}
	apoc := append([]byte{}, req...)
	apoc[16] = 0x26 // 类型 38（末世录）
	if _, err := protocol.DecodeVenusStandbyParty(apoc); err == nil {
		t.Fatal("party type 0x26 must be rejected")
	}
	mode := append([]byte{}, req...)
	mode[17] = 2
	if _, err := protocol.DecodeVenusStandbyParty(mode); err == nil {
		t.Fatal("mode 2 must be rejected")
	}
	cap := append([]byte{}, req...)
	cap[7] = 2
	if _, err := protocol.DecodeVenusStandbyParty(cap); err == nil {
		t.Fatal("capacity 2 must be rejected")
	}
	nul := append([]byte{}, req...)
	nul[6] = 0
	if _, err := protocol.DecodeVenusStandbyParty(nul); err == nil {
		t.Fatal("NUL in name must be rejected")
	}
}

// 应答必须是 2.38.2 原生黑鸦族 NOTI9 语法（107+名长），伊斯/黑鸦已实证
// 可解析；队伍类型槽写入维纳斯 0x22，其余语义字段同伊斯。
func TestVenusStandbyPartyReplyNativeGrammar(t *testing.T) {
	got, err := protocol.VenusStandbyPartyReply([]byte("1"), 7, [2]byte{0x03, 0x56})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 108 {
		t.Fatalf("reply len=%d, want 108 (107+name)", len(got))
	}
	if binary.LittleEndian.Uint16(got[0:2]) != 1 || binary.LittleEndian.Uint16(got[2:4]) != 9999 {
		t.Fatalf("flag/party = %x", got[0:4])
	}
	if m := binary.LittleEndian.Uint16(got[4:6]); m != 1 {
		t.Fatalf("member count slot p[4:6]=%d, want 1", m)
	}
	if !bytes.Equal(got[6:8], []byte{0x03, 0x56}) {
		t.Fatalf("channel context = %x", got[6:8])
	}
	if n := binary.LittleEndian.Uint32(got[14:18]); n != 1 {
		t.Fatalf("name len=%d, want 1", n)
	}
	if string(got[18:19]) != "1" {
		t.Fatalf("name = %x", got[18:19])
	}
	q := got[1:] // q = p[len(name):]，黑鸦同构
	if q[20] != 4 {
		t.Fatalf("capacity = %d, want 4", q[20])
	}
	if q[25] != 5 {
		t.Fatalf("difficulty = %d, want 5", q[25])
	}
	if q[29] != 0x22 || q[95] != 0x22 {
		t.Fatalf("party type = %d/%d, want 34/34 (维纳斯军团普通队伍)", q[29], q[95])
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
	// 尾段扩展类型 0：q[95] 之后全零（同伊斯/黑鸦结论）。
	for i, b := range q[96:] {
		if b != 0 {
			t.Fatalf("tail byte q[%d]=%02x, want 0", 96+i, b)
		}
	}
	// 拒绝：空名 / 超长名 / actor 0 / 65535 / 零频道。
	if _, err := protocol.VenusStandbyPartyReply(nil, 7, [2]byte{3, 86}); err == nil {
		t.Fatal("empty name must be rejected")
	}
	if _, err := protocol.VenusStandbyPartyReply(bytes.Repeat([]byte{0x31}, 64), 7, [2]byte{3, 86}); err == nil {
		t.Fatal("overlong name must be rejected")
	}
	if _, err := protocol.VenusStandbyPartyReply([]byte("1"), 0, [2]byte{3, 86}); err == nil {
		t.Fatal("actor 0 must be rejected")
	}
	if _, err := protocol.VenusStandbyPartyReply([]byte("1"), 65535, [2]byte{3, 86}); err == nil {
		t.Fatal("actor 65535 must be rejected")
	}
	if _, err := protocol.VenusStandbyPartyReply([]byte("1"), 7, [2]byte{}); err == nil {
		t.Fatal("zero channel context must be rejected")
	}
}

// 处理器门禁：仅 Type 99 待机区连接、已选角、CMD12/13 触发；其它频道/
// 命令一律不接手。成功路径照黑鸦/伊斯先例：队长资料两个 op=2 先行，
// NOTI9 收尾；离队发 NOTI9 action3 清队。
func TestVenusStandbyPartyHandlerGate(t *testing.T) {
	req, err := hex.DecodeString(venusLivePartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	town := &worldSession{channelType: 1, role: database.Character{WireID: 7, ID: 7}}
	if handled, _, _ := town.venusStandbyPartyHandle(12, req); handled {
		t.Fatal("town channel must not be handled by the venus standby party handler")
	}
	ispinsChannel := &worldSession{channelType: 81, role: database.Character{WireID: 7, ID: 7}}
	if handled, _, _ := ispinsChannel.venusStandbyPartyHandle(12, req); handled {
		t.Fatal("ispins channel must not be handled by the venus standby party handler")
	}
	unselected := &worldSession{channelType: 99}
	if handled, _, _ := unselected.venusStandbyPartyHandle(12, req); handled {
		t.Fatal("CMD12 before character selection must be ignored")
	}
	// 无角色服务：接手但拒绝，不能静默吞掉建队请求。
	noService := &worldSession{channelType: 99, role: database.Character{WireID: 7, ID: 7}}
	if handled, _, err := noService.venusStandbyPartyHandle(12, req); !handled || err == nil {
		t.Fatal("standby CMD12 without a character service must be handled with an error")
	}
	// 成功路径：黑鸦族原生 NOTI9 + 队长资料两个 op=2 先行。
	standby := &worldSession{
		channelType: 99,
		role:        database.Character{WireID: 7, ID: 7, Name: "VenusCap", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		characters:  &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
	}
	handled, plan, err := standby.venusStandbyPartyHandle(12, req)
	if !handled || err != nil {
		t.Fatal(handled, err)
	}
	want := []struct {
		name string
		id   uint16
	}{{"维纳斯队长资料", 2}, {"维纳斯队长详细资料", 2}, {"维纳斯待机区队伍创建", 9}}
	if len(plan) != 3 {
		t.Fatalf("plan len=%d, want 3 (captain basic/addition/NOTI9)", len(plan))
	}
	for i, w := range want {
		if plan[i].Name != w.name || plan[i].ID != w.id {
			t.Fatalf("plan[%d] = %s/%d, want %s/%d", i, plan[i].Name, plan[i].ID, w.name, w.id)
		}
	}
	if len(plan[2].Payload) != 108 {
		t.Fatalf("NOTI9 body len=%d, want 108", len(plan[2].Payload))
	}
	if !standby.soloPartyReady {
		t.Fatal("standby party creation must arm soloPartyReady")
	}
	// 离队：NOTI9 action3 清队并复位 soloPartyReady。
	handled, plan, err = standby.venusStandbyPartyHandle(13, nil)
	if !handled || err != nil {
		t.Fatal(handled, err)
	}
	if len(plan) != 1 || plan[0].ID != 9 || len(plan[0].Payload) != 12 {
		t.Fatalf("leave plan = %v, want single 12B NOTI9", plan)
	}
	if plan[0].Payload[9] != 3 || plan[0].Payload[10] != 1 {
		t.Fatalf("party-gone action flags = %x", plan[0].Payload[9:11])
	}
	if standby.soloPartyReady {
		t.Fatal("leave must clear soloPartyReady")
	}
	if handled, _, _ := standby.venusStandbyPartyHandle(36, req); handled {
		t.Fatal("CMD36 must not be handled by the party handler")
	}
}

// isVenusRequest 按内容号分流家族命令；伊斯（101）与末世录（107）同信封
// 请求必须不受影响。
func TestIsVenusRequest(t *testing.T) {
	if !isVenusRequest(legion.CmdVenusOperationSelect, nil) {
		t.Fatal("CMD2290 must be recognized as a Venus request")
	}
	if !isVenusRequest(legion.CmdVenusEndAtPhase4, nil) {
		t.Fatal("CMD2293 must be recognized as a Venus request")
	}
	body := append(make([]byte, legion.EnvelopeSize), 106, 0, 0, 0)
	if !isVenusRequest(legion.CmdStart, body) {
		t.Fatal("CMD2043 with content 106 must be recognized")
	}
	if !isVenusRequest(legion.CmdEnterDungeon, body) {
		t.Fatal("CMD2045 with content 106 must be recognized")
	}
	for _, content := range []uint32{legion.IspinsContentID, 107} {
		other := append(make([]byte, legion.EnvelopeSize), byte(content), byte(content>>8), byte(content>>16), byte(content>>24))
		if isVenusRequest(legion.CmdStart, other) {
			t.Fatalf("CMD2043 with content %d must not be claimed", content)
		}
	}
	if isVenusRequest(legion.CmdStart, nil) {
		t.Fatal("short body must not be claimed")
	}
	if isVenusRequest(72, body) {
		t.Fatal("CMD72 must not be claimed")
	}
}
