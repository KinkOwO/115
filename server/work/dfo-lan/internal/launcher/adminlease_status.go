package launcher

// adminlease_status.go：只读地判断 SQLite 管理租约是否仍被视为"有人持有"。
//
// 为什么要单独一个函数：adminlease.go 的 ClearStaleAdminLease 面向"业主明确要求停掉这套环境"
// 的场景，结论是**删文件**；而双端同步要的是相反的保守判断 —— 只要说不清持有者是不是还活着，
// 就当作有人持有并拒绝动手（拷一个正在被写的库是这套功能里最不能出的事）。

import (
	"fmt"
	"time"
)

// adminLeaseTTL 必须与 internal/database 的 AdminLeaseTTL 一致（60s）。
//
// 这里不 import database：internal/launcher 至今不依赖持久化层，为了一个常量把整条依赖
// 引进来不划算。两处都写死了同一个口径，改动时两边一起改。
const adminLeaseTTL = 60 * time.Second

// AdminLeaseHeld 报告该存储档的 SQLite 管理租约是否仍被视为有人持有，并给出一句可打印的说明。
//
// 三种"持有"：持有者进程还活着；持有者是否存活问不出来（保守）；租约还在续租窗口内
// （可能是刚起来的持有者，还没写下一次的 mtime）。只有"持有者明确不存在且已超出续租窗口"
// 才算废弃，此时返回 false。
func AdminLeaseHeld(cfg StorageConfig) (bool, string) {
	state, present, err := InspectAdminLease(cfg)
	if err != nil {
		return true, fmt.Sprintf("读管理租约失败，按有人持有处理：%v", err)
	}
	if !present {
		return false, ""
	}
	if state.HasHolder {
		switch processStatus(state.Pid) {
		case PidAlive:
			return true, fmt.Sprintf("%s 由进程 %d 持有（最近一次续租 %s 前）",
				state.Path, state.Pid, state.Age.Round(time.Second))
		case PidUnknown:
			return true, fmt.Sprintf("%s 的持有进程 %d 是否存活无法确认（最近一次续租 %s 前）",
				state.Path, state.Pid, state.Age.Round(time.Second))
		}
	}
	if state.Age >= adminLeaseTTL {
		return false, fmt.Sprintf("%s 已废弃（持有进程不存在且超过 %s 续租窗口）",
			state.Path, adminLeaseTTL)
	}
	return true, fmt.Sprintf("%s 仍在续租窗口内（最近一次续租 %s 前，窗口 %s）",
		state.Path, state.Age.Round(time.Second), adminLeaseTTL)
}
