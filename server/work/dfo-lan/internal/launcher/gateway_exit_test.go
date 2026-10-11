package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayExitSummaryUsesRecordedCause(t *testing.T) {
	cases := []struct{ name, log, want string }{
		{"low", "MEMORY_GUARD stopped scope=gateway reason=memory_pressure trigger=persistent_low_memory private_mib=1234 system_available_mib=1500\nMEMORY_GUARD ended scope=gateway peak_private_mib=1300\n", "连续 30 秒低于 2 GiB（系统可用 1500 MiB，服务端私有提交 1234 MiB）"},
		{"critical", "MEMORY_GUARD stopped scope=gateway reason=memory_pressure trigger=persistent_critical_memory private_mib=1200 system_available_mib=400\n", "连续 2 秒低于 512 MiB"},
		{"commit", "MEMORY_GUARD stopped scope=gateway reason=memory_pressure trigger=process_commit_limit private_mib=3900 system_available_mib=9000\n", "私有提交内存接近保护上限"},
		{"measurement", "MEMORY_GUARD stopped scope=gateway reason=measurement_failed error=denied\n", "这不等于内存不足"},
		{"only-warning", "MEMORY_GUARD warning scope=gateway reason=system_memory_pressure\n", "未确认触发内存保护"},
		{"other-child", "MEMORY_GUARD stopped scope=probe-client reason=memory_pressure trigger=persistent_low_memory\n", "未确认触发内存保护"},
		{"crash", "fatal error: unrelated failure\n", "未确认触发内存保护"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "gateway.err")
			if err := os.WriteFile(path, []byte(c.log), 0600); err != nil {
				t.Fatal(err)
			}
			got := gatewayExitSummary(path, 1)
			if !strings.Contains(got, c.want) || !strings.Contains(got, "服务端退出码 1") {
				t.Fatalf("summary=%s; want %s", got, c.want)
			}
		})
	}
}

func TestGatewayExitSummaryBoundsTailAndHandlesMissingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.err")
	if got := gatewayExitSummary(path, 7); !strings.Contains(got, "未能读取退出原因") {
		t.Fatal(got)
	}
	oldStop := "MEMORY_GUARD stopped scope=gateway reason=memory_pressure trigger=persistent_low_memory\n"
	if err := os.WriteFile(path, []byte(oldStop+strings.Repeat("ordinary output\n", 6000)), 0600); err != nil {
		t.Fatal(err)
	}
	if got := gatewayExitSummary(path, 2); !strings.Contains(got, "未确认触发内存保护") {
		t.Fatal(got)
	}
}
