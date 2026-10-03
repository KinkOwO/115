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
func SettlementExitRefused(option byte) []byte      { return append(Refusal(4), option) }
