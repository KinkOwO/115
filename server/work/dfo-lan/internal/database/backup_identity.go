package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExistingSQLitePath resolves an already selected file without opening or
// creating a database. An expected path must identify that same file, including
// through directory junctions or hard links; equal contents are not identity.
func ExistingSQLitePath(actual, expected string) (string, error) {
	if strings.TrimSpace(actual) == "" {
		return "", fmt.Errorf("未指定现有SQLite文件")
	}
	abs, err := filepath.Abs(actual)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("找不到现有SQLite文件 %s；不会创建空库", abs)
	}
	if strings.TrimSpace(expected) != "" {
		want, err := filepath.Abs(expected)
		if err != nil {
			return "", err
		}
		other, err := os.Stat(want)
		if err != nil || !other.Mode().IsRegular() || !os.SameFile(info, other) {
			return "", fmt.Errorf("数据库身份不符：活动配置 %s，预期 %s", abs, want)
		}
	}
	return filepath.Clean(abs), nil
}

// VerifyBackupDatabase binds a restore to the selected database. Old manifests
// remain usable only when their original SQLite label supplies the same identity.
func VerifyBackupDatabase(entry BackupEntry, actual string) error {
	origin := entry.Database
	if origin == "" && strings.HasPrefix(entry.Source, "SQLite（") && strings.HasSuffix(entry.Source, "）") {
		origin = strings.TrimSuffix(strings.TrimPrefix(entry.Source, "SQLite（"), "）")
	}
	if origin == "" {
		return fmt.Errorf("备份 %s 缺少数据库来源，无法确定属于当前存档", entry.Name)
	}
	if _, err := ExistingSQLitePath(actual, origin); err != nil {
		return fmt.Errorf("备份 %s 不属于当前数据库：%w", entry.Name, err)
	}
	return nil
}
