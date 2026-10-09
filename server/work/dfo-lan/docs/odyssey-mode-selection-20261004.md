# 取证：「奥德赛模式消失 / 奥德赛角色进游戏变普通角色」

> 业主报告：普通模式已正常，但奥德赛模式消失；按奥德赛方式创建的角色进游戏是普通角色。
> 本记录只写已复算的事实；未启动客户端、未访问玩家库（PG 25438）。

## 0. 结论

**不是本次存储修复引入的，也不是存档丢了**：角色把「奥德赛建号」标记**完整保存下来了**
（`create_request` 里 `Options[10] == 2`），消失的原因是**启动入口的环境变量把模式强制成了普通档**：

```
scripts\启动游戏.cmd        → set DFO_ODYSSEY_MODE=0
scripts\启动游戏-奥德赛.cmd → set DFO_ODYSSEY_MODE=1
```

## 1. 证据

活动库三个角色（只读解码，布局同 `internal/game/protocol/characters.go:DecodeCreateRequest`）：

| id | 名字 | 职业 | `Options[10]` | 判定 |
| --- | --- | --- | --- | --- |
| 1 | `qqqq` | 0 | 0 | 普通建号 |
| 2 | `12312` | 0 | 0 | 普通建号 |
| 3 | `asdasd` | 11 | **2** | **奥德赛建号**（`create_request` 24B，suffix 校验通过，growth_type=3） |

`asdasd` 是 23:37:41 用新候选建的（`events.jsonl` 有 `character_committed id=3 name=asdasd`），
客户端 CREATE_CHARACTER 原文 `0b06000000617364617364000000000000ff000300020000`
里 `Options[10] = 02` —— **建号链路把奥德赛标记正确写进了存档**。

## 2. 机制

`internal/character/odyssey.go`：

```go
func OdysseyRole(role Character) bool {
	if OdysseyGraduated(role) { return false }
	if mode := os.Getenv("DFO_ODYSSEY_MODE"); mode != "" {
		return mode == "1"          // ← 只要环境变量非空，就完全压过角色自己的标记
	}
	return CreatedAsOdyssey(role)   // ← env 为空时，才按建号 Options[10]==2
}
```

而同一份代码里另有一个**按角色**的判据，用于城镇/世界准入：

`internal/character/odyssey_graduate.go:31`
```go
func OdysseyMember(role Character) bool { return CreatedAsOdyssey(role) && !OdysseyGraduated(role) }
```

`cmd/wireprobe/world_flow.go:224-228` 的注释明确写了准入用按角色标记、**不用** launcher 的
`DFO_ODYSSEY_MODE`。于是 `DFO_ODYSSEY_MODE=0` 时同一个角色处于**半档状态**：

| 层面 | 判据 | `asdasd` 的结果 |
| --- | --- | --- |
| 城镇/世界准入 | `OdysseyMember`（按角色） | 按奥德赛处理 |
| 奥德赛玩法系统（等级动作/礼物/传送/奥德赛副本准入） | `OdysseyRole`（env 优先） | **按普通处理 ⇒ 奥德赛内容全部不生效** |

这就是「创建了奥德赛角色，进游戏却是普通角色」。

另注：`启动游戏-奥德赛.cmd` 的 `=1` 是**整档强制**——它会让 `qqqq`、`12312` 这些普通角色
也走奥德赛系统。分档语义下这是设计如此，但和「同一个库里两种角色共存」不相容。

## 3. 与本次存储修复的关系

本次改动只把 `internal/database` 里 41 处「查无此行就继续」的判断从 PG 专有哨兵改成
`isNoRows`（见 `sqlite-absence-continue-fix-20261004.md`）。模式判定读的是
`os.Getenv("DFO_ODYSSEY_MODE")` 与 `role.Request`，两者都不经过那些调用点；而且
`Options[10]=2` 被正确落库，正说明建号/存档链路没被影响。

## 4. 两条可选路线（需业主定）

| 路线 | 做法 | 效果 |
| --- | --- | --- |
| **A 保持分档**（现状，不改代码） | 普通内容用 `启动游戏.cmd`，奥德赛内容用 `启动游戏-奥德赛.cmd` | 一次只跑一档；奥德赛档里普通角色也会被当奥德赛 |
| **B 改为按角色**（推荐，与客户端一致） | `启动游戏.cmd` 不再设置 `DFO_ODYSSEY_MODE`（留空），由建号 `Options[10]` 决定；`启动游戏-奥德赛.cmd` 可保留 `=1` 作为「整档强制」入口 | 同一个库里普通角色与奥德赛角色共存，且与客户端自己读的 per-character 标记一致；同时消除上面那张表的半档不一致 |

路线 B 只动启动入口（仓库根的两个 `.cmd`），不动服务端判定逻辑、不动存档格式；
按角色判据 `CreatedAsOdyssey` 本来就已实现，路线 B 等于把它从「env 未设置时才生效」变成常规路径。

## 5. 立即可用的验证方式（不改任何东西）

```
scripts/启动游戏-奥德赛.cmd --source-build
```

用角色 `asdasd` 进城，应能看到奥德赛系统生效。若仍不行，请把该轮 `gateway.err`
与 `events.jsonl` 里 `entry_*`/`odyssey*` 相关事件发回。

## 6. 边界

未启动客户端、未代替玩家操作；未访问玩家库；未改任何代码或配置（本文档只是取证记录）。

## 7. 已实施：路线 B（业主 2026-10-04 选定）

### 改动

| 文件 | 改动 |
| --- | --- |
| `scripts/启动游戏.cmd`（原根目录） | **删除** `set DFO_ODYSSEY_MODE=0`，原地留三段中文 `rem` 说明为什么不再设置、以及需要整档强制时该用哪个入口 |
| `server/work/dfo-lan/internal/character/odyssey_test.go` | 在 `TestCreatedAsOdysseyIgnoresLauncherMode` 开头显式 `t.Setenv("DFO_ODYSSEY_MODE", "")`，把「按角色档」钉成确定性结论，不再依赖测试进程的外部环境 |

`scripts/启动游戏-奥德赛.cmd` **保持不动**（仍是 `=1` 的整档强制入口）。服务端判定逻辑、存档格式、
协议一律未改。

### 为什么这是「回到启动器自己的默认档」

相邻启动器仓库本来就以「按角色」为默认，且已把这件事写进注释：

* `115us-dfolauncher/internal/config/settings.go:237-247`：`GameModeAuto`（按角色，默认）/
  `GameModeScenario`（`=0`）/ `GameModeOdyssey`（`=1`），并说明「上游用两个入口脚本表达强制语义，
  所以默认档是按角色：不注入变量」；
* `settings.go:292-300` `GameModeEnv`：按角色档返回空串 → 不注入 `DFO_ODYSSEY_MODE`；
* `settings.go:464-473` `ServerSupportsPerRoleMode`：探服务端脚本是否已按角色准备。
  已核对当前 `server/work/dfo_probe_tools/channel_probe.py` **不含**小写 `odyssey_mode`
  ⇒ 返回 true（已支持），不会回落到强制奥德赛；
* `internal/run/run.go:1306-1333` `withEnv`：同名键以启动器注入的为准；
  按角色档不注入，所以真正决定权回到角色存档。

而 `channel_probe.py:91-94`、`:481-484` 也已写明「模式该由角色存档决定
（`internal/character/odyssey.go`），而非启动参数」。

### 预期效果（待实机验收）

用 `scripts/启动游戏.cmd --source-build`（**不再需要**奥德赛入口）应当：

1. `asdasd`（`Options[10]=2`）按奥德赛模式进游戏：奥德赛成长/礼物/传送/副本准入生效；
2. `qqqq`、`12312`（`Options[10]=0`）仍是普通角色；
3. 奥德赛角色不再出现「准入按奥德赛、玩法按普通」的半档状态
   （`OdysseyMember` 与 `OdysseyRole` 现在一致）。

判定依据：`gateway.err` 无异常；奥德赛角色进城/升级时能看到奥德赛等级动作与内容。
若仍表现为普通角色，请把该轮 `events.jsonl` 的 `entry_*` 事件与 `gateway.err` 发回。

> 门禁：`go build ./...` = 0、`go vet ./...` = 0；
> `internal/character`、`internal/database`、`internal/workflow`、`internal/quest`、
> `cmd/wireprobe` 全部 `ok`（含被改动的 `TestCreatedAsOdysseyIgnoresLauncherMode`）。
> 服务端二进制未变（本次只动 `.cmd` 与测试），候选仍为
> `bin/wireprobe-handoff-source.exe` SHA256 `C738A15415B97D3BC5B61AD9A7F2BDDFFEEFE713BEB609C880D71C7C9E38CF50`。

