# configs JSON 字面引用清单

由 `scripts/audit_config_references.py` 生成。扫描范围为模块的 cmd、internal、scripts，以及不超过 1 MiB 的 configs JSON。

引用包含注释和历史分支；数字是不同引用位置数，不表示正式运行必读。零引用也不能作为删除依据：动态拼路径、模块外启动器、GM 代理及历史二进制未由本清单证明。

共 71 个顶层 JSON，扫描 1505 个文件；只输出文件名、大小和引用位置，不输出配置值。

| JSON | MiB | 网关 | internal | 工具 | 测试 | 脚本 | 配置引用 | 非测试引用示例 |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `account-options.current35.json` | 0.000 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `apocalypse.generated.json` | 0.055 | 2 | 2 | 1 | 6 | 0 | 0 | `cmd/apocalypseimport/main.go:4`<br>`cmd/wireprobe/config.go:105` |
| `attunement-rewards.generated.json` | 0.026 | 0 | 1 | 2 | 4 | 0 | 0 | `cmd/attunementimport/main.go:5`<br>`cmd/attunementimport/main.go:46` |
| `black-purgatory-rewards.json` | 0.107 | 1 | 1 | 0 | 0 | 0 | 0 | `cmd/wireprobe/bootstrap.go:1464`<br>`internal/gamedata/catalogs_content.go:121` |
| `bleeding-mine-rewards.json` | 0.674 | 2 | 2 | 0 | 0 | 0 | 0 | `cmd/wireprobe/bootstrap.go:952`<br>`cmd/wireprobe/config.go:94` |
| `boxes.json` | 0.029 | 2 | 1 | 0 | 8 | 0 | 0 | `cmd/wireprobe/bootstrap.go:992`<br>`cmd/wireprobe/config.go:80` |
| `cards.compat90.json` | 0.000 | 1 | 0 | 1 | 0 | 0 | 0 | `cmd/charactercheck/card_check.go:30`<br>`cmd/wireprobe/config.go:81` |
| `channel.local28.json` | 0.001 | 0 | 0 | 0 | 1 | 0 | 0 | 需追踪动态路径或外部入口 |
| `channel.local31.json` | 0.001 | 0 | 0 | 0 | 2 | 0 | 0 | 需追踪动态路径或外部入口 |
| `channel.local32.json` | 0.002 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `channel.local34.json` | 0.006 | 0 | 0 | 0 | 3 | 0 | 0 | 需追踪动态路径或外部入口 |
| `character-probe.json` | 0.000 | 1 | 0 | 0 | 0 | 0 | 0 | `cmd/wireprobe/config.go:51` |
| `character-rules.jobs-release.json` | 0.000 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `character-rules.odyssey-release.json` | 0.000 | 0 | 0 | 1 | 0 | 0 | 0 | `cmd/initialrepair/main.go:35` |
| `characters.alljobs-pilot.json` | 0.066 | 0 | 0 | 0 | 3 | 0 | 0 | 需追踪动态路径或外部入口 |
| `characters.auto-skills-candidate.json` | 0.074 | 0 | 0 | 0 | 3 | 0 | 0 | 需追踪动态路径或外部入口 |
| `characters.awakening-candidate.json` | 0.070 | 0 | 0 | 0 | 4 | 0 | 0 | 需追踪动态路径或外部入口 |
| `characters.generated.json` | 0.120 | 1 | 0 | 2 | 18 | 0 | 0 | `cmd/catalogimport/main.go:15`<br>`cmd/shieldaudit/main.go:18` |
| `characters.next25.json` | 0.125 | 0 | 0 | 0 | 17 | 0 | 0 | 需追踪动态路径或外部入口 |
| `characters.skycastle-release.json` | 0.074 | 0 | 0 | 1 | 41 | 0 | 0 | `cmd/pvfaudit/main.go:47` |
| `characters.swordmaster-pilot.json` | 0.053 | 0 | 0 | 0 | 1 | 0 | 0 | 需追踪动态路径或外部入口 |
| `clear-cube-source.json` | 0.002 | 0 | 1 | 0 | 1 | 0 | 0 | `internal/gamedata/catalogs_content.go:166` |
| `drop.compat90.json` | 0.000 | 1 | 0 | 1 | 12 | 0 | 0 | `cmd/charactercheck/loot_check.go:22`<br>`cmd/wireprobe/config.go:66` |
| `drop.current36.json` | 0.001 | 0 | 0 | 2 | 7 | 0 | 0 | `cmd/audit36/verify.go:50`<br>`cmd/audit36/verify.go:51` |
| `dungeons.layer-revisits.json` | 0.001 | 1 | 1 | 0 | 1 | 0 | 0 | `cmd/wireprobe/bootstrap.go:743`<br>`internal/gamedata/catalogs_scenes.go:194` |
| `dungeons.maze-chance-rates.json` | 0.000 | 2 | 4 | 0 | 1 | 0 | 0 | `cmd/wireprobe/bootstrap.go:759`<br>`cmd/wireprobe/maze_chance.go:16` |
| `dungeons.terminal-scenes.json` | 0.006 | 1 | 1 | 0 | 0 | 0 | 0 | `cmd/wireprobe/bootstrap.go:739`<br>`internal/gamedata/catalogs_scenes.go:42` |
| `dungeons.training-room.json` | 0.183 | 1 | 1 | 0 | 3 | 0 | 0 | `cmd/wireprobe/bootstrap.go:729`<br>`internal/gamedata/catalogs_scenes.go:342` |
| `equipment-create-cost.generated.json` | 0.006 | 0 | 1 | 1 | 6 | 0 | 0 | `cmd/equipmentjournalimport/main.go:5`<br>`internal/catalog/equipment_create_cost.go:17` |
| `equipment-journal.generated.json` | 0.027 | 0 | 0 | 2 | 5 | 0 | 0 | `cmd/equipmentjournalimport/main.go:4`<br>`cmd/equipmentjournalimport/main.go:50` |
| `equipment-knight-shield.full-candidate.json` | 0.007 | 1 | 1 | 1 | 4 | 0 | 0 | `cmd/shieldaudit/main.go:22`<br>`cmd/wireprobe/config.go:87` |
| `equipment-wear.current35.json` | 0.001 | 0 | 0 | 6 | 11 | 1 | 0 | `cmd/charactercheck/wear_check.go:49`<br>`cmd/gmtool/index.go:147` |
| `equipment-wear.full-candidate.json` | 0.001 | 0 | 1 | 1 | 2 | 0 | 0 | `cmd/avatarrestorecheck/main.go:63`<br>`internal/game/protocol/equipment_journal.go:127` |
| `equipment.current35.json` | 2.126 | 0 | 0 | 0 | 14 | 0 | 1 | `configs/drop.current36.json:10` |
| `equipment.current37.json` | 4.333 | 0 | 2 | 6 | 21 | 0 | 0 | `cmd/gmtool/index.go:10`<br>`cmd/gmtool/index.go:180` |
| `experience.compat90.json` | 0.000 | 1 | 0 | 4 | 2 | 0 | 0 | `cmd/charactercheck/clear_reward_check.go:24`<br>`cmd/charactercheck/progression_check.go:29` |
| `fatigue-probe.json` | 0.000 | 1 | 1 | 0 | 0 | 0 | 0 | `cmd/wireprobe/config.go:61`<br>`internal/savecontract/normalize.go:38` |
| `inventory.compat90.json` | 0.000 | 1 | 0 | 2 | 3 | 0 | 0 | `cmd/charactercheck/card_check.go:26`<br>`cmd/charactercheck/loot_check.go:26` |
| `inventory.current37.json` | 0.000 | 0 | 0 | 0 | 11 | 0 | 0 | 需追踪动态路径或外部入口 |
| `inventory.next29.json` | 0.000 | 0 | 0 | 6 | 12 | 0 | 0 | `cmd/admin/main.go:70`<br>`cmd/charactercheck/grant_check.go:24` |
| `itemshop-candidate.json` | 0.836 | 3 | 3 | 2 | 3 | 0 | 0 | `cmd/itemshopimport/main.go:10`<br>`cmd/itemshopimport/main.go:153` |
| `legion-contents.generated.json` | 0.035 | 0 | 0 | 2 | 2 | 0 | 0 | `cmd/legionimport/main.go:3`<br>`cmd/legionimport/main.go:47` |
| `loot.next25.json` | 3.124 | 0 | 1 | 1 | 23 | 0 | 0 | `cmd/gmtool/index.go:10`<br>`internal/loot/dungeon_group_drop.go:143` |
| `oath-grades.json` | 0.034 | 1 | 3 | 2 | 1 | 0 | 0 | `cmd/oathgradeimport/main.go:13`<br>`cmd/oathgradeimport/main.go:56` |
| `odyssey-chapter-drop-release.json` | 0.001 | 0 | 2 | 0 | 5 | 0 | 0 | `internal/gamedata/catalogs_content.go:314`<br>`internal/loot/odyssey_chapter_drop.go:13` |
| `odyssey-chapters-release.json` | 0.003 | 0 | 2 | 0 | 2 | 0 | 0 | `internal/catalog/odyssey_chapters.go:10`<br>`internal/gamedata/catalogs_content.go:293` |
| `odyssey-currency.json` | 0.004 | 0 | 1 | 0 | 4 | 0 | 0 | `internal/gamedata/catalogs_content.go:334` |
| `odyssey-growth-release.json` | 0.034 | 0 | 1 | 0 | 10 | 0 | 0 | `internal/gamedata/catalogs_content.go:270` |
| `odyssey-weapon-box-release.json` | 0.061 | 0 | 1 | 0 | 3 | 0 | 0 | `internal/gamedata/catalogs_content.go:354` |
| `pvf-box-policy.json` | 0.001 | 1 | 0 | 0 | 3 | 0 | 2 | `cmd/wireprobe/config.go:41`<br>`configs/pvf-default.json:17` |
| `pvf-character-policy.json` | 0.001 | 1 | 1 | 2 | 9 | 0 | 2 | `cmd/audit36/verify.go:27`<br>`cmd/charactercheck/native_catalogs.go:45` |
| `pvf-default.json` | 0.002 | 0 | 0 | 0 | 6 | 4 | 0 | `scripts/launch_local.py:21`<br>`scripts/launch_local.py:25` |
| `pvf-drop-policy.json` | 0.020 | 1 | 1 | 7 | 11 | 0 | 2 | `cmd/audit36/main.go:198`<br>`cmd/audit36/verify.go:59` |
| `pvf-enhancement-policy.json` | 0.005 | 1 | 0 | 0 | 10 | 0 | 2 | `cmd/wireprobe/config.go:36`<br>`configs/pvf-default.json:8` |
| `pvf-item-shop-policy.json` | 0.052 | 1 | 0 | 0 | 2 | 0 | 2 | `cmd/wireprobe/config.go:40`<br>`configs/pvf-default.json:18` |
| `pvf-layer-revisit-policy.json` | 0.000 | 1 | 0 | 0 | 2 | 0 | 2 | `cmd/wireprobe/config.go:43`<br>`configs/pvf-default.json:14` |
| `pvf-mine-policy.json` | 0.001 | 0 | 0 | 0 | 6 | 0 | 2 | `configs/pvf-default.json:12`<br>`configs/repair-profile.example.json:12` |
| `pvf-scene-policy.json` | 0.000 | 1 | 0 | 0 | 9 | 0 | 2 | `cmd/wireprobe/config.go:46`<br>`configs/pvf-default.json:11` |
| `pvf-script-warp-policy.json` | 0.005 | 1 | 0 | 0 | 2 | 0 | 2 | `cmd/wireprobe/config.go:44`<br>`configs/pvf-default.json:13` |
| `pvf-vault-policy.json` | 0.000 | 1 | 0 | 0 | 4 | 0 | 2 | `cmd/wireprobe/config.go:37`<br>`configs/pvf-default.json:9` |
| `randomoption.current37.json` | 0.137 | 2 | 1 | 1 | 5 | 0 | 0 | `cmd/randomoptionimport/main.go:16`<br>`cmd/wireprobe/bootstrap.go:345` |
| `refine.json` | 0.002 | 1 | 3 | 0 | 3 | 0 | 0 | `cmd/wireprobe/bootstrap.go:881`<br>`internal/inventory/equipment.go:123` |
| `repair-profile.example.json` | 0.002 | 0 | 0 | 0 | 1 | 0 | 0 | 需追踪动态路径或外部入口 |
| `select-parser-probe.json` | 0.000 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `select-world-probe.json` | 0.000 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `town-entry-probe.json` | 0.000 | 0 | 0 | 0 | 0 | 0 | 0 | 需追踪动态路径或外部入口 |
| `town.generated.json` | 0.001 | 0 | 1 | 1 | 0 | 0 | 0 | `cmd/towncatalog/main.go:16`<br>`internal/gamedata/catalogs_scenes.go:291` |
| `tutorial-dungeons.current36.json` | 2.098 | 0 | 1 | 2 | 4 | 0 | 0 | `cmd/audit36/verify.go:45`<br>`cmd/audit36/verify.go:46` |
| `tutorial-routes.current35.json` | 0.030 | 0 | 0 | 2 | 2 | 0 | 0 | `cmd/audit36/verify.go:34`<br>`cmd/audit36/verify.go:35` |
| `vault.generated.json` | 0.002 | 0 | 5 | 1 | 3 | 0 | 0 | `cmd/charactercheck/module_check.go:22`<br>`internal/gamedata/catalogs_equipment.go:183` |
| `world-probe.json` | 0.000 | 1 | 0 | 0 | 0 | 0 | 0 | `cmd/wireprobe/config.go:57` |
