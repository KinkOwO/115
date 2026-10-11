package main

// **构造**出来的 NOTI108 活动行（官服抓包里没有这一期的那一条）。
//
// 与 event_info_activity.go 的分工：那里是**逐字切片**（真源 = 官服抓包 77 条记录，
// 有就是有、没有就是没有）；这里是我们**按客户端的行长自己拼**的行，用在
// 「PVF 里有内容、但抓包那一期这个活动不在线」的情况。两者都必须与官方的行长一致：
//
//	u16 id, u8 present(1), u8 a, u8 b, str name, str s2, str url,
//	u32 start, u32 end, str calendar, str extra, u8 flag
//	str = u32 字节长 + 窄字节串（无结尾 0）
//
// 拼出来的行**没有官服字节可以对照** ⇒ 只能靠两件事守：① 行长与官方行一致（用例钉形状）；
// ② 实机看客户端认不认（`[event on]` 里出现该活动号）。**不要**把它当成"与官服一致"。
import (
	"dfolan/internal/aradpass"
	"encoding/binary"
)



// aradPassEventID / aradPassAltEventID 是战令（阿拉德通行证）在客户端活动表里的
// 「开放时段 id」。
//
// 来源：抓包实证 —— 玩家点 Get 发的 CMD680 正文第一格就是 **516**；10005 是同一活动
// 的另一个开放时段（来源实现的开放时段基线表里 516 与 10005 同时列出）。两个都下发，
// 免得客户端的页签门禁查的是另一个。语义定义在 `internal/aradpass`（IsSchedule）。
const (
	aradPassEventID    uint16 = uint16(aradpass.ClaimScheduleID)
	aradPassAltEventID uint16 = uint16(aradpass.AltScheduleID)
)

// aradPassEventStart / aradPassEventEnd 是战令本季的运营窗口，取自脚本自己的
// `[event period]`：`2026-08-04 09:00:00` → `2026-10-20 09:00:00`。
//
// ⚠️ 与 313 同一条换算纪律（`.evt` 里是 KST，op108 行里是 Unix 秒）：
// 09:00 KST = 前一日 00:00 UTC。
const (
	aradPassEventStart uint32 = 1785801600 // 2026-08-04 00:00 UTC
	aradPassEventEnd   uint32 = 1792454400 // 2026-10-20 00:00 UTC
)

// constructedActivityEventIDs 是本文件里**构造**出来的活动行 id（表头 count 要算上它们）。
var constructedActivityEventIDs = []uint16{aradPassEventID, aradPassAltEventID}

// constructedActivityEventRows 返回构造行的字节（顺序与 constructedActivityEventIDs 一致）。
func constructedActivityEventRows() []byte {
	var out []byte
// 战令：与其它构造行同一取舍（A/B 取 2/4、URL/s2/calendar/extra 全空）。
	out = appendActivityEventInfoRow(out, activityEventInfoRow{
		ID:    aradPassEventID,
		A:     2,
		B:     4,
		Name:  "Arad Pass",
		Start: aradPassEventStart,
		End:   aradPassEventEnd,
		Flag:  1,
	})
	out = appendActivityEventInfoRow(out, activityEventInfoRow{
		ID:    aradPassAltEventID,
		A:     2,
		B:     4,
		Name:  "Arad Pass",
		Start: aradPassEventStart,
		End:   aradPassEventEnd,
		Flag:  1,
	})
	return out
}

// activityEventInfoRow 是行长字段的具名形态，只在本文件用。
type activityEventInfoRow struct {
	ID       uint16
	A, B     byte
	Name     string
	URL      string
	Start    uint32
	End      uint32
	Calendar string
	Extra    string
	Flag     byte
}

func appendActivityEventInfoRow(dst []byte, r activityEventInfoRow) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint16(buf[:2], r.ID)
	dst = append(dst, buf[:2]...)
	dst = append(dst, 1, r.A, r.B) // present 恒 1
	dst = appendActivityEventString(dst, r.Name)
	dst = appendActivityEventString(dst, "") // s2：官服也全空
	dst = appendActivityEventString(dst, r.URL)
	binary.LittleEndian.PutUint32(buf[:], r.Start)
	dst = append(dst, buf[:]...)
	binary.LittleEndian.PutUint32(buf[:], r.End)
	dst = append(dst, buf[:]...)
	dst = appendActivityEventString(dst, r.Calendar)
	dst = appendActivityEventString(dst, r.Extra)
	return append(dst, r.Flag)
}

func appendActivityEventString(dst []byte, s string) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(len(s)))
	dst = append(dst, buf[:]...)
	return append(dst, s...)
}
