# 城主宫殿（天空之城 18 号主线任务 3185）全流程排查与修复报告

## 1. 业务背景与环境
- **角色**：ID 11（骑士，当前等级 19）。
- **任务**：任务 ID 3185（天空之城 18 号任务，`skycastle_18.qst`，目标 `[clear map] 57962`）。
- **副本**：城主宫殿（副本 15，迷宫 4）。

## 2. 现场复盘与精准定位
### 2.1 第一阶段：过场动画后不刷 Boss
- **现象**：进入 Boss 房播放过场动画后，房间空无一物，不刷新 Boss。
- **根因**：迷宫 4 的 Boss 房 `(1, 0)` 配置为 `boss 1 0 100008324` 并带 `layered 1 0 57962 100008595`。开场动画 `14646.cmt` 在基础地图 `100008324` 播放完毕后发出 CMD 45 请求切层到光之城主 Boss 房 `57962`。服务端原先在 `MoveScene` 中硬编码阻断非奥德赛副本，回执 ErrCode 4，导致未能切入地图 `57962`。
- **修复**：放宽常规副本切层通道，按 `s.Maze.Layers` 映射顺序平滑切层。

### 2.2 第二阶段：击杀 Boss 后在阳台地图卡住
- **现象**：击杀 Boss 后，过场动画切换到阳台（外观同前一地图 `57958`，实为层 2 地图 `100008595`），动画播放完毕后流程卡死，无法翻牌或退出。
- **排查实时日志（2026-09-21 07:43:30）**：
  1. `07:43:30.686`：`map_clear_quest_triggers 291`、`boss_check_confirmed 115`、`dungeon_clear_enabled 31` 均已全部成功发送！
  2. `07:43:30.707`：客户端收到通关允许后，立即发送 CMD 46（`ENUM_CMDPACKET_DUNGEON_RESULT`）请求结算与卡牌奖励。
  3. `07:43:30.838`：服务端拒绝 CMD 46，报错：`dungeon_request_refused 46: source card gold level absent`，向客户端回复了错误码 4（`Refusal 4`）。
  4. 客户端收到结算错误后卡死在当前画面，未弹出结算界面与翻牌。
- **根因剖析**：
  - 在 `settlement_flow.go` 处理 CMD 46 时调用了 `loot.FreezeCards` 计算翻牌金币。
  - `FreezeCards` 原逻辑仅从当前房间 `d.Monsters` 提取非战斗怪物最高等级（`!m.NonCombat && m.Level > level`）。
  - 而此时角色所处的阳台地图 `100008595` 是剧情过渡地图，其中所有怪物（Dummy Boss `75099` 与 NPC 剧情怪）均为 `NonCombat: true`，提取出的 `level` 为 0。
  - 随后调用 `CardGold` 查表：PVF 原生金币表 `[gold drop ref table]` 仅包含 1..200 级，查找 0 级直接报错 `source card gold level absent`。
- **修复**：
  - 在 `server/work/dfo-lan/internal/loot/cards.go` 的 `FreezeCards` 中增加回溯：若当前房间无战斗怪导致 `level == 0`，则遍历本轮副本已访问的所有房间（`d.Visited`，包含击杀光之城主的房间 `57962`）提取有效怪物等级；若仍为 0 则取副本基准等级 `d.Definition.BasisLevel`（保底 1 级）。
  - 确保翻牌计算百分之百命中合法等级区间。

## 3. 产物与验证
- 修改文件：
  - `server/work/dfo-lan/internal/loot/cards.go`
  - `server/work/dfo-lan/internal/dungeon/scene_transition.go`
  - `server/work/dfo-lan/internal/dungeon/completion.go`
  - `server/work/dfo-lan/cmd/wireprobe/main.go`
- 测试用例：
  - `server/work/dfo-lan/internal/loot/cards_test.go`
  - `server/work/dfo-lan/internal/dungeon/standard_layer_test.go`
  - `go test ./...` 与 `go vet ./...` 全部通过。
- 编译生成物：`server/work/dfo-lan/bin/wireprobe-handoff-source.exe`。
