# 骑士盾 Shield Safe 服务端候选 — 2026-09-30

状态：**用户实机确认（2026-09-30，城镇穿戴同步）**。本轮由用户要求实施尚未实现部分；仅修改 Go 服务端及配置，不修改客户端、DLL、PVF、sk.dat 或权威 IDB。工作途中 main 增加了融合石提交 `8c304ed`，已保留并在其上验证；用户已有的 launcher.settings.json 修改未触碰。确认范围是骑士货架盾穿戴后装备格与纸娃娃同步显示；Reserved 全流程和副本即时外观不包含在此次确认中。

## 当前实现

- 只读导出器 `cmd/shieldaudit` 从配置基线内层 PVF 生成 `configs/equipment-knight-shield.full-candidate.json`；职业、穿戴表和全量装备目录必须同源。它不重写角色表或 config_version。
- 窗口表 25 行：19 面等级门、6 面任务门。任务门因没有已闭环的任务准入证据仍拒绝；窗口未装载时 31/32 整族拒绝。
- CMD19：32→31/0 穿上、31/0→32 卸下；31/N→31/M 更新真实牌组，涉及 0 格时同步穿戴槽 24。空源、越界、错误职业、未知模板、非法形状和不满足门禁均拒绝。盾牌窗口拒绝码为 5，普通移动仍保留现有码 4。
- CMD649：从 byte 0 读取五个 u32，只接受 20 字节或尾部全零的 24 字节。通过角色锁和 `CommitCharacterEvent` 保存，model=`knight-shield-deck-v1`；每个已验证的成功或失败请求都回 `[00][u16 0]`，不通过应答改写客户端牌组。
- 存档新增 `inventory.knight_shield_deck`，omitempty，老档无需迁移。读取 0 格始终取穿戴表；SaveBag 在写出已有牌组时也同步 0 格，防普通背包路径形成第二真源。新功能不改数据库结构。
- 登录 NOTI567 恢复五格；未存牌组且未穿盾时不发。运行期不发 567。
- 城镇里，Reserved 换盾或 649 导致穿戴变化后发 `AppearanceProbe → EntryBasicProbe → NOTI14`，NOTI13 穿戴数据在前；货架带模板 ID 的换装不额外重建角色。`DFO_EQUIP_AVATAR_REFRESH=0` 关闭角色重绘，仍发送装备数据。卸掉最后一件盾时显式清空槽 24，不依赖空更新。
- **副本内保留当前仓库已有的 mode0 保护**：当前分支实机证据表明角色重建可阻断 CMD45 房间切换，因此本轮副本中只更新装备/属性数据，外观随下一次进场恢复。副本即时盾牌外观尚未交付，不能套用外部文档的全场景签收。
- 商城旧币和礼包迁移的局部 inventory 投影改为字段合并，保留牌组、pet_items、expand_equip_flags、creature_*、武器皮肤及未知附加字段；不修改或删除玩家现有字段。
- 会话事件记录请求空间/槽号/模板、上行 deck、saved_deck、equipped、worn_changed、拒绝原因/码、副本重绘延后标志；实际下行记录 kind/type、ID、长度和明文。

## 本仓真源复核

权威 `client/DFO.exe.i64` 只复制到 `.tmp/knight-idb/DFO.exe.i64` 后使用 IDA headless 只读分析，不直接打开原库；会话已以 save=false 关闭。伪代码与失败分支摘录在根目录 `analysis/knight-shield-evidence/`，已确认函数追加到 `analysis/dumps/idb_funcs.tsv`。

| 函数 | 本轮确认 |
|---|---|
| 0x14505DAB0 | mode=1 构造 CMD19；另一分支开始 CMD649 后仅循环五次 writeDword，无头部 DWORD |
| 0x14524FCA0 | 649 status=0 时不读体、不写表；成功臂先读一个 byte，随后循环五次 readDword 并写表/UI |
| 0x145310A90 | NOTI567 无 status 前缀，五次 readDword 分别写 map/UI |
| 0x145057BB0 | 目标 index>4 拒绝，合法五格 |
| 0x145A0E8C0 | 空间 31/32 无背包容器，落入返回 NULL 分支 |
| 0x145283750 的失败分支 | 码 4/17 选择 DSTR39348；5 不在该函数的本地文案选择分支中 |

**外部文档更正**：其“成功应答 1+5×(byte+dword)=26 字节”与本仓原生控制流冲突。`readByte@14524FD13` 在循环外，按成功 reader 本身连同 dispatcher status 是 `1+1+5×4=22` 字节；其额外 byte 语义未证，本轮不构造成功形。惰性形和 NOTI567 与本仓证据一致。

配置基线 checksum：`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。
当前客户端解壳内层 checksum：`be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0`。
两者的盾牌窗口脚本 SHA-256 同为 `cf9bdccaa0bb958cb197e9e538739961b0be999dafeabf9efa870da67c73fc80`；窗口涉及的 25 个 .equ 在当前运行 PVF 中逐一核对，25/25 与导出表 equ_sha256 相同。因此不更换已有角色 config_version。

首次实现时本地既有 events.jsonl 未发现可固化的 CMD649 原生上行样本。9 月 30 日用户手动操作现已获得原生请求，见下方 attempt 2；静态构造测试与原生请求测试分别标注，外部文档所述 21 条历史帧未移植为本仓事实。

## attempt 2/3：启动路径修复及首次 live 失败

用户截图显示 Shield Safe 已选取，但装备格没有变化。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260930_130807_741338_next37` 的 `gateway.err` 于 13:08:20 记录 `knight shield window disabled: catalog absent`；13:12:03 和 13:12:28 的 32→31/0 请求均以 `knight shield catalog unavailable` 拒绝，saved_deck 仍全零。因此本场尚未执行成功穿戴，不能将截图的 Equipped 或持盾姿态认作服务端接受证据。

根因是网关由启动器从项目根目录启动，而新增 flag 默认 `configs/...` 误依赖模块工作目录。修复只改变配置装载：默认文件名改为 `equipment-knight-shield.full-candidate.json`，相对路径以已加载穿戴规则的目录为基准；绝对路径原样保留，空字符串仍禁用。日志现在写明实际路径及装载行数。新增测试从临时工作目录按启动器的绝对 wear-rule 路径加载，确认 25 行均能读取。未改变包布局或重绘顺序。

本场 checksum_ok=true 的 2 条货架 CMD19 与 4 条 CMD649（Reserved 的 index 2/3/4 及清空）已固化到 `internal/game/protocol/testdata/native_knight_shield_requests_20260930.json`。原生 body 为 CMD19 32 字节、CMD649 24 字节（五 DWORD 加四字节零 padding）；回放测试确认源盾 113370003、空间32→31/0、CMD19 count=0 及 CMD649 从 byte0 读取五格。保留原始帧及日志来源，但本场没有成功穿戴的 S2C 消费证据。

回滚本次 attempt：还原 main.go 的默认路径/装载日志和 knightShieldCatalogPath 调用；仅还原这个启动路径改动，不删除角色存档键。无需更改数据库或客户端。

## 实机确认：货架盾穿戴同步

用户在 2026-09-30 确认“成功”，截图同时显示右侧装备格中的盾牌图标和纸娃娃持盾。最新会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260930_134312_605827_next37` 的启动日志记录从配置目录加载 25 面盾；13:44:31 的 CMD19 从 32/0 移至 31/0，`equipped=113370003`、`worn_changed=true`、拒绝码为 0，随后发出 ID14 穿戴窗口更新。该会话未有 Reserved 调位或重登验收，因此本基线只确认货架盾穿戴、装备格和纸娃娃同步显示。

## 验证结果

- `go test ./...`：通过；未开启的既有集成测试保持原有 skip。
- `go vet ./...`：通过，零输出。
- `CONFIG_SWEEP=1 go test -count=1 -run TestConfigCrossCatalogSweep -v ./internal/catalog`：通过，覆盖盾牌 side-car、职业、部位和全量装备 SHA。
- `KNIGHT_SHIELD_INTEGRATION=1 go test -count=1 -run TestKnightShieldTransactionsIntegration -v ./internal/inventory`：通过。使用并核实独立 PostgreSQL schema，验证保存、同 key 重放不二次应用、任务盾失败无回执/无状态改变，以及真实旧币与礼包迁移后牌组/金币/宠物/解锁字段保留，结束删除该临时 schema。
- 针对性测试：连续两次 Reserved 换盾、卸盾、Reserved 调位、空格/越界/形状/职业门、19 面等级盾真穿戴闸、6 面任务盾拒绝、上行重复、旧档恢复、字节预算、城镇三帧顺序、货架不重绘、副本保护和关闭开关均通过。
- 通用 `go run ./cmd/charactercheck`：**失败**，临时 schema 缺少 `character_quests`（SQLSTATE42P01），与本轮之前 CHANGELOG 记载的既有问题相同；本轮未扩大修改该工具。

## 用户手动验收

使用重新构建的 `bin/wireprobe-handoff-source.exe`（启动器加 `--source-build`）；客户端由用户手动操作。

attempt 2 候选 SHA-256：`C0D12AF09F723F9DAC771A24F30AE4CEEB46C5CB028780648D605BAB980EABDC`（在当前 HEAD `5d81ec0` 工作树上构建）。`go test ./...` 和 `go vet ./...` 再次通过，含启动路径回归及原生上行回放。仅替换源码候选，未覆盖 `wireprobe-dungeon39.exe`。attempt 1 旧候选 SHA-256：`9490710FCF0191BAD54BFB09097B3AE0E8B0B27AD3ADCE132EA4C209DF359629`。

1. 已确认：在城镇用骑士从货架穿盾后，穿戴栏显示盾牌且纸娃娃持盾。
2. 将一面 Reserved 拖进 equipped，核对属性与纸娃娃同时改变；再换第二面，连续操作仍应生效。
3. 将 equipped 拖回 Reserved 空行，核对穿戴槽 24 与纸娃娃清空。
4. 调整 Reserved 行顺序，小退再登，核对五格与穿戴一致。
5. 最后在副本只验证盾牌属性换装及下一房可进入；不把副本即时外观当本候选已经完成。

日志确认 `knight_shield_move` 的 `saved_deck[0]` 与 `equipped` 均为 113370003，且 ID14 更新成功发送。货架操作由客户端立即绘制持盾外观；Reserved 换装后的三帧重绘和 649 保存/ack、567 登录恢复仍按前文实现及离线测试覆盖，尚不属于此次实机验收。

回滚边界：恢复本任务的服务端源文件/候选，或直接运行服务器时指定 `-knight-shield-catalog ""` 禁用 31/32 路由；保留新增 JSON 存档键，不删除存档、不重写角色目录。权威39版归档不覆盖。
