package protocol

import "fmt"

// 组队邀请/入队流的 S→C 通知构造。
//
// 每个布局都来自客户端消费侧处理器的读取顺序（donor 2.38.3.25 库反编译，
// E:\115us\ida-work\r2-decomp\，处理器地址与NOTI注册表见 r2-maps.json），
// 字段宽度与顺序 1:1 对应客户端的流读取调用：
//   sub_146EA09F0(&v, n) = 读 n 字节、sub_146EA1920 = 读 u16、sub_146EA0BA0 = 读 u32。
// 这些是「客户端怎么读，服务端就怎么写」的直译，不含猜测字段。

// PartyInviteMember115 是 NOTI 348 (ENUM_NOTIPACKET_INVITE_MEMBER) 的载荷。
// 客户端处理器 sub_146924880：kind 为 100/102 时整包忽略；随后读
// u8 lo、u32 成员id、u8 extra 计数与 extra 个单字节，成员名先以
// dstr 669（"Nameless"）占位。
func PartyInviteMember115(kind, keyLo byte, memberID uint32, extra []byte) ([]byte, error) {
	if len(extra) > 255 {
		return nil, fmt.Errorf("invite-member extra overflow")
	}
	p := []byte{kind, keyLo}
	p = add32(p, memberID)
	p = append(p, byte(len(extra)))
	p = append(p, extra...)
	return p, nil
}

// PartyInviteRow 是 NOTI 645 (ENUM_NOTIPACKET_ENTRY_PARTY_WAIT) 名册的一行。
// 两个状态字节与两个 u32 的游戏语义尚未取证（处理器 sub_144647F50 原样透传）。
type PartyInviteRow struct {
	Actor    uint16
	StateA   byte
	StateB   byte
	DungeonA uint32
	DungeonB uint32
}

// PartyEntryWait115 渲染 NOTI 645：u16 本机actor、u32 行数、每行
// {u16 actor, u8, u8, u32, u32}。
func PartyEntryWait115(self uint16, rows []PartyInviteRow) ([]byte, error) {
	if self == 0 || self == 65535 {
		return nil, fmt.Errorf("entry-wait invalid self actor")
	}
	p := add16(nil, self)
	p = add32(p, uint32(len(rows)))
	for _, r := range rows {
		if r.Actor == 0 || r.Actor == 65535 {
			return nil, fmt.Errorf("entry-wait invalid row actor")
		}
		p = add16(p, r.Actor)
		p = append(p, r.StateA, r.StateB)
		p = add32(p, r.DungeonA)
		p = add32(p, r.DungeonB)
	}
	return p, nil
}

// PartyEntryFinish115 渲染 NOTI 646 (ENUM_NOTIPACKET_ENTRY_INTO_PARTY_FINISH)：
// u16 actor、u32、u32。处理器 sub_144647CB0 只在本机 actor 命中时生效。
func PartyEntryFinish115(actor uint16, valueA, valueB uint32) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("entry-finish invalid actor")
	}
	p := add16(nil, actor)
	p = add32(p, valueA)
	p = add32(p, valueB)
	return p, nil
}

// PartyEntryUpdateRow 是 NOTI 647 (ENUM_NOTIPACKET_ENTRY_INTO_PARTY_UPDATE) 的一行。
type PartyEntryUpdateRow struct {
	ID    int32
	State byte
}

// PartyEntryUpdate115 渲染 NOTI 647：u32 行数、每行 {s32 id, u8 state}
// （处理器 sub_144647DF0）。
func PartyEntryUpdate115(rows []PartyEntryUpdateRow) []byte {
	p := add32(nil, uint32(len(rows)))
	for _, r := range rows {
		p = add32(p, uint32(r.ID))
		p = append(p, r.State)
	}
	return p
}

// PartyWalkOutNotice115 渲染 NOTI 10（踢人/离队通知；处理器 sub_1452ECD30，
// 老核心注册表 sub_1452F9420 槽位 10）：
//
//	u8 队伍槽位, u8 类型 —— 类型 0 = "%s has been expelled from the Party."
//	（队友被移出）、类型 1 = "You have been expelled from the Party."（你被移出）。
func PartyWalkOutNotice115(slot, kind byte) ([]byte, error) {
	if slot > 7 {
		return nil, fmt.Errorf("walk-out slot overflow")
	}
	return []byte{slot, kind}, nil
}

// PartyEntryRequest115 渲染 NOTI 697（S→C ENTRY_INTO_PARTY；处理器
// sub_1452E0710，老核心注册表槽位 697）：u16 actor —— 「该玩家请求加入/受邀」，
// 名字由客户端按 actor 从本地区域用户表补齐后弹邀请窗。
func PartyEntryRequest115(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("entry-request invalid actor")
	}
	return add16(nil, actor), nil
}

// PartyEntryResult115 渲染 NOTI 698（S→C ENTRY_INTO_PARTY_FINISH；处理器
// sub_1452E1B40，老核心注册表槽位 698）：u8 结果（0=接受）+ u16 actor。
func PartyEntryResult115(result byte, actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("entry-result invalid actor")
	}
	return append([]byte{result}, add16(nil, actor)...), nil
}
