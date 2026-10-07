# 合并版双客户端进城确认基线（2026-10-04）

用户反馈“测试了，没有问题，做收尾工作”。本轮确认主工作区合并版在新旧客户端下进城可用；保留已有玩法确认边界，不宣称上游新增玩法全部逐项验收。

## 确认身份与发布

- 工作区：`D:/115us/115-server`。
- 合并源码提交：`f2ba2b28`，合入远端main `5c6fb951`，保留本地兼容修复。
- 用户验收的精确程序：`.tmp/upstream-merge-20261004/wireprobe-merged.exe`。
- SHA256：`cc9fd242069cb7d8ef291e9d21fca093e8f1ae06f0c1f582ed801a9c0a06bb86`，29,857,792字节。
- 发布后 `bin/wireprobe-pvf.exe` 与 `bin/wireprobe-handoff-source.exe` 同上述指纹，不重新构建后冒充已验收文件。
- 发布前默认5ffe3136原件保存在 `runtime/baselines/client-2.38.3.25-merged-20261004/wireprobe-pvf.before.exe`；确认程序与默认profile在同目录保存。运行产物不提交Git。
- 默认入口 `launch_local.py --check --server-only` 通过，选择当前旧客户端与默认确认程序；没有重启或关闭用户正在运行的合并会话。

## 新旧版本实机日志

时区为Asia/Singapore（UTC+8）；events.jsonl的Z时间需加8小时。

| 会话 | 实际内层资源 | 入场证据 |
| --- | --- | --- |
| `20261004_190921_433112_next37` | 新2.38.3.25：`c3801215cef3720d75c40592a98b3329d24fb087ce8f128fd38438052f5c5f71` | 19:11:20角色7预检，技能19、城镇24、完成124 |
| `20261004_191247_540430_next37` | 旧2.38.2.34：`3966e79d97274cea2b586aef33041463f8d943f9f8546d7e4c99599564204a27` | 19:14:16角色7预检，技能19、城镇24、完成124 |

完整会话目录前缀为 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_`。资源身份来自各会话gateway.err的inner archive记录，客户端实际正常依据用户确认。

当前 `server/launcher.local.json` 已由用户切回 `D:/115us/client`，该DFO.exe产品版本2.38.2.34，当前内层/manifest匹配旧源。收尾保留该选择；下次选新版时继续通过三件套/ensure匹配资源，不固定把新版内层塞给旧客户端。

## 已完成验证与残留

- 17职业、85职业/转职组合四向原生EntrySkills回归共340次通过；用户状态不被重写，错误职业引用拒绝保持。
- 合并后cmd四入口/internal源码测试、vet和构建通过，Python27项通过。
- 全仓 `./...` 仍因 `runtime/update-backup/20261004-002153` 旧Go代码参与扫描失败；上游已修GM退休storage包引用。没有删除运行备份来伪造全仓成功。
- 两轮会话仍有已记录的 `quest_rejected: quest is completed or requires configuration migration`，未当作本轮进城缺陷扩展修复。
- ApplyAwakening raw hash校验仍待单独回归，不能由EntrySkills兼容推断觉醒操作全通过。
- 数据库/schema、玩家存档、客户端资源未因本次发布改写；正式程序升级和本地源码收尾提交是本轮交付范围，未自动推送远端。
