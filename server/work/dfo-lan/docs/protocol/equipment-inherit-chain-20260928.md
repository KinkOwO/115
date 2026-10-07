# 装备继承（继承窗口）完整链路取证 — 2026-09-28

> 目标：解释「继承」窗口点继承没反应的原因，并给出把它修通的完整链路与证据。
> 方法：IDA 9.4 无头探针（E:\115us\DFO.exe.i64，探针脚本与反编译产物在 E:\115us\ida-work\inherit-*），
> 加上 56 个历史实机会话的抓包核对。中间产物：`inherit-a/b/c/d/e/f/g.out` 与 `*-decomp\*.c`。

## 一、最重要的实测事实

**56 个历史实机会话（runtime/roles_*）的事件日志里，CMD 1722/1723 一次都没有出现过。**

也就是说：玩家在继承窗口里点了「继承」之后，客户端**根本没有发出任何网络包**。
卡点在客户端侧（发包之前），不是服务端漏了应答。这与「修复不生效」的现象完全一致——
不管服务端怎么做，客户端不发包就没有链路可言。

## 二、命令号与窗口（已证实）

| 项 | 值 | 证据 |
| --- | --- | --- |
| 继承命令 | **C1723** `ENUM_CMDPACKET_EQUIPMENT_INHERIT` (0x6BB)，名字串 `0x14B0647C0`，名字表槽 `0x14EF3C538` | opcode 表转储 + `sub_140069BB0`（名字表初始化） |
| 雕纹命令（姊妹系统） | C1722 `ENUM_CMDPACKET_EQUIPMENT_CARVING` (0x6BA) | 同上 |
| 确认弹窗 | `UI/itemtoolwindow/itemCarving/inheritConfirm.xui`，类构造 `sub_1430DB660`，主 vtable `off_149D0B878` | xorstr 转储 + 探针A/B |
| 主窗口/完成窗 | `equipUpgradeSeason3.xui`（构造 `sub_1430FBD60`，xui 757）、`equipUpgradeSeason3ConfirmWindow.xui`（`sub_14316F3A0`，xui 758）、`equipUpgradeSeason3CompleteWindow.xui`（`sub_14316F250`） | 探针B |
| 四窗统一工厂 | `sub_146753320` 构造全部四个继承窗口 | 探针C |
| 结果文案 | dstr 101036882 "Inherit Equipment Complete!"、101036957 "Non-enhanced equipment can only be registered onto Inherited equipment."、101037079 "Remove item lock to use inherit feature."、69060 "This item can't Inherit." | dstr 转储 |

## 三、客户端网络的两层模型（已证实，解释为什么发包点难找）

1. **net 层**（按命令号建真包）：
   - `sub_14668C520(net单例 qword_14E683C78, CMD, 0, 0)` —— 新建 CMD 的请求包。
     实证：`sub_1428B0030` 里 `sub_14668C520(net, 12, 0, 0)`（12=SET_PARTY_INFO，继承窗关闭时刷新队伍信息）；
     season3 主窗口关闭方法 `sub_1431009E0` 里 `sub_14668C520(net, 537, 0, 0)`。
   - `sub_14667BB90(net, CMD, 0)` —— 取**未完成**的同号请求对象；`sub_1428B0030` 取到 1723 的 pending 后调 `sub_142C81700(v5, 0)` 取消（这是继承窗的取消/收尾路径）。
   - `sub_146682140(net, CMD, flag)` —— 挂起期间锁输入；`sub_1428B03A0` 对 1723 置位。
2. **actor 请求层**（请求对象池 + 提交）：
   - `sub_144B99B30(池单例 qword_14E634220, 族号)` 造请求对象（627 族、241 族等），
     经若干 setter（`sub_14259E1C0/E1D0/E1E0/E1F0`、`sub_1407074B0`、`sub_140833E90`、`141EE9820`、vtable+392）填字段，
     最后 `sub_145C01600(目标对象, 请求对象, 目标对象, 0, &回调, **arg6**, -1, 0, 0)` 提交。
   - `sub_145C01600` 只是把 {请求对象, 回调, arg6} 打包后调目标对象 vtable+7560（提交槽），序列化在请求对象类内部完成——**静态拿不到字节布局**。

## 四、全局扫描「谁引用了 0x6BB」（探针D，全 .text 145MB 原始扫描）

352 个原始字节命中、35 个指令级立即数命中。**全部 0x6BB 命中都不是继承窗口的发包**，逐一看过：

| 函数 | 用途 | 证据 |
| --- | --- | --- |
| `sub_140D6F9B0` | **[AUTO DASH COMMAND]** 回调（字符串解密见 xorstr：注册串 `0x1494D6620`="[AUTO DASH COMMAND]"，分支串 `0x149277060`="LEFT"、`0x149277078`="RIGHT"） | 探针F/G |
| `sub_140D6FB90` | 响应分发：`sub_145F440A0(item, &views, 1723)` 找绑定 1723 的 UI 视图刷新（槽+432） | 探针D |
| `sub_1428B0030/03A0/0450` | 继承窗关闭/取消：pending 1723 的取消、输入解锁 | 探针D |
| `sub_142C7DAE0` | `UI/Event/20210304_WantedChase/main.xui` 活动窗构造，注册 1723 | 探针D/G |
| `sub_144D2C990` / `sub_140E17BE0` | 物品事件驱动的**自动补发**路径：627 族对象 + 单个 u32（默认 250，`unk_1494FF954`=fa 00 00 00）+ `sub_145C01600(..., 1723, ...)` | 探针D/F |
| 其余 | 大函数里的比较/分派 | — |

结论：**0x6BB 在客户端代码里同时是「继承请求类型」和多个系统的复用号；继承 UI 的确认按钮并不直接以立即数形式出现**——它经由请求对象池+提交槽的抽象，静态还原序列化格式需要仿真切请求对象类（vtable+7560 之后的序列化器），成本高、且仍属猜测。

## 五、为什么点击没发包（ assessed 卡点）

结合客户端资格文案（dstr）与窗口布局（截图：材料装备列全空、继承装备列放了 1 件）：

1. **材料位必须放「已强化」的装备**（dstr 101036957 非强化装备只能登记到被继承装备上；
   dstr 400002962 继承=转移当前穿戴装备的强化）。本服装备几乎全部来自掉落/奖励，行偏移 10 低 5 位
   （强化等级，见 `internal/inventory/reinforcement.go:208` 取证）全是 0 —— 客户端判定没有可继承材料，
   材料列表为空，发送列表为空 ⇒ `sub_140D6F9B0` 式的发送循环**一次都不执行**。
2. 其他客户端门槛（dstr）：物品锁定必须解除（101037079）、已附魔装备不能直接当材料（100087771，要走 NPC Loton 转移）、
   PVF 资格标记 `[equip inherit]`/`[restrict inherit]`（解析点 `sub_14740D700`/`sub_1477CE1D0`/`sub_1479248D0`，
   资格权威是**客户端自己的 Script.pvf**，服务端目录不影响）。
3. 所以「修复不生效」的直接原因：**客户端在发包前就判定没有合法材料**。这不是服务端能单独修好的环节。

## 六、修通的完整路径（下一步，按顺序）

1. **生产一件已强化装备**：服务端已有普通强化券全流程（`cmd/wireprobe/reinforcement_flow.go`，固定 +1..+15）。
   用券把任意可强化装备上到 +1 以上，物品行偏移 10 低 5 位即非 0，客户端就有合法材料了。
2. **服务端已铺好抓包**（本次改动）：1722/1723 加入 `observedGameRequest` 全量解密白名单——
   实机第一次点「继承」就会在 events.jsonl 里保留完整明文体（`plain_hex`），不再受 8 次采样限制。
3. **按样本实现**（拿到 plain_hex 后的下一步，本次不做猜测）：
   - `internal/game/protocol/equipment_inherit.go`：按样本定 C1723 解码（预计含材料/目标的空间+槽位+模板）；
   - `internal/inventory/inherit.go`：同代校验 → 材料/目标资格（强化位、类型族、锁定状态）→
     行实例字节转移（强化/品质等，参照 14576D8B0→145770B50 的偏移证据）→ 扣 25,000 金币（窗口显示的固定费用）→
     幂等键提交（`CommitCharacterEvent`，同强化券模式）；
   - 应答按客户端等待链实现：ack + NOTI13/14 刷新两件装备（`sub_140D6FB90` 的视图刷新由物品行更新驱动），
     结果窗由客户端本地弹。
4. **验收**：实机 2 次继承（成功 1 次 + 金币不足拒绝 1 次），事件日志对账。

## 七、本次改动与验证

- `cmd/wireprobe/request_scope.go`：1722/1723 加入全量解密白名单（附取证文档指引）。
- 临时诊断工具 `cmd/inheritpvfscan`（PVF 继承标记普查；本格式枚举器只覆盖到动画节点，统计不可用，留作后续修复参考）。
- 探针与反编译产物：`E:\115us\ida-work\inherit_probe_a..g.py`、`inherit-{a..g}.out`、`inherit-*-decomp\`。
- `go build ./cmd/... ./internal/...`、`go vet ./cmd/... ./internal/...` 通过；`go test ./cmd/wireprobe` 通过。
