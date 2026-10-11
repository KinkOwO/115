package protocol

import "fmt"

// Avatar Closet（寄存衣柜）：基础 1 槽，买 Tier1/2/3 各 +1，上限 4 槽。
// 已购档位数（0..3）与槽数（1..4）的关系：slots = expansion + 1。
const (
	MaxAvatarClosetExpansion byte = 3                  // Tier1/2/3
	MaxAvatarClosetSlots     byte = MaxAvatarClosetExpansion + 1 // 4
)

// AvatarClosetSlots 把已购档位数换算成槽数（1..4）。
func AvatarClosetSlots(expansion byte) byte {
	if expansion > MaxAvatarClosetExpansion {
		expansion = MaxAvatarClosetExpansion
	}
	return expansion + 1
}

// AvatarClosetInfo 构造 ENUM_NOTIPACKET_AVATAR_CLOSET_INFO (noti 1076) 的包体。
//
// 官方 2026-10-02 抓包（每会话登录一帧，10 帧全同）：
//
//	01 00 00 00 00 00 00 00 ae 6b cc 64 3c 00 00 00
//
// 首字节 = 衣柜槽数（官方那份是 1 槽基础态；实测下发 01 客户端衣柜衣架解锁）。
// 其余 15 字节原样保留官方定值（末段 ae6bcc64 疑为固定时间戳，3c=60 疑为容量，语义未逆清）。
func AvatarClosetInfo(slots byte) ([]byte, error) {
	if slots < 1 || slots > MaxAvatarClosetSlots {
		return nil, fmt.Errorf("invalid avatar closet slots")
	}
	return []byte{slots, 0, 0, 0, 0, 0, 0, 0, 0xae, 0x6b, 0xcc, 0x64, 0x3c, 0, 0, 0}, nil
}

// AvatarClosetSet 构造 ENUM_NOTIPACKET_CHANGE_AVATAR_CLOSET_SET (noti 1077) 的包体。
//
// 反汇编客户端 reader（DFO.exe 0x1449e8100）：先 ReadU32(n) 读条目数，n<=0 直接返回；
// 否则逐条 ReadU8 + ReadU16 + ReadU8(mode) + ReadU16 + mode 专属载荷。承载衣柜里
// “摆了什么”的内容列表。当前实现只发**空列表**（条目数 0），即“衣柜里没摆东西”，
// 与服务端尚未实现摆放一致；待抓/逆到真实条目结构后再补充。
func AvatarClosetSet() []byte {
	return []byte{0, 0, 0, 0}
}

