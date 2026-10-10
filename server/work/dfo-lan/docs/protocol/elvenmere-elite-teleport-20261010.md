# 艾尔文泪湖精锐换层回归（入口已确认）

业主已确认“能进去了，提交这里吧”，换层入口收口见 [确认基线](elvenmere-elite-teleport-confirmed-baseline-20261010.md)。随后反馈 APC 留在首层，跟随问题不在本次确认范围。下文保留候选阶段的验证与限制。

## 证据与原因

attempt 1/3：恢复原有 CMD2015 服务端处理入口，不新增字段、响应或客户端补丁。

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_134350_538431_next37/`：

- `gateway.err:634` 入场副本 100003126、地图 100007201。
- `events.jsonl:14606` 已发送 NOTI2193 初始化层数。
- `events.jsonl:14617` 14:51:29（北京时间）收到校验通过的原生 CMD2015，解密正文 `0000000000000000000000000000000000ffffffffffffffff00000000000000`。
- `events.jsonl:14618–14619` 精锐门禁拒绝：`精锐普通战斗候选尚未接入专用传送/直进副本`，无应答。

与 `legacy`（23b9d26b）对比，`elvenmereTeleport` 的副本限定、长度校验、层数推进及 ACK2015/NOTI2193 仍在；本次请求被新增的 `eliteEntryProbeRequest` 无条件拒绝，未到达处理器。该门禁失败足以解释当前第一层入口卡住，不能据此声称后续全部楼层已验收。

当前 PVF `server/work/client-build/Script.inner.pvf` SHA256 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`：

- `contents/2022/elvenmere/dungeon/elvenmere.dgn` SHA256 `6a27fc89a79ff2095f35d78712dc7f50979324ba170d489730888fd52763c637`，`[map specification]` 首房为 100007201，`[dungeon type]` 为 `elvenmere dungeon`。
- `contents/2022/elvenmere/etc/elvenmere.cos` 原文 SHA256 `58b5ba92a44540d1b64fca74e21f30dde1ec257ff03b870264c81c50f17a9c87`，`[area group]` 首区 `[floor] 1 35`、`[waiting map index] 100007201`；另有 `[max floor]`、`[weekly reward]`、`[season reward]` 和 `[floor clear exp]`。

本轮只修复执行门禁，不改变 PVF 内容、原有奖励、封包/codec、存档或 schema；没有新增协议布局，未重新开展 IDA reader 取证。

## 变更与验证

CMD2015 复用房间切换现有的精锐冻结身份、频道、设置、名单、源范围、WireID 和加载状态校验，之后由 `elvenmereTeleport` 保持副本限定及长度校验。无精锐准备时继续走原有入口。奥德赛 CMD2062 的结算和目标限制保持。

`adventure_elite_elvenmere_test.go` 使用上述原生请求走 `dispatchDungeon` 完整分派/传输计划：修复前只有带精锐的合法换层用例失败；修复后验证层 1→2、ACK2015→NOTI2193、原会话和冻结名单保持，以及身份失效、未加载、非本副本、短包、校验失败均不推进。专项同时覆盖现有精锐、奥德赛和 Elvenmere 回归。

实际执行 `go build ./...`、`go vet ./...`、`go test ./... -count=1 -json` 均退出 0。修改前基线 4140 项通过/273 项跳过，修改后 4154 项通过/273 项跳过，逐名失败集合均为空；原生分派回归修复前仅 `prepared` 子用例失败，修复后专项通过。完整 JSON 报告本地保存在 `.tmp/elvenmere-teleport/{baseline,candidate}-tests.jsonl`，不入库。测试使用 Go1.26.0、GOTOOLCHAIN=local、GOPROXY=https://goproxy.cn,direct、GOPATH=C:/Game/dof/115us/tools/gopath，GOCACHE 使用当前工作区 runtime/gocache。

另以 `go build -trimpath -ldflags '-X main.serverBuildVersion=0.3.19-elvenmere-attempt1' -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe` 构建候选成功：31,107,072 字节，SHA256 `dda10d39b4d89528a6380365ebfa33b57452c0de475b7a64cfc4863c0e0dd947`，构建版本 `0.3.19-elvenmere-attempt1`。构建前用进程名/原会话 PID 检查未见运行中的候选；CIM 查询被本机权限拒绝，未将其视为成功检查。默认 `wireprobe-pvf.exe` 保持 SHA256 `a6f269603887f18fa8bb60814a655641f12c5eeaf1a07f89d840e166a393be6c`。本机未发现 39 归档文件，本轮没有创建或覆盖它。

本轮源码文件：`cmd/wireprobe/adventure_elite_entry_probe.go`、`adventure_elite_entry_probe_test.go`、`adventure_elite_direct_move_test.go` 及新增 `adventure_elite_elvenmere_test.go`；文档为本记录和 PVF 依赖台账补项。用户已有未跟踪文件未修改或纳入提交。`git diff --check` 通过；本轮待实机确认，未暂存或提交，未运行提交门禁，也未更新 confirmed baseline。

## 手动验收与回滚

候选已准备，由业主关闭当前会话，再执行 `scripts/启动游戏-SQLite.cmd --source-build`，用同一精锐名单进入艾尔文泪湖，先验证第一层入口到第二层，再观察连续换层、怪物及结算。读取新会话 CMD2015、`elvenmere_teleport_ack`、`elvenmere_info_sent` 和后续场景请求确认结果。AI 不启动客户端或代跑。

源码回滚只撤销本轮 CMD2015 门禁差异；旧候选另存 `.tmp/elvenmere-teleport/wireprobe-handoff-source.before.exe`。默认程序与 39 基线不覆盖；未获用户实机确认，不更新 CHANGELOG 或 confirmed baseline。

遗留内容重复规则见 `docs/todo/pvf/PVF运行依赖台账.md` 本次补项：不将入口修复计作奖励真源迁移或全副本验收。
