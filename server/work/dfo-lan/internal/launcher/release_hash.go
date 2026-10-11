package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// releaseHashWarnings is informational. A release manifest never authorizes
// execution or changes a selected path, and never gates startup on a fingerprint.
func releaseHashWarnings(root, client string) []string {
	data, err := os.ReadFile(filepath.Join(root, "release-manifest.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("提示：发行清单无法读取，继续启动：%v", err)}
	}
	var manifest struct {
		Files []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return []string{"提示：发行清单无法解析，继续启动"}
	}
	var warnings []string
	for _, row := range manifest.Files {
		if row.Path == "" || strings.ContainsAny(row.Path, "\\:") || strings.HasPrefix(row.Path, "/") {
			continue
		}
		valid := true
		for _, part := range strings.Split(row.Path, "/") {
			if part == ".." || part == "." || part == "" {
				valid = false
			}
		}
		if !valid {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(row.Path))
		if strings.HasPrefix(row.Path, "client/") {
			path = filepath.Join(client, filepath.FromSlash(strings.TrimPrefix(row.Path, "client/")))
		}
		actual, err := fileSHA256(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("提示：组件哈希无法读取，继续启动：%s：%v", row.Path, err))
			continue
		}
		if !strings.EqualFold(actual, row.SHA256) {
			warnings = append(warnings, fmt.Sprintf("提示：哈希值不一致，继续启动：%s；记录=%s；实际=%s", row.Path, row.SHA256, actual))
		}
	}
	return warnings
}

func startupSourceHashWarnings(report LaunchReport) []string {
	if report.ProfileEnv["DFO_PVF_CATALOGS"] == "" {
		return nil
	}
	expected := report.ProfileEnv["DFO_PVF_SHA256"]
	var policy struct {
		SourceChecksum string `json:"source_checksum"`
	}
	data, err := os.ReadFile(report.ProfileEnv["DFO_PVF_CHARACTER_POLICY"])
	if err == nil {
		_ = json.Unmarshal(data, &policy)
	}
	if expected == "" && policy.SourceChecksum == "" {
		return nil
	}
	actual, err := fileSHA256(report.InnerPVF.Path)
	if err != nil {
		return nil
	}
	var warnings []string
	for _, item := range []struct{ name, hash string }{{"服务端 PVF 配置", expected}, {"角色 PVF 配置", policy.SourceChecksum}} {
		if item.hash != "" && !strings.EqualFold(actual, item.hash) {
			warnings = append(warnings, fmt.Sprintf("提示：%s哈希值不一致，继续启动；记录=%s；实际=%s", item.name, item.hash, actual))
		}
	}
	return warnings
}
