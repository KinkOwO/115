package protocol

import (
	"encoding/binary"
	"fmt"
)

// 征服/攻坚队频道的 (Mode, Route, 是否带成员计数区)。
//
// 来源：《多人组队：完整实现参考》§6.1 的频道映射表（L1，由 L0 推出）。
// **独立佐证**：官服 2026-10-03 蔚蓝号抓包（ch.76）实测 Mode=0x1f(31)、route 字节 0x25(37)，
// 与该表的 102 行一致（见 conquest_party115_test.go 的 golden 用例）。
//
// 表里没有的频道一律报错 —— 不替客户端猜 Mode/Route。
type ConquestChannelMode115 struct {
	Mode     byte
	Route    byte
	Counters bool
}

var conquestPartyChannels115 = map[uint32]ConquestChannelMode115{
	101: {Mode: 27, Route: 41, Counters: true},  // Moon Lake
	102: {Mode: 31, Route: 37, Counters: true},  // Azure Main（蔚蓝号）
	103: {Mode: 30, Route: 36, Counters: true},  // Temple of Death
	108: {Mode: 35, Route: 74, Counters: false}, // Unshackled Nightmare
	116: {Mode: 41, Route: 83, Counters: true},  // Constellation Turtle Library
	117: {Mode: 42, Route: 84, Counters: true},  // Castle of the Apostate
}

func ConquestPartyModeForChannel115(channel uint32) (ConquestChannelMode115, bool) {
	m, ok := conquestPartyChannels115[channel]
	return m, ok
}

// 名册里两段变长 LENBLOB 在**契约布局**上的位置（u32 长度 + 数据，长度字段本身也要写）。
//
// 契约那一套（legion_party_base115.go）把两段都留空 —— 空 LENBLOB 仍是 4 个 0 字节，
// 所以对它有队名/无队名的两种形态都是恒等变换。官服 102 帧里两段都非空（3 / 60），
// 因此契约字段整体后移：
//
//	shift(k) = k + titleLen                  (k <  65)
//	shift(k) = k + titleLen + secondLen      (k >= 65)
//
// 已对官服 #369/#427 逐字段核过（conquest_party115_test.go）。
const (
	partyRosterTitleBlob115   = 14
	partyRosterSecondBlob115  = 65
	partyRosterSecondBound115 = 65 // 从该偏移起受第二段长度影响
	partyRosterBaseRows115    = 90 // 基础布局 = 90 + 17N
	partyRosterRowBytes115    = 17
)

// ConquestRoster115 是征服队（N9 子命令 0 全量名册）的编码输入。
type ConquestRoster115 struct {
	PartyID uint16
	Context [2]byte
	// Seats 是**原生槽位号 → actor** 的映射；0 表示空槽。基础布局只支持 4 席
	// （与其上游 PartyRosterSeats115 的 capacity<=4 校验一致；官服 102 帧的 capacity 也是 4）。
	Seats  [4]uint16
	Leader uint16
	Title  string
	// Options 来自 CMD12 解码；Capacity/Info/Byte5/Word6/Byte8/ModeValue/SlotFilters/
	// Selection/Variant 都会被写进名册。
	Options PartyCreateOptions115
	// Mode/Route 由 ConquestPartyModeForChannel115 给出，必须与 Options.Mode 一致。
	Mode  byte
	Route byte
	// SecondBlock 是契约 [65:69] 那一段 LENBLOB 的数据。
	//
	// ⚠️ 语义未闭环：官服 102 帧实测长度 60、内容 60 个 0。本字段**必须**与官服等长，
	// 否则它后面的 N / 行 / 第二处 Mode 全部落到错误偏移上（模型里最容易踩的一脚）。
	// nil 表示按官服形态补 60 个 0；显式给空 slice 表示留空（契约形态）。
	SecondBlock []byte
	// RouteInTail 决定 route 是否写到 base-1（契约/沉月湖的放置）。
	// 默认 false = 写 [8]（与官服 102 帧一致）。两个位置都有证据，留开关由实机定夺。
	RouteInTail bool
	// Counters 为 nil 表示不追加成员计数区；非 nil 时追加 u32(席位数) + 这些字节（每席位 2 字节）。
	// ⚠️ 语义未闭环（官服帧此处 22 字节与该形状不符），见测试里的说明。
	Counters []byte
}

// Encode 产出 N9 全量名册载荷。
func (r ConquestRoster115) Encode() ([]byte, error) {
	if r.Options.Mode != r.Mode {
		return nil, fmt.Errorf("conquest mode mismatch: options=%d channel=%d", r.Options.Mode, r.Mode)
	}
	base, err := PartyRosterSeats115(r.PartyID, r.Context, r.Seats, r.Leader, r.Options.Capacity)
	if err != nil {
		return nil, err
	}
	seats := 0
	for _, a := range r.Seats {
		if a != 0 {
			seats++
		}
	}
	second := r.SecondBlock
	if second == nil {
		second = make([]byte, 60) // 官服 102 帧形态：长度 60、内容全 0
	}
	out, err := insertLenBlob115(base, partyRosterTitleBlob115, []byte(r.Title))
	if err != nil {
		return nil, err
	}
	out, err = insertLenBlob115(out, partyRosterSecondBlob115+len(r.Title), second)
	if err != nil {
		return nil, err
	}
	if err = applySpecialPartyOptionsShifted115(out, r.Options, r.Mode, len(r.Title), len(second)); err != nil {
		return nil, err
	}
	// 契约的 base-1 = 89+17N（基础布局坐标），位移到实际载荷坐标。
	if r.RouteInTail {
		out[len(r.Title)+len(second)+89+partyRosterRowBytes115*seats] = r.Route
	} else {
		// 官服 102 帧实测：[8] = 0x25 = 37 = 该频道 route；base-1 = 0。
		out[8] = r.Route
	}
	if r.Counters != nil {
		if len(r.Counters)%2 != 0 {
			return nil, fmt.Errorf("conquest member counters must be 2 bytes per seat")
		}
		var n [4]byte
		binary.LittleEndian.PutUint32(n[:], uint32(len(r.Counters)/2))
		out = append(out, n[:]...)
		out = append(out, r.Counters...)
	}
	return out, nil
}

// insertLenBlob115 把 [at:at+4] 那 4 个 0 换成「u32 长度 + 数据」。
//
// 空数据时**逐字节恒等**（4 个 0 换成 4 个 0）—— 这正是沉月湖（无队名）与契约那套
// 能直接吃基础布局的原因，也是本函数最重要的不变量（有测试）。
func insertLenBlob115(p []byte, at int, data []byte) ([]byte, error) {
	if at < 0 || at+4 > len(p) {
		return nil, fmt.Errorf("len blob offset %d out of range (len %d)", at, len(p))
	}
	if len(data) > 255 {
		return nil, fmt.Errorf("len blob too long: %d", len(data))
	}
	out := make([]byte, 0, len(p)+len(data))
	out = append(out, p[:at]...)
	var ln [4]byte
	binary.LittleEndian.PutUint32(ln[:], uint32(len(data)))
	out = append(out, ln[:]...)
	out = append(out, data...)
	out = append(out, p[at+4:]...)
	return out, nil
}

// applySpecialPartyOptionsShifted115 与 legion_party_base115.go 的 applySpecialPartyOptions115 同义，
// 但偏移按两段 LENBLOB 的实际长度后移，且 mode 由参数给出（该频道自己的 Mode）。
//
// titleLen==secondLen==0 时必须与原版逐字节一致 —— 有测试兜底，防止两套实现漂移。
// route **不在**这里写：官服 102 帧里 base-1 是 0、route 在 [8]，由调用方决定（见 Encode）。
func applySpecialPartyOptionsShifted115(p []byte, o PartyCreateOptions115, mode byte, titleLen, secondLen int) error {
	if titleLen < 0 || secondLen < 0 {
		return fmt.Errorf("negative conquest roster shift")
	}
	baseLen := len(p) - titleLen - secondLen
	seats := 0
	if baseLen > partyRosterBaseRows115 && (baseLen-partyRosterBaseRows115)%partyRosterRowBytes115 == 0 {
		seats = (baseLen - partyRosterBaseRows115) / partyRosterRowBytes115
	}
	if seats < 1 {
		return fmt.Errorf("conquest roster base length %d is not 90+17N", baseLen)
	}
	at := func(k int) int {
		if k >= partyRosterSecondBound115 {
			return k + titleLen + secondLen
		}
		return k + titleLen
	}
	p[at(20)] = o.Capacity
	binary.LittleEndian.PutUint32(p[at(21):at(25)], o.Info)
	p[at(25)], p[at(28)] = o.Byte5, o.Byte8
	binary.LittleEndian.PutUint16(p[at(26):at(28)], o.Word6)
	p[at(29)] = mode
	binary.LittleEndian.PutUint16(p[at(30):at(32)], o.ModeValue)
	copy(p[at(46):at(54)], o.SlotFilters[:])
	binary.LittleEndian.PutUint32(p[at(60):at(64)], o.Selection)
	p[at(64)] = o.Variant
	// 第二处 Mode：原生 1452F4413 读、1452F44C5 写到 PartyData+122；
	// 不写它，普通尾部的一串 0 会把 p[29] 的效果抹掉。
	p[at(78+partyRosterRowBytes115*seats)] = mode
	return nil
}

// ConquestPartyGone115 是征服队的「队伍消失」通知（N9 子命令 3）。
//
// 形状取自契约 §6.3（12 字节）。⚠️ 官服 102 会话里那条同语义的帧是 32 字节，
// 与这里不同 —— 本函数走契约形态，由实机判定（见证据文档 §7.7）。
func ConquestPartyGone115(partyID uint16, context [2]byte, route byte) ([]byte, error) {
	if partyID == 0 || partyID == 65535 {
		return nil, fmt.Errorf("invalid party id")
	}
	p := make([]byte, 12)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], PartyInfoType)
	binary.LittleEndian.PutUint16(p[4:], partyID)
	copy(p[6:8], context[:])
	p[8] = route
	p[9] = 3
	p[10] = 1
	return p, nil
}
