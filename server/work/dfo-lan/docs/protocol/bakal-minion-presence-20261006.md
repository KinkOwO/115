# 巴卡尔小怪入场死亡：区域首领存在符号缺失

日期：2026-10-06。用户要求优先分析小怪入场死亡，没有新的实机日志。本轮使用现有运行日志、官服抓包、当前内层PVF，只借鉴整合交付实现；未整包导入。

## 结论与证据

当前原生脚本链：`list/monster.lst` → `contents/2022/bakalraid/monster/normal/*/*.mob` → `Action/Proc.act` 的 `[SET CUSTOM ACTION] 0` → 出生动作 `Incoming_ready.act` / `Incoming_Dummy.act`。

确认以下四种小怪的出生动作均包含相同的区域判断：109014490 drake、109014491 drake_soldier、109014492 drake_rider、109014525 dragon_soldier。

出生动作首先按 `[MAP INDEX]` 或 `[DUNGEON INDEX]` 选择区域，随后 `[CHECK RAID SYMBOL] <id> [==] 0`，命中时执行 `[DESTROY]`。非命中执行 `[SET VISIBLITY] 1`、`[SET STAND ACTION]`。例如地图100007497检查211，即同源 `.symbol` 的 `[IS EXIST BASILISK]`。

| 符号 | 源名称 | 生效区域 |
|---|---|---|
| 208 | IS EXIST SWAN | DGN100003153 |
| 209 | IS EXIST STEICH | DGN100003155 |
| 210 | IS EXIST ECLAIR | DGN100003159 |
| 211 | IS EXIST BASILISK | 出生动作列出的100007497等地图 |
| 212 | IS EXIST BLONA | DGN100003161 |
| 213 | IS EXIST GERDA | 出生动作列出的100007519等地图 |
| 214 | IS EXIST NYMPHA | DGN100003162 |

当前会话 `20261006_143658_409112_next37`：14:39:15.370地图100007497发送N29；14:39:15.837收到37、14:39:15.915收到2073；14:39:15.981起收到小怪CMD39，其中4096对应109014492，4097对应109014491，4098对应109014490。整场缺少208～214的N570更新。已有428条死亡在加载请求后100ms内出现。这里是出生动作主动销毁，不应直接修改N29未知255字段，也不应过滤CMD39掩盖源条件。

官服 `D:\115us\实时抓包\captures\20261005-015111\frames.jsonl`：N570前9字节为单个符号更新，采集明文后附对齐字节，总长16B。能读到212/214/211/213初始化为1、稍后208/209/210为1；对应首领击杀后归0。N2286初始路线首领更新的type9/10/11、state0、HP10000也吻合。不能因要求明文恰好9B漏掉这些原生向量。

交接成功日志 `20261005_200137_561796_next37` 同样有211～214初始1、208～210初始0，约10秒后后三项变为1。交付源码 `internal/raid/bakal_script.go` 的 `createScriptMonster` / `deleteScriptMonster` 调用 `presence(kind)`；`presence` 从当前创建对象判断同类是否仍存在，并更新 `[IS EXIST <kind>]`。这与当前PVF出生条件及官服动态向量一致。

## 修复范围

1. `BakalOpening` 保存本次源创建对象的位置/类型。存在符号由这些位置与确认击杀状态计算，来源索引仍是同源 `.symbol`，没有另建符号ID、模板或格点常量表。
2. 激活时发布存在符号；每次进图的 `SymbolsSnapshot` 在N29之前恢复同一状态。首领存活为1、确认击杀后为0，换房不会错误复活或重置。
3. `[1PHASE INIT]` 中 `[RESERVE CREATE MONSTER] 10 <kind>` 的10是延迟秒数，不是位置。现有catalog的legacy `ReserveMonsters.Location` 字段保留数据兼容并明确此语义；按该源延迟创建，候选空位来自 `[LOCATION INFO] / [CREATABLE MONSTER]`，均匀选择，发布N2286 state0及对应存在符号。网关加载和击杀消费同一当前创建对象清单，避免仅将符号强制设1却没有实际首领。
4. 原网关错误地用 `RoomCleared` 阻止指定首领的确认击杀投影。现在只要求当前会话已加载、真实注册的源模板rank3实体经CMD39确认死亡，立即更新源首领状态；原生返回的RoomCleared门禁及终局源演员的清场门禁保留。参考实现同样不以普通杂兵全灭作为 `DefeatMonster` 前置条件。

仍未完成：后续反复刷新、移动路径、其它计时波次与完整通用事件执行。只实现源INIT的首批延迟创建，未直接把存在符号全设1，不宣称巴卡尔全部机制已经补全。

## 验证

真实内层PVF专项覆盖：出生ACT确实检查该源符号；存活值1在N29之前发送；留有存活普通怪的情况下确认首领死亡仍发布0；后续快照仍为0；更改源符号索引后输出跟随；更改源预留延迟和CREATABLE候选后按新源执行；同tick不重复创建；预留首领在对应源房间实际生成N2194实体。

Bakal专项通过，全仓vet通过；全量test仍仅两项既有character来源兼容失败（TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference）。实机修复效果待用户手动验证。未启动服务/客户端、修改客户端/PVF资源或玩家库。

最终候选：`bin/wireprobe-bakal-minion-presence-final-candidate.exe`，SHA256 `4bb31c2e6dcb0a0bba04577528e8ff81da6b788241b76483ff5057e2c254ac5c`。默认与隔离profile已指向同一文件，25项Python启动/profile测试通过，check确认实际客户端`D:\115us\DFO`、PostgreSQL路线、内层PVF复用。原候选文件保留，未覆盖正在使用的老程序。用户关闭当前游戏后手动运行`启动游戏.cmd`即可验证；本轮不将自动测试计为实机验收。
