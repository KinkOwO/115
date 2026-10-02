package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CMD1722（装备继承）曾经「服务层 + 流程层都在、就是没人调」：
// 客户端发 1722 之后服务端既不处理也不回包，实机表现就是「按下继承毫无效果」。
// 这条用例把分派钉住 —— 光有 inherit() 这个函数不算接线成功。
func TestInheritDispatchIsWired(t *testing.T) {
	for _, verified := range []bool{false, true} {
		client, conn, events := newDispatchTestClient()
		result := client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 1722}, verified: verified})
		require.Equal(t, dispatchHandled, result)
		kind := "inherit_rejected"
		if verified {
			kind = "inherit_refused"
		}
		require.Equal(t, kind, (*events)[len(*events)-1]["kind"])
		assert.Zero(t, conn.Len(), "继承拒绝没有合法的 1722 回包通道")
	}
}

// 1722 必须同时进两道闸门，缺任何一道处理器都跑不到（CMD205 踩过同一个坑）：
//
//	dungeonRequest      —— 分派白名单
//	observedGameRequest —— 每次请求都解密校验，不受 BodySampleLimit(8) 采样上限限制
//
// 少了后者，玩家点满 8 次之后服务端就不再解密这条命令，请求以「明文为空」失败，
// 表现是「前几次能继承、之后毫无反应」。
func TestInheritIsRegisteredInBothRequestGates(t *testing.T) {
	src, err := os.ReadFile("request_scope.go")
	if err != nil {
		t.Fatal(err)
	}
	if !dungeonRequest(1722) {
		t.Error("1722 不在 dungeonRequest 白名单里：分派层根本不会进去")
	}
	if !observedGameRequest(1722) {
		t.Error("1722 不在 observedGameRequest 里：用满 8 次采样上限后不再解密，继承会「突然失灵」")
	}
	if !bytes.Contains(src, []byte("1722")) {
		t.Error("request_scope.go 里找不到 1722 的登记注释")
	}
}

// ★★ 制度性防线：任何地方都不许出现 `sendPayload(1, 1722, …)`。
//
// 根因：客户端的 opcode 表是两套独立命名空间，kind=0 → NOTI 表（qword_14E683700）、
// kind=1 → CMD 表（qword_14E6836F8）。CMD 表那一侧 1722 是**客户端自己发出去的命令**，
// 没有接收 handler —— 服务端回 kind=1 + 1722 会被当成「自己发的继承命令」解析、格式不符。
//
// 这条用例同时守住「成功路径」和「拒绝路径」——无论成功还是拒绝都绝不能走 kind=1。
// kind=0 那一侧的禁令由 TestInheritNeverSendsAnyOutbound1722 单独把守（NOTI 1722
// 是小游戏道具计数通知，与继承结果无关）。
func TestInheritNeverRepliesWithKindOne(t *testing.T) {
	// 容忍空格差异：`sendPayload(1, 1722` / `sendPayload(1,1722`。
	pattern := regexp.MustCompile(`sendPayload\(\s*1\s*,\s*1722\b`)
	for _, file := range []string{"client_dispatch_inventory.go", "inherit_flow.go", "inherit.go"} {
		src, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(src, []byte("\n")) {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("//")) {
				continue
			}
			if pattern.Match(trimmed) {
				t.Errorf("%s 里出现了 kind=1 的 1722 回包：%s —— 1722 在 CMD 表侧没有接收 handler", file, trimmed)
			}
		}
	}
}

// 成功路径必须发 kind=0 的 NOTI14 行刷新（基础件 + 材料件各一行），
// 且**不能**退回 id13 全量重建：继承窗口确认后仍持有两件装备的对象引用，
// 整包重建会打断这些引用（强化 / 增幅路径早有同一条结论）。
func TestInheritRefreshesViaIncrementalSlots(t *testing.T) {
	src, err := os.ReadFile("inherit_flow.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte(`outboundPacket{"inherit_slots_updated", 0, 14`)) {
		t.Error("继承成功没有发 kind=0 / id14 的槽位行刷新：客户端窗口里的数字不会变")
	}
	if bytes.Contains(src, []byte("{0, 13")) || bytes.Contains(src, []byte("InventoryRestore")) {
		t.Error("继承成功走了 id13 全量重建：会打断继承窗口持有的装备对象引用")
	}
}

// ★★ 制度性防线：CMD 1722 **双向都没有继承结果的接收通道**，任何 1722 出站包都禁止。
//
//   - kind=1（CMD 表）：那一侧 1722 是客户端自己发出去的命令、没有接收 handler，
//     回了会被当成「自己发的继承命令」解析、格式不符。
//   - kind=0（NOTI 表）：那一侧 1722 的 handler 是**小游戏道具使用计数通知**
//     （sub_143348A90），与继承结果无关。
//
// ⇒ 成功 / 失败 / 拒绝都只落库 + 发 id14 行刷新，**不回任何 1722 包**。
func TestInheritNeverSendsAnyOutbound1722(t *testing.T) {
	patterns := []*regexp.Regexp{
		// 容忍空格差异：`sendPayload(1, 1722` / `sendPayload( 0 , 1722` 等。
		regexp.MustCompile(`sendPayload\(\s*[01]\s*,\s*1722\b`),
		regexp.MustCompile(`outboundPacket\{[^}]*1722`),
		regexp.MustCompile(`protocol\.InheritResult\(`),
	}
	for _, file := range []string{"client_dispatch_inventory.go", "inherit_flow.go", "inherit.go"} {
		src, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(src, []byte("\n")) {
			trimmed := bytes.TrimSpace(line)
			if bytes.HasPrefix(trimmed, []byte("//")) {
				continue
			}
			for _, pattern := range patterns {
				if pattern.Match(trimmed) {
					t.Errorf("%s 里出现了 1722 出站包：%s —— CMD 1722 双向都没有接收通道", file, trimmed)
				}
			}
		}
	}
}
