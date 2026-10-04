# 客户端 2.38.3.25 进城确认基线（2026-10-04）

用户在修复验证入口后反馈“完美了”。本轮确认隔离候选可让旧角色进入城镇；不推断全部玩法已验收，不将此前仅有角色列表和 CMD2127 的会话算作通过。

## 程序与资源身份

- 程序：`.tmp/town-entry-20261004/wireprobe-town-entry.exe`（相对 Go 模块）。
- SHA256：`4de6789797616cbfd93b7640ebea5bdf02517c4e4f4705ce76bed1ca328517f3`。
- profile：同目录 `profile.json`，复制默认环境，仅替换 binary。
- 客户端：`D:/115us/NEW_DFO115/DFO_2.38.3.25`。
- 内层 PVF SHA256：`c3801215cef3720d75c40592a98b3329d24fb087ce8f128fd38438052f5c5f71`，资源三件套见 `server/work/client-build/Script.inner.manifest.json`。
- 默认 `bin/wireprobe-pvf.exe` 未在本轮自动覆盖；候选程序基于现有含合并变更的工作树构建，不能认为仅凭本轮提交在任意旧源码上重编译就得到相同 SHA。

## 日志与确认范围

会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261004_180414_318465_next37`：18:05:36（Asia/Singapore）角色7、18:07:49角色8、18:08:17角色7再次出现 `entry_preflight_passed`、`entry_skills_sent`（19）、`town_entry_probe_sent`（24）和 `enter_gameworld_complete_sent`（124）。客户端进城正常的验收依据为用户反馈，服务端日志提供对应入场发送证据。

源码修复为 `internal/character/learning.go: skillRows` 的职业引用校验由 RawSHA256 改为 SourcePath；保留原始审计 SHA，不迁移数据库、不改客户端资源/协议字段。入口修复为 profile 参数绝对路径，ASCII/CRLF 批处理正文，权限提升和失败窗口保留。

## 验证与残留

- 新 EntrySkills 兼容/错误引用回归和既有原生技能向量通过。
- Go1.26.0 的 `go test ./internal/... ./cmd/wireprobe ./cmd/admin ./cmd/dfo-tool` 与对应 vet 通过。
- 全仓 `./...` 仍受既有 `cmd/gmtool` 引用已删除 storage 包、运行备份参与 Go 扫描导致缺类型影响。
- `ApplyAwakening` 仍有职业 RawSHA256 校验，需单独处理觉醒操作兼容；当前会话 `quest_rejected` 也不在进城确认范围内。
- 升级流程详见仓库 `docs/客户端升级与新版本适配详细流程.md` 与工作区根目录同名手册。
