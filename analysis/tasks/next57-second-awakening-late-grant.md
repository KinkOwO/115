# next57 — 二觉技能不随等级自动加点（觉醒段授予改为读时推导）

## 1. 现象

玩家反馈：**二觉（stage 2）技能没有根据等级自动加点**，一觉（stage 1）正常。
实机形状：在 75 级完成二觉，二觉段里的 85 级技能没有自动出现，之后升到 85 级也不出现。

## 2. 数据量化（`configs/characters.skycastle-release.json` + `configs/skills.next27.json`）

觉醒门槛是 `ApplyAwakening` 里的 `[4]byte{0, 50, 75, 100}[stage]`（一觉 50 / 二觉 75 / 三觉 100），
而 `.chr` 的 `[awakening N]` 段里的技能带自己的 `[required level]`：

| stage | 门槛 | 授予总数 | 门槛之上的授予（会被跳过） |
| ----- | ---- | -------- | -------------------------- |
| 1（一觉） | 50 | 140 | **0** |
| 2（二觉） | 75 | 140 | **67**（剑魂 adv1 有 15 条是 85 级） |
| 3（三觉） | 100 | 68 | 0（段内是 95 级行，低于 100 门槛） |

一觉之所以"正常"，是因为它的授予等级全部 ≤ 50，觉醒当下就全部达标。

## 3. 根因

`internal/character/awakening.go` 的 `ApplyAwakening` 是**一次性写存档**：

```go
if int(state.Level) < levels[0] {
    continue // 源授予可能点名 85 级技能，二觉时先实例化为未学
}
```

75~84 级二觉的角色被这里跳过，而**之后没有任何补发路径**：

- `internal/character/automatic_skills.go` 的 `automaticSkills` 只推导转职段
  `advancement_skills`，不包含觉醒段；
- `cmd/wireprobe/automatic_skills.go` 的升级刷新（`automaticSkillRefresh`，副本杀怪 /
  elvenmere / 奥德赛清关升级时下发 `id-19` 技能帧）当时只在该职业带
  `advancement_skills` 时才发帧，且帧内容同样来自上面那条只懂转职段的读取路径。

## 4. 实现

与历史上"转职段起始技能"的修复（`automaticSkills` + `advancement.go` 注释所写的
"granted at read time … only the advancement/awakening fields are persisted"）同一套路：
**把觉醒段授予也改成从存档的等级/阶段现算**，不改存挡。

- `internal/character/automatic_skills.go` 新增 `awakeningSkills(role, state)`：逐
  `stage = 1..state.Awakening` 取 `prof.AwakeningSkills[state.Advancement][stage]`，
  技能必须在本职业目录里、`[required level]` 必须是单值，**等级达标才计入**（低于门槛的授予挂起）。
- `knownSkills` 把 `automaticSkills` 与 `awakeningSkills` 合并后并入读取集合，并把合并结果
  作为"分支清理"的豁免集合（本分支源授予的行不因 `ForAdvancement/ForAwakening` 判定被删）。
  技能树、进城技能帧、加点响应、Reset 全部走这一条读取路径，所以一处修复即全链路生效。
- `internal/character/learning.go`（`Learn` 的退点 floor）与 `internal/character/skill_variation.go`
  （`skillFloor`）删除各自手写的觉醒段循环，统一调用 `awakeningSkills`：退点口径不变
  （源授予不可退成 SP），两处不再有分叉。
- `cmd/wireprobe/automatic_skills.go` 的发帧条件从"有 `advancement_skills`"放宽为
  "转职段或觉醒段任一有授予"，因此 84→85 级升级时会随 `id-19` 帧把新授予送到客户端。

副产物：不需要任何存档迁移；**75 级就二觉的老角色**下次登录/升级即自动修复。

## 5. 测试

- 新增 `internal/character/awakening_grant_test.go`：
  - `TestSecondAwakeningLateGrantFollowsLevel`：全目录 **67 个**"二觉段 + 自身等级 76..115"的样本，
    逐条断言 75 级不授予、达标授予、二觉前（Awakening=1）不授予；
  - `TestSecondAwakeningLateGrantReachesKnownSkills`：同一份存档把等级从 75 提到门槛，
    `knownSkills` 随之下发该技能；
  - `TestAwakeningGrantsCoverTheWholeSource`：115 级三觉角色对全部 **348 条**觉醒授予全量覆盖，
    且 `skillRows` 能把含觉醒专属技能的行完整下发；
  - `TestAwakeningGrantStaysOnReset`：Reset（CMD483）保留授予、SP 不变。
- `cmd/wireprobe/automatic_skills_test.go`：新增"职业只带 `awakening_skills` 时也发帧"用例。
- `go build ./internal/... ./cmd/...`、`go vet ./internal/... ./cmd/...`、
  `go test -count=1 ./internal/... ./cmd/...`（19 包）全绿。

## 6. 实机验证（2026-09-22，用户操作）

- 二觉角色的二觉段技能按等级自动加点生效，判定通过：技能自身 `[required level]` 达标后自动出现
  （未二觉 / 等级不足时不授予）；
- 回归未受影响：一觉、普通加点、Reset 退点口径照常。

保留的观察项（本次未单独验证，逻辑上已由测试覆盖）：三觉段（100 级门槛、段内 95 级行）走同一函数。

## 7. 背景（本轮之前的丢失）

本轮开始时工作区里**没有**二觉相关源码改动：`fix/second-awakening-late-grant` 分支的
worktree（`%TEMP%\...\awk2`）已被删除，残留 `.git/worktrees/awk2/index` 与 `HEAD` 无差异
（改动从未 `git add`/`commit`），`git fsck` 的悬挂对象里也没有对应源码；只留下
`server/work/dfo-lan/bin/wireprobe-handoff-source.exe.bak-20260922-2300-preawk2grant`
的编译产物痕迹。本文件即按该缺口重新落地的实现与取证记录。
