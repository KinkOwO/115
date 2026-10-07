package launcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 停止链的租约回收：判据是「记录的 pid 明确不存在」。
// 关键安全性质：**活的持有者绝不能被删**（那是 GM 正在写存档），
// 「问不出来」（权限不足）也必须保守不动，否则就是两个写者同时进。
func TestClearStaleAdminLeaseOnlyWhenHolderIsGone(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "dfolan.sqlite3")
	lease := db + ".admin-guard"
	cfg := StorageConfig{Driver: "sqlite", SQLitePath: db}

	original := processStatus
	t.Cleanup(func() { processStatus = original })

	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(lease, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// 1. 记录里的进程不存在 ⇒ 删除（这就是业主实机撞到的 13248 那一格）。
	write("999999")
	processStatus = func(int) PidStatus { return PidGone }
	cleared, note, err := ClearStaleAdminLease(cfg, nil)
	if err != nil {
		t.Fatalf("ClearStaleAdminLease: %v", err)
	}
	if !cleared {
		t.Fatalf("a lease whose holder is gone was not cleared (note %q)", note)
	}
	if _, err := os.Stat(lease); !os.IsNotExist(err) {
		t.Fatal("the lease file survived a stale clear")
	}

	// 2. 持有者还活着 ⇒ 必须保留（可能正是 GM 在写）。
	write("1234")
	processStatus = func(int) PidStatus { return PidAlive }
	cleared, note, err = ClearStaleAdminLease(cfg, nil)
	if err != nil {
		t.Fatalf("ClearStaleAdminLease: %v", err)
	}
	if cleared {
		t.Fatal("a lease held by a live process was cleared")
	}
	if !strings.Contains(note, "still running") {
		t.Fatalf("note = %q, want it to name the live holder", note)
	}
	if _, err := os.Stat(lease); err != nil {
		t.Fatal("the live lease file was removed")
	}

	// 3. 问不出来（例如 Windows 上的 Access denied）⇒ 也必须保留。
	processStatus = func(int) PidStatus { return PidUnknown }
	cleared, _, err = ClearStaleAdminLease(cfg, nil)
	if err != nil {
		t.Fatalf("ClearStaleAdminLease: %v", err)
	}
	if cleared {
		t.Fatal("an unverifiable lease was cleared; that is how two writers get in")
	}

	// 4. 读不出 pid（空文件 / 内容异常）⇒ 保留并说明，交给 TTL 或 clear-guard。
	write("")
	processStatus = func(int) PidStatus { return PidGone }
	cleared, note, err = ClearStaleAdminLease(cfg, nil)
	if err != nil {
		t.Fatalf("ClearStaleAdminLease: %v", err)
	}
	if cleared {
		t.Fatal("a lease without a recorded pid was cleared")
	}
	if !strings.Contains(note, "no holder pid") {
		t.Fatalf("note = %q, want it to explain the missing pid", note)
	}

	// 5. 没有租约文件 ⇒ 什么都不做，也不报错。
	if err := os.Remove(lease); err != nil {
		t.Fatal(err)
	}
	cleared, note, err = ClearStaleAdminLease(cfg, nil)
	if err != nil || cleared || note != "" {
		t.Fatalf("missing lease: cleared=%v note=%q err=%v, want a silent no-op", cleared, note, err)
	}

	// 6. 档里没写 sqlite_path ⇒ 没有租约文件。PostgreSQL 支持已移除（2026-10-05，
	//    见根 AGENTS.md §0.6），所以这里不再有"非 SQLite 档"这一说，只剩"没配库路径"。
	if _, applicable := AdminLeasePath(StorageConfig{Driver: "sqlite"}); applicable {
		t.Fatal("a profile without sqlite_path reported an admin lease")
	}
	if cleared, note, err := ClearStaleAdminLease(StorageConfig{Driver: "sqlite"}, nil); cleared || note != "" || err != nil {
		t.Fatalf("profile without sqlite_path: cleared=%v note=%q err=%v, want no-op", cleared, note, err)
	}
}

// realProcessStatus 必须能分辨「不存在」与「存在但问不了」——停止链的安全性建立在
// 这个分别上，所以直接对本机真实进程表断言。
func TestRealProcessStatusOnThisMachine(t *testing.T) {
	if got := realProcessStatus(os.Getpid()); got != PidAlive {
		t.Fatalf("realProcessStatus(self) = %v, want PidAlive", got)
	}
	if runtime.GOOS != "windows" {
		t.Skip("the missing-pid branch is asserted on Windows; POSIX ESRCH is covered by the same code path")
	}
	// 一个不可能存在的 pid：Windows 上 OpenProcess 报 ERROR_INVALID_PARAMETER ⇒ PidGone。
	if got := realProcessStatus(999999); got != PidGone {
		t.Fatalf("realProcessStatus(999999) = %v, want PidGone", got)
	}
}

// 停止计划里必须有这一步，而且排在强杀之后（先杀进程，再判断租约值不值得清）。
func TestStopPlanPlansTheLeaseClearLast(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "dfolan.sqlite3")
	cfg := StorageConfig{Driver: "sqlite", SQLitePath: db}

	plan, err := StopPlan(cfg)
	if err != nil {
		t.Fatalf("StopPlan: %v", err)
	}
	for _, action := range plan {
		if action.Kind == "lease-clear" {
			t.Fatal("a plan without a lease file already wants to clear one")
		}
	}

	if err := os.WriteFile(db+".admin-guard", []byte("4242"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err = StopPlan(cfg)
	if err != nil {
		t.Fatalf("StopPlan: %v", err)
	}
	if len(plan) == 0 || plan[len(plan)-1].Kind != "lease-clear" {
		t.Fatalf("plan = %+v, want lease-clear as the last action", plan)
	}
	if plan[len(plan)-1].Path != db+".admin-guard" {
		t.Fatalf("lease-clear carries %q, want the lease path", plan[len(plan)-1].Path)
	}
	if plan[0].Kind != "kill" {
		t.Fatalf("plan starts with %q, want the kills first", plan[0].Kind)
	}
}

// 真跑一次 Stop 的租约那一步（只清租约，不碰任何进程）：dry-run 只报告、实跑才删。
func TestStopDryRunReportsTheLeaseWithoutClearingIt(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "dfolan.sqlite3")
	lease := db + ".admin-guard"
	if err := os.WriteFile(lease, []byte("999999"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := StorageConfig{Driver: "sqlite", SQLitePath: db}

	original := processStatus
	processStatus = func(int) PidStatus { return PidGone }
	t.Cleanup(func() { processStatus = original })

	var logged []string
	if _, err := Stop(context.Background(), cfg, true, func(format string, args ...any) {
		logged = append(logged, format)
	}); err != nil {
		t.Fatalf("dry-run Stop: %v", err)
	}
	if _, err := os.Stat(lease); err != nil {
		t.Fatal("a dry run removed the lease file")
	}
	found := false
	for _, line := range logged {
		if strings.Contains(line, "would clear") {
			found = true
		}
	}
	if !found {
		t.Fatalf("dry run did not report the pending lease clear: %v", logged)
	}
}

func TestInspectAdminLeaseReportsAgeAndHolder(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "dfolan.sqlite3")
	lease := db + ".admin-guard"
	if err := os.WriteFile(lease, []byte(" 4321 \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * time.Second)
	if err := os.Chtimes(lease, old, old); err != nil {
		t.Fatal(err)
	}
	state, present, err := InspectAdminLease(StorageConfig{Driver: "sqlite", SQLitePath: db})
	if err != nil || !present {
		t.Fatalf("InspectAdminLease: present=%v err=%v", present, err)
	}
	if !state.HasHolder || state.Pid != 4321 {
		t.Fatalf("state = %+v, want pid 4321 parsed from the padded content", state)
	}
	if state.Age < 80*time.Second {
		t.Fatalf("age = %s, want the file's real age", state.Age)
	}
}
