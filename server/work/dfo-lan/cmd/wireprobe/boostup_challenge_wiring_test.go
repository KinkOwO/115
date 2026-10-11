package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

// 665「直升毕业后的挑战」面板**只由服务端推的 NOTI2722 渲染**（客户端全程不发 CMD681，
// 见 boostup_challenge.go 的注释）。所以「进城调用对账 + 把 body 填进 entryPayloads」
// 这一小段就是接线本身 —— 光有 boostChallengeEntrySync 这个函数不算接线成功。
//
// 实机 2026-10-11 会话 `..._20261011_172251_037171_next37`：角色存档里
// `challenge.enrolled=true`、NOTI2639 状态字节 = 2（已毕业）、NOTI108 表里 665 行也在，
// 但 events.jsonl 全程 **零条 boost_challenge_*** ⇒ 毕业角色进城看不到 665 面板。
// 根因就是 next164 落进 client_entry.go 的那段调用点在后续合并里丢了，
// 而 boostup_challenge_entry_test.go 直接调函数、测的是行为不是接线，所以一直是绿的。
func TestBoostChallengeEntryRestoreIsWired(t *testing.T) {
	call := regexp.MustCompile(`\.boostChallengeEntrySync\(`)
	planAssign := regexp.MustCompile(`^[\w.]+\.BoostChallenge\s*=`)

	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	wired := false
	for _, entry := range files {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var calls, assigns bool
		for _, line := range bytes.Split(src, []byte("\n")) {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("//")) {
				continue
			}
			calls = calls || call.Match(trimmed)
			assigns = assigns || planAssign.Match(trimmed)
		}
		if calls && assigns {
			wired = true
			break
		}
	}
	if !wired {
		t.Error("没有任何生产源码在进城路径上调用 boostChallengeEntrySync 并填写 plan.BoostChallenge：" +
			"665 面板不会再出现（2722 只在 entryPayloads.BoostChallenge 非空时下发）")
	}

	// 帧本身还得留在进城序列里，并且排在 124 之后（面板对象在那之前不存在，早到会被丢掉）。
	src, err := os.ReadFile("entry_flow.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte(`outboundPacket{"boost_challenge_progress_restored", 0, 2722, p.BoostChallenge}`)) {
		t.Error(`entry_flow.go 不再下发 2722（boost_challenge_progress_restored / p.BoostChallenge）`)
	}
}
