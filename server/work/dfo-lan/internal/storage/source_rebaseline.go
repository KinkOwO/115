package storage

import (
	"context"
	"fmt"

	"dfolan/internal/savecontract"
)

// 2026-10-01（next146）：PVF 直读模式的**存档身份归一**（存档兼容，最高优先级）。
//
// ## 问题
//
// 直读模式下，服务端曾把**内层归档** `Script.inner.pvf` 的 SHA256 当作「配置来源身份」
// 写进存档：`characters.config_version`、`character_world.config_version`、
// `character_quests.config_version`、`character_events.config_version`、
// `character_map_clears.source_version`、`character_quest_rewards.source_version`。
// 运行时拿 `role.ConfigVersion` 与当次目录的 `Catalog.Source.Checksum` 逐一比对，
// 不等就**硬拒**（`quest %d requires source migration` / `character event source mismatch` …）。
//
// 内层归档是**本地构建产物**：换一次客户端、甚至同一份客户端三件套重新解包一次，
// 哈希就会变（实测 `7ef2db59…` → `b2b503b5…` → `be95d64e…`）。⇒ 每变一次，
// **全体存档**都会因身份不符而进不去角色（2026-10-01 实机：12 角色全部被拒）。
//
// ## 处置
//
// 存档身份现在由**服务端契约**定义（`internal/savecontract.Identity()`，与客户端资源无关）。
// 本迁移把盘上**任何历史身份**（任意 64 位 hex：内层归档哈希、旧客户端指纹…）
// 一次归一为当次契约身份。
//
// ## 为什么是「形状」而不是白名单
//
// 首版实现用「已知历史内层哈希白名单」，2026-10-01 当日连失两次：
//   - 换客户端 ⇒ 新哈希不在白名单 ⇒ 0 行命中 ⇒ **静默不生效**（无任何日志）；
//   - 白名单在「换一次客户端就要改一次代码并重新编译」的意义上不可持续。
//
// 判据换成形状（`~ '^[0-9a-fA-F]{64}$'`）后：
//   - 任何来源的旧身份一次归一，运维**不需要**知道历史哈希；
//   - 仍然**绝不**碰 `character_vaults` / `character_secondary_vaults`
//     （它们的 `config_version` 是 `vault.generated.json` 的 `source_sha256`，
//     属**另一套**服务端生成物身份，与内层归档无关）；
//   - 幂等：`WHERE col <> $1` ⇒ 第二遍 0 行。
//
// ## 调用方必须可见
//
// 迁移结果**无论是否为 0** 都要打日志（见 `cmd/wireprobe/main.go`）：
// 首版失败之所以难查，就是因为 `if n > 0` 才打日志，命中 0 行时完全无声。

// saveIdentityTargets 列出「以**目录来源**为身份」的表与列。
// `character_vaults` / `character_secondary_vaults` **故意不在列**（见文件头注释）。
var saveIdentityTargets = []struct {
	Table  string
	Column string
}{
	{"characters", "config_version"},
	{"character_world", "config_version"},
	{"character_quests", "config_version"},
	{"character_events", "config_version"},
	{"character_map_clears", "source_version"},
	{"character_quest_rewards", "source_version"},
}

// MigrateSaveIdentity 把任意历史来源身份的存档行归一为 `current`（当次契约身份）。
//
// 必须在本进程**所有**建表迁移之后调用（否则表还不存在）。
// 返回被归一的总行数，便于日志与测试断言。
func (s *Store) MigrateSaveIdentity(ctx context.Context, current string) (int64, error) {
	if !savecontract.IsIdentity(current) {
		return 0, fmt.Errorf("invalid save identity")
	}
	var total int64
	for _, t := range saveIdentityTargets {
		// 表名/列名来自包内常量，不是外部输入；仍用 quoteIdent 拼装以杜绝注入。
		// 判据 = 「不等于当次契约身份」，**不限定形状**：
		//   - 老玩家的身份可能是任意历史形态（内层哈希 / 更早的批次标记 / 空串 / NULL）；
		//   - 这 6 列的**唯一**语义就是「目录来源身份」，任何取值都该被归一；
		//   - 用 `IS DISTINCT FROM` 而不是 `<>`：`NULL <> $1` 恒为 NULL，会整行漏掉。
		// 仍然幂等：已经是 current 的行不匹配。
		q := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s IS DISTINCT FROM $1`,
			quoteIdent(t.Table), quoteIdent(t.Column), quoteIdent(t.Column))
		tag, err := s.DB.Exec(ctx, q, current)
		if err != nil {
			return total, fmt.Errorf("normalize %s.%s: %w", t.Table, t.Column, err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

// quoteIdent 是**极简**标识符引用：只允许小写字母、数字、下划线（本包全部表/列名都满足）。
// 任何不合规的字符直接拒绝，避免把标识符拼接变成注入面。
func quoteIdent(name string) string {
	for _, r := range name {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			panic("unsafe SQL identifier: " + name)
		}
	}
	return `"` + name + `"`
}
