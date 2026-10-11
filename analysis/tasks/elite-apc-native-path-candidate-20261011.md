# APC 原生寻路补救候选 v0.3.12

用户已明确授权 DLL，并将检测范围指定为“原生寻路无法到达就传送”。补救 attempt 2/3；没有修改 C2S/S2C、Go、PVF、SQLite/schema、存档或默认程序。本候选未实机确认，不更新 CHANGELOG / confirmed baseline。

## 上一候选 checkpoint

v0.3.11 / PID17816 / runTick2828687 / `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_011052_626688_next37`：ready、APC catchup ready 均真；同线程16444首次三名精锐登记结果1，首房向量成员匹配1，主人绑定/控制器65535一致。但状态指定的本代次 catchup JSONL不存在。用户报告未跟随；未取得采样，所以不能将800距离阈值判为根因。

原生145B892C0/145B89EC0等调用者直接读取actor+368并调用145E08940，绕过145B8C7D0包装。本候选hook共同位置组件入口；这是静态覆盖范围改善，不能用它反推旧实机缺采样的唯一原因。初始化立即写started，前16次入口与身份/线程/玩家/场景/同伴拒绝均有记录，便于下一次区分入口与门禁。

## 当前115原生契约

- `[MOVE TO ASTAR TARGET]` at1494C1248注册链140CC29B0→140CF13B0；移动动作调用145DCE4D0，后者读取AI目标与16×12格子坐标、原生导航/通行getter，调用146D64950。
- 146D64950调用146D64820搜索，随后清搜索缓存。它返回0时不区分无路与上限；145DCE4D0自身返回输出格子坐标，也不能用作boolean可达性。
- 146D64820八参数：arena、起点X/Y、目标X/Y、导航类型、通行byte、actor。500次展开上限；null且open list(+112)为空表示候选已耗尽，null且open仍非空为预算未知；非null须核对节点+12/+16终点，非目标cost-bound不算可达或无路。
- 146D64330按当前地图tile/actor参数判断单格通过；不是路径完整性检测。当前地图宽/高在arena+114924/+114928，tile指针+114936。
- 146D642C0节点池从arena+8424开始、104B×1024，计数+114920；达到1024会原生回绕。本DLL仅对自己的查询工作区在第1025次分配前中止为未知，其它原生分配透传。
- 三名实机虚表14A59DDC0/14AD21370/1495AD718的+2632为145D714E0（actor+3856），+2896为145D70740（actor+2304）；原生145D732A0取后一getter>0。直接复用客户端已加载的参数，不新增tile/PVF规则表。标签`[astar target pass]`/`[astar monster]`可作为内容解析线索，尚未将其逐一绑定到这两个运行字段，不冒充已确认源映射。
- 145E08940参数是位置组件RCX与坐标轴EDX（反编译遗漏了EDX）；保留64位RAX、轴、LastError与原生异常。所有原生函数/loop上限先核对当前EXE字节；静态索引已补idb_funcs.tsv，未修改/保存权威IDB。

## 候选行为与边界

每500ms、只在冻结的主人原线程查询当前存活落地精锐到玩家有效格子。独立114944B工作区只借用当前只读tile指针/宽高，不改全局A*缓存、APC AI目标或路径缓存。原生确认无路持续3秒才调用145B960D0原生立即位置setter；落点通过145C70EE0原生检查，主人左右64单位不合法则尝试主人点。可达/未知、身份或场景/槽位/loader代次变化、采样间断均重计时。

地图未就绪、目标格暂时阻挡、坐标不在当前地图、未知getter、搜索/节点池上限均跳过，日志pathDetail解释。精锐必须在当前场景向量中恰好登记一次；缺登记记录not-current-scene，重复/不稳定登记拒绝。跨场景离场/重挂契约未闭环，不能以位置setter冒充迁移。剧情演员保护、全部剧情/特殊副本均不据离线测试认证。

日志解析DLL自身目录；`adventure-elite-catchup-<PID>-<runTick>.jsonl` schema2包含started、入口/拒绝、path-reachable/unreachable/unknown、pathDetail/grid、teleport前后坐标。1024条诊断预算不影响补救；采集传送观察仍保持apcCatchupVerified=false。

## 实际验证与产物

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-adventure-elite.ps1`退出0，MSVC x64 /MT /W4 /WX构建，全14组机制测试通过。startup夹具直接执行当前EXE复制的原生A*，可达、完整墙隔无路、tile1通行标志改变、500次上限未知、同格、独立节点池限制与游戏分配透传通过；组件hook轴与完整64位返回/LastError/SEH、3秒无路传送、可达/未知拒绝、所有字节门禁与旧归属/登记保护通过。

`scripts/test-adventure-elite-capture.ps1`退出0：原78种及后续全部兼容夹具、v0.3.11旧schema、v0.3.12/v2与错schema拒绝通过。`scripts/collect-adventure-elite.ps1 -CheckOnly`退出0，旧状态v0.3.11不认证新候选运行。只改C++，本轮未执行Go build/vet/test。

- DLL：`client-patchs/adventure-elite/dist/AdventureElite.dll`，238592B，SHA256 `7412986a47bf473fdfb7ff321fdfe241c965c936668be482cd251597c098b492`。
- schema2包：`dist/adventure-elite-eligibility-0.3.12.zip`，144621B。现有Job注入加载dist DLL；未安装mod包、未启动游戏。
- 回退：`.tmp/adventure-elite/native-path-backup`保留v0.3.11 DLL/清单；更早v0.3.10备份仍在`.tmp/adventure-elite/apc-catchup-backup`。
- modkit工具未找到；verify/plan/install/uninstall未执行，不能用打包和机制测试代替安装卸载闭环。

下一次由用户手动退出旧客户端并运行`scripts\启动游戏-SQLite.cmd --source-build`；保留名单，只做一次艾尔文换层，等3～5秒观察。成功或失败均保留日志，不要求反复跑图。用户确认前不提交候选/二进制，不扩大confirmed baseline。
