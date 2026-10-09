# 网关游戏端口「整组重抽」（Windows 保留段导致的启动失败）

> 2026-10-05 记录。对应源码改动：`cmd/wireprobe/listen.go`、`cmd/wireprobe/main.go`、
> `cmd/wireprobe/listen_test.go`。**尚未实机验收、尚未发布 `server-bin` 预编译包**，
> 按 `AGENTS.md` §0 铁律 7 等业主收口。

## 1. 现场

- **2026-10-05 20:32:12 会话**（真机，业主点「开始游戏」的第一次）：
  `runtime/roles_..._20261005_203212_156217_next37` 里只有 `gateway.err`，**没有** `ready.json`
  / `run.json` / `client.log`；`gateway.err` 末行：

  ```
  listen tcp4 127.0.0.2:61857: bind: An attempt was made to access a socket in a way
  forbidden by its access permissions.            （= WSAEACCES）
  ```

  该次**没有任何** `game listen: … rejected: … (attempt N/8)` 重试日志 ⇒ 主监听（基准端口 61856，
  以 `-game-listen 127.0.0.2:0` 由内核抽签）绑得上，失败的是**频道端口 base+1 = 61857**。
  隔 95 秒再点（20:33:46）就成功：只有一件事变了 —— 系统给这一批端口分到了另一段
  （61953…62002 共 50 个，见同轮 `ready.json`：`{"address":"127.0.0.2:61953",…,"channels":50}`）。
- `netsh int ipv4 show excludedportrange protocol=tcp` 当时列出的保留段
  （5357、49152-49251、49452-49551、50000-50059、53336-53535、62787-62886）**不含** 61857
  ⇒ 是 WinNAT/Hyper-V 一类的**动态保留**（同一台机器 2026-10-05 21:20 又观测到一次：保留段
  62787-62886 仍然在列，而实际被拒的是**不在列**的 57347）。
- 同类历史现场：`docs/archive/CHANGELOG-20260916-20261004.md` L3040-L3044（2026-09-21，
  `-game-listen 127.0.0.2:0` 抽到 3306/61222，bind 报 WSAEACCES，网关 exit 1）。

## 2. 根因

1. `listen.go` 的 `listenGamePort` 只在**主监听**上做换端口重试（≤ `ephemeralListenAttempts = 8`），
   它的注释本身就写着这个 WSAEACCES 场景。
2. 但**频道监听在重试之外**（修复前 `cmd/wireprobe/main.go`）：

   ```go
   for i, ch := range channelCfg.Channels {
       ln := l
       if i > 0 {
           ln, err = net.Listen("tcp4", net.JoinHostPort(bindHost, strconv.Itoa(basePort+i)))
           if err != nil {
               return err          // ← 任意一个频道端口不可用就立即退出
           }
           defer ln.Close()
       }
   ```

   基准端口取自**主监听**的临时端口，频道占 `base+1 … base+49`（`configs/channel.local35.json`
   共 50 行）；`ready.json` 在这之后才写。于是只要基准端口落在某个保留段**下方 49 个之内**，
   网关就在写出 `ready.json` 之前退出，启动器只能转述成「会话编排异常退出（尚未进入会话）」。
3. 2026-09-21 的「主 + 频道**整组**打开、最多重试 16 次」实现（`cmd/wireprobe/game_listen.go` 的
   `openGameListeners`）在这次改造中丢失：全树 `openGameListeners` 0 处，`game_listen.go` 不存在。

## 3. 修复

- `listen.go` 新增 `openGameListeners(addr, extra)`：每次尝试用端口 0 **重新抽一个基准端口**，
  然后把主监听与 `base+1 … base+extra` 作为**一组**依次打开；组内任意一个绑定失败就**关掉本组
  已打开的全部监听**，重新抽基准端口再来一次，直到成功或达到上限。日志前缀沿用
  `game listen: block base … rejected: … (attempt N/8)`，成功后若重试过则补一条
  `… bound after N rejected attempt(s) (last error: …)`。
- **重抽前先推过刚失败的那一组**（`skipPastGroup`：反复 `bind :0` 再关闭，直到下一枚可用端口
  超过本组最后一个端口），让相邻两次尝试落在**不重叠**的端口段上。原因（本机实测）：只靠
  「下一次 :0 抽签」时基准端口每次只前进 **1**，而本机保留段 60~200 端口宽 —— 实测基准
  62764→62765→…→62771 连续 8 次全部撞在 62787-62886 上、重试用尽后仍然退出。
  有推游标后，8 次尝试覆盖的是 8 段互不重叠的端口（≈ 8 × (extra+1) 个）。
- 规则保持：**端口是固定值（例如 7001）时不换端口、也不推游标**，只按同一基准重试；重试用尽后
  返回的错误用 `%w` 保留**最后一次真实 bind 错误原文**（WSAEACCES 是这条缺陷唯一的现场特征，
  不改写、不包装）。`ready.json` 的字段与写入时机（全部监听就绪之后）不变。
- `main.go`：把频道目录装载/解析提到打开监听之前（要知道频道个数才能定组大小），监听段改用
  `openGameListeners(startup.GameListen, extraPorts)`，成功返回的一组监听统一由
  `closeGameListeners` 关闭；端点端口仍按 `basePort + i` 计算，行为不变。

## 4. 验证

### 4.1 单测（`cmd/wireprobe/listen_test.go`，修复前失败、修复后通过）

修复前跑新增用例：`go test ./cmd/wireprobe/ -run "TestOpenGameListeners|TestListenGamePort"`
→ `[build failed]`：`undefined: listenTCP4 / openGameListeners / closeGameListeners`
（旧代码只有「主监听重试」，整组语义确实不存在）。

修复后 9 个用例全过（含 3 个既有主监听用例）：

| 用例 | 断言 |
| --- | --- |
| `TestOpenGameListenersRedrawsWholeBlockWhenChannelPortRejected` | 真实 socket：第一次抽到的基准 + 1 被占住（模拟保留段）→ 丢弃整组、重抽后成功、端口连续；且被拒那一组**释放了端口**（不泄漏 fd） |
| `TestOpenGameListenersSkipsPastRejectedGroup` | 重抽必须与上一组**不重叠**（推游标） |
| `TestOpenGameListenersKeepsLastBindErrorAndClosesEveryBlock` | 每组都被拒时重抽 8 次、返回值保留 WSAEACCES 原文、所有临时监听都关闭 |
| `TestOpenGameListenersFixedPortDoesNotRedraw` | 固定端口不换端口、不推游标，只在同一基准上重试 8 次 |
| `TestOpenGameListenersFixedPortBlockKeepsConsecutivePorts` / `…EphemeralBlockIsContiguous` | 成功时端口就是 `base … base+extra` |

### 4.2 全量门禁

`go build ./...`、`go vet ./...`、`go test ./... -count=1` 在 `server/work/dfo-lan` 全部退出码 0
（58 个包 = 41 个 `ok` + 17 个 `no test files`，`FAIL` 0 行；完整输出见 `CHANGELOG` 2026-10-05 节）。

### 4.3 真机复现（同一台机器、同一套生产 argv）

工具与产物在 `.tmp/listenfix-20261005/`（未入库）：

- `wireprobe-prefix.exe` = HEAD 源码构建（修复前）；`wireprobe-listenfix.exe` = 本次源码构建。
- 用 2026-10-05 生产会话的**逐字 argv**（`gateway.out`）+ `configs/pvf-default.json` 的 profile
  环境启动，只有 `-output`/`-fixture`/`-responses`/`-character-storage` 换成 `.tmp` 下的隔离副本
  （临时 SQLite，绝不碰玩家库）；频道目录服务改到 `127.0.0.1:7099`，避免与业主正在跑的 7001 会话冲突。
- 失败样本（修复前，`session-A-prefix`）：`-game-listen 127.0.0.2:0` 抽到 63676，频道 63677 被
  （当时存在的）瞬时保留段拒绝 → **无 ready.json、无重试日志**，与现场 20:32 的形状一致。
- 成功样本（修复后，`session-E-fixed`）：

  ```
  game listen: block base 127.0.0.2:57341 rejected: listen tcp4 127.0.0.2:57347: bind: An attempt
    was made to access a socket in a way forbidden by its access permissions. (attempt 1/8)
  game listen: block base 127.0.0.2:57392 bound after 1 rejected attempt(s) (last error: listen
    tcp4 127.0.0.2:57347: bind: An attempt was made to access a socket in a way forbidden by its
    access permissions.)
  ready.json = {"address":"127.0.0.2:57392","advertise":"127.0.0.2:57392","pid":24708,
                "fixture_bytes":1060,"channels":50}
  ```

  即：组内频道端口被 Windows 保留段拒 → 丢弃整组（57341..57390）→ 推过该组后重抽 57392 成功，
  **写出 ready.json 而不是退出**；57347 不在 `netsh` 列表里，属动态保留。
- 修复前 8 次重抽全被拒的真实样本（保留段 62787-62886，`session-D2-fixed`）：基准
  62764…62771 连续 8 次被 62787 拒 → 重试用尽后返回最后一次真实 bind 错误（此时仍会退出，
  但启动器已能把 `gateway.err` 末尾带回错误与状态栏）。

## 5. 残留边界（未闭环）

- 单个保留段**宽于** `ephemeralListenAttempts × (extra+1)`（默认 8 × 50 = 400 端口）时仍可能重试
  用尽；此时进程照旧退出，错误原文保留。要彻底收敛可再提高上限或按"被拒地址"继续向后跳。
- 端口 0 的抽签是系统全局游标，**无法保证**每次启动拿到同一段；本修复只保证"撞上保留段后能自己
  换一组、并写出 ready.json"。

## 6. 发布（玩家拿到修复的必要步骤；本节是清单，未执行）

只改源码、不重打预编译包，玩家跑的还是旧二进制。

1. 候选：`pwsh -NoProfile -File ./server/Build-Server.ps1`
   → 跑 `go test ./...`、`go vet ./...`，产出 `server/work/dfo-lan/bin/wireprobe-handoff-source.exe`。
2. 业主实机验收：`scripts\启动游戏-SQLite.cmd --source-build`。
3. 验收通过后发布为默认程序：`pwsh -NoProfile -File ./server/Build-Server.ps1 -UpdatePVFDefault`
   → `bin/wireprobe-handoff-source.exe` 覆盖 `bin/wireprobe-pvf.exe`（旧程序先备份；`bin/*.exe`
   不入库，`bin/wireprobe-dungeon39.exe` 归档基线严禁覆盖）。
4. 重打 `server-bin` 预编译包：更新 `tools/tools-server-bin.zip`（85 条目，含
   `bin/wireprobe-pvf.exe`、`bin/dfolauncher.exe`、`dfo_probe_tools/probe.exe`），同步
   `tools/manifest.json` 里 `server-bin` 的 `size`/`sha256`/`files`，并做 5 个包
   （go / gopath-mod / server-src / server-configs / server-bin）的 `size`/`sha256`/`files` 全量自检。
5. 把 `tools/*.zip` 与 `tools/manifest.json` 发布到更新源（启动器按 manifest 补 `server-bin`）；
   整包/增量走 `build-publish.ps1` 与 `scripts\一键打包.cmd`、`scripts\打包增量更新.cmd`。
