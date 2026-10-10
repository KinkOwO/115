package main

// 活动行（NOTI108 EVENT_INFO）—— 除 662/665/10017/10018 之外，按需追加的运营活动。
//
// ## 为什么需要这个文件
//
// 客户端的「活动总开关」是一张位图，**只由 NOTI108 填写**（见 event_info_variant.go 的
// 结论与本仓 `分析：next192`）。本仓此前只下发两类行：
//
//	① 19 条军团/攻坚战页签门（event_info_generated.go，实测有效）；
//	② Boost Up 662 家族的 4 行（protocol.BoostOpeningEvents115）。
//
// 其余活动即使 PVF 里有完整内容（脚本 + UI + `list/event.lst` 登记），因为表里没有行，
// 客户端一律视为「活动关闭」⇒ 入口、横幅、图标都不出现。
//
// ## 行的来源与纪律
//
// 本文件里的行**逐字节取自官服抓包** `internal/legion/event_info_official.plain`
// （同一套 115 客户端，77 条记录；与 eventinfogate 工具用的是同一份真源），
// **不做任何改写**。逐字性由 `event_info_activity_test.go` 从该文件重新切片做逐字节比对来守。
//
// 为什么不整表下发：官服那 77 条里带 `url` / `extra` 的记录（129 / 295 / 521 / 456 / 885 …）
// 引用的是**更早期版本的资源**，本仓 2026-10-04 实测过一次整表下发 ⇒ **选角屏崩溃**，
// 所以 event_info_generated.go 只保留了 19 条「无 banner 载荷」的固定形状记录。
// 新增行沿用同一条判据：**优先选 `url` 与 `extra` 都为空的记录**。
//
// ## 开关
//
// 默认开启。`DFO_EVENT_INFO_ACTIVITY=off`（也接受 `0` / `false`）⇒ 不追加任何活动行，
// 回到加这个文件之前的行为。**换值只需重启，不必重编** —— 这是探针（活动行会不会让
// 选角屏崩）能快速 A/B 的前提。
//
// ⚠️ 只能设在**启动那个 cmd 会话的环境里**，不要写进 `configs/pvf-default.json`：
// `internal/launcher/profile.go` 的 `applyEnvironment` 对 environment 键名是**严格白名单**，
// 兜底分支 `default: return invalidProfileValue(key)` 会把未知键当错误 ⇒ 服务端起不来。
// 启动器用 `os.Environ()` 打底拼子进程环境（internal/launcher/env.go 的 CurrentEnv、
// gateway.go 的 newChildEnv），所以会话环境变量能传到网关。
//
//	set DFO_EVENT_INFO_ACTIVITY=off && 调试启动-本地构建.cmd
import (
	"os"
	"strings"
)

// eventInfoActivityIDs 是要下发的活动行的 id，顺序即表内顺序。
//
// 目前只有一条：
//
//	331  每日签到「7-Day Journey for Sky of a Thousand Seas」
//	     `list/event.lst` → live/event/kor/2026/0326_attendancedailyevent/attendancedailyevent.evt
//	     官服记录：start=1785801600(2026-08-04) end=1791277198(2026-10-06)
//	     cal=`Live/Event/Kor/\2026\0326_AttendanceDailyEvent/cal.xui/0/0`（该 xui 在本机 PVF 里存在）
//	     url 与 extra 本来就都是空串 ⇒ 属「无 banner 载荷」记录，风险面与 662 那几行同级。
//
// 加新行时：只往这个切片里加 id，并把对应记录的原始字节追加到 eventInfoActivityHex 末尾，
// 两者条数必须一致（测试会断言）。
var eventInfoActivityIDs = []uint16{331}

// eventInfoActivityHex 是 eventInfoActivityIDs 各条记录在官服抓包里的**原始字节逐字连接**，
// 不含表头 count(2) 与尾部 0x00（那两个由 buildTownEventInfoTable 统一处理）。
// 顺序与 eventInfoActivityIDs 一一对应。
const eventInfoActivityHex = "" +
	"4b0101020428000000372d446179204a6f75726e657920666f7220536b79206f" +
	"6620612054686f7573616e6420536561730000000000000000802b716a8eb8c4" +
	"6a3a0000004c6976652f4576656e742f4b6f722f5c323032365c303332365f41" +
	"7474656e64616e63654461696c794576656e742f63616c2e7875692f302f3000" +
	"00000001"

// eventInfoActivityTable 解码上表（只解一次）。
var eventInfoActivityTable = mustHexDecode(eventInfoActivityHex)

// activityEventInfoRows 返回本次要追加的活动行；开关关闭时返回 nil。
//
// **在调用时读环境变量**（不是 init 期读一次）：这样测试可以用 t.Setenv 两个方向都覆盖，
// 也让 profile 改值后重启即可生效。
func activityEventInfoRows() []byte {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DFO_EVENT_INFO_ACTIVITY"))) {
	case "off", "0", "false":
		return nil
	}
	return eventInfoActivityTable
}
