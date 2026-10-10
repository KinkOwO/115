package launcher

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// gatewayExitSummary only diagnoses a gateway that exited before normal cleanup.
// Read a bounded tail; routine samples and unrelated stderr are not copied into
// the launcher UI, and missing evidence must not be reported as memory pressure.
func gatewayExitSummary(logPath string, exitCode int) string {
	summary := fmt.Sprintf("本地服务端在客户端结束前已退出（服务端退出码 %d）。", exitCode)
	file, err := os.Open(logPath)
	if err != nil {
		return summary + "未能读取退出原因，请保留本次运行目录。"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return summary + "未能读取退出原因，请保留本次运行目录。"
	}
	const tailLimit = 64 << 10
	offset := info.Size() - tailLimit
	if offset < 0 {
		offset = 0
	}
	body, err := io.ReadAll(io.NewSectionReader(file, offset, tailLimit))
	if err != nil {
		return summary + "未能读取退出原因，请保留本次运行目录。"
	}
	lines := strings.Split(string(body), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "MEMORY_GUARD stopped scope=gateway ") {
			continue
		}
		fields := map[string]string{}
		for _, field := range strings.Fields(line) {
			key, value, found := strings.Cut(field, "=")
			if found {
				fields[key] = value
			}
		}
		if fields["reason"] == "measurement_failed" {
			return summary + "内存监测读取失败，保护程序停止了服务端；这不等于内存不足。"
		}
		if fields["reason"] != "memory_pressure" {
			continue
		}
		var reason string
		switch fields["trigger"] {
		case "persistent_low_memory":
			reason = "系统可用物理内存连续 30 秒低于 2 GiB"
		case "persistent_critical_memory":
			reason = "系统可用物理内存连续 2 秒低于 512 MiB"
		case "process_commit_limit":
			reason = "服务端私有提交内存接近保护上限"
		default:
			continue
		}
		return summary + "内存保护已触发：" + reason + fmt.Sprintf("（系统可用 %s MiB，服务端私有提交 %s MiB）。", fields["system_available_mib"], fields["private_mib"])
	}
	return summary + "现有记录未确认触发内存保护，请保留 gateway.err 和 client.log 继续排查。"
}
