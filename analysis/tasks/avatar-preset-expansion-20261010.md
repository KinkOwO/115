# Avatar Preset 扩展券（Avatar Preset Expansion Ticket）取证 — 2026-10-10

> 起因：业主问「能实现 avatar preset expansion ticket 的功能吗？添加 avatar preset 栏位」。
> 本文只记**实证结论 + 原始证据位置**。凡是没证据的都标了「未定」。

---

## 0. 一句话结论

| 问题 | 结论 | 类 |
|---|---|---|
| 券存在吗？ | **存在**。`cerashop` product `3003017` / template `590723098` / **150 Cera**，名字就叫 `Avatar Preset Expansion Ticket` | A |
| 是"购买即生效"还是"右键使用"？ | **右键使用**。脚本有 `[action type] [add avatar preset]` + `[use action packet] 0` + `[action usable place] [village]` | A |
| 服务端有实现吗？ | **完全没有**。preset 全套 opcode / noti / 存档字段 0 命中 | A |
| 客户端支持吗？ | **完整支持**。UI（含**锁定按钮**）+ 5 个 CMD + 2 个 NOTI 全在 | A/B |
| 现在能直接做吗？ | **能，但不是"改个数字"** —— 是从零做一个系统，且**有 1 个必须抓包的硬缺口**（右键时 CMD507 的**动作号**） | C |

---

## 1. 道具链（A — PVF 直读）

解 `etc/(r)cerashop.etc`（1062204 字节 / 441 行 / 23818 个反引号名）得到：

```
3003017 590723098 1 0 0 150 0 0 `Avatar Preset Expansion Ticket`
```

对照同表内其它扩展券（同一行格式）：

| 商品号 | 模板 | 价格 | 名字 |
|---|---|---|---|
| 3000150 | 821 | 390 | Dual Skill Build License（第二技能页） |
| 3001197 | 50022562 | 2000000（金币） | Avatar Closet Expansion Kit: Tier 1 |
| 3001198 | 50022563 | 2500000 | Avatar Closet Expansion Kit: Tier 2 |
| 3001199 | 50022564 | 3000000 | Avatar Closet Expansion Kit: Tier 3 |
| **3003017** | **590723098** | **150（Cera）** | **Avatar Preset Expansion Ticket** |
| 3001201 | 10309084 | 390 | Creature Skin Slot Expansion Ticket |

道具脚本 `stackable/cash/590723098.stk`（165 B，**注意路径是平铺的 `stackable/cash/<模板号>.stk`**，不是衣柜那种 `stackable/dfo/cash/2017/0613/avatar_closet/`）：

```
[rarity] 2
[attach type] `[account]`
[icon] `Item/stackable/cash.img` 3219
[grade] 2
[usable job] `[all]`
[cool time] 5000
[minimum level] 1
[stackable type] `[etc]` 0
[move wav] `SCRAP_TOUCH`
[action type] `[add avatar preset]`        ← ★ 关键
[/action type]
[use action packet] 0
[action usable place] `[village]`
[/action usable place]
```

对照 `stackable/dfo/cash/2017/0613/avatar_closet/closet_expand_1.stk`：**没有任何 `[action type]`**，
所以衣柜券是"购买即生效"（`internal/cashshop/avatar_closet_expansion.go` 那一路）。
**preset 券不是** —— 它走"物品使用动作"那一路。

★ `[use action packet]` **不是动作号**：幻化栏券（`stackable/10309001/10309084.stk`，已知动作号 **197**）该字段同样是 `0`。
⇒ **动作号只存在于客户端二进制里，PVF 查不到。**

---

## 2. 服务端现状（A — 全树 grep）

`115/server/work/dfo-lan`（排除 `reapply/snapshot`）：

- opcode `1585 / 1586 / 1587 / 1588 / 2064`：**0 命中**
- `AVATAR_PRESET` / `AvatarPreset` / `avatarPreset`：**0 命中**
- `cerashop.json` 无 preset 条目
- 存档（`inventory.Bag`）无 preset 字段

⇒ 从零开始。参照物只有衣柜（closet）那一套。

---

## 3. 客户端协议（B — 反汇编，**已逆清读包序列**）

处理器地址来自 `115/analysis/dumps/skin-noti/df7-registrar-table.json`（1442 条 id→handler）：

| id | 名字 | noti handler | cmd handler |
|---|---|---|---|
| 1585 | AVATAR_PRESET_LIST / CHANGE_AVATAR_PRESET_NAME | **0x14335A230** | 0x143358C90 |
| 1586 | SELECT_OR_DELETE_AVATAR_PRESET | — | 0x143359170 |
| 1587 | UPDATE_AVATAR_PRESET / EXCHANGE_AVATAR_PRESET_POSITION | **0x14335A940** | 0x143358E80 |
| 1588 | SELECT_AVATAR_PRESET | 0x143352040 | — |
| 2064 | SAVE_AVATAR_PRESET | 0x14286A9C0 | **0x143359060** |

读包原语（本项目既有结论）：`0x146ea09f0`=ReadU8、`0x146ea1920`=ReadU16、`0x146ea0ba0`=ReadU32、
`0x146ea0be0`=Read N bytes、`0x146d78070`=**读长度前缀字符串**（edx = 长度上限）。

### 3.1 noti1585 `AVATAR_PRESET_LIST` ← ★ 这是"栏位列表"那一帧

反汇编（`0x14335A230`）读包序列：

```
ReadU8  -> [rsp+0x30]           ; 用途未确认（官方值 1）
ReadU8  -> [rsp+0x32] == 外层循环次数   ; 官方值 1  → 1 个"页/组"
  for r13 in 0..[rsp+0x32):
    ReadU8 -> [rsp+0x34]        ; 每页 1 字节（官方值 1）
    ReadString(max=0x100) -> buf ; 页名（官方 = "New Presets"）
    ReadU8 -> [rsp+0x31] == 内层循环次数 ; 官方值 12 → 每页 12 个预设
      for r14 in 0..[rsp+0x31):
        ReadU8  -> [rsp+0x35]   ; 预设序号（官方 = 0,1,2,…,11）
        ReadU32 -> [rbp-0x20]
        ReadU32 -> [rbp-0x1c]
        ReadU16 -> [rbp-0x18]
        ReadU16 -> [rbp-0x16]
        ReadU32 -> [rbp-0x14]
        Read(30) -> [rbp-0x10]  ; 30 字节（官方全 0；内容未知）
        ReadU16 -> [rbp+0xe]
        ReadU8  -> [rbp+0x10]
```

⇒ **每条预设 = 1+4+4+2+2+4+30+2+1 = 50 字节**（与官方样本实测步长 50 完全吻合）。
⇒ **头部 = 1+1+1+(4+len(页名))+1 = 19 字节**（官方：`01 01 01 | 0b000000 "New Presets" | 0c`）。
⇒ 官方整包 = 19 + 12×50 = **619**，而包体是 **624**（尾部 5 字节本处理器不读，可能是保留/我未识别的尾段）。

### 3.2 cmd2064（S→C）= "**打开 Avatar Preset 窗口**"，不是保存回包

反汇编 `0x143359060`：读首字节 `dl` 作状态；失败按 `r8w ∈ {3, 0x11, 0x13, 0x17, 0x10C}` 弹不同 dstr，
成功则 `0x14668c520(obj, 0xB3B /*窗口 2875*/, …)`；另有 `0x14667bb90(…, 0x81A /*窗口 2074*/)`。

### 3.3 noti1587 `UPDATE_AVATAR_PRESET`

`0x14335A940`：`ReadU8 | ReadString(max=0x100) | ReadU8` 然后构造与 1585 相同的 0x58 字节对象
⇒ 形状 = **单页的更新**（无外层计数）。

### 3.4 cmd1588 `SELECT_AVATAR_PRESET`（S→C 处理）

`0x143352040`：`ReadU8 | ReadU32 | ReadU16 | ReadU8 | ReadU8 | ReadU8 | ReadString(max=0x80)`
（`0x1456918f0` 是 PcBangDataInfo 单例 ctor，此处只是取容器）。

### 3.5 UI（`ui/inventory/`）

| 文件 | 大小 | 说明 |
|---|---|---|
| `avatarpresetwindow.xui` | 7336 | 小弹窗（189×152） |
| `avatarpresetedit/main.xui` | 46204 | 编辑主窗 |
| `avatarpresetedit/preset_btn_slot.xui` | 3164 | ★ 含 **`lock_btn`（挂锁按钮）** + `cur_preset` ⇒ **栏位有"锁定/解锁"表现** |
| `avatarpresetedit/preview_slot.xui` | 11788 | 单个外观预览（126×208） |
| `avatarpresetnamechange.xui` | 6128 | 改名窗 |

★ `main.xui` 里 `preset_btn` **只出现 1 次** ⇒ 按钮是**按数据克隆**的，不是写死 N 个
⇒ **栏位数量很可能由服务端 noti1585 的计数决定**（正向信号，但未最终证实）。

⛔ PVF 里**没有任何 `etc/` 或 `stackable/` 的 preset 表**
（`-find "preset"` 全库 6807 命中，全是 `contents/`/`live/` 的怪物与特效 `.act/.ani` + `ui/inventory/avatarpreset*`）
⇒ **没有本地表约束栏位数**。

---

## 4. 官方样本（A — 抓包，已解密）

两处官方抓包都有 noti1585，且**内容完全相同**（md5 `6a751013…`）：

| 来源 | 命中 |
|---|---|
| `115/analysis/captures/official_20261002/20261002-211855_192_168_1_31_DFO_exe/frames.jsonl` | op=1585 **10 帧**，全 640B / 体 624B / 内容完全一致 |
| `C:/Users/muyue/Desktop/official_20261009-223033_live/.../session_{s3,s6,s30}_s2c.txt` | 各 **1 帧** id=1585 size=640 body=624 |

官方默认值（624 字节，前 24 字节）：

```
01 01 01 0b000000 4e65772050726573657473 0c 0000...
│  │  │  └ len=11   └ "New Presets"      └ 预设数=12
│  │  └ 每页 1B（=1）
│  └ 页数 = 1
└ 用途未确认（=1）
```

12 条预设的 50 B 记录里**只有序号字节非零**（0,1,2,…,11），其余全 0 ⇒ 官方玩家**从没存过预设**。

★ **C→S 侧 0 帧**：`cmd 1585/1586/1587/1588/2064` 在两个抓包里**都没有出现**
⇒ **没有"保存/应用预设"的请求样本**，也没有**用券的样本**。

---

## 5. 缺口台账

| # | 缺什么 | 为什么重要 | 怎么补 |
|---|---|---|---|
| 1 | **右键用券时 CMD507 的 `action` 值** | 服务端要认这个动作（现有 `internal/game/protocol/stackable_action.go` 的白名单里没有）。PVF 的 `[use action packet]` 是 0，读不出来 | ★ **抓包**：进村庄 → 右键用券 → 取 CMD507 的 `p[7:11]`（u32）。**顺带看服务端回的是哪一帧** |
| 2 | 用券后官方回什么 | 决定"加一栏"怎么表达：重发 noti1585？还是别的 | 同上一次抓包 |
| 3 | 保存/应用预设的载荷（cmd2064-C→S / 1588-C→S / 1585 / 1586 / 1587） | 要做完整系统才需要 | ★ 抓包：存一个预设、应用一次、改名一次、拖位置一次 |
| 4 | 第 1 个 u8（`[rsp+0x30]`）语义 | 未定 | 靠 #2 的抓包对照 |
| 5 | 50B 记录里 30B 那段装什么 | 官方样本全空 ⇒ 逆不出 | 靠 #3（存了预设的样本） |
| 6 | 官方**默认栏位数**（=12？）还是"官方玩家已满 12" | 决定券加的是第几栏 | 靠 #1/#2 的抓包（用券前后列表条数对比） |

### 可先做（A 类，不需要抓包）

1. **存档位**：给 `inventory.Bag` 加 `AvatarPresetSlots`（或一个 preset 列表），默认档位待 #6 定。
2. **noti1585 编码器**：按 §3.1 的布局写 `protocol.AvatarPresetList(...)`，
   先用官方 624B 原样复刻（1 页 / "New Presets" / 12 条）做**登录即显示**的冒烟验证。
3. **cmd2064 编码器**：`0x01` + 窗口参数（§3.2）。
4. **CMD507 拦截**：动作号先用**变量/环境开关**占位（拿到 #1 再钉死），
   或按"背包该槽位的物品脚本 `[action type] == [add avatar preset]`"反查 ——
   这条不依赖动作号也能认（照 `DecodeQuestAirshipAction` 的思路）。

### 证据文件位置

- 反汇编：`E:/DNF/DNF-115US/.workbuddy/tmp/preset-dis/{14335a230,14335a940,143359060,143352040}.txt`
- cerashop 导出：`E:/DNF/DNF-115US/115/server/work/dfo-lan/.workbuddy/tmp/pvf-cera/entry.txt`
- stk 脚本：`.../pvf-stk/`、`.../pvf-stk2/`
- 注册表：`115/analysis/dumps/skin-noti/df7-registrar-table.json`

---

## 6. 已实现（2026-10-10 21:00，本轮）

> 目标：先落地"装扮预设**解锁**"。默认档位数取官方值 12，券 +1。

| 文件 | 改动 |
|---|---|
| `internal/game/protocol/avatar_preset.go` **(新)** | `AvatarPresetList(slots, page)` 编码器（§3.1 布局）；`OfficialAvatarPresetSlots=12`、`MaxAvatarPresetSlots=60`、`DefaultAvatarPresetSlots()` |
| `internal/game/protocol/avatar_preset_test.go` **(新)** | ★ **硬判据**：`slots=12` 时产物与官方 624B 帧**逐字节相同**（金标准内嵌）+ 帧长公式 `19+50*slots+5` |
| `internal/inventory/avatar_preset_ticket.go` **(新)** | `AvatarPresetTicketTemplate=590723098` + 按槽位识别（**不依赖动作号**） |
| `internal/inventory/bag.go` | 新增存档字段 `AvatarPresetSlots`（+ 读/写两侧范围校验） |
| `cmd/wireprobe/avatar_preset_flow.go` **(新)** | `useAvatarPresetTicket`：事务内 +1 档位 + 消耗券；回包 = noti1585 新版列表 + 用掉那格的绝对行（id 14）。键 `avatar-preset-expand:<前缀>:<帧哈希>`（同项目口径，可重复用券） |
| `cmd/wireprobe/client_dispatch_inventory.go` | CMD507 分支**最前面**按槽位拦截预设券；命中即记 `avatar_preset_ticket_use_seen`（**含 action + 整条明文 hex**，把缺口 #1 的取证顺带做掉） |
| `cmd/wireprobe/entry_flow.go` + `client_entry.go` | 登录下发 noti1585（紧跟 noti1077 之后），栏位数取存档、缺省 12 |

- 构建：`go build ./cmd/... ./internal/...` + `go vet` ✅；`go test ./internal/game/protocol/ ./internal/inventory/ ./cmd/wireprobe/` ✅（含新增 3 个用例）。
- 产物：`bin/wireprobe-pvf.exe` 重编（`-trimpath`，MD5 `136bdee365909bfa186cb89d25684eb7`），旧版备份 `bin/wireprobe-pvf.exe.bak-preAvatarPreset-20261010-2116`。
  **已按字节核对**：新 exe 含 `avatar_preset_ticket_use_seen` / `avatar-preset-expand-v1`，旧备份为 0。
- 快照：9 个改动文件已同步进 `.workbuddy/reapply/snapshot/`（含上一轮未同步的 `avatar_closet_flow.go`）。

### 仍未闭环

- **#6 默认档位数语义**：本轮按"官方下发 12 = 服务端给所有人的基线"实现，券 +1 → 13。
  若客户端实际是"固定 12 格 + 前 N 个解锁"，则 13 无效、需把语义倒过来（把"解锁数"放进别处）。
  **一次登录 + 一次用券即可判定**：看预设界面格子数有没有变。
- **#1 动作号**：本轮不依赖它也能跑；无论成功失败，日志里都会留下 `avatar_preset_ticket_use_seen` 的 `action` 与 `plain_hex`。
- 保存/应用/重命名/换位（cmd2064-C→S、1588、1585、1586、1587）仍未实现 —— 等 #3 抓包。

---

## 7. 实测结果（2026-10-10 21:11~21:12，业主用券 3 次："用掉了，但是没解锁"）

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_211029_590005_next37`。

### 7.1 服务端链路 100% 成功

| 证据 | 值 |
|---|---|
| 登录下发 | `avatar_preset_list` id=1585 `plain_bytes=624` ✅ |
| ★ **CMD507 动作号** | **`action = 184 (0xB8)`**，槽位 `4f00`=79，整帧 59 字节、其余字节全 0（标准 CMD507 形状）⇒ **缺口 #1 关闭，不需要抓包** |
| 事务 | `avatar_preset_expansion_saved` `applied=true`，slots **12 → 13 → 14 → 15**（3 次） |
| 券消耗 | `avatar_preset_ticket_spent` id=14 行 `slot=004f` `template=1ab835`=**0x2335B81A=590723098** ✅，数量 `2316→2315→2314`（22→21→20）✅ |
| 重发列表 | noti1585 计数字节 `0c → 0d → 0e → 0f` ✅ |
| 客户端后续 | **1585 / 1586 / 1587 / 1588 / 2064 各 0 帧** —— 它没跟我们谈预设，窗口是纯客户端侧自己开的 |

### 7.2 ★ 新决定性事实：**编辑窗把槽位写死成 10 个**

`ui/inventory/avatarpresetedit/main.xui`（导出 23102 B）：

- **恰好 10 个** `UIPath="STR:UI/Inventory/AvatarPresetEdit/preset_btn_slot.xui"` 子控件
- 静态 ID `presetBtn_0` … `presetBtn_9`（**10 个**）
- 另有 `ID="STR:tab"`（**页签**）、`saveBtn` / `applyBtn` / `exportBtn` / `resetBtn` / `revertBtn` /
  `preset_name` / `preset_name_change_btn` / `previewBtn` / `register_all_Btn` / `register_clone_Btn`

⚠️ **官方默认下发 12 条记录 > UI 的 10 个格子** ⇒ **noti1585 的"预设数"不等于"可见格子数"**。

⇒ 两条互斥解释，静态无法再分：
- **(a)** 格子固定 10，券加的是**页签(page)** ⇒ 该改的是 noti1585 的**页数**（`ReadU8 -> 页数`，官方=1），不是每页的预设数；
- **(b)** 格子数确实由数据决定但 UI 只能画 10 ⇒ 券的效果在这个界面**本来就不可见**，"解锁"另有所指。

⇒ 需业主**一次观察**（不是枚举试错）：打开界面后报"几个格子 / 有没有页签 / 用券前后变没变"。

### 7.3 顺带修正

- `client_acceptance:"pending"` 只是 `client_entry.go:1006` 给**所有**入场帧打的静态标记，**不是**客户端真的确认了 ⇒ 不能拿它判断 noti1585 是否被消费。

---

## 8. 修正：栏位是**页**，不是每页条目数（2026-10-10 21:20~21:30）

### 8.1 业主观察（一次观察，非枚举试错）

| 问题 | 回答 |
|---|---|
| 预设界面里几个槽位按钮 | **10 个** |
| 有没有页签 | **有，数量没变** |
| 在哪点的使用 | **背包里右键** |

### 8.2 两条新证据把"12"排除了

1. **UI 写死 10 格**：`main.xui` 里 `preset_btn_slot.xui` 实例恰好 10 个、静态 ID `presetBtn_0..9`；
   另有 `ID="STR:tab"`、`preset_name` / `preset_name_change_btn`（**页可以改名** ⇒ 页是有名字的容器）。
2. **反汇编 handler 收尾**（`0x14335a6d4..0x14335a7f4`）：
   - `0x14335a6eb` 还有一个 `ReadU8`（落在官方帧尾部 5 字节里的第 1 个）——
     **非 0 时弹 UI2875 并显示 dstr 0x605AE13**（一个"有更新"的提示开关）。
   - `0x14335a7e1..0x14335a7ef`：把**第 1 字节**写进 UI 属性 **0x0F**。
   - `0x14335a7c0`：拿**第 1 字节**与一个**步长 0x38 的对象数组**逐个比对
     ⇒ 第 1 字节 = **当前页索引**；配合外层循环 ⇒ **第 2 字节 = 页数**。

⇒ **栏位 = 页签**，由 noti1585 的第 2 字节决定；官方那 12 是"每页条目数"，与 10 个按钮对不上。
⇒ 上一版把券加在"每页条目数"上（12→15），所以界面毫无变化 —— **服务端全对，只是加错了计数器**。

### 8.3 本轮改动

- `protocol/avatar_preset.go` 重写：`AvatarPresetList(pages byte)`；
  `OfficialAvatarPresetPages=1`、`OfficialAvatarPresetRecordsPerPage=12`（固定、转发不参与解锁）、
  `MaxAvatarPresetPages=20`；第 1 字节固定为 1（不做"跳到新解锁页"，免得改客户端选中态）。
  ★ **金标准仍成立**：`pages=1` 的产物与官方 624B 帧**逐字节相同**（`TestAvatarPresetListMatchesOfficial` PASS）。
- 存档字段**改名** `avatar_preset_slots` → **`avatar_preset_pages`**（语义从"条目数"纠正为"页数"）：
  上一轮测试写进去的 15 因此**自然作废**，**不需要改正在运行的存档**（Go `json.Unmarshal` 忽略未知键）。
- 券落地：`pages = 1 + 已解锁页数`，上限 20；日志字段 `slots` → `pages`。
- 登录下发同改。build/vet/test 全过；`bin/wireprobe-pvf.exe` 重编，
  备份 `bin/wireprobe-pvf.exe.bak-preAvatarPresetPages-20261010-2130`，产物含 `avatar_preset_pages`。

### 8.4 实测确认（2026-10-10 21:27~21:29，业主："好了，我已经全解锁了，实验通过"）

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_212622_683396_next37`：

| 时间 | 事件 | pages |
|---|---|---|
| 13:27:34 | `avatar_preset_list` 624B（登录，1 页） | 1 |
| 13:27:48 | `ticket_use_seen` → `expansion_saved applied=true` → `list` → `ticket_spent` | **2** |
| 13:28:17 | 同上 | **3** |
| … | … | … |
| 13:29:13 | 同上 | **19** |

- **连续 18 次解锁全部 `applied=true`**，`pages` 严格 1→19 递增，**幂等键没有误吞**。
- ★ **客户端全程 0 帧** `1585/1586/1587/1588/2064` ⇒ 页签数量**纯由服务端 noti1585 驱动**，
  "add avatar preset" = **加一页**，语义**已实机确认**。
- 上限 `MaxAvatarPresetPages=20` 未撞顶（当前 19 页）；继续用券会在事务内报 `avatar preset fully expanded`。

---

## 9. 仍未闭环：预设的保存/应用/改名

页签只是"容器"。要让玩家真正用起来，还得实现这四个 C→S：

| id | 名字 | 作用 |
|---|---|---|
| 1585 | CHANGE_AVATAR_PRESET_NAME | 改名（页名就在 noti1585 里） |
| 1586 | SELECT_OR_DELETE_AVATAR_PRESET | 选中 / 删除 |
| 1587 | EXCHANGE_AVATAR_PRESET_POSITION | 换位（拖动排序） |
| 1588 | SELECT_AVATAR_PRESET | 应用某个预设 |
| 2064 | SAVE_AVATAR_PRESET | 保存当前外观为预设 |

客户端侧入口已经找到：`ui/inventory/avatarpresetwindow.xui` 有 `addPresetBtn` / `applyBtn` /
`saveBtn` / `presetList`；`avatarpresetedit/main.xui` 有 `exportBtn` / `resetBtn` / `revertBtn` /
`register_all_Btn` / `register_clone_Btn` / `preset_name_change_btn`。

⇒ **抓包需求（一次就够）**：进预设界面 → 点"添加预设" → 存一个 → 改一次名 → 拖一次位置 → 应用一次。
官方 live 抓包里这五个 opcode **C→S 侧 0 帧**，纯静态逆不出载荷。
