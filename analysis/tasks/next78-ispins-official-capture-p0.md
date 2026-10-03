# next78：伊斯大陆（Ispins）官服抓包逐帧解析（P0 取证真源）

> 日期：2026-10-02。数据源：`D:\115us\analysis-tools\output\official_20261002-160349_decoded\session_s4_{c2s,s2c}.{txt,bin}`（官服 115 客户端完整伊斯大陆军团通关，10.667s→398.571s，509 帧）。
> 用途：私服 `internal/legion` 扩展伊斯大陆的**唯一字节级依据**。一切结论附帧号+hex；未闭环推断标注「推测」。
> 已核对：私服 `internal/game/wire/server.go` 的 `EncodeXTEA` 用标准库 XTEA（32 轮），与官服一致，无轮数 bug。

## 0. 核心修正：伊斯大陆 ≠ 末世录协议族

- **伊斯大陆内容号 = 101（0x65）**，末世录（Apocalypse）= 107。
- 伊斯大陆走 **STOLEN_LAND_ISPINS 族**：CMD **2043/2045/2046/2047**、NOTI **2252/2253/2254/2255/2256**。
- **全程未出现**：NOTI2895/2896（LEGION_INFO/OPERATION）、NOTI2657（PHASE_CLEAR_TICK）、NOTI2568（PREPARE_LEGION_ENTER_DUNGEON）、CMD2354/2355/15。
- ⇒ 私服现有 `internal/legion/legion_info.go`（NOTI2895 = u16 内容号+204B，ApocalypseContent=107）**不适用于伊斯大陆**，需要全新 NOTI2255 状态机实现。
- 共享的只有 CMD2043/2045/2046 三个信封（body 宽度也比私服现口径宽：官服 2043/2045 body=24B、2046 body=32B、2047 body=32B；信封 13B 后字段见 §B.6）。

## 1. 全程时间线（因果序）

### 1.1 开战
| 时间(s) | 方向/帧 | id | 事件 |
|---|---|---|---|
| 10.667 | C f93 | 35 | SET_USER_POSITION（入频道） |
| 16.814 | C f125 | 12 | SET_PARTY_INFO |
| 19.130 | C f145 | 36 | SET_USER_AREA（进军团等待区） |
| 22.380 | C f170 | 2043 | LEGION_START，body 24B：`98f55f00 00000000 0d000000 65 00...`——[13]=0x65=101 内容号 |
| 22.623 | S f374 | 2254 | LEGION_ENTRY_CHARAC_INFO 272B（**先于 ACK2043**） |
| 22.913 | S f376 | 2255 | STOLEN_LAND_ISPINS_INFO 88B，@3=01 初始态 |
| 22.913 | S f377 | 2043 | ACK 16B：`01 00000000 3bfff77e43 ...`（@1=01 成功，@5:10 5B token） |
| 25.623 | S f384 | 2255 | @3=06 待选态（**无 CMD 触发的服务端主动推送**） |

### 1.2 操作选择（CMD2047，全新命令）
| 时间 | 方向/帧 | 事件 |
|---|---|---|
| 41.355 | C f281 | 2047 变体A 32B：@0:3=token 50a9c69f、@13=01、@17:18=ffff |
| 41.600 | S f430 | N2255 @3=02 已选态 |
| 41.894 | S f431 | ACK2047-A：@1=01、@12:15=**LE unix 时间戳**（0x6ABF662D≈2026-10-02 16:03）、@20:25 5B token |
| 54.910 | C f283 | 2047 变体B：@8=计数(0x29)、@13=02、@17=0b |
| 55.153 | S f433 | ACK2047-B：@1=02、@5=0b、@20:25=9c14c6d43c（准恒定） |

### 1.3 进本 = CMD2045 → 服务端 18 连包（X10 已解）
CMD2045 body 24B：`[13]=65`（内容号）、**`[17]=00/01/02/03`（0 基阶段号）**。
服务端随后推 18 连包（f439–f456，~0.25s 内）：

```
N1539 MEMBER_PREMIUM_INFO 16B
N23   (区域/实例对象)         
N2    USERINFO 848B (zlib 789c...)
N26   UDP_HOST 16B
N781  INFINITE_DIFFICULTY_INFO_USER 272B
N782  INFINITE_DIFFICULTY_INFO_CHARAC 400B
N27   ENTER_SELECT_DUNGEON 48B
N2255 STOLEN_LAND_ISPINS_INFO 88B (@2=02 本内, @3=02)
N476  USER_FATIGUE_ACCELERATION_STATE 64B
N1584 STACKABLE_DUNGEON_LIMIT 16B (slot2 RC6，本次未解)
N28   DUNGEON_INFO 48B          ← 副本 id @0:3 LE
N629  LINKED_DUNGEON_INFO 16B
N29   START_MAP 72B              ← 地图 id @0:3 LE，boss id @43:44
N465  MONSTER_MOVE_SYSTEM 16B
N1712 SECRET_SHOP_WONHEE_EVENT_INFO 16B
N475  CHARACTER_BUFF_DUNGEON 8B
ACK2045 24B（@1=01, @5=65, @9=阶段回显, @14:19 5B token）
N3    USER_STATE 16B
```

**四阶段副本/地图/boss 实测表**（注意：与 legion-contents.generated.json 的 dungeon_info_data 顺序不同——进本顺序由 CMD2047 选择决定，本局玩家选的顺序为）：

| 阶段 | CMD2045[17] | 副本 id | 地图 id | boss id |
|---|---|---|---|---|
| S1 | 00 | 100002987 (`abecf505`) | 100006476 (`d928da30`) | 255 (`ff00`) |
| S2 | 01 | 100002988 (`acecf505`) | 100006485 (`3b36775f`) | 2188 (`8c08`) |
| S3 | 02 | 100002985 (`a9ecf505`) | 100006468 (`4c06cf6d`) | 1712 (`b006`) |
| S4 | 03 | 100002986 (`aaecf505`) | 100006470 (`02cb1569`) | 2216 (`a808`) |

（100002985=ispins_ashcore、100002986=ispins_itrenog，见 legion-contents.generated.json；100002987/100002988 为另外两个阶段本。）

### 1.4 每阶段战斗与结算链
战斗：CMD2059（复活，body 24B，@12:15=2c012c01 恒定）→ CMD39 DIE_MONSTER（128/144B，@0:3=boss id LE）→ N38 回显。

结算链（以 S1 为例，105.69s–120.33s）：
```
N2204 UPDATE_MONSTER_PIECE_DATA → N2201 → N279
N31  ENABLE_CLEAR_DUNGEON            ← 首 2B = 阶段 token（938c/a769/7577/ac87）
N2256 STOLEN_LAND_ISPINS_OPERATION 16B（S3 缺失，见 §4）
N2168×n ACHIEVEMENTS_DATA
N14  UPDATE_ITEM_LIST（道具 0x9DB1BC×20）
N2252 LEGION_BASIC_CLEAR_REWARD     ← 64B zlib → 7772B；尾 @7760:7761 == N31 首 2B
C: CMD117 BOSS_DIE_CHECK + CMD46 SET_PLAY_RESULT
N2255 @7=03（本阶段通关）+ N1658 REQ_DUNGEON_CLEAR_INFO (0B)
N2253 LEGION_ADDITIONAL_CLEAR_REWARD ← 64B zlib → 2405B（40B 记录数组）
N2254 @2=01（阶段 1 通关标志点亮）
C: CMD1654 RES_DUNGEON_CLEAR_INFO
C: CMD2046（body 32B：[13]=65, [17]=阶段号, [21]=(stage+1)%4, @0:3=token 00699a03）
S: ACK2046（@9=阶段回显, @13=@21 回显, @14:19 5B token）
C: CMD72 EPLP → N2255 @3=06（回待选态）
```

### 1.5 终局与离场
| 时间 | 帧 | 事件 |
|---|---|---|
| 312.601 | C f501 | CMD2046 [17]=03, [21]=01（终局结算） |
| 312.846 | S f784 | **N2255 @3=03 终局态**（先于 ACK2046） |
| 312.887 | C f503 | CMD191 STORY_PAUSE `00 01`（剧情暂停） |
| 313.131 | S f785 | ACK2046 @9=03 |
| 313.422 | S f787 | N170 DUNGEON_EVENT_STORY_PAUSE |
| 384.844 | S f792 | N2255 @3=05（剧情结束，准备离场） |
| 384.865 | C f508 | CMD191 `01 01`（恢复） |
| 386.5–387.6 | C f509/510 | CMD72 EPLP×2 |
| 394.225 | C f521 | CMD13 LEAVE_PARTY |
| 394.468 | S f813 | N2254 全阶段通关（@2/@4/@6/@8=01，107 条目 flag=7f） |
| 398.571 | C f536 | SET_USER_POSITION（回城镇） |

## 2. NOTI 载荷详解

### 2.1 N2255 STOLEN_LAND_ISPINS_INFO（88B, slot1）— 核心状态机
已解字段：
- **@2**：0xff=大厅/城镇态，0x02=副本内
- **@3（State）**：01=初始 → 06=待选 → 02=已选/本内 → 06（结算后）→ 03=全部通关终局 → 05=离场
- **@7（Outcome）**：00=未通关，03=本阶段通关
- **@20-23/@28-31/@36-39/@44-47**：四阶段槽（u32 BE 末字节）：**00=当前阶段、01=已通关、02=锁定**（通关后旋转，如 S1 后变 01/02/03/00）
- @12-15 恒 `00000001`；@53：57→64；@56-59 BE：06/07/04/05；@61：0x32→05→06→0f；@64-75：`00 00 00 0b 00 04 00 03 00 00 00 00`（S3 异常为 `0a 96`）；@76：0xd2 标记入口序列帧；@79-83 5B token 每帧变化
- 初始帧 @20-47 全 `ff`（未初始化哨兵）

### 2.2 N2254 LEGION_ENTRY_CHARAC_INFO（272B, slot0）— 资格/进度
- @0：00=登录期 / 01=军团局内
- **@2/@4/@6/@8：四阶段通关标志**（0/1）
- @16 起 **7×24B 条目**，每条 @0:3=内容号 u32 LE（101..107）、@+5=flag
- @256-260 5B token
- flag 演化实测：
  - 登录期（f257，早于时间线窗口，属于世界装载推送串）：101=7f, 102=7f, 105=7f（其余 00）
  - 进局时（f374）：全部条目 00
  - S1 通关后（f490）：101/102/103=7f, 104=39
  - 全通关（f777→f813）：全 00，**f813 时 107=7f**
  - ⇒ flag 语义未解（推测：资格/周常状态位掩码），**需 IDA**。f257 是登录期下发——**军团频道入口锁定状态大概率由此帧条目决定**，待 IDA 确认。

### 2.3 N2252 LEGION_BASIC_CLEAR_REWARD（slot13，zlib：64B→7772B）
- @1600-1602+@1604：道具 `0x009DB1BC`（LE：bcb19d00）× **20**（每阶段固定）
- @7760-7761：**2B 阶段 token == N31 首 2B**（S1 `938c` / S2 `a769` / S3 `7577` / S4 `ac87`）

### 2.4 N2253 LEGION_ADDITIONAL_CLEAR_REWARD（slot13，zlib：64B→2405B）
- 40B 固定记录数组：`[0]=flag, [1:5]=道具 id LE, [5]=数量, [9]=03`
- 记录数实测 **S1=5, S2=4, S3=4, S4=5**：
  - 公共：9DB1BC×28, 9DB1BD×12, 9D94F3×50, 9D94F1×100
  - S1 多：9D94F6×25；S4 多：**9E0E20×2（终局额外）**
- N14 累积互证：B1BC 每阶段 N2252 +20、N2253 +28 ⇒ 20→48→68→96

### 2.5 CMD/ACK 字段表（信封 13B 后）
| CMD | body | 关键字段 |
|---|---|---|
| 2043 START | 24 | @0:3=token(98f55f00), @13=**65**, @8:12=0d000000 |
| 2045 ENTER | 24 | @13=65, **@17=00..03 阶段号(0基)** |
| 2046 REWARD_END | 32 | @13=65, @17=阶段号, **@21=(stage+1)%4**, @0:3=token(00699a03; S3=0) |
| 2047 OP_SELECT A | 32 | @0:3=token, @13=01, @17:18=ffff |
| 2047 OP_SELECT B | 32 | @8=计数(29/2a/24/2c), @13=02, @17=0b(S3=0a) |
| 2059 REVIVE | 24 | @8:11=随机, @12:15=2c012c01 恒定 |
| 39 DIE_MONSTER | 128/144 | @0:3=boss id LE |
| 1654 RES_CLEAR | 16 | @0=0b/00/06/1d, @12:15=a6e50100 恒定 |
| 191 STORY_PAUSE | 16 | @0=00 暂停/01 恢复, @1=01 |

ACK（s2c 无 13B 前缀）：
- ACK2043 16B：@1=01, @5:10 5B token
- ACK2045 24B：@1=01, @5=65, @9=阶段回显, @14:19 token
- ACK2046 32B：@1=01, @5=65, @9=阶段回显, @13=@21 回显, @14:19 token
- ACK2047-A 32B：@1=01, @5:6=ffff, **@12:15=LE unix ts**, @17=f0, @20:25 token
- ACK2047-B 32B：@1=02, @5=0b/0a, @20:25=9c14c6d43c（S3=cda2b2c735）
- ACK2059 16B：@1=01, @5=ed00, @8:13=94f5c28b3c（八帧恒定，推测实例标识）

## 3. 服务端主动推送清单（已验证）
1. f257 N2254：登录装载期（非军团触发）
2. f384 N2255 @3=06：军团局初始化完成（无 CMD 触发）
3. f784 N2255 @3=03：终局态，先于 ACK2046
4. f792 N2255 @3=05：剧情结束

## 4. 阶段 3（S3）官服异常清单（照抄即还原）
- 结算链缺 N2256
- CMD2046 @0:3=00000000（token 清零）
- ACK2047-B @5=0a、token=cda2b2c735
- N2255 @64=0a、@69=96（其他 0b/04）
- TCP 乱序：奖励帧（f665-672, 232.357s）早于 DIE_MONSTER 回显（f652-661, 232.839s）

## 5. 未解项（全部标注「推测」，待 P0.5 IDA）
1. **5B token 家族**（N2254 尾/N2255 @79-83/ACK2043/2045/2046/2047 @20:25/N1474/N170/N28 尾）：每帧变化、末字节 0x33-0x45，推测会话/防重放 nonce
2. **N2254 条目 flag（7f/00/39）**：军团资格语义——**与频道入口锁定直接相关**
3. N2255 @53(57/64)/@56-59(6/7/4/5)/@61(50/5/6/15)：难度/子状态编码
4. CMD2047 变体 A/B 语义（A 带 ffff 与时间戳回显、B 带计数与 0b）
5. N781/782 INFINITE_DIFFICULTY 字段
6. 0xED(237) 常量（频道/实例 id？）
7. N28 尾 5B（S1 为 ASCII "fpcIB"）
8. slot2(RC6)/9(AreaCipher)/3(Twofish)/6(Kasumi) 未移植：N1584 被 SKIP，主链不受影响

## 6. 对 P1–P3 实现的直接影响
- **P2 入场计划**：`BuildEntryPlan` 需按内容号参数化；Ispins 4 副本、阶段顺序由 CMD2047 决定（非 dungeon_info_data 顺序）；body 宽度按本表（2043/2045=24B、2046/2047=32B）。
- **P3 副本流程**：进本 = 18 连包（含 N28/N29）；阶段推进 = N2254 标志位 + N2255 状态机；结算 = N2252/2253 zlib + N31 token 关联；全程无 2657/2568/2895。
- **P0.5 IDA 优先项**：N2254/N2255 reader 闭环、CMD2047 语义、入口解锁条件（sub_142510A50/sub_142511D10）、5B token。
