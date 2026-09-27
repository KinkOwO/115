# 城镇门户零落点：Pandemonium Junction（35/2）确认基线

日期：2026-09-26。状态：35/0 → 35/2 窄范围候选经用户手动实机确认可进入；随后按同一 IDB 默认落点分支推广到符合条件的地图，通用范围尚未逐图实机验证。C2S attempt 1/3。

## 用户提供的实机证据

- 截图显示地图上的 Pandemonium Junction 图标；截图只证明用户选择的位置，不证明实际场景落点。
- CMD36 明文 `230000000200000000000000052300000000000000000000`，按当前 `DecodeAreaChangeRequest` 解码为目标 `35/2`、坐标 `(0,0)`、Flag `5`、来源 `35/0`、TailFlags `[0,0]`。
- 服务端记录 `area_refused town=35 area=2 reason="position outside source walkable rectangles"`。按 `world.Service.transition` 顺序，该拒绝发生在目标区域查找、等级和出边检查之后的 `ValidatePosition` 阶段。

## 当前客户端导出资源与服务端路径

- `configs/world.generated.json` 中 `35/0` 的 `[town movable area]` 有边界 `[883,364,100,50]`、目标 `35/2` 的门户行。
- `35/2` 使用 `map/town/centralpark/dvildom_c_waiting_down.map`，`[virtual movable area]` 为 `[13,180,1100,250]`。当前服务端允许 128 像素边距，`(0,0)` 仍不在范围内（Y 下界为 52）。
- 请求的 TailFlags 为 `[0,0]`，因此不走 `teleportTransition` 的地图传送分支；即使走该分支，也会在相同的目标落点校验处被拒绝。
- 资源包含区域、可行走矩形和入口边。`[pvp start area]` 不是普通城镇传送落点；落点规则以下述 IDB 链为准。

## 实机复现

用户已于 07:20 UTC 手动走到蓝色 GO 入口，提交前后截图和第二条 CMD36。`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_151815_689448_next37/events.jsonl` 的第 177 行 CMD35 报位置 `(903,353)`，紧接第 179 行 CMD36 仍为 `35/0 -> 35/2`、`(0,0)`、Flag 5、TailFlags `[0,0]`；第 180 行以相同原因拒绝。此时源门户 `[883,364,100,50]` 与玩家位置相距 11 像素，符合现有门户边距。扫描当日保存的会话日志共有四条零落点 CMD36，全部指向 `35/2`（07:11、07:13 两条、07:20）。

当前客户端 `client/Script.pvf` SHA-256 为 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`，只读解包后的内层 SHA-256 为 `be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0`。从这一当前内层 PVF 读取的 `map/town/centralpark/dvildom_c_waiting_down.map` 原始 SHA-256 为 `7f7d74e89bf43519f203631467018a4ea6e050640b7e3d2a4f0fdbc432dc90c1`，与服务端 `world.generated.json` 的该图 SHA-256 完全一致；因此这次拒绝不能归因于该地图的旧版几何配置。

最初对当前 `client/DFO.exe` 的只读静态反汇编确认：`0x146D0758A..0x146D075E5` 把调用参数中的 Town/Area/X/Y 原样写入 CMD36，`0x146D075EA..0x146D07671` 再写 Flag、来源和两个 TailFlags。随后在已存在的权威 IDB 会话中作只读查询，见下节；本任务未保存或改写 IDB。

## IDB 静态链与确认候选（attempt 1/3）

- 权威 IDB 中 `sub_146D07390` 的 `0x146D0758A..0x146D07676` 将调用参数 Town/Area/X/Y/Flag/TailFlags 原样写入 CMD36；该函数本身没有修正零坐标。
- `sub_146CF32E0` 的 `0x146CF450D..0x146CF4582` 将输出 X/Y 初始化为零，然后调用 `sub_146D02440` 查找目标地图对象；只有非空时才调用 `sub_144D26750` 写入落点，并将结果存入后续发送对象 `+0x108/+0x10C`。`sub_146D0CE30` 从这两个偏移读取坐标，并在 `0x146D0CEA2` 调用 CMD36 发送函数。该调用链能产生实测的零坐标形态；本次动态命中哪一个 CMD36 调用点尚未单独证明。
- `sub_144D26750` 的 `0x144D26755..0x144D267EF` 明确规定原生落点：优先查匹配来源 Town/Area 的入口矩形，取其中心并加该入口的偏移；若无匹配，则取首个默认矩形中心。`35/2` 的 `[town movable area]` 只有到 `-1/-1`、`54/1`、`35/1` 的行，没有到 `35/0` 的行；其首个 `[virtual movable area]` 是 `[13,180,1100,250]`，中心为 `(563,305)`。当前配置 `Walkable[0]` 来自这个 PVF 段。
- 权威 IDB 中上述发送点、落点函数和角色放置函数各抽取一段机器码，与当前 `client/DFO.exe` 对应地址逐字节一致，排除了本次取证使用的 IDB 所关联旧输入路径导致代码不一致的风险。查询只读；未保存或修改权威 IDB。
- ACK36 成功分支 `sub_145296B40` 不读取坐标。NOTI23 的自角色分支 `sub_145311AE0` 读取 X/Y，并在已加载角色对象时调用 `sub_145BECAD0`；NOTI24 的 `sub_1452FC5B0` 也通过该函数放置角色。`sub_145BECAD0` 首先通过虚函数验证坐标，失败时调用另一虚函数修正，然后设置角色位置。因此服务端回送正确落点仍须经过实机确认，不能仅凭静态链宣布切图成功。

第一版候选只在 **来源 35/0、目标 35/2、CMD36 坐标 (0,0)、Flag 5、TailFlags [0,0]** 且现有门户授权与等级门槛已通过时，把落点替换为当前地图首个 `[virtual movable area]` 的中心。用户手动实机确认此版可以进入。确认仅覆盖这条路线，不推断其它地图已经实机通过。

主线通用版移除 35/0、35/2 常量：请求须为零坐标、Flag 5、TailFlags [0,0]，并命中来源地图的明确门户且通过原有接近判定、等级与区域校验；目标地图没有指回来源 Town/Area 的入口时，取目标首个 `[virtual movable area]` 的中心。若目标有匹配入口，客户端使用入口中心加有符号偏移，当前导出目录没有完整偏移字段，故本规则不替它生成落点。动态门户宽松授权、赛丽亚回图及其它请求形态不触发此默认落点。没有修改数据库结构、存档或客户端。测试覆盖实测明文、另一组区域 ID、匹配入口排除、动态宽松授权排除、等级及位置拒绝。

窄范围候选验证：`go test ./...`、`go vet ./...` 在独立 Go 缓存下通过；`launch_local.py --check --repair-profile server/work/dfo-lan/runtime/pandemonium35-profile.json` 通过。用户手动运行的独立候选程序为 `bin/wireprobe-pandemonium35-candidate.exe`，SHA-256 `3b678f1cae35ed4f03f9dcc019618996ebbd8a685ddb004d5078e674a9bbb3c4`。原 39 版未覆盖。

主线通用版 `go test ./...`、`go vet ./...` 通过，`bin/wireprobe-handoff-source.exe` 已构建，SHA-256 `D3D289FB13587090FF709663FFEECD29575E9924B4C170078BD2B4CC3CE1A6D8`。此哈希仅标识构建文件；通用范围尚未逐图实机验收。
