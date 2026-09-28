# White Land / Evil Justice 下方房间拒绝，2026-09-29

## 实机证据与源资源

- 任务 12920，副本 100002746，maze 0。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_035716_603877_next37` 已进入 100004522、100004524、100004525。
- 用户提供的 `2026-09-28T20:08:33.7353516Z` CMD45，目标 `(2,1)`，当前 `(2,0)` 地图 100004525；服务端拒绝 `unsupported random monster placement or invalid source row`。同一目标此前也多次拒绝。
- 目标地图 100004527，源路径 `contents/2022/110levelscenario/sanctusbellum/dungeon/thelandofwhite/map/100004527.map`。前三行均为 `[fixed] [normal]`，第四行 template 109013403、相对等级 `1,0`、坐标 `(982,287)`、数量尾部 `1,1`，仅带 `[fixed]`，无显式阶级。
- 只读解包当前 `client/Script.pvf` 并直接读取该 MAP：原始 MAP SHA-256 `f8dd5ef4c2e5b97f36806167a18a1feba46e3cf92fdd824fc3de1c81e8388e2a`，与 `dungeons.full.json` 的源记录一致。当前外层 PVF SHA-256 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`。未改资源。

## 当前客户端原生默认值复核

既有 IDB 索引的函数为 `sub_1471C18C0`（怪物行构造）、`sub_1471E9060`（怪物行解析）。本轮 IDA MCP 8745 未连接，因此未声称重新反编译 IDB；直接只读反汇编当前 `client/DFO.exe` 中相同地址，并用当前 XORSTR dump 确认原生字符串地址。EXE SHA-256 `1d3948784e5e0f77ed744017bf82bf0c50de9421423f9e59f6f70aed609d8ffa`。

- `1471C18DC xor ecx,ecx`，`1471C18FC mov qword ptr [rdi+20h],rcx`：同时将 +20h 的放置选项与 +24h 的阶级置零。
- `1471E90A2 call 1471C18C0`：每个源怪物行先构造默认数据。
- 原生 rank 字符串：`1491B1F60 [normal]`、`14B22DA20 [champion]`、`14B22DA40 [super champion]`、`1491B1AF0 [boss]`。
- `1471E92CB mov dword ptr [rbx+24h],ebp`（ebp=0）、`1471E92F2 ...=1`、`1471E9336 ...=2`、`1471E9368 ...=3`。
- 最后匹配失败的 `1471E9366 je 1471E936F` 直接跳过 rank 写入，保留构造默认 0。缺阶级标签因此不是随机放置或无效源行。

同时只读核对 `server/launcher.local.json` 指向的 `F:/wip/dof/115US`：其 PVF 哈希与上面完全相同。该目录 EXE 整文件 SHA-256 为 `5543c382287bfd5354c3d8c32fd0ce2f1bac572adcc57563c29e6091e332e1cb`；构造函数起始 0x120 字节和解析函数起始 0x390 字节与权威 `client/DFO.exe` 逐字节一致，本轮所依赖的默认阶级逻辑一致。

## attempt 1/3：服务端修复，实机已确认可进入下方房间

`fixedMonsters` 不再要求显式 `rankSeen`；省略阶级时保持 rank 0。`rankSeen` 继续拒绝重复阶级；固定放置、模板、数量尾部、等级、未知选项与队伍校验均保留。无 MAP 或任务 ID 专用放行，不改协议布局、DLL、客户端、数据库和存档。

回滚范围：`internal/dungeon/session.go` 的该条件及新增 `white_land_room_test.go`；没有其他运行路径尝试。

回归：改前重放用户 CMD45 到 `(2,1)` 复现相同拒绝；改后进入 100004527，保持 4 只怪物及第四行 SourceIndex=3、template=109013403、rank=0、level=100、team=100。7 张源路线 MAP 全部解析和 StartMap 编码通过；另外覆盖缺阶级行后紧跟下一行、未知选项和非固定放置拒绝。

全量 `go test ./...`、`go vet ./...`、候选构建与 `git diff --check` 通过。新增校验用例还覆盖重复显式阶级拒绝。候选 `bin/wireprobe-handoff-source.exe` SHA-256 `99A3F17BA083754B0270E0AA6883CB6CCD85BBA2E34C1F9F8002DD3F5AEDDBFB`；原39归档未覆盖。

用户在该候选版上手动实机确认可进入下方房间。本次实机确认边界仅覆盖进入该房间；战斗、剧情、Boss 和任务完成仍需分别验证。基线已更新，本次提交包含代码、针对性回归、取证记录与交接文档。
