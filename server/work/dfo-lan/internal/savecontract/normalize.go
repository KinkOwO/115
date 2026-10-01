package savecontract

import (
	"os"
	"strings"
)

// 2026-10-01（next146）：**身份归一化** —— 把任何「来源身份」字符串折叠成契约身份。
//
// ## 背景
//
// 排查发现 `configs/*.json` 里有 **92 处**把 `source` / `source_sha256` /
// `source_checksum` 硬编码成 `7ef2db59…`（一个 2026-09-12 的**内层 PVF 哈希**）。
// 这些值在直读模式下**必然失配**：
//
//   - 内层归档每次重新解包都换哈希（`7ef2db59…` → `b2b503b5…` → `be95d64e…`）
//   - 于是「目录自校」（`a.Snapshot().Checksum != index.Source.Checksum`）全线失败
//
// 上一轮的应急做法是**逐个人工清空**（把值改成 `""` 表示「自动派生」），
// 但 92 处里只清了少数几处 ⇒ 留下大量「已配了但永远不会匹配」的僵尸门禁 ——
// 这正是业主观察到的「很多修复失效」。
//
// ## 本函数的作用
//
// 与其再手工清 92 处（且下次换客户端又要清），不如**在读取时归一化**：
//
//	Normalize(v) == "" 表示「该配置没有有效身份，按自动派生处理」
//
// 判据是**形状**而不是**字面值** —— 任何 64 位 hex 都可能是某个内层哈希，
// 而配置里**不该**出现内层哈希（那是构建产物，不是配置身份）。
// 因此：**凡是 64 位 hex 的 source 值，一律视为「历史内层哈希残留」并归一为 ""。**
//
// ## 为什么不会误伤
//
// 需要保留的 source 值都不是 64 位 hex：
//
//   - `vault.generated.json` 的 `source_sha256: fda6c33f…` —— 是 64 位 hex ⚠️
//   - `fatigue-probe.json` 的 `source: 'Explicit local development policy…'` —— 自然语言
//   - `apocalypse.generated.json` 的 `source: 'contents/2026/…'` —— 路径
//   - `refine.json` 的 `source: 'manual:dfoneople-refine-page …'` —— 手工标注
//
// `vault.generated.json` 是**唯一**例外：它的 sha256 由**服务端生成器**产出
// （`vault.generated.json` 由服务端脚本写、随服务端版本走），**不随客户端 PVF 变**，
// 所以它是合法的「服务端定义身份」，必须保留。
//
// ⇒ 归一化**只作用于「由 PVF 导入派生」的 source 字段**，由调用方决定是否调用；
// `vault` 路径不调用本函数。这样既清掉了僵尸门禁，又不碰 vault 的合法身份。

// Normalize 把配置里携带的「来源身份」折叠成契约侧语义。
//
// 返回空串 = 「该配置未声明有效来源身份，请按自动派生处理」。
// 返回非空 = 明确保留的外部身份（目前只有自然语言/路径类标注）。
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	// 64 位 hex = 内层 PVF 哈希残留（构建产物，不是配置身份）⇒ 丢弃。
	if IsHex64(v) {
		return ""
	}
	return v
}

// IsHex64 判断是否为 64 位十六进制串（= SHA256 的形状）。
func IsHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// EnvContractOverride 允许运维用环境变量**临时**把契约身份钉到某个特定值。
//
// 用途：跨服务端版本导入存档时，如果两边契约世代不同但用户明确知道可以兼容，
// 可以显式指定，而不是去改代码。**默认不设置**，走 `Identity()`。
//
// ⚠️ 设成内层哈希是**错误用法** —— 那等于把身份又绑回构建产物。
// 这里只做格式校验，不做语义劝阻（运维自负其责），但文档必须写明。
func EnvContractOverride() string {
	v := strings.TrimSpace(os.Getenv("DFO_SAVE_CONTRACT"))
	if IsHex64(v) {
		return v
	}
	return ""
}
