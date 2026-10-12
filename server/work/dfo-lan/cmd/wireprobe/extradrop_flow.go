package main

// 活动 1093「额外掉落奖励」的帧构造与常量。
//
// ## 帧（长度来自 IDA，见 internal/game/protocol/extradrop115.go 的头注）
//
//	NOTI2880 ADDITIONAL_DROP_REWARD_EVENT_INFO      8 B（两个 u32）
//	NOTI2881 ADDITIONAL_DROP_REWARD_EVENT_GET_REWARD 9 B（u64 + u8）
//
// ## 为什么两个 u32 要能被环境变量覆盖
//
// 语义**未证**（只知道长度与落点 `obj+0x58`）。本仓对未证字段的既定做法是**做成启动期开关**，
// 让业主在副本里 A/B 看面板怎么解读，而不是在代码里赌一种解释：
//
//	DFO_EXTRADROP_INFO="第一个u32,第二个u32"     # 十进制；缺省 = "0,目录里的 [max amount]"
//
// 覆盖值**不写进 profile**（`internal/launcher/profile.go` 的 environment 是严格白名单）。
import (
	"dfolan/internal/extradrop"
	"dfolan/internal/game/protocol"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	// extraDropInfoNotiID 是 NOTI2880。
	extraDropInfoNotiID uint16 = 2880
	// extraDropGetRewardNotiID 是 NOTI2881。
	extraDropGetRewardNotiID uint16 = 2881
)

// extraDropInfoPayload 编 NOTI2880 的载荷。
//
// 缺省 = `(0, 目录的 [max amount])` —— 一个"还没开始刷"的中性态；语义定了以后改这里。
func extraDropInfoPayload(c *extradrop.Catalog) []byte {
	first, second := uint32(0), uint32(0)
	if c != nil {
		second = c.MaxAmount
	}
	if raw := strings.TrimSpace(os.Getenv("DFO_EXTRADROP_INFO")); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) == 2 {
			if v, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32); err == nil {
				first = uint32(v)
			}
			if v, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32); err == nil {
				second = uint32(v)
			}
		}
	}
	return protocol.ExtraDropEventInfo115(first, second)
}

// extraDropFixedPerRun 读 `DFO_EXTRADROP_FIXED` —— 活动 1093「每局固定掉落能量」的开关。
//
// 这是**玩法口径开关**，不是用来 A/B 的诊断探针 —— 所以它**保留**、
// 不需要像 `DFO_ORACLE_*` 那类一次性探针那样在收尾时清掉。
//
//	未设     ⇒ 10（业主 2026-10-13 定的缺省）
//	off / 0  ⇒ 0  ⇒ 关掉固定模式，回到官方稀有度表（Rare1/Unique2/…）
//	N (>0)   ⇒ 每局固定 N 个
//
// 解析不了的值**不静默**：记一条 warning 后按缺省走 ——
// 参数写错时宁可表现成「缺省生效」，也不要变成「整段悄悄不发」。
//
// 与 `DFO_EXTRADROP_INFO` 同类，**不写进 profile**（`internal/launcher/profile.go` 的
// environment 是严格白名单，未知键会让服务端起不来）。
func extraDropFixedPerRun() uint32 {
	const def = 10
	raw := strings.TrimSpace(os.Getenv("DFO_EXTRADROP_FIXED"))
	if raw == "" {
		return def
	}
	switch strings.ToLower(raw) {
	case "off", "false", "no", "0":
		return 0
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		log.Printf("warning: DFO_EXTRADROP_FIXED=%q 既不是 off 也不是正整数，按缺省 %d 处理", raw, def)
		return def
	}
	return uint32(n)
}
