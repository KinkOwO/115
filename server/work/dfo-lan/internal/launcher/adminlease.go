package launcher

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SQLite 管理租约（GM 互斥）在停止链上的收尾。
//
// ## 为什么需要
//
// SQLite 没有 advisory lock，GM 互斥改用数据库旁的租约文件 `<db>.admin-guard`：文件在就是锁在，
// 持有者每 10 秒刷新一次 mtime，超过 60 秒没人刷新才算废弃（见 internal/database 的
// AdminLeaseTTL / AdminLeaseRefresh 与 docs/sqlite-operations.md §4）。
//
// 于是「停止环境 → 立刻再启动」会撞上一次：停止脚本用 taskkill 强杀服务端，进程没机会
// 自己删租约，而 TTL 还有几十秒，新的服务端一律被拒——
//
//	已有 GM 写入正在进行（SQLite 管理租约文件 …dfolan.sqlite3.admin-guard 由进程 13248 持有，
//	最近一次续租 24s 前）；若确认没有 GM 在写，删除该文件后重试      （2026-10-05 实机）
//
// ## 判据与边界
//
// 停止链**只在确认记录里的持有进程已经不存在**时才删除租约；这一步是安全的，因为：
//   - 记录里的 pid 是我们自己写进去的（`TrySharedAdminGuard` 创建即写 pid）；
//   - 判「不存在」用的是平台 API 的明确错误，**不是**「查不到就算死」：Windows 上
//     OpenProcess 对不存在的 pid 报 ERROR_INVALID_PARAMETER（实测），而受保护进程报
//     Access denied —— 后者归为「不知道」，一律不动文件。可能出现的误判只有「活的被当成
//     不知道」（保守方向：多等 60 秒），不会出现「活的被当成死」。
//   - 租约里读不出 pid（空文件/内容异常）同样归为「不知道」，不删；那种情况等 TTL，
//     或由业主显式跑 `scripts\storage-route.cmd clear-guard`。
//
// 这与守卫自身「不自动回收」的决定并不冲突：守卫面对的是并发索取锁的人，它必须假设最坏；
// 停止链面对的是业主明确要求的「停掉这套环境」，而且刚把服务端进程全部强杀过。

// PidStatus 是启动器对「记录里的持有进程」能诚实给出的三种结论。
type PidStatus int

const (
	// PidAlive：进程存在（可能正是那个持锁的 GM，绝不能动它的租约）。
	PidAlive PidStatus = iota
	// PidGone：平台明确回答「这个 pid 不存在」。
	PidGone
	// PidUnknown：问不出来（权限不足、平台不支持、pid 读不出来）——保守处理，不动文件。
	PidUnknown
)

// processStatus 可注入，测试不依赖本机进程表。
var processStatus = realProcessStatus

// AdminLeasePath 返回该存储档的 SQLite 管理租约路径；非 SQLite 档返回 ok=false
// （PostgreSQL 用 advisory lock，连接断开即释放，没有文件要清）。
func AdminLeasePath(cfg StorageConfig) (string, bool) {
	if cfg.DriverName() != "sqlite" {
		return "", false
	}
	path := strings.TrimSpace(cfg.SQLitePath)
	if path == "" {
		return "", false
	}
	return path + ".admin-guard", true
}

// AdminLeaseState 是租约文件的现状。HasHolder=false 表示文件在、但读不出 pid。
type AdminLeaseState struct {
	Path      string
	Pid       int
	HasHolder bool
	Age       time.Duration
}

// InspectAdminLease 读取租约现状。第二个返回值 false 表示当前没有租约文件（无需处理）。
func InspectAdminLease(cfg StorageConfig) (AdminLeaseState, bool, error) {
	state := AdminLeaseState{}
	path, applicable := AdminLeasePath(cfg)
	if !applicable {
		return state, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, false, nil
		}
		return state, false, err
	}
	state.Path = path
	state.Age = time.Since(info.ModTime())
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AdminLeaseState{}, false, nil
		}
		return state, true, err
	}
	if pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil && pid > 0 {
		state.Pid = pid
		state.HasHolder = true
	}
	return state, true, nil
}

// ClearStaleAdminLease 清掉「持有者已不存在」的 SQLite 管理租约。
//
// 返回 cleared 表示这次是否真的删掉了文件；note 是一句可打印的结论（也可能是空串）。
// 只有两种情形会删：记录里的 pid 存在且平台明确回答「不存在」。其余一律保留并说明原因。
func ClearStaleAdminLease(cfg StorageConfig, logf func(string, ...any)) (bool, string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	state, present, err := InspectAdminLease(cfg)
	if err != nil {
		return false, "", err
	}
	if !present {
		return false, "", nil
	}
	if !state.HasHolder {
		note := fmt.Sprintf("kept %s: no holder pid recorded (age %s) - retry after the %s TTL or run the clear-guard entry",
			state.Path, state.Age.Round(time.Second), "60s")
		logf("%s", note)
		return false, note, nil
	}
	switch processStatus(state.Pid) {
	case PidGone:
		if err := os.Remove(state.Path); err != nil && !os.IsNotExist(err) {
			return false, "", fmt.Errorf("clear stale admin lease %s: %w", state.Path, err)
		}
		note := fmt.Sprintf("cleared stale SQLite admin lease (holder pid %d is gone): %s", state.Pid, state.Path)
		logf("%s", note)
		return true, note, nil
	case PidAlive:
		note := fmt.Sprintf("kept %s: holder pid %d is still running (a GM may be writing)", state.Path, state.Pid)
		logf("%s", note)
		return false, note, nil
	default:
		note := fmt.Sprintf("kept %s: cannot tell whether holder pid %d is alive", state.Path, state.Pid)
		logf("%s", note)
		return false, note, nil
	}
}
