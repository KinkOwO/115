package main

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"dfolan/internal/modpolicy"
)

// TestLogModPolicyPrintsDropRules 钉住启动日志的**掉落规则行**：
// 业主口径是"规则没生效必须永远看得出来"，所以这一行不能只在有 mod 时出现，
// 也不能只打一半 —— 关闭态要写"关闭"，开启态要把倍率与来源都写出来。
func TestLogModPolicyPrintsDropRules(t *testing.T) {
	defer modpolicy.Reset()
	modpolicy.Reset()

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)

	logModPolicy()
	out := buf.String()
	for _, want := range []string{"odyssey mode rules:", "drop rate rules:", "关闭"} {
		if !strings.Contains(out, want) {
			t.Fatalf("启动日志缺少 %q：\n%s", want, out)
		}
	}

	buf.Reset()
	modpolicy.ConfigureDrops(modpolicy.DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "odyssey.hardcore"})
	logModPolicy()
	out = buf.String()
	for _, want := range []string{"drop rate rules:", "×5.00", "世界掉落", "小怪专属物品池", "odyssey.hardcore"} {
		if !strings.Contains(out, want) {
			t.Fatalf("启动日志缺少 %q：\n%s", want, out)
		}
	}
}
