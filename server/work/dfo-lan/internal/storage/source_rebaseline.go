package storage

import (
	"context"
	"fmt"
)

// 2026-10-01（next146）：PVF 直读模式的**存档来源身份重钉**（存档兼容，最高优先级）。
//
// ## 问题
//
// 直读模式下，服务端把**内层归档** `Script.inner.pvf` 的 SHA256 当作"配置来源身份"
// 写进存档：`characters.config_version`、`character_world.config_version`、
// `character_quests.config_version`、`character_events.config_version`、
// `character_map_clears.source_version`、`character_quest_rewards.source_version`。
// 运行时拿 `role.ConfigVersion` 与当次目录的 `Catalog.Source.Checksum` 逐一比对，
// 不等就**硬拒**（`quest %d requires source migration` / `character event source mismatch` …）。
//
// 内层归档是**本地构建产物**（`ensure_inner_pvf.py` 从客户端三件套现场解出）。
// 关键事实：**即使客户端三件套完全没变，重新解包出来的 inner 哈希也会变**
// （实测 `7ef2db59…` 760,530,763 B → `b2b503b5…` 761,764,363 B，同一 DFO.exe/sk.dat/Script.pvf）。
// ⇒ 每次重新生成 inner，**全体存档**都会因身份不符而进不去角色。
//
// ## 处置
//
// 把**已知的历史内层哈希**（`historicalInnerChecksums`）重钉到**当次**内层哈希。
// 白名单式而非"凡不等皆改"，因为同一个库里还有**别的来源身份**共用同名列：
// `character_vaults.config_version` 存的是 `vault.generated.json` 的 `source_sha256`
// （如 `fda6c33f…`），它与内层无关，**绝不能**被本迁移碰。
//
// 迁移写成**幂等**：`WHERE col = ANY(白名单)`，跑第二次就没有匹配行。
// 白名单**只增不改**：将来内层再换代，把旧值追加进 `historicalInnerChecksums` 即可。

// historicalInnerChecksums 是**曾经**作为内层归档身份、且已知被写入过存档的哈希。
//
//	7ef2db59…  = 2026-09-12 版 inner（760,530,763 B）—— 所有既有存档的来源。
//
// 追加规则：只加入「确实是某个历史 inner 的 SHA256」的值，不要加入别的来源身份，
// 否则会把 vault 等无关表误伤。改动此表务必同步本文件注释与 LESSONS §G6b。
var historicalInnerChecksums = []string{
	"7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
}

// sourceIdentityTargets 列出「以**内层 checksum** 为来源身份」的表与列。
// 顺序无所谓；`character_vaults` / `character_secondary_vaults` **故意不在列**（见文件头注释）。
var sourceIdentityTargets = []struct {
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

// MigrateSourceIdentity 把历史内层哈希的存档行重钉到 `current`（当次内层 checksum）。
//
// 必须在本进程**所有**建表迁移之后调用（否则表还不存在）；
// 只在直读模式、且 `current` 是合法 64 位 SHA256 时才有意义。
// 返回被重钉的总行数，便于日志与测试断言。
func (s *Store) MigrateSourceIdentity(ctx context.Context, current string) (int64, error) {
	if len(current) != 64 {
		return 0, fmt.Errorf("invalid current source identity")
	}
	// 把白名单里等于 current 的值剔掉（正常情况下不会出现，防御性处理）。
	old := make([]string, 0, len(historicalInnerChecksums))
	for _, v := range historicalInnerChecksums {
		if len(v) == 64 && v != current {
			old = append(old, v)
		}
	}
	if len(old) == 0 {
		return 0, nil
	}
	var total int64
	for _, t := range sourceIdentityTargets {
		// 表名/列名来自包内常量，不是外部输入；仍用 quote_ident 拼装以杜绝注入。
		q := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = ANY($2::text[])`,
			quoteIdent(t.Table), quoteIdent(t.Column), quoteIdent(t.Column))
		tag, err := s.DB.Exec(ctx, q, current, old)
		if err != nil {
			return total, fmt.Errorf("rebaseline %s.%s: %w", t.Table, t.Column, err)
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
