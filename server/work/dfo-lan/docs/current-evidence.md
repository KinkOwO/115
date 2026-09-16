# 当前构建与第一阶段证据

更新：2026-09-11 00:29 UTC。**第一目标已完成：LanTest01 已由真实客户端创建、持久保存、在重开后的列表显示，并进入城镇。** 以下早期排查记录保留历史状态，以本节最新验收为准。

## 最新状态：真实客户端城镇入场通过

用户确认：“直接进入城镇了  跳过了新手教程”。运行 roles_persist_select_actor_town_08 在 00:23:18 UTC 收到 LanTest01 的 SELECT，依次发送 SELECT 成功、USERINFO mode0 和 NOTI24 AREA_USERS。00:24:36 UTC 的只读 town_acceptance.json 确认 actor/self ID 均为3，场景角色和虚拟角色存在，地图 town38/area0，位置(561,234)，职业0、等级1。用户视觉反馈与原生地图状态一致。

当前使用独立配置的直接进城路径，因此跳过新手教程。教程、任务链、完整属性/技能/背包、切区和多人同步尚未完成。后续35/36等请求只看到编号，不能证明相关玩法已实现。新建角色成功后自动入场的整个流程，尚未在本次修订后另建角色重测。

建角和重开列表截图在工作区 outputs/LanTest01创建成功.png、outputs/LanTest01角色列表.png。本次城镇视觉证据来自用户反馈，没有另存城镇截图。验收索引为 outputs/DFO第一阶段验收.json。00:29 UTC 核查隔离客户端35368、网关29968、probe34020均存活响应，当前窗口保持运行。原始 EXE 与 PVF 哈希重算均一致。

## 00:15 UTC 修订说明（随后已完成入场验收）

`roles_persist_select_actor_07` 的 SELECT 时间/编号修订已实测：原生日志 createdTime=1789081575、MyServerId=3。新 USERINFO mode 0 316 字节（加密填充后 320）被接受；日志 `<read minimum Information>`、`user : LanTest01 Lv1 Slayer`。00:12 UTC 只读进程检查确认 actor、virtual character、scene character 均存在，但地图 town=0/area=-1，游戏模块仍是选角，不能称为已进城。用户反馈“闪了一下”“再次点击无反应”，当时进程实际仍存活响应。

当前新实验 `roles_persist_select_actor_town_08` 增加 NOTI 24 AREA_USERS。格式由本构建 `1452fc5b0` 恢复：u32 town、u32 area、u16 count，每行 u16 actor/x/y、三个 u8。slot 10 的重复 XOR32 实现通过 16 组原生模拟向量（包括 SIMD 长度），块对齐是 4 而不是密钥长度 8。现在共实现 9 个加密槽。

`cmd/towncatalog` 只读 PVF 导出 `configs/town.generated.json`：town=38、area=0、Elvengard、最低等级 1、七个可走矩形，并带源文件哈希。出生坐标 (561,234) 放在独立 `configs/town-entry-probe.json`，是本地实验策略；不是恢复出的官方教程出生规则。Go 全部测试、vet、构建通过。真实地图加载随后已通过验收，见上节。

mode 1 全状态/技能/背包尚未发送。基本 actor 的未知字段为空实验值；初始装备字段仍待恢复，不得把它当完整角色状态。当前客户端还记录 ARRUseItem1.ani 和 HeroesDungeonPairList.cos 缺失，但没有证据说明它们是这次未进城的直接原因。

## 原始文件与隔离

- 原目录：`F:/dnfop/DFO`，原文件未改动；实际测试使用 `../dfo_probe_client` 独立副本。
- DFO.exe 2.38.2.34，x64 原生 C++。SHA-256：`fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c`。
- Script.pvf SHA-256：`2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167`。脚本版本 20260901。
- 不能把主程序没有明显整体压缩壳等同于没有保护：附带 CEF/安全组件有保护机制；该包亦存在清单差异和部分资源读取错误。
- 副本只在 `0x1452c803e` 修正 tagged ArenaStringPtr 到实际会话密钥字节的引用。当前副本 SHA256 为 `5543c382287bfd5354c3d8c32fd0ce2f1bac572adcc57563c29e6091e332e1cb`；`../dfo_probe_tools/key_pointer_patch.json` 是早期记录，不能直接按旧记录重打补丁。修正后 334 字节密钥一致，CRC、预检查和登录原生解析已验证。
- probe 通过 Windows WFP 动态过滤器限制测试副本及其子进程只连回环地址；退出时撤销自身过滤器，不改系统防火墙规则。Job Object 管理测试子进程。

## 原生协议已验证的范围

`runtime/precheck_01`、`login_01`、`roles_01`、`roles_persist_01` 保存原生断点和本地收发证据。

- CHANNELINFO protobuf、334 字节会话密钥和自定义 CRC 已对上。
- 原生客户端接受 PRECHECK 成功、LOGIN 成功字段，并主动发送 GET_USERINFO。
- USERINFO 模式 2 空列表和实际角色行解析通过。`roles_persist_01` 是早期零角色快照；当前真实用户已创建角色，见后续记录。
- 角色名检查 684、创建 5 的布局从当前 EXE 的发送/接收函数恢复；LanTest01 已由真实界面创建、存档，并在重开后的角色列表显示。
- 8 个加密槽已有原生验证：大端 XTEA（预检查实测）、CAST5、当前构建 RC6 变体、AES-128 ECB、小端 XTEA、Skipjack、当前构建 Noekeon 变体、MULTI2。不能以标准算法名称直接替代本构建的实现。
- `runtime/roles_row_01` 的临时 WireProbe01 单行数据已通过原生角色行字段、角色初始化与 USERINFO 尾部解析；客户端日志明确读到该名字。该行没有保存到数据库，也不是 LanTest01 的建角证据。SELECT_CHARACTER 的完整应答和入城初始化尚未完成。

`cmd/wireprobe` 是仅监听回环的开发验证接入端。它使用确定性测试密钥和开发账号 probe，不是账号密码认证服务，不可对局域网发布。部分启动包仍由固定实验样本应答。创建和姓名修改要求解密和校验通过。原生业务错误码尚未完整恢复，创建/姓名业务拒绝暂用这两个原生处理器明确支持的代码 2 回复并保持连接；不能伪造创建成功。

## 2026-09-10 23:02 UTC 建角退出修复

`roles_persist_live_01` 收到了姓名检查 684 及创建 5，CRC 均通过。实际测试名为 6666、职业 16（当前 PVF 对应 Archer）。旧校验仅参考了早期十字节选项发送函数，拒绝新版 naming window 的选项 8 值 1，接入端随后关闭 TCP；client.log 显示客户端正常退出码 0。此次“闪退”的直接原因是后端拒绝后断连，并非已证实的本机访问异常。

原生 `0x141733fe7` 至 `0x1417340d9` 确认新版界面发送十二个选项字节。解析器已兼容十/十二字节布局，保留动态选项。真实报文加入回归测试，业务拒绝改为应答后继续读连接。`roles_persist_live_02` 已由用户创建 6666、66666、LanTest01，成功提示截图位于 outputs/LanTest01创建成功.png；live_03 重开后的三角色截图为 outputs/LanTest01角色列表.png。

## 23:11 UTC 真正的访问异常与位置编号修复

live_03 创建 monv（职业 3）后已写入数据库，接入端回成功并推送完整列表。客户端退出码为 0xC0000005。崩溃调用链：145637df6 -> 14022a995 -> 1401f8b12 -> 140236145 -> 14444e28d -> 14444c724，最后一条指令读取空角色信息指针的 +0x10 字段。

原生 1401f64d8..1401f6519 建立的列表索引从 0 开始；14021adf0 按该位置查对象，创建应答的 u16 经 14022af30 写入 +0x1d60 后也用于此查询。原实现混用了数据库从 1 开始的 wire_id，第四角色回 4，而本地有效位置为 0..3。仅修改推送时机不能解决这个映射错误。

修复：协议行显式使用 Slot，允许第一个位置 0，拒绝顺序不一致；创建回包在账号自身有序列表中解析位置，不泄露或更改存储主键。原有数据库 ID、名字和状态保留。协议回归、临时 schema 的账号隔离/幂等重试检查通过。

live_04 已实测创建 nvgui（职业 11，数据库 wire_id=5，slot=4），成功回包及列表刷新后进程继续运行，未再出现上述访问异常。用户仍反馈成功提示后没有回到列表，自动返回流程尚未验收。该次随后发送 848、433（MERCENARY_INFO）、407（CHARAC_SLOT_EXTEND_EFFECT），尚在恢复后续处理，不能认定 433 必然是卡点。

创建成功后仍按原生创建标志触发列表刷新；早期“只是立即推送时机”的判断已被更具体的位置映射证据修正。SELECT_CHARACTER 已捕获真实 slot=2 的请求，但完整应答/城镇仍未实现。

## 23:40 UTC 选角应答与黑屏返回

用户进一步确认：成功 Notice 的 OK 能关闭弹窗，但仍在创建职业页面。live_05 创建 777 后约 11 ms 即出现 slot=6 的 SELECT 请求，结合 1401f8a30 的原生调用链，证明当前创建回调会自动选中新角色；它依赖后续入场链路，不应凭空修改界面关闭函数。

角色列表后续 848 和 433 已接入回环开发端。848 成功体为 19 个字节的本地零计数样本，字段全意仍未恢复；433 空佣兵集合为 success + u8(0)。MULTI2 的原生往返向量最初误把两个方向命名颠倒，live_05 校验拒绝保留了证据。用真实 433 报文分别运行两个原生函数，只有 146d96330 解出 `0601020304050600` 且 CRC 正确；最终加密 146d964c0 / 解密 146d96330。live_06 的真实日志已连续记录 MERCENARY_INFO Result: Ok。

新增 opt-in SELECT 解析实验、账号所属角色位校验和会话选角标记。GET_USERINFO 非模式 2 时只记录缺口，不再错误回整份选角列表。`go test ./...` 及独立 schema 的角色创建、幂等、账号隔离、越界选角测试通过。

live_06 23:40:30 收到 LanTest01 slot=2，客户端记录 SELECT_CHARACTER (Size: 240) Result: Ok，随后发送 495、433、67、707。用户观察为黑屏后返回角色列表，再点 Start 无反应。**这只证明当前应答被解析，不是完成进城。** 当前还没有 USERINFO 的入场模式、场景对象/出生坐标和切场结束消息。

进一步核对原生日志参数发现：SELECT 第一个 u32 暂未使用，第二个 u32 是 createdTime；紧接的第一个 u16 是 MyServerId，后面才是三个疲劳值。live_06 的旧实验样本错误填 MyServerId=0。代码已改为读取数据库 created_at、单开发账号下角色 wire_id 作为非零入场 actor ID，并与列表 Slot 分开；该修订已构建到 `bin/wireprobe-followup.exe`，**尚未重启给真实客户端验证**。未来多人世界需要独立 actor ID 分配，不能把账号内 wire_id 直接当成全局唯一 ID。

23:40:37 只读存储核查有七个用户角色：6666、66666、LanTest01、monv、nvgui、666666、777；均保留。最后两个是用户在调试窗口新建的角色，不是回归测试数据。

## PVF 读取和配置来源

`../dfo_probe_tools/unwrap_pvf.py` 只读当前 EXE、sk.dat 和 Script.pvf，生成独立内层文件 `runtime/pvf_source/Script.inner.pvf`。RSA/AES 密钥只在内存使用，不另存密钥。外层处理遵循当前原生加载器：每 10 MiB 分段处理前 10 KiB。

内层 SHA-256：`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。格式 `dfo_20260901_inner`，5,650,173 个文件、82,571 个数据组。Go reader 验证段边界、哈希表长度、字符串解压长度及数据组覆盖范围。

当前构建的原生入口：归档加载 `0x147c4d990`，通用流解码 `0x147c4d810`，字符串池 `0x147c995b0`，正文组读取 `0x147c46ae0`。文件名/目录偏移符合原生 `0x147cd0410` 与 `0x147c99570` 的解析。

`cmd/catalogimport` 从 `list/character.lst`、17 个职业 `.chr` 和 `character/characterinfo.etc` 导入 `configs/characters.generated.json`。生成文件保存归档哈希、条目路径、原始字节哈希、初始属性、技能原始单元、创建装备原始单元、外观索引。

不丢弃未知脚本单元，也不把未知单元解释成数值。创建装备里的转职列选择、装备槽映射、出生点与剧情初始状态还需恢复；目前只保留原始配置单元。`configs/character-probe.json` 中的角色数量和初始等级属于显式本地测试策略，未冒充从 PVF 恢复的规则。

## 存储与测试

PostgreSQL 17.10 使用项目独立数据目录，回环端口 25438；Redis 8.10.1 使用回环端口 26388、独立密码和前缀。连接配置位于被忽略的 `runtime/storage/local.json`，不要复制到报告或提交。

- `go test ./...` 通过，涵盖协议畸形输入、空列表原生样本、原生加密向量。
- `go run ./cmd/charactercheck` 在独立临时 schema 验证并发最后一个角色位只有一次成功、跨账号重名约束、重新连接后的持久化及 PVF 数值/来源保存。临时 schema 验证后清理。
- `runtime/character_validation.json` 明确标记 `real_client_character_created=false`。该测试不会创建 LanTest01。
- `go run ./cmd/storagecheck` 在 23:18 UTC 确认四个存档；23:20 UTC live_04 再创建 nvgui，当前已有五个用户角色。只读核查应以最新数据库为准。

后续账号密码登录器、Redis 登录会话和账号缓存、多客户端同城及移动仍未验收。当前 Redis 是连通性与存储基础，尚不能声称完整多人缓存链路已经实现。

## 画面验证限制

Computer Use 截图接口报 `SetIsBorderRequired failed: 不支持此接口 (0x80004002)`，游戏亦没有可用控件树；后续读窗口还出现句柄变化和 foreground process id 读取失败。不能据此猜点击位置或宣称看到角色进入城镇。需要一次真实界面操作或可读取的画面来继续确认建角/选角流程。
