package protocol

import (
	"bytes"
	"fmt"
)

type CardSelection struct{ Side, Index byte }

// Current146ab9103/8f47 write two u8 values. Transport pads to a block.
func DecodeCardSelection(p []byte) (CardSelection, error) {
	if len(p) != 2 && len(p) != 8 && len(p) != 16 {
		return CardSelection{}, fmt.Errorf("invalid card request size")
	}
	for _, b := range p[2:] {
		if b != 0 {
			return CardSelection{}, fmt.Errorf("nonzero card padding")
		}
	}
	if p[0] > 1 || p[1] > 3 {
		return CardSelection{}, fmt.Errorf("invalid card side/index")
	}
	return CardSelection{p[0], p[1]}, nil
}
func CardLayout() []byte {
	p := []byte{1}
	for i := 0; i < 8; i++ {
		n := uint16(65535)
		if i == 0 {
			n = 1
		}
		p = add16(p, n)
	}
	return p
}

// CMD71's native handler unconditionally reads all eight rows. A short
// generic refusal would overrun the packet; return a full owned snapshot.
func CardSelected(index int) ([]byte, error) {
	if index < -1 || index > 3 {
		return nil, fmt.Errorf("invalid selected card")
	}
	p := []byte{1}
	for i := 0; i < 8; i++ {
		a := byte(255)
		if i == index {
			a = 0
		}
		p = append(p, a, 255, 0, 0)
	}
	return p, nil
}

type SettlementExit struct{ State, Option byte }

// SettlementExitSeamless is option 5: the EPLP seamless rechallenge the
// right-edge of the clear panel sends (CMD 72 ENUM_CMDPACKET_EPLP_COMMAND,
// body 01 05 01). The sender is the client's own dungeon module, so the
// gateway only has to stop rejecting it.
const SettlementExitSeamless byte = 5

// KeepsDungeonSelection reports whether this settlement exit leaves the client
// in the dungeon-selection flow. Derive it from the request rather than the
// outgoing acknowledgement's envelope or byte offsets.
func (r SettlementExit) KeepsDungeonSelection() bool { return r.Option == 1 }

func DecodeSettlementExit(p []byte) (SettlementExit, error) {
	if len(p) != 3 && len(p) != 8 && len(p) != 16 {
		return SettlementExit{}, fmt.Errorf("invalid exit body")
	}
	// Current146ab8b43/54/63 writes state, option, literal1.
	// 2026-10-03 伊斯实测与官服 s4 对照确认存在三种 source 字节与一个常量 token：
	//   - p[2]=1：结算面板（边界之调律 RE 先例）；
	//   - p[2]=0：副本内「撤退」对话框发送器（私服 2.38.2 实测 5× `01 02 00`，
	//     此前被拒导致撤退是死按钮）；
	//   - p[2]=2：官服伊斯客户端奖励领取后的退出（s4 帧 498 `01 01 02`，
	//     紧跟 CMD2046 帧 496）。
	// 官服 s4 帧 342/402/498 与私服实测 5 帧的 16B 体在 p[3:8] 逐字节一致地
	// 携带常量 token c5 20 24 76 3f（跨新旧客户端相同 ⇒ 客户端侧常量或对
	// 服务端某帧的回执），按已知常量放行；普通副本的 16B 体仍是全零 padding。
	if p[2] > 2 {
		return SettlementExit{}, fmt.Errorf("unsupported exit source")
	}
	if len(p) >= 8 {
		if bytes.Equal(p[3:8], settlementExitEchoToken) {
			for _, b := range p[8:] {
				if b != 0 {
					return SettlementExit{}, fmt.Errorf("nonzero exit padding")
				}
			}
		} else {
			for _, b := range p[3:] {
				if b != 0 {
					return SettlementExit{}, fmt.Errorf("nonzero exit padding")
				}
			}
		}
	} else {
		for _, b := range p[3:] {
			if b != 0 {
				return SettlementExit{}, fmt.Errorf("nonzero exit padding")
			}
		}
	}
	// Option 5 is the EPLP seamless rechallenge (right-edge walk-in). The
	// native sender writes state, option, literal1 just like options 0..3, so
	// only the range guard had to widen. Its body shape is pinned below by
	// the same length and padding rules.
	if (p[0] != 1 && p[0] != 2) || (p[1] > 3 && p[1] != SettlementExitSeamless) {
		return SettlementExit{}, fmt.Errorf("unsupported exit action")
	}
	return SettlementExit{p[0], p[1]}, nil
}

// settlementExitEchoToken 是伊斯大陆会话 CMD72 16B 体内 p[3:8] 的常量回执
// （官服 s4 与私服 2.38.2 逐字节一致，语义未解——字节必须来自实证）。
var settlementExitEchoToken = []byte{0xc5, 0x20, 0x24, 0x76, 0x3f}

// SettlementExitSuccess includes the common CMD success byte consumed at
// 0x1459a1ca2 before dispatch to handler 0x145244570. The handler then reads
// state and option at 0x1452445ad/5b9. The native_card_exit fixtures cover only
// those two handler bytes; transport does not prepend the common status.
func SettlementExitSuccess(r SettlementExit) []byte { return []byte{1, r.State, r.Option} }

// azureSettlementOptionToken 是 CMD72 16B 体内 p[3:8] 的客户端常量
// （与 settlementExitEchoToken 同一个值，跨新旧客户端一致）。
var azureSettlementOptionToken = [5]byte{0xc5, 0x20, 0x24, 0x76, 0x3f}

// ⚠️ 实测（2026-10-04）：**这三帧不要发**。
//
// 先把结算尾帧从 CMD1654 的应答挪到发奖那一步、同时登记 1654 路由 ⇒ 客户端**闪退**；
// 回退后**只保留 1654 路由**（azureClearInfo 只回 NOTI2621 阶段 4/5 + N31 全 0）⇒
// `Back to Town (F12)` 就**已可用**（业主实机截图确认）。也就是说这一组并非必需，
// 而很可能是那次闪退的来源（形状虽按官服逐字节复刻，但官服把它们夹在 N9/N435/N261
// 之间，我全挤在 N35 后面，**时机/顺序**可能与形状一样关键）。
// 构造函数与测试先留着作为取证底稿，但**不要在游戏链路里调用**。
//
// AzureSettlementOptionEnable 构造 NOTI70：结算面板的**选项使能掩码**。
//
// 官服尾段在 NOTI72 之前先发这一帧（F16-s2c.txt #687，32B）：
//
//	01 01 00 <14 x FF> <5B 值> <10 x 00>
//
// 一串 0xFF 读作"全部使能"。上一轮只补了 NOTI72，面板画出来了但
// 「Back to Town」是**灰的**（业主实机 2026-10-04 截图）—— 缺的就是这一帧。
// 尾部的 5B 值属本仓已多次验证「可省略」的那一类（N29 尾部 / N38 尾部），留 0。
func AzureSettlementOptionEnable() []byte {
	p := []byte{1, 1, 0}
	for i := 0; i < 14; i++ {
		p = append(p, 0xff)
	}
	return append(p, make([]byte, 15)...)
}

// AzureSettlementOptionRows 构造 NOTI71：结算面板的**选项行表**。
//
// 官服 F16-s2c.txt #692（40B）：`01 00 ff 00 00` + 7 x `ff ff 00 00` + <5B 值> + `00 00`。
// 与 NOTI70 一起在 NOTI72 之前发；5B 值同样留 0。
func AzureSettlementOptionRows() []byte {
	p := []byte{1, 0, 0xff, 0, 0}
	for i := 0; i < 7; i++ {
		p = append(p, 0xff, 0xff, 0, 0)
	}
	return append(p, 0, 0, 0, 0, 0, 0, 0)
}

// AzureSettlementOptionOffer 构造一帧 NOTI72，把结算面板上的一个退出选项摆出来。
//
// 官服尾段在通关之后连发两帧（F16-s2c.txt #695 `01 02 02 …`、#696 `01 01 02 …`），
// 也就是依次提供 option 2 与 option 1；体是 `State, Option, 2, <5B 常量>`，
// 与客户端随后的 CMD72 体逐字节同形（cards.go 的注释已记录那个常量）。
//
// 本仓此前只在客户端**先**发 CMD72 时才回一个 ACK，等于从没告诉过客户端「有哪些退出选项」，
// 所以结算面板上一直没有那个按钮（业主实机 2026-10-04）。
func AzureSettlementOptionOffer(state, option byte) []byte {
	p := []byte{state, option, 2}
	p = append(p, azureSettlementOptionToken[:]...)
	return append(p, 0, 0, 0, 0, 0, 0, 0, 0)
}
func SettlementExitRefused(option byte) []byte { return append(Refusal(4), option) }
