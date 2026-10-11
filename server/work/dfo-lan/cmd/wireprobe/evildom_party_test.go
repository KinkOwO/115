package main

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// 建队链（CMD12 → N9）回归测试。
//
// 参考对象是**本仓已经修好的那条路**（伊斯 ispins_party.go / 森林 forest_flow.go）：
// 它们都用 2.38.2 原生 NOTI9 模板（107+len(name) 字节）承载各自的队伍类型。
// 次元回廊只换队伍类型 0x0d，其余一律照抄 —— 这是 2026-10-10 两轮实机换来的结论：
// 照官服抓包的 176B 新版布局回放，本机客户端会读错字段，「输入队名点确认没反应」。
//
// 语料：
//
//	officialCmd12         官服 _dump_s30_c2s.txt frame #434（队名 "11"）
//	realCmd12FiveByteName 业主 2026-10-10 会话 events.jsonl 的真实请求（队名 "55555"）
//	realCmd12FourByteName 同上（队名 "2221"）
const officialCmd12 = "00000200000031310400000000000000000d01000101020407070707ffffffff00000000000000000000000000000000"

const realCmd12FiveByteName = "00000500000035353535350400000000000000000d01000101020407070707ffffffff00000000000000000000000000"
const realCmd12FourByteName = "000004000000323232310400000000000000000d01000101020407070707ffffffff0000000000000000000000000000"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDimCloisterPartyDecodeMatchesOthers 钉住解码偏移与伊斯/森林完全同构：
// 名长 @2、队名 @6、容量 u32@(6+n)、队伍类型 @(n+15)、模式 u16@(n+16)。
func TestDimCloisterPartyDecodeMatchesOthers(t *testing.T) {
	cases := []struct {
		hex      string
		name     string
		capacity uint32
	}{
		{officialCmd12, "11", 4},
		{realCmd12FiveByteName, "55555", 4},
		{realCmd12FourByteName, "2221", 4},
	}
	for _, tc := range cases {
		req, err := legion.DecodeEvildomParty(mustHex(t, tc.hex))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if req.Name != tc.name {
			t.Fatalf("队名 %q, want %q", req.Name, tc.name)
		}
		if req.Capacity != tc.capacity {
			t.Fatalf("%s: 容量 %d, want %d", tc.name, req.Capacity, tc.capacity)
		}
		if req.PartyType != legion.EvildomPartyType {
			t.Fatalf("%s: 队伍类型 %#x, want %#x", tc.name, req.PartyType, legion.EvildomPartyType)
		}
		if req.Mode != legion.EvildomPartyMode {
			t.Fatalf("%s: 模式 %d, want %d", tc.name, req.Mode, legion.EvildomPartyMode)
		}
	}
}

// TestDimCloisterPartyReplyMatchesProvenTemplate 是这一轮最关键的回归测试：
// 次元回廊的建队应答必须与「已经修好的那条路」**逐字节同形**，
// 只有队伍类型字节（@29 与 @95）不同。
func TestDimCloisterPartyReplyMatchesProvenTemplate(t *testing.T) {
	name := []byte("55555")
	channel := [2]byte{0x2e, 0x00}
	actor := uint16(7)

	evildom, err := protocol.EvildomPartyReply(name, actor, channel, 4)
	if err != nil {
		t.Fatal(err)
	}
	// 伊斯包装同样走 legionStandbyPartyReply，只是队伍类型不同，所以它是
	// 「已修好模板」的公开对照物。
	ispins, err := protocol.IspinsStandbyPartyReply(name, actor, channel, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(evildom) != len(ispins) {
		t.Fatalf("长度 %d, want %d（应与伊斯模板同长）", len(evildom), len(ispins))
	}
	diff := 0
	for i := range evildom {
		if evildom[i] == ispins[i] {
			continue
		}
		diff++
		// 模板把队伍类型写在两处：q[29] 与 q[95]（q = p[len(name):]），
		// 即 p[29+len(name)] 与 p[95+len(name)]。
		if i != 29+len(name) && i != 95+len(name) {
			t.Fatalf("第 %d 字节不同于已修好的模板: got %02x, want %02x（只允许队伍类型两处不同）",
				i, evildom[i], ispins[i])
		}
		if evildom[i] != legion.EvildomPartyType {
			t.Fatalf("@%d 队伍类型 %02x, want %02x", i, evildom[i], legion.EvildomPartyType)
		}
	}
	if diff != 2 {
		t.Fatalf("与模板的差异字节数 %d, want 2（队伍类型两处）", diff)
	}
}

// TestDimCloisterPartyCreateEmitsProvenSequence 钉住处理器行为：与森林/伊斯同序的
// 三帧（N2 队长资料、N2 队长详细资料、N9 队伍创建），并置位进图链依赖的标记。
func TestDimCloisterPartyCreateEmitsProvenSequence(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{
		channelType: legion.DimCloisterChannelType,
		// 与 ispins_party_test.go 同一份最小角色状态（EntryBasicProbe/EntryAddition 需要）。
		role: database.Character{
			WireID: 4242, ID: 4242, Name: "001",
			State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`),
		},
		characters: &character.Service{ChannelContext: [2]byte{0x2e, 0x00}},
	}
	result, err := s.dimCloisterCreateParty(w, mustHex(t, officialCmd12))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 3 {
		t.Fatalf("应答帧数 %d, want 3（N2/N2/N9，与伊斯/森林同序）", len(result.Packets))
	}
	wantIDs := []uint16{2, 2, 9}
	for i, pkt := range result.Packets {
		if pkt.Kind != 0 || pkt.ID != wantIDs[i] {
			t.Fatalf("第 %d 帧 op/kind = %d/%d, want 0/%d", i, pkt.Kind, pkt.ID, wantIDs[i])
		}
	}
	if !w.soloPartyReady {
		t.Fatal("建队后没有置位 soloPartyReady")
	}
	if s.dimCloisterPartyName != "11" {
		t.Fatalf("记录的队名 %q, want %q", s.dimCloisterPartyName, "11")
	}
	if !s.dimCloisterPartyActive {
		t.Fatal("建队后没有置位 dimCloisterPartyActive（CMD13 判据依赖它）")
	}
	nine := result.Packets[2].Payload
	// 模板把队伍类型写在 q[29] 与 q[95]（q = p[len(name):]），队名 "11" → p[31]/p[97]。
	typeA, typeB := 29+len("11"), 95+len("11")
	if len(nine) <= typeB || nine[typeA] != legion.EvildomPartyType || nine[typeB] != legion.EvildomPartyType {
		t.Fatalf("N9 队伍类型位置不对: @%d=%02x @%d=%02x, want %02x",
			typeA, nine[typeA], typeB, nine[typeB], legion.EvildomPartyType)
	}
}

// TestDimCloisterPartyRejectsForeignLegionType 钉住「不为别的内容建队」，
// 并验证类型偏移跟着名长走（与伊斯/森林同构）。
func TestDimCloisterPartyRejectsForeignLegionType(t *testing.T) {
	base := mustHex(t, realCmd12FourByteName)
	for _, other := range []byte{0x26, 0x0b, 0x22, 0x19, 0x00} {
		body := append([]byte(nil), base...)
		body[19] = other // 名长 4 时类型在 @19
		if _, err := legion.DecodeEvildomParty(body); err == nil {
			t.Fatalf("队伍类型 %#x 未被拒绝（会替别的内容建队）", other)
		}
	}
	five := mustHex(t, realCmd12FiveByteName)
	if five[20] != legion.EvildomPartyType {
		t.Fatalf("夹具本身不对：@20 = %02x, want %02x", five[20], legion.EvildomPartyType)
	}
	five[20] = 0x26
	if _, err := legion.DecodeEvildomParty(five); err == nil {
		t.Fatal("名长 5 时把别的内容类型放过了")
	}
}

// TestDimCloisterPartyReplyRejectsBadCapacity 钉住容量范围（与伊斯同口径 1..4）。
func TestDimCloisterPartyReplyRejectsBadCapacity(t *testing.T) {
	channel := [2]byte{0x2e, 0x00}
	for _, bad := range []byte{0, 5, 255} {
		if _, err := protocol.EvildomPartyReply([]byte("x"), 7, channel, bad); err == nil {
			t.Fatalf("容量 %d 未被拒绝", bad)
		}
	}
	if _, err := protocol.EvildomPartyReply(nil, 7, channel, 4); err == nil {
		t.Fatal("空队名未被拒绝")
	}
	if _, err := protocol.EvildomPartyReply([]byte("x"), 7, [2]byte{}, 4); err == nil {
		t.Fatal("缺频道上下文未被拒绝")
	}
}
