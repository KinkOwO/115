# 巴卡尔进战斗后的双向协议与当前源码审查

审查对象：用户修改后默认`bin/wireprobe-bakal-hall-create-candidate.exe`、最新会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261006_104333_506060_next37`、官服抓包`D:\115us\实时抓包\captures\20261005-015111\frames.jsonl`。本轮仅取证、运行现有专项与审查，未替换程序、未改玩家库或客户端，未覆盖用户源码修改。

## 已确认修正

成员记录应使用当前area，不能使用town152。用户将MemberTown改回MemberArea有官服area6与交接area2向量支持，本轮10:45:20已经收到2089，随后2062进入地下城100003160、位置22，37/2073完成加载。此前我把客户端比较的脚本值解释成等待城镇是错误结论，不能继续据旧文字回滚用户修正。

大厅创建→营地编队→开战已有实机证据。原始ETC待机区152/2与世界大厅152/1不同职责，不等于只能在camp1建团。

## P1：死亡确认被经验字段解析失败整体丢弃

10:45:39.158–.192收到8次CMD39，entity4096–4103、killer7；8次均记录`dungeon_request_refused: ambiguous dungeon experience weight`。没有对应的`monster_death_confirmed`。日志中35项id38是`door_ack`命令应答，不能误计为怪物死亡通知。

官服对首批CMD39在01:55:51.495起发送Kind0/N38，例如entity2070的有效前缀`160800000000ff00`。当前死亡处理先`ConfirmDeath`标记实体，再构造N38，之后`w.progression.Monster`失败返回`nil,error`，网关因而不发送已构造的死亡确认。尸体不消失不是客户端没发死亡请求。

源读取证：`contents/2022/bakalraid/dungeon/100003160_kingsroad/100003160_kingsroad.dgn`原始token33有`[experience increasing point] 0`，token118又有同标签float0.83（迷宫作用域）。`internal/character/growth_rules.go`的growthSection跨整个Script.Cells收集标签值，把不同作用域混成两项，触发len!=1错误。此前DGN等级读取已经区分头/迷宫，但经验读取仍未区分。

建议：按源作用域解析经验值，并保证可重试的奖励/经验失败不吞掉实体死亡确认；核对官服巴卡尔死亡包及发奖边界，不能随意删经验校验或加零经验常量掩盖解析错误。实体先标死后经验失败会让后续重发不再获得confirmed=true，也要验证幂等和重试，不仅检查尸体消失。

现有`TestBakalNativeDirectMoveAndOwnedDeath`的worldSession没有配置progression/实际运行的quest/loot服务，其“死亡通过”并未覆盖完整运行路径。

## P1：N2286地图怪物数据与官服状态/关联字段不一致

当前初始化发8行，例：巴卡尔type1/location24/state1/HP10000/tailFFFFFF。官方01:55:41同位置行有效19B为`01000000180000000000000010270000041919`：type1/location24/state0/HP10000/tail04,19,19。初始化另外几行也为state0，有实际尾部索引；清除帧为type0/state1且常见tail19,19,19。末尾其余抓包字节是填充/未定义尾数据，不复制其随机值。

当前`internal/legion/bakal_script.go:BakalOpeningRoster`统一Count1，`internal/legion/bakal.go:BakalMonstersFrame`统一写FFFFFF。字段不是“数量＋三个无意义尾字节”：

- 客户端144CD5BB4注册N2286→144CDE400，按19B读取每行→142543160。
- row+8==2是object refresh分支；其它分支持有状态与怪物类型，row+8==1另更新历史类型。
- row+16/17/18分别以有符号byte读入地图位置记录的三个独立字段（142543344..361）。因此FFFFFF实际写入三个-1，不能认为无消费者。

这解释了“服务端已经发了2286”仍不能证明地图资料完整。需要按PVF怪物/关联规则与客户端绘制消费链补齐状态和尾字段，再加入官服向量。仅把FFFFFF换成某一张图的04/19/19是新的硬编码，不能作为通用修复。

定时场景怪波次仍未执行，官服在开战+5秒后新增type13等移动怪，而当前仅初始8行与+1秒重复/四首领HP行。即使初始图标修复，完整地图的动态信息仍存在这项已记录缺口。

## P1：开战仍用固定角色成员帧覆盖真实成员信息

10:45:12当前661通知成员actor7、profession5、advancement5、area2；10:45:20开战`bakal_real_member_assigned`却为actor2、profession0、advancement0x31、area2、固定teamID0x01520001。名字替换成001不使其它身份字段变成角色7。

来源是`BakalOpening.Start`→`BakalMemberAssignedFrame(name)`。该构造器保留通关样本actor2与固定职业/频道等值，与此前已修正为实际角色的RaidRecruitment不一致。用户这次能开战不能证明后续队员管理仍使用正确身份。建议复用已核对的RaidAssignmentUpdate和会话持有的成员资料，避免另维护一份固定N578。

## P2：用户大厅门禁改动仍硬编码area1

`internal/catalog/bakal_waiting_room.go:OwnsWaitingRoom`以`area==1`容许大厅。当前实机行为正确，但违反PVF单一规则的目标：源`town/bakal_raid.twn`的party-list大厅、等待NPC与ETC待机区都已有绑定依据。应分别保存源发现的创建大厅与ETC开战等待区，而不是抹掉二者差异或常量area1。此项不阻塞这次尸体问题，也不能通过再次收紧到area2来“修复”。

## 验证与输出

真实PVF专项：wireprobe/legion/protocol/catalog的Bakal/RaidEntrance现有测试均通过；这4个包go vet通过。通过不等于当前实机症状消失，上述测试覆盖缺口已明确列出。本轮没有为了审查生成新默认服务器，也没有自动运行客户端。

只读取证输出保存在模块`.tmp/bakal-ui-20261006/`：`inspect_growth.go`报告源两处经验字段；`144CDE400.asm`/`142543160.asm`为实际客户端N2286消费链；`monster-info-objdump.txt`为交接构造器；`user-review-tests.txt`为专项结果。建议后续先闭合死亡确认，再修复地图数据与实际成员身份。

## 用户授权后的战斗修复候选

- `growthDungeonSection`限定DGN文件头作用域；怪物经验与通关经验共用该读取边界。实际100003160的header0与maze0.83不再合并成两个字段。保留同一作用域的歧义校验，未用“巴卡尔固定0经验”替代源值。源0改为2的小输入证明结果随源改变。
- `BakalOpeningRoster`使用官服N2286的state0放置，+1秒object refresh仍为state2，死亡状态1保留。已读取的怪物类型/位置/HP仍从同源阶段规则形成，没有抄官服随机移动怪表。
- 网关开战在转换阶段前验证实际RaidRecruitment，替换旧actor2的N578样本，发送当前角色/职业/编队/所在area的mode3成员通知。保留用户已验收的MemberArea和大厅建团逻辑。
- 真实源100003160回归先调用实际GrowthMonsterGain确认无歧义及header0结果，再验证各原生怪物的CMD39死亡计划包含N38；这是源经验解析＋死亡计划回归，不冒充玩家库完整经验事务验收。开战回归校验N578 actor等于会话角色。

地图初始怪物图标仍需实机验证。三字节Buff关联没有完整来源选择/授予执行链，本轮继续使用已有无激活选择，未把某次官服随机选择硬编码进运行；完整Buff预览/授予与定时移动怪仍为显式缺口。本轮不宣称完整地图所有机制已完成。通用死亡链其它独立奖励事务失败的幂等处理仍需另审，当前日志中的具体经验歧义已经修复。

候选`bin/wireprobe-bakal-combat-candidate.exe` SHA256 `7b53402198445708bfaa6c7a0569d173a7b04bbd0a49445ec9e0f9c736f8907e`，默认与隔离profile切换至该文件；旧hall-create程序保留。客户端及玩家存档未修改，用户直接运行根启动游戏.cmd手动验收。

验证完成：真实源专项、全仓vet与默认启动/profile25项测试、实际launch_local.py --check通过。全仓go test仍仅两项既有character失败：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference。输出combat-fix-tests.txt与combat-fix-vet.txt保存在本轮取证目录。
