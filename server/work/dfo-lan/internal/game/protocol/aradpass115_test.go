package protocol

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"
)

// 战令（活动 516 / 阿拉德通行证）的协议用例。
//
// ⚠️ 本文件是**移植**来的（来源：另一棵同血脉源码 `internal/game/protocol/aradpass.go`，
// 其证据是 `D:\wd\20261002-211855` 抓包的 18 份 0x08A1 + 21 份 0x08A2 + 一次领奖请求）。
// 移植时只改了文件名（→ `aradpass115.go`，与本仓 `attendance115.go` / `honey_time115.go`
// 同一命名习惯）；结构、常量、偏移**一字未改**。这里的断言把那份抓包证据钉住。

// TestAradPassEventInfoLayout 钉住 0x08A1 的长度与关键偏移。
//
// 760B 是结构性常量（21 份里 18 份 0x08A1 的正文长度），尾部（任务列表条数、
// 0x1C2 状态字节）都由抓包样本定过；偏移错一个字节客户端就会读歪。
func TestAradPassEventInfoLayout(t *testing.T) {
	body, err := AradPassEventInfo(AradPassProgress{
		Level: 3, Exp: 1200, MaxExp: 50000, PurchaseExp: 5000, Royal: true, ClaimedSlots: []byte{1}, // 下标 0 = 普通轨第 1 档
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(body) != 760 {
		t.Fatalf("0x08A1 正文 = %d 字节，抓包实证应为 760", len(body))
	}
	if body[4] != 3 {
		t.Fatalf("等级字节 = %d，应为 3", body[4])
	}
	if got := uint32(body[5]) | uint32(body[6])<<8 | uint32(body[7])<<16; got != 1200 {
		t.Fatalf("u24@5 累计经验 = %d，应为 1200", got)
	}
	// 三个 ×256 的定点字段，与抓包一致：满级 50000、当前 1200、差额 (50000-1200)、购票 5000。
	for _, tc := range []struct {
		at   int
		want uint32
	}{{8, 50000 * 256}, {12, 1200 * 256}, {16, (50000 - 1200) * 256}, {20, 5000 * 256}} {
		if got := binary.LittleEndian.Uint32(body[tc.at:]); got != tc.want {
			t.Fatalf("@%d = %d，应为 %d", tc.at, got, tc.want)
		}
	}
	if body[25] != 1 {
		t.Fatalf("头部启用位 @25 = %d，应为 1", body[25])
	}
	// 已购皇家：抓包里那位玩家**从没买过**，所以"已购"该写哪个字节**无样本**；
	// 当前按最强候选写 0x18=1（可用 DFO_ARAD_PASS_ROYAL_BYTE 逐个试而不重编）。
	// ⚠️ 注意 0x18 就是那个候选字节本身，所以"已购"时 @0x18 由 1 覆盖掉原来的 0 ——
	// 未购时必须仍是 0（下面单独断言一次，免得有人把候选写死成常量）。
	t.Setenv("DFO_ARAD_PASS_ROYAL_BYTE", "")
	if body[0x18] != 1 {
		t.Fatalf("已购皇家候选字节 @0x18 = %d，应为 1", body[0x18])
	}
	plain, err := AradPassEventInfo(AradPassProgress{Level: 1, MaxExp: 50000})
	if err != nil {
		t.Fatalf("encode(未购): %v", err)
	}
	if plain[0x18] != 0 {
		t.Fatalf("未购时 @0x18 = %d，应为 0", plain[0x18])
	}
	if body[0x19] != 1 {
		t.Fatalf("启用位 @0x19 = %d，应为 1", body[0x19])
	}
	// 两条任务列表：条数 + 每行「序号字节 + 6 个 0」。
	if body[0x1A] != 16 {
		t.Fatalf("普通任务条数 @0x1A = %d，应为 16", body[0x1A])
	}
	for i := 0; i < 16; i++ {
		if body[0x1B+7*i] != byte(i+1) {
			t.Fatalf("普通任务第 %d 行序号 = %d，应为 %d", i, body[0x1B+7*i], i+1)
		}
		if got := body[0x1B+7*i+1 : 0x1B+7*i+7]; strings.Count(string(got), "\x00") != 6 {
			t.Fatalf("普通任务第 %d 行尾部应为 6 个 0", i)
		}
	}
	if body[0xED] != 12 {
		t.Fatalf("赛季任务条数 @0xED = %d，应为 12", body[0xED])
	}
	for i := 0; i < 12; i++ {
		if body[0xEE+7*i] != byte(i+1) {
			t.Fatalf("赛季任务第 %d 行序号 = %d，应为 %d", i, body[0xEE+7*i], i+1)
		}
	}
	// 已领标记是**每档 1 字节**（不是位图）：下标 0 = 普通轨第 1 档，写在 0x1C2。
	// 实机对照：0x1C2=0x01 只点亮第 1 格；0x1C2=0xff 只点亮 2 格（位图会说 8 格）。
	if body[aradPassOffsetClaimedNormal] != byte(1) {
		t.Fatalf("普通轨标记 @0x%X = %d，应为 1", aradPassOffsetClaimedNormal, body[aradPassOffsetClaimedNormal])
	}
	// ★ 两条轨**基址不相邻**：皇家轨在 0x28A（实机窗口计数法定案），不是 0x1C2+25。
	if body[aradPassOffsetClaimedRoyal] != 0 {
		t.Fatalf("皇家轨基址 0x%X 不该被普通轨的写入碰到", aradPassOffsetClaimedRoyal)
	}
	royal, err := AradPassEventInfo(AradPassProgress{
		Level: 1, MaxExp: 50000,
		ClaimedSlots: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if royal[aradPassOffsetClaimedRoyal] != 1 {
		t.Fatalf("皇家轨第 1 档标记 @0x%X = %d，应为 1", aradPassOffsetClaimedRoyal, royal[aradPassOffsetClaimedRoyal])
	}
	if royal[aradPassOffsetClaimedNormal+25] != 0 {
		t.Fatal("皇家轨不该写到 0x1C2+25（那个位置实机证明不亮）")
	}
	// 再验一次"没有已领档时整段必须是 0"——这正是 2026-10-11 实机"绿勾点不动"的根因。
	zero, err := AradPassEventInfo(AradPassProgress{Level: 1, MaxExp: 50000})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for i := 0; i < aradPassClaimedPerTrack; i++ {
		if zero[aradPassOffsetClaimedNormal+i] != 0 {
			t.Fatalf("未领任何档时普通轨标记 @0x%X 应为 0，实际 %d",
				aradPassOffsetClaimedNormal+i, zero[aradPassOffsetClaimedNormal+i])
		}
		if zero[aradPassOffsetClaimedRoyal+i] != 0 {
			t.Fatalf("未领任何档时皇家轨标记 @0x%X 应为 0，实际 %d",
				aradPassOffsetClaimedRoyal+i, zero[aradPassOffsetClaimedRoyal+i])
		}
	}
	// 每日礼包"今天已领" = 正文 0x1C0 的 u16。
	//
	// ★ 实机锚点（2026-10-11，业主截图）：只把这一格写成 0xFF，Royal Pass 页签底部
	// 那行 `Royal Pass Daily Login rewards` 当场从 `Get` 变成绿勾"已领取"。
	// 同一个谓词还是客户端发包前的门禁（`sub_1422A07B0`），所以这一格必须与
	// `state.RoyalDailyKey` 严格同源，否则会出现"领不了/看不出已领"。
	daily, err := AradPassEventInfo(AradPassProgress{Level: 1, MaxExp: 50000, RoyalDailyDay: AradPassRoyalDailyClaimed})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if daily[aradPassOffsetRoyalDailyDay] != 0xFF || daily[aradPassOffsetRoyalDailyDay+1] != 0xFF {
		t.Fatalf("每日礼包已领标记 @0x%X = %02x %02x，应为 ff ff",
			aradPassOffsetRoyalDailyDay, daily[aradPassOffsetRoyalDailyDay], daily[aradPassOffsetRoyalDailyDay+1])
	}
	// 未领时必须是 0 —— 否则客户端会一直认为今天已领（门禁永远关着）。
	if zero[aradPassOffsetRoyalDailyDay] != 0 || zero[aradPassOffsetRoyalDailyDay+1] != 0 {
		t.Fatal("未领时正文 0x1C0 应为 0")
	}
	// 0x1C0 的 u16 占 0x1C0..0x1C1，紧接着就是普通轨的标记数组基址 0x1C2 ——
	// 二者不重叠（这是实测出来的硬关系，改任何一边都要重新核对布局）。
	if aradPassOffsetRoyalDailyDay+2 != aradPassOffsetClaimedNormal {
		t.Fatal("每日礼包已领日与普通轨标记数组的间隔变了，需要重新核对布局")
	}
	// 越界入参必须拒绝（等级 0 / 经验超上限 / 缺满级经验）。
	for _, bad := range []AradPassProgress{
		{Level: 0, MaxExp: 50000},
		{Level: 1, Exp: 60000, MaxExp: 50000},
		{Level: 1, Exp: 0, MaxExp: 0},
	} {
		if _, err := AradPassEventInfo(bad); err == nil {
			t.Fatalf("非法进度没有被拒: %+v", bad)
		}
	}
}

// TestAradPassRoyalByteOverride 钉住"已购皇家候选字节"的开关语义。
func TestAradPassRoyalByteOverride(t *testing.T) {
	// ⚠ 候选偏移必须落在**真正的保留区**里：0x1B..0x8A 是普通任务行、0xEE..0x141 是赛季任务行，
	// 往那里写会被任务行覆盖（2026-10-11 实测：旧用例用 0x20，正好落在第 1 行 27..33 之内，
	// 任务行一上线它就红了）。这里用 0xC0 —— 位于 0x8B..0xEC 这段保留区中间。
	t.Setenv("DFO_ARAD_PASS_ROYAL_BYTE", "0xC0,7")
	body, err := AradPassEventInfo(AradPassProgress{Level: 1, MaxExp: 50000, Royal: true})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if body[0xC0] != 7 || body[0x18] != 0 {
		t.Fatalf("覆盖失败：@0xC0=%d @0x18=%d", body[0xC0], body[0x18])
	}
	// 非法取值回落默认（不报错、不炸）。
	for _, bad := range []string{"0,1", "abc", "0x18", "0x18,999", "-1,1"} {
		t.Setenv("DFO_ARAD_PASS_ROYAL_BYTE", bad)
		off, val := aradPassRoyalByte()
		if off != aradPassRoyalDefaultOffset || val != aradPassRoyalDefaultValue {
			t.Fatalf("%q 应回落默认，得到 %d,%d", bad, off, val)
		}
	}
}

// TestAradPassCharacInfoLayout 钉住 0x08A2：4B 头 + 20 槽 × 55B = 1104，空槽 charID = 0xFFFFFFFF。
func TestAradPassCharacInfoLayout(t *testing.T) {
	body, err := AradPassCharacInfo(AradPassCharacHeader, []AradPassCharSlot{{CharID: 0x12345678}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(body) != 4+20*55 {
		t.Fatalf("0x08A2 正文 = %d 字节，应为 %d", len(body), 4+20*55)
	}
	if got := binary.LittleEndian.Uint32(body[4+5:]); got != 0x12345678 {
		t.Fatalf("第 0 槽 charID = %#x，应为 0x12345678", got)
	}
	// 其余 19 槽必须是空槽哨兵。
	for i := 1; i < 20; i++ {
		if got := binary.LittleEndian.Uint32(body[4+i*55+5:]); got != 0xFFFFFFFF {
			t.Fatalf("第 %d 槽 charID = %#x，应为 0xFFFFFFFF（空槽）", i, got)
		}
	}
	if _, err := AradPassCharacInfo(AradPassCharacHeader, make([]AradPassCharSlot, 21)); err == nil {
		t.Fatal("21 个槽应被拒（上限 20）")
	}
}

// TestDecodeAradPassClaimRealSample 钉住领奖请求口径。
//
// 抓包唯一一份样本（scheduleId 516，2026 抓包 21:34:51.600）：
//
//	04 02 00 00 | 00 00 00 00 | 01 00 00 00 | 00 00 00 00
//	<516>         <轨道 0>      <奖励槽 1>    <保留 0>
//
// ⇒ 「第 2 个 u32 = 轨道、第 3 个 = 奖励槽（1 起）」是**推断**（只有一个样本），
// 所以这里只钉"读出来的四个数"，范围校验留给调用方。
func TestDecodeAradPassClaimRealSample(t *testing.T) {
	p, err := mustHexStrict("04020000" + "00000000" + "01000000" + "00000000")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeAradPassClaim(p)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Event != 516 || got.Track != 0 || got.Slot != 1 || got.Reserved != 0 {
		t.Fatalf("样本解出 %+v，应为 {516 0 1 0}", got)
	}
	// 8 字节形态：只有赛季与轨道（其余为 0）。
	if got, err := DecodeAradPassClaim(p[:8]); err != nil || got.Event != 516 || got.Slot != 0 {
		t.Fatalf("8B 形态解出 %+v err=%v", got, err)
	}
	// 长度非法 / 活动号缺失：必须拒绝。
	for _, bad := range [][]byte{{}, p[:4], p[:12], make([]byte, 16)} {
		if _, err := DecodeAradPassClaim(bad); err == nil {
			t.Fatalf("非法载荷 %d 字节没有被拒", len(bad))
		}
	}
}

// mustHexStrict 是给本文件用的小工具（包内已有 mustHex，这里只是避免依赖它的签名）。
func mustHexStrict(s string) ([]byte, error) {
	out := make([]byte, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		var b byte
		for _, c := range []byte(s[i : i+2]) {
			switch {
			case c >= '0' && c <= '9':
				b = b<<4 | (c - '0')
			case c >= 'a' && c <= 'f':
				b = b<<4 | (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				b = b<<4 | (c - 'A' + 10)
			default:
				return nil, os.ErrInvalid
			}
		}
		out = append(out, b)
	}
	return out, nil
}
