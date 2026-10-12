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
	"dfolan/internal/honeytime"
	"encoding/binary"
)

// honeyTimeEventID 是 HoneyTime（每日在线奖励）的活动号。
//
// 来源：`list/event.lst` 的 `313 \`Live/Event/Kor/2023/1026_HoneyTime/HoneyTime.evt\“ ——
// ★ **这一份才是客户端实际加载的**（`internal/honeytime` 现在按索引解析同一份，
// 见 `honeytime.ResolveScriptPath`）。仓库里另有一份
// `live/event/kor/2026/0326_honeytime/honeytime.evt`（「2026 那一期」）
// **没有被任何索引引用**；2026-10-11 之前我们把活动内容建在了它上面，是错的
// （症状：面板图标与邮件奖励对不上）。
const honeyTimeEventID uint16 = 313

// honeyTimeEventStart / honeyTimeEventEnd 是 313 的运营窗口，取自**索引版脚本**（客户端实际
// 加载的那一份，见 `honeytime.ResolveScriptPath`）自己的 `[control panel]`：
// `2026-07-09 09:00:00` → `2026-08-04 09:00:00`。
//
// ⚠️ `.evt` 里的时刻按 **KST(UTC+9)** 折算（旁证：331 的 `.evt` 写 `09:00`，而官服 op108 行
// 的 start=1785801600 正好是 09:00 KST）。折算后 = UTC `2026-07-09 00:00` → `2026-08-04 00:00`。
// 交叉验证：结束值 1785801600 与本文件战令的 `aradPassEventStart` **是同一个数** —— 战令本季
// 就从这一刻开始、两期活动首尾相接，两个独立来源同时指向它。
//
// 客户端**不比较**这个窗口（`[event on]` 只看 present，2026-10-10 实测）⇒ 它对奖励与
// HoneyTime 面板都没有影响（奖励来自脚本的 `[reward data]`，面板只看 1294 + 脚本）。
// 对齐它只为「op108 与脚本自洽」。
//
// ⚠️ 2026-10-11 订正（`next197`）：这两个值原来取自**没被索引引用的那一份**脚本
// （`2026/0326_honeytime`，窗口 `2026-03-26 06:00` → `2026-05-07 06:00` KST）。
// 注意 `honeyTimeEventStart` 就是 `honeytime.CycleStart`（那边是唯一定义），而后者还是
// 角色进度的**期号键** ⇒ 换值会清掉所有角色在本活动上的进度（本次已用
// `dfo-tool honeytime -set-seconds/-set-period-seconds` 把测试角色的进度推回）。
const (
	// honeyTimeEventStart 不手抄数字：它与 `internal/honeytime.CycleStart` 必须是同一个值
	// （那边是唯一定义，网关的 Normalize 与 `dfo-tool honeytime` 都引它）。
	honeyTimeEventStart        = uint32(honeytime.CycleStart)
	honeyTimeEventEnd   uint32 = 1785801600 // 2026-08-04 09:00 KST = 2026-08-04 00:00 UTC
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

// dropsLuckyBoxEventID 是「掉落幸运箱」的活动号。
//
// 来源：list/event.lst 的 766 → Live/Event/Kor/2026/0326_DropsLuckyBox/DropsLuckyBox.evt。
// 它**不在官服那 77 条抓包里** ⇒ 这一行只能按客户端行长自拼（同 313/516）。
// 显示名取自客户端字符串表（`drops lucky box`）。
//
// ⚠️ 2026-10-11 封存：**入口已关闭**（不下发 op108 行），因为该活动只有韩文文案。
const dropsLuckyBoxEventID uint16 = 766

// dropsLuckyBoxEventStart / End 取自该脚本自己的 `[event period]`：
// `2026-03-26 06:00:00` → `2026-04-26 06:00:00`（KST）⇒ UTC `2026-03-25 21:00` → `2026-04-25 21:00`。
// ⚠️ 与 313 同一条换算纪律；客户端不比较该窗口（`[event on]` 只看 present）。
const (
	dropsLuckyBoxEventStart uint32 = 1774472400 // 2026-03-25 21:00 UTC
	dropsLuckyBoxEventEnd   uint32 = 1777150800 // 2026-04-25 21:00 UTC
)

// extraDropEventID / 窗口 / 名字**逐字取自官服 1093 那行**（`event_info_official.plain`，59 B）：
// 它本身 calendar 为空、没有 HUD 图标 ⇒ 这里改用构造行补上 calendar（见下面那段注释）。
const extraDropEventID uint16 = 1093

const (
	extraDropEventStart uint32 = 1785801600 // 2026-08-04 00:00 UTC（官服行原值）
	extraDropEventEnd   uint32 = 1791277194 // 官服行原值
)

// extraDropCalendar 取自**同族另一条官服行 295 的 calendar**（同一份抓包、同一个活动家族）。
// 只借用这一个字段：295 那条还带 url/[show_item_index] extra，那属于「选角屏崩」风险面，不用。
// 该资源在本机 PVF 里存在（pvf 导出 4982 B），与 331 的 cal.xui 同一前提。
const extraDropCalendar = "Live/Event/Kor/2026/0326_OracleResearch/cal.xui/0/0"

// constructedActivityEventIDs 是本文件里**构造**出来的活动行 id（表头 count 要算上它们）。
// ⚠️ 766 已**封存**（2026-10-11）：它的 op108 行不再下发 —— 该活动的 UI 文案在英文表
// `String/UI.uv.str` 里是空的，客户端回落成韩文（详见 next199 §9）。
// 要重开：把 dropsLuckyBoxEventID 加回下面的切片，并恢复 constructedActivityEventRows 里那段。
var constructedActivityEventIDs = []uint16{honeyTimeEventID, extraDropEventID, aradPassEventID, aradPassAltEventID}

// constructedActivityEventRows 返回构造行的字节（顺序与 constructedActivityEventIDs 一致）。
func constructedActivityEventRows() []byte {
	var out []byte
	out = appendActivityEventInfoRow(out, activityEventInfoRow{
		ID:    honeyTimeEventID,
		A:     2, // 与官服活动行同形（331 是 2/4；这里取同一档）
		B:     4,
		Name:  "Honey Time",
		Start: honeyTimeEventStart,
		End:   honeyTimeEventEnd,
		// URL / s2 / calendar / extra **一律留空**：按 next192 §5 R1，带 banner 或脚本串的
		// 记录是「选角屏崩」的风险面。探针阶段只开开关、不引资源；要开面板再逐步加 calendar。
		Flag: 1,
	})
	// 1093 额外掉落奖励：字段沿用官服那行，**只补 calendar**（HUD 图标；空 calendar 就没图标）。
	out = appendActivityEventInfoRow(out, activityEventInfoRow{
		ID:       extraDropEventID,
		A:        2,
		B:        4,
		Name:     "Oracle's Module Research!",
		Start:    extraDropEventStart,
		End:      extraDropEventEnd,
		Calendar: extraDropCalendar,
		// 官服 1093 那行是 0，但**能显示且有图标的 331 是 1**，我们所有构造行也都是 1
		// （军团那 19 条 flag=0 也能显示，说明它不像是"可见性"门；取 1 与 331 同档最稳）。
		Flag: 1,
	})
	// 766 掉落幸运箱**封存**，不再下发（原因见上面的 IDs 注释）。恢复用的那段在这里留档：
	//	out = appendActivityEventInfoRow(out, activityEventInfoRow{
	//		ID: dropsLuckyBoxEventID, A: 2, B: 4, Name: "Drops Lucky Box",
	//		Start: dropsLuckyBoxEventStart, End: dropsLuckyBoxEventEnd, Flag: 1,
	//	})
	// 战令：与 313 同一取舍（A/B 取 2/4、URL/s2/calendar/extra 全空）。
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
