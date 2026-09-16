# 35轮调查状态（未交付合并修复）

## 2026-09-11 本轮更新（以下优先于后面的早期调查记录）

本轮共 **7 项**：原六项加上“新角色先进入本职业初始地图和任务，完成后再入城”。不能宣布全部修复。用户希望合并测试，避免反复让其逐项操作。

### 已实现及证据

- **弓箭手基础技能页已定位到实际 UI 配置缺口，并已修补隔离客户端资源。** 用户配合让角色6666进城后，`runtime/archer-entered35.json`证实profession16、advancement0、level4，技能3/7等在两个技能树中已经实例化，排除了服务端漏发/原生技能工厂无对象。`runtime/skill-index-live35.json`证实218个弓箭手源索引，3=Archer/CrackArrow.skl，7=Archer/RisingMoon.skl。
- 原`clientonly/skilltree/archer_sp.co`的`[archer] none`只有169/170/174/175/186/190/452七个通用格；Slayer的none段有本职业技能，吻合用户截图。`archer_layout35.py`只在Archer none段补3、7、511：原技能脚本明确允许grow0且该阶段最大等级大于0，使用原有布局中无冲突的位置。原转职布局、技能等级、费用、属性均未修改；技能7仍需10级才能学习，不会让4级角色使用。
- 同时修复`LearningDefinition.ForAdvancement`：必须同时满足fitness和grow阶段上限；501虽然fitness含0，但grow0最大等级为0，不再出现在未转职可学投影。
- PVF修改工具`internal/catalog/pvf/rewrite.go`+`cmd/pvfpatch`只写新文件，只替换原路径脚本记录；在所属解压块末尾追加新脚本，保留同块其他资源原字节/原偏移。回归验证覆盖同块邻居、其他压缩块、源对象不变、拒绝覆盖既有文件。解包回读确认Archer原8900→9005字节，Slayer SHA保持不变。
- 新资源包装位于`runtime/archer-layout-patch35/Script.pvf`；内层SHA `8186c5a5f2b0b86be2a19fc9be34a08ba4491adaf11fb1e8353ec2e05f437071`，包装SHA `ba2c94feaafdb2d23435672d9c51c66d57cfe9537937921cdcf280c4ddb6353b`。`rewrap_pvf35.py`在内存使用原包装材料，验证全部73段AES包装回读一致；密钥不输出、不写入清单。
- 已将**隔离副本**`work/dfo_probe_client/Script.pvf`切换为该文件；旧文件先移动保存到`runtime/client-pvf-backups/Script.before35.pvf`。原始`F:/dnfop/DFO/Script.pvf`仍为`2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167`，未更改。服务端规则来源仍是原始inner PVF，新资源仅修改UI布局。
- **反复推荐指引**：当前真实协议是NOTI2826账户选项、2827角色选项，旧173不读取此表。实现`AccountOptions`采用原生构造器1475757f0的3648字节模板，稀疏覆盖账户选项198=1。原生147578720合并及1403d4720读取模拟验证其余285项默认不变。`entryPayloads`在SELECT后、入场前发2826，配置`configs/account-options.current35.json`可修改；未实现一般CMD2377设置保存。
- 已在用户6666真实入城后读取确认：source_defaults_ready=1、account_received=1、account/merged选项198均为1且set标志为1。自动推荐入口1433e656a读取该项，非0跳过自动打开3576。该设置实际到达客户端；并非伪造接取任务。
- **装备穿戴**：当前C1/19业务已接线，支持普通背包装备与身上槽位间穿脱/交换，验证归属、源实例、职业、等级、目标槽位及基础装备种类；事务提交和事件去重先于ACK，按当前原生布局发送ACK及NOTI14变更槽位，重进用NOTI13恢复身上装备。`configs/equipment.current35.json`补全原PVF穿戴条件，共1536基础装备；不是全品级/强化等复杂装备实现。实际属性增益/外观仍需客户端回归确认，不可宣称完整装备系统完成。
- 全部`go test ./...`、`go vet ./...`及wireprobe编译通过。`go run ./cmd/charactercheck`全套临时schema回归通过，含WEAR_STORAGE_PASS（并发一次、归属、穿脱、重连、其他模块不变）。角色库未重置。

### 新手出生流程与尚未完成项

- 新增`catalog.TutorialCatalog`及`cmd/tutorialimport`，从`etc/tutorial/tutorialflow.etc`读出16条路线，其中15个普通职业教程、65张地图，保留片头在教程前/后的区别、返城坐标和显式首任务。普通枪手不会误用活动专属grow5/任务12486。普通鬼剑士7115；弓箭手100003327（必须uint32）。导出`runtime/tutorial-source35/`与`configs/tutorial-routes.current35.json`，对应source/parser测试已过。
- **这只是源数据及路由解析完成，尚未把新建角色接入完整教程状态机**。当前`dungeonGate`仍拒绝非0直接教程请求，`dungeon.Select`拒绝Tutorial。需继续验证当前客户端首入场、CMD15→16、地图脚本触发、教程完成→首任务/返城的顺序。不得套用90US参考“发入场包即标记教程完成”的策略，不得重置现有角色进度。
- 怪物HP的0.0143倍率来源尚未修正；金币头顶显示尚未验收；任务3147等受真实等级门槛限制，当前等级6下一任务7级，经验模型/任务类型扩展仍待做。
- **修正早期只读观察器错误**：145f13700/145f12d90的接收者是`global14e683c20`的PartyManager，**不是**`14e683c08`的UserManager。原`archer-entered35.json`里最早的party_slots杂值不能用作party缺失证据；观察器现已改到正确对象。待有角色/副本时重采。`SoloPartyInfo`99字节原生读包原型仍受开关控制，**本轮没有启用 -solo-party-bootstrap**，不能称金币修好。

### 当前启动与窗口

- 默认`Start-DFO.cmd`→`scripts/launch_local.py`已指向profile35。`channel_probe.py`的candidate35继承已真实成功的34频道配置（channel.local34.json、game-listen127.0.0.2），使用新二进制、穿戴目录及账户选项；不启用试验party包。
- `bin/wireprobe-dungeon35.exe` SHA `e9ffa2d23f236df4c331929b45695cea625994e0753c23e49b6ce92d8f728813`。
- 旧35采集：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260911_091038_812_next35`，用户确认6666进城。PID18612已在采集后按原路径核验并停止，不可再用。
- **新资源35窗口**：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260911_094218_958_next35`；启动时server2908/probe33596/client9044，game49212。操作前重查PID。已启动、收到登录阶段请求，弓箭手新UI尚无用户截图验收。
- Sky仍两次报`SetIsBorderRequired failed ...0x80004002`，不能自行截图或猜坐标代点。用户本轮已协助一次进城采集；后续应尽量合并测试。

---

## 早期调查记录（下列“尚未实现”状态可能已被上方更新替代）

当前已实际通过的基础是34：频道→登录→角色列表→LanTest01进城/移动。运行目录为`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260911_062445_980009_next34`，底层二进制仍为wireprobe-dungeon33.exe，启动配置channel.local34.json。

本轮待完成六项：推荐窗口反复自动弹出、怪物HP过低、金币头顶数值不显示、装备穿戴未处理、后续任务可见/衔接、弓箭手技能显示。尚未生成35运行版，不应称这些已经修复。

## 明确证据

- 原客户端窗口3576（0xdf8）从11:25:58至11:26:39反复Close/Open，期间服务端没有重复推送任务列表，也没有收到关闭偏好请求；接任务3146后停止。当前自动入口`1433e62b0`中`1433e6562`查询游戏选项198（0xc6），值0且在城镇时持续打开3576。`guide_loop35.asm`保存此函数、选项getter1403f5b80及NOTI21处理器。游戏选项未初始化时getter返回0；不能把关闭指南伪装成接任务/完成任务。
- 当前NOTI173注册到1452d1260，但它只调用选项应用函数，未发现旧90版“79个u16”的直接reader。1403f5b80使用profile+60，要求对象+6419已初始化；1403d4720返回配置项指针。`options_*35.asm`保存调查。不能直接移植90版00AD零表，尤其不能破坏场景状态。
- 怪物只读观察器已取得实际描述块。`evaluate_monster_stats35.py`用原147220e40/147221040/147221db0计算：普通怪未缩放HP652/733/817，最终有效HP9/10/11，描述块+30倍率约0.01430000085。此程序只替代146e922e0的保护数值写入，省去它的TLS记录器，保留原计算及原快照数值。24个采样最终范围9..3527。结果在该运行目录`monster-hp-evaluated35.json`。
- HP倍率写入函数`145c08420`：从actor+6808复制倍率到描述块+30；actor+6810经145c1f3b0读取某种限制代码。调用者145c26fc0和142a1f0d0；145d724f0是条件判断，pdata是分段的，dump_containing只能得到前一小段，需要按指令边界继续。`hp_revision35.asm`、`hp_cap_rate_refs35.txt`、`hp_multiplier_writer_refs35.txt`保留。不可凭猜测把HP乘100。
- 金币实际已入账，NOTI39已发送party0 flag1/amount/extraCount0，但用户仍看不到头顶提示。可能缺少单人PARTY_INFO初始化：当前从未发送NOTI9，旧90US参考明确要求入场前建立slot0。尚未证明根因，不能直接当定论。
- 当前NOTI9入口1452dc3d0调用1452f2620，完整解析见`party_parse35.asm`（6797行）。头部读取u16 blockCount、u16 topField；每个块读u16 partyId后6个u8，明显不同于90版。必须重建当前布局。旧参考在`work/90us_reference/source/server_proto/game/dungeon/helper-party_info.py`。
- 装备点击于11:28:24产生C1/19，共45字节；旧观察白名单不保存19正文，因此该次没有raw。还没有实际穿戴处理器。当前writer145af7135..145af71eb写29字节，32字节密文填充，无4字节摘要（否则至少48字节）；字段为sourceList:u8,sourceSlot:u16,sourceInstance:u32,count:u32,destList:u8,destSlot:u16,destInstance:u32,extra:u32,selection:u32,flags:3*u8。`item_move_senders35.asm`为原指令。
- 新增未接线协议文件`internal/game/protocol/item_move.go`及测试：DecodeItemMove、ItemMoveSuccess、ItemMoveRefused。成功ACK在success外部flag后读11字节（旧90版少最后1字节）；拒绝在u16错误后读7字节（源list、目标list、u32、u8）。`item_move_oracle35.py`已验证两个cursor，Go协议测试通过。业务移动、归属/等级/槽位检查、穿脱持久化及重连恢复尚未实现。
- 任务3146已实际完成，LanTest01持久等级6；当前PVF下一条3147最低7级，其后3148=8、3149=10、3150=11、3151=12。定义已导入，并非3147不存在。应核查经验模型、未达等级的显示/引导及更多任务类型；不要改完成标记或任意放开等级。现在InitialProgress仅支持单图clear map及单NPC meet npc。
- 弓箭手6666是DB id3、profession16、level4、advancement0，持久初始技能与当前.chr一致：179/7、174/1、169/1、3/1、190/1、452/1，未购买技能、无手动槽位。当前目录有218个archer定义；base-grow候选约30个。7=risingmoon最低10级；3=crackarrow最低1级。用户截图仍显示不正确，尚未界面验收。已询问具体是基础射击缺失、期望转职技能，还是槽位/加点问题。不要给全职业或已转职技能冒充修复。

## 工具状态

- 19032为该次客户端PID，但操作前重查；用户可能换角色/退出。
- Sky截图仍报SetIsBorderRequired 0x80004002；可访问树只有窗格。不可用shell抓屏替代。过去Space快速按键没有有效验证。
- Go: E:/codex/2026-08-10/gwjatkre8cunepb/work/tools/go1.26.5/go/bin/go.exe。
- Python: C:/Users/Administrator/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe。
- 存储local.json含密码，读取只在脚本内部使用，禁止打印；现有角色库不清空。
- 多个dump工具会覆盖startup_mode.asm/latest_xrefs.json，要给关键输出独立文件。dump_range起始在指令中间会停止，不能把空输出当无代码。
