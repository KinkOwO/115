package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// TestEventRequest115LiveWidths 钉住两种实机/夹具正文宽度：
// 662 的 680/681 是两包（事件号 + 关卡号），665 是 A 类三包。
// 8 字节那帧来自实机 2026-10-04 18:04:04（第一关对话后点「Get」）。
func TestEventRequest115LiveWidths(t *testing.T) {
	p, e := hex.DecodeString("9602000001000000")
	if e != nil || len(p) != 8 {
		t.Fatal(len(p), e)
	}
	r, e := DecodeEventRequest115(p, true)
	if e != nil || r.Event != 662 || r.Parameter != 1 || !r.HasParameter {
		t.Fatal(r, e)
	}
	if _, e := DecodeEventRequest115(p[:4], true); e == nil {
		t.Fatal("one-word body accepted")
	}
	wide := make([]byte, 12)
	live := append([]byte{}, p...)
	wide[0] = 0x99
	wide[1] = 0x02 // 事件号 665
	if q, e := DecodeEventRequest115(wide, false); e != nil || q.Event != 665 || q.Parameter != 0 || !q.HasParameter {
		t.Fatal(q, e)
	}
	// 三包正文仍按第三包取参数，两包口径不得回吞 665。
	three := append(live, 0, 0, 0, 0)
	three[8] = 7
	if q, e := DecodeEventRequest115(three, true); e != nil || q.Parameter != 7 {
		t.Fatal(q, e)
	}
}

// TestBoostTrainingStatusGraduatedFrame 钉住官服的**毕业态** 2638：
// `02 0c 00 00 01`（mode=2 / step=12 / phase=0 / active=1）。
//
// 真源：参考包 `活动Boost与胶囊教学-20260927.zip` 的
// `规格文档/2638-BOOSTUPCHARACTER.md`「已推翻」一节 ——
// 「第12步不是第12张训练副本：该源只有 11 个 step info，官服在最后领取后转为 02/0c/00/00/01」。
// 与本仓 `boostup.next()` 越过最后一关后的 `Step=12 / Phase=0 / Finished=true` 正好吻合。
//
// 原实现有一条 `mode==2 && active` 的守卫会拒发这条帧（零测试覆盖），
// 于是第 11 关领奖整笔被拒 —— 实机 2026-10-06 03:39:44
// `boost_event_request_refused: finished boost training cannot be active`。
// 这条测试把官服口径钉住，防止守卫被"修"回来。
func TestBoostTrainingStatusGraduatedFrame(t *testing.T) {
	got, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 2, Step: 12, Phase: 0, Active: true})
	if e != nil {
		t.Fatalf("毕业态被拒：%v", e)
	}
	if want := []byte{0x02, 0x0c, 0x00, 0x00, 0x01}; !bytes.Equal(got, want) {
		t.Fatalf("毕业帧 = % x, want % x", got, want)
	}
	// 保留的不变量：未激活不得携带进度。
	if _, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 2, Step: 12, Active: false}); e == nil {
		t.Fatal("未激活却携带进度被放过")
	}
	// 训练中（mode=0）不受影响。
	if got, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 0, Step: 11, Phase: 1, Active: true}); e != nil || !bytes.Equal(got, []byte{0x00, 0x0b, 0x01, 0x00, 0x01}) {
		t.Fatalf("训练中帧 = % x err=%v", got, e)
	}
}

// TestBoostTrainingTrackByte 钉住 mode 字节的取值域是**轨道号** {0 普通, 1 奶系, 2 未参与/
// 已结束}，不是完成标志。依据（Dump analysis/dumps/boost-buffer-route/）：
// sub_14074C6B0 的路线内层键 variant = (mode==1)、sub_140C13440 按 mode==1 选
// [buffer reward]、sub_14074DEF0 按 mode!=2 判定参与。原守卫只放行 {0,2}，
// 奶系轨被编码错误挡住 ⇒ 客户端回落普通轨副本（2026-10-07 实机第二关进不去）。
func TestBoostTrainingTrackByte(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		got, e := BoostTrainingStatus115(BoostTrainingState115{Mode: mode, Step: 2, Active: true})
		if e != nil || !bytes.Equal(got, []byte{mode, 0x02, 0x00, 0x00, 0x01}) {
			t.Fatalf("轨道 %d 帧 = % x err=%v", mode, got, e)
		}
	}
	if _, e := BoostTrainingStatus115(BoostTrainingState115{Mode: 3, Step: 2, Active: true}); e == nil {
		t.Fatal("未知轨道被放过")
	}
}

// TestBoostRosterRowWidth115 钉住 NOTI2639 的行宽契约（IDA：handler sub_140C131D0
// 每行读 u8+u32+u8+u8 = 7 字节；官服 cap43 单行帧 `01000000 03 00000000 01 00`）。
// 旧 donor 布局行宽 5 字节：1 行时恰好落在补齐后的 16 字节里，2 行时客户端要 18 字节
// 却只有 16 ⇒ 游标越界，客户端回 CMD217(OVERFLOW) 体内 0x0A4F=2639（2026-10-06 实机：
// 同账号第二个角色直升后卡在赛利亚、选角名单空白）。
func TestBoostRosterRowWidth115(t *testing.T) {
	empty, e := BoostRoster115(nil)
	if e != nil || hex.EncodeToString(empty) != "00000000" {
		t.Fatalf("空名单 = % x err=%v", empty, e)
	}
	one, e := BoostRoster115([]BoostRosterRow115{{Slot: 0, Mode: 0}})
	if e != nil {
		t.Fatal(e)
	}
	if want := "0100000003000000000000"; hex.EncodeToString(one) != want {
		t.Fatalf("单行 = %s, want %s（官服同位只差状态字节 01/00）", hex.EncodeToString(one), want)
	}
	two, e := BoostRoster115([]BoostRosterRow115{{Slot: 0, Mode: 0}, {Slot: 1, Mode: 2}})
	if e != nil {
		t.Fatal(e)
	}
	if want := "020000000300000000000003010000000200"; hex.EncodeToString(two) != want {
		t.Fatalf("双行 = %s, want %s", hex.EncodeToString(two), want)
	}
	// 客户端按 7 字节行宽读取，正文长度必须正好 4+7N；短于它就越界（CMD217 触发条件）。
	for n, body := range map[int][]byte{0: empty, 1: one, 2: two} {
		if len(body) != 4+n*BoostRosterRowBytes115 {
			t.Fatalf("%d 行正文 %d 字节，客户端需要 %d", n, len(body), 4+n*BoostRosterRowBytes115)
		}
	}
	if _, e := BoostRoster115([]BoostRosterRow115{{Slot: 3, Mode: 0}, {Slot: 3, Mode: 2}}); e == nil {
		t.Fatal("重名单位次被放过")
	}
}

// TestDecodeBoostCapsule115BufferVariant 钉住两种胶囊的实机正文（同会话
// 2026-10-06 18:38:49 / 18:41:44，均 64 字节明文）：
//
//	43000000000000510100000000000000…  普通胶囊 590015870，源变体 0，参数格 0
//	43000000000000510100000100000000…  缓冲（奶系）胶囊 590015871，源变体 1，参数格 1
//
// 第二帧曾被通用 CMD507 零校验拒成 "unsupported stackable action fields"，
// 玩家侧表现为「奶系专用胶囊使用没有效果」。
func TestDecodeBoostCapsule115BufferVariant(t *testing.T) {
	plain, _ := hex.DecodeString("43000000000000510100000000000000" + strings.Repeat("00", 48))
	buffer, _ := hex.DecodeString("43000000000000510100000100000000" + strings.Repeat("00", 48))
	for name, p := range map[string][]byte{"普通": plain, "缓冲": buffer} {
		if len(p) != 64 {
			t.Fatalf("%s 帧长度 %d", name, len(p))
		}
		r, e := DecodeBoostCapsule115(p)
		if e != nil || r.Slot != 67 || r.Space != 0 {
			t.Fatalf("%s 胶囊帧被拒：%v (%v)", name, r, e)
		}
	}
	// 参数格不参与变体判定：两帧解出的请求必须完全相同（变体只认模板）。
	a, _ := DecodeBoostCapsule115(plain)
	b, _ := DecodeBoostCapsule115(buffer)
	if a != b {
		t.Fatalf("参数格影响了请求：普通=%+v 缓冲=%+v", a, b)
	}
	// 通用零校验仍然照旧拒这一帧——分流是对的，不是把通用解码器放松。
	if _, _, e := DecodeStackableAction(buffer); e == nil {
		t.Fatal("通用 stackable 解码器接受了缓冲胶囊的参数格")
	}
	// 动作号与长度仍是硬门禁；槽后第 3~6 字节的 u32 必须为 0。
	wrong := append([]byte{}, buffer...)
	wrong[7] = 0x52
	if _, e := DecodeBoostCapsule115(wrong); e == nil {
		t.Fatal("非胶囊动作被接受")
	}
	if _, e := DecodeBoostCapsule115(buffer[:58]); e == nil {
		t.Fatal("短帧被接受")
	}
	if _, e := DecodeBoostCapsule115(append([]byte{}, buffer[:59]...)); e != nil {
		t.Fatalf("59 字节原生长度帧应可解：%v", e)
	}
	mutated := append([]byte{}, buffer...)
	mutated[15] = 1
	if _, e := DecodeBoostCapsule115(mutated); e == nil {
		t.Fatal("参数格之后的非零字节被接受")
	}
}
