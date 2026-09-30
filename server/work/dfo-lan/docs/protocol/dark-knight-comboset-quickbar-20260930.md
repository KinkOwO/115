# 黑暗武士组合技与技能栏：已确认基线（2026-09-30）

状态：用户已确认当前环境实机生效。`attempt 1/3`：移植已有115客户端取证与原环境验收过的布局，本轮没有试探新的字段、codec或客户端等待态。

## 1. 来源与实施范围

接收包：`docs/todo/黑武士组合技与技能栏修复-交接-20260927/`。包内9个文件SHA-256全部匹配清单。交接说明记载原环境已确认排列重选保留及槽位修复；该确认不等于2026-09-30当前主线已实机通过。

采用包内CMD500解码、NOTI433编码、角色状态保存/清空测试；在当前主线逐处接线，不覆盖包内的旧main.go、entry_flow.go或service.go。范围仅黑暗武士prof=9，不改客户端、PVF、DLL或数据库schema。

## 2. 保留的协议证据

原包记录的CMD500正文：`u8 0, u8 cellCount, repeat(u16 comboSkill, u8 chainCount, u16 chain[chainCount])`，末尾零对齐。已保留原样本测试，包含六个空组合格、单技能、五技能链。

原包记录的NOTI433处理器为`0x1452A85B0`；读取T1/A1/PAGE的地址分别为`0x1452A860F`、`0x1452A8623`、`0x1452A866B`。后续读取外层数量、组合技能键、链长度、子技能键的地址分别为`0x1452A86B5`、`0x1452A8715`、`0x1452A8755`、`0x1452A8775`。

S2C正文为`00 01 00 + CMD500正文去掉首字节`。PAGE固定0，原取证记录的两个写点`0x145CAD9E6`、`0x145CCFD26`都写0。禁止直接回显CMD500：其第三字节118会被当PAGE使用，触发越界。CMD502为无正文清空请求。本轮复用这些记录，没有重新运行IDA或启动客户端。

CMD433与NOTI433属于不同方向/类型：主线原有type=1的角色列表佣兵响应保持；组合技恢复使用type=0。普通日志中的`roster_followup_response id=433`不是组合技验收证据。

用户于2026-09-30确认实机生效。会话目录`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260930_145815_365096_next37`有10条CMD500保存、9条S2C NOTI433回放、2条入场`combo_skill_info_restored`；重选后客户端再次上报同一排列，PostgreSQL角色20保存了对应最后状态。日志同时捕获CMD502的实际明文为16个零填充字节；先前候选曾拒绝该帧，现已修正为接受0～16字节的全零加密块。修正经过单元及临时PostgreSQL集成测试；这条清空路径尚未在该会话中用修正后的二进制单独复验。

## 3. 主线适配

- State新增`combo_skill_info`，显式null支持清空；沿用行锁事务和JSON合并，保留旧存档未知字段。
- CMD500/502每次都解密、校验，不受八次采样限制；只接受当前选中的黑暗武士。
- 保存事件键使用会话随机数和递增序号，避免A→B→A被永久正文哈希误判为首次编辑重放。
- CMD500保存后按S2C布局回放并去重；CMD502落库清空、清除去重状态。
- 入场NOTI19、技能预设NOTI2758和装备技能NOTI2609之后回放NOTI433，保持主线预设顺序约束；重选角色时重置通知去重状态。
- 普通技能树刷新和CMD331按键保存刷新之后，也恢复已有组合排列，避免NOTI19重建导致空排列上报覆盖存档。
- 默认实际加载的`characters.skycastle-release.json`将prof9技能118～123绑定0～5（ASDFGH），其他职业与source checksum保持。
- 未改变catalogimport生成逻辑；重新生成该配置时须保留本职业槽位修正。

## 4. 存量升级和回退

本机实际只有一个黑暗武士角色：ID20。改前组合槽位为118→1、119→3、120→2、121→4、122→18、123→19；已改为0～5。没有使用交接包原环境的ID3/4。

迁移：`scripts/migrations/dark_knight_combo_slots_v1.sql`。不增加表或列；使用现有character_events保存完整before_state/after_state，保留角色config_version。按prof9筛选、只合并六个键；表锁保护检查与更新；同一审计键使重复执行不再改动。普通技能占用0～5时拒绝，不覆盖玩家普通技能；没有显式第一页槽位的旧角色不改存档，走修复后的配置默认值。

执行前确认客户端和游戏网关均未运行。备份存放于忽略目录：

`runtime/storage/backups/dark-knight-combo-20260930/`

其中包含`before-characters.json`、`after-characters.json`、`characters.skycastle-release.before.json`、`wireprobe-handoff-source.before.exe`与`deployment.json`。迁移后逐字段核对：仅角色20的六个组合槽位改变，普通技能槽位及其他存档字段未变。

若回退程序，先关闭游戏与网关，再恢复该目录中的旧exe和旧配置。数据回退只应恢复审计中的六个组合键，不覆盖之后的新存档；先锁住角色并核对当前六键仍为本次迁移的after值，然后以当前state合并原六键，保留后续其他字段和组合排列。需要再次迁移时也须核对并处理已有审计记录，不能盲目删除审计。

## 5. 验证与部署

- 全量`go test ./...`与`go vet ./...`通过；候选版构建通过。
- `COMBO_INTEGRATION=1 go test ./cmd/wireprobe -run 'TestCombo|TestEntryReplaysCombo|TestDefaultCatalogDarkKnightComboShortcuts' -count=1 -v`通过：真实PG临时schema验证A→B→A落库、16字节零填充CMD502清空、未知存档字段保留、外账号拒绝与事件流水数量；临时schema已删除。
- 迁移在临时表上执行两次，验明幂等、其他角色与其他存档字段保持；构造普通技能占槽时验证迁移拒绝。
- 旧`go run ./cmd/charactercheck`未通过：测试库初始化缺少现有`account_unified_options`依赖。调查中临时补依赖后还遇到既有疲劳检查失败；没有将这些无关工具改动纳入本轮，不将该检查标为通过。
- 已部署至`bin/wireprobe-handoff-source.exe`，当前launcher.local.json默认指向它。部署前旧exe已备份。

候选SHA-256：`A9EC4F0FE15AE2668BC0269B849C1D8D611F6EA3444B21257ACDE0741E8A97C6`，24,828,416字节。

## 6. 已完成人机验收与剩余边界

用户已实机确认组合技能栏修复生效。运行日志确认多次CMD500已保存并按NOTI433回放，重选入场恢复且客户端重新上报相同排列。存量角色20六个槽位已调平；源配置单测确认新黑暗武士的六槽位置。

CMD502由实机捕获的16字节零填充已纳入当前实现和PG集成测试；修正后的清空路径待下一次用户清空操作从日志确认。type=0/id=433正文以`000100`开头。H格显示仍有交接包所述PVF空宏限制，本轮未修资源、未独立复验该资源结论。

confirmed baseline已升级；测试库初始化缺项导致的旧`charactercheck`限制见第5节。
