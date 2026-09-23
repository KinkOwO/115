# 普通装扮与克隆装扮共存（confirmed baseline）

## 已确认的依据

- 当前 115 客户端 `DFO.exe.i64` 的 mode-1 装备读取函数 `sub_1452C1540` 在每行 40 字节头的偏移 24、28 连续读取两个 `u32`。第一格所指物品若满足 `sub_147184660(itemdef, 25)`（随机透明装扮），客户端将第一格交给虚表偏移 1480 的处理函数，并将第二格交给虚表偏移 1472；否则将第一格交给偏移 1472。
- 当前完整装备目录以 `[item category] clear avatar` 区分克隆装扮。服务端存档使用 `Group=0` 表示克隆或普通装备、`Group=1` 表示外观装扮；同一装扮部位允许两组各一件。
- 2026-09-22 实机 CMD19 记录表明，客户端请求可将空背包格作为 Source、已穿戴装扮作为 Destination；`DestinationItem` 是目标物品身份，而 `Flags` 均可为零。换装按两个槽位交换处理。
- mode-0 外观行的 `AttachA/AttachB` 曾在非零尝试后触发客户端 `0xC0000005`；当前仍保持已运行基线的零值。

## 2026-09-22 旧候选（已被否证）

- 同部位换装只替换同组物品；旧存档中没有 `Group` 的普通装扮在首次换装时按目录归入外观组，不改 PostgreSQL 表结构。
- NOTI13/14 的穿戴槽每部位仍只发一条基础行，优先克隆物品；mode-0 模型优先显示外观物品。
- 入场 mode-1 详细行在克隆和外观共存时，于头偏移 24 写克隆模板、偏移 28 写外观模板，与上述客户端读取分支对应。此编码尚待实机确认显示效果。

## 2026-09-23 实机否证与剩余工作

`go test ./...`、`go vet ./...` 已通过；候选程序 `bin/wireprobe-handoff-source.exe` SHA-256 为 `705ADA594D52623527126194571B83F890B7450987AF4DF9F0E5336584C0F5E3`。用户以该候选程序重登后，Clone 页 0–7 号槽能显示且可正常穿脱；Avatar 页这些槽仍为空，只有用另一件同部位外观装扮交换才会在本次会话中刷新。不能把 mode-1 双模板编码认定为已经修复本地 Avatar 格子。

运行中角色 11 的 PostgreSQL `inventory.worn` 经只读查询确认：0–7 号部位各保存 Group 0 克隆和 Group 1 外观；8、9 号仅保存 Group 1。入场日志 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_124032_517508_next37/events.jsonl` 显示 `worn_equipment_restored`、屏障后的同包以及 NOTI14 穿戴刷新都只携带每部位优先选择的 Group 0 基础行。服务端存档未丢失外观；Avatar 页缺少的是客户端本地穿戴对象/格子的恢复。

下一步必须通过当前客户端 IDB 确认本地 Avatar 页在重登时由哪条 reader、哪个物品对象字段填充，以及如何在同部位克隆仍在时应用 Group 1。NOTI13/14 的 list3 管理器按槽读取单个对象，不能仅凭快照缺一行就盲目叠发重复槽位行。当前服务端进程仍在运行，确认编码与时序前不覆盖候选程序，也不升级归档基准。

### 本轮补充取证

- 同一实机日志 04:42:26 的一次外观换装：CMD19 从 list1 槽8 换到 list3 槽1；服务器随后依次发送 list0/list3 的 NOTI13 重整、NOTI14 源/目标单槽更新、list3 全量窗口刷新和 mode0 外观刷新。目标单槽 NOTI14 是普通外观模板，之后的全量 list3 刷新仍选择克隆模板。客户端当场能显示两页，但这也可能依赖 CMD19 在客户端的本地预更新，不能单凭此证明重登时补发 NOTI14 会生效。
- 当前客户端 `sub_1452D5A80`（NOTI13）经 `sub_145A0E8C0` 选管理器，按槽 `sub_145AD8D10` 查询已有物品，并由 `sub_145AF7970` 插入新对象；同一槽不能直接推断可保留两个独立对象。`sub_1452E9810`（NOTI14）也有按槽查询和重建路径。`sub_14576D8B0` 读取当前 181 字节物品行，包括靠后的扩展字段，但尚未追到 Avatar/Clone 页各自从哪个字段取穿戴身份。
- 仅含 Group1 的槽8、9与 Group0/Group1 重叠的槽0–7可作为下次人工核验的对照；目前不改运行包、不请求用户盲测多个编码。

### 2026-09-23 13:09 实机追加现象

用户截图确认普通 Avatar 页中也出现带 `CLONE` 角标的克隆图标，Clone 页仍显示同一批克隆装扮；脱下一个克隆部位后，普通 Avatar 对应格也会清空。这比“普通外观格子空白”更明确地指向两页穿戴身份混用，不能把 Avatar 页出现图标视为正确恢复。

最新运行日志 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_130933_121382_next37/events.jsonl` 的 05:10:37 入场 list3 NOTI13 首行仍是克隆模板 `513550010`。之后 05:14:04–05:14:05 头发部位 CMD19 换装时，list3 槽1 的单槽 NOTI14 包含克隆模板 `513560010`，完整 list3 重整也一直优先克隆；与两个页签同时引用克隆身份的截图一致。

当前客户端 mode1 reader `sub_1452C1540` 对详细行头偏移24的模板先通过 `sub_147184660(itemdef,25)` 判断：命中才把它写进随机装扮字段（虚表偏移1480）并把偏移28写进普通外观字段（1472）；不命中则把偏移24本身写进普通外观字段（1472），偏移28不参与该分支。旧候选把克隆 ID 放在偏移24，足以解释普通 Avatar 页的 CLONE 图标。

### 2026-09-23 静态闭环与下一轮单点候选

进一步追 `sub_147179530` 的物品分类字符串映射及 `sub_147196800` 对 itemdef+841 分类位图的设置，可确定 `[item category] clear avatar` 对应 bit 21；`random clear avatar` 才对应 bit 25。当前资源 `513560010.equ` 只有 `clear avatar`，没有 `random clear avatar`。`sub_14564D850` 的 bit25 分支创建 `CNRandomAvatar`，从对象类型侧再次印证 bit25 不是普通克隆装扮开关。旧候选的 bit25 前提错误。

据此下一轮仅改变同槽位共存时的 mode1 详细行：主模板仍为 Group0 克隆；row+24 改送 Group1 普通外观模板；row+28 清零。NOTI13/14 原有基础行、mode0 外观、数据库存档与 CMD19 换装均不动。这个变更与客户端 reader 分支吻合，但 UI 实际显示仍待用户手动重登验证，不能先宣称修复。若仍串位，下一步取客户端原生同部位克隆+普通外观的入场向量或实机动态命中，不继续叠加未知包。

隔离候选已编译为 `bin/wireprobe-avatar-visual-candidate.exe`（SHA-256 `15CB886CE11CA70C0C45B6DF461FA79EE76600D5408ECAED9A97E48BD8CB20BE`），`go test ./...` 和 `go vet ./...` 使用项目内隔离 Go 缓存均通过。没有覆盖 `wireprobe-handoff-source.exe` 或归档39版，也没有修改启动器 `server_binary`，所以当前客户端不会自行变化。下一轮用户手动验证前需在服务已停止后，将启动器临时指向此候选；只重登一次，记录两个页签的同一部位与最新服务日志。若不对，先恢复原启动目标，不继续叠包。

### 2026-09-23 13:35 实机结果：入场修好，实时换装未闭环

服务停止后已将 `server/launcher.local.json` 的 `server_binary` 临时指向隔离候选（旧 exe 未覆盖）。用户重登截图确认 Avatar 页显示普通外观、Clone 页显示克隆，入场投影假设得到实机支持。但 05:35:20 UTC 手动脱下 Clone 槽2 后，Avatar 槽2 也被清空，不能收口。

用户提供的 CMD19 明文以 `01 08 00 ... 03 02 00 da749c1e` 开始：list3 槽2 的克隆模板 `513570010` 被换到 list1 槽8。服务端返回 CMD19 成功，然后发 NOTI13 bag/list3 重整、NOTI14 源/目标单槽更新、NOTI14 list3 全量刷新、mode0 外观刷新。目标单槽 NOTI14 的模板为普通外观 `513572726`（`76 7f 9c 1e`），没有发送删除普通外观的行。用户约 05:37:24 UTC 又手动穿回该克隆，因此当前数据库再次同时包含 Group0 `513570010` 与 Group1 `513572726`；不能拿当前存档双件状态否定 05:35 的脱下操作。

当前 `equipment_flow.go` 的实时换装路径只重发 NOTI13/14 与 mode0 外观；与成功入场相比，它没有通过 mode1 的 `sub_1452C1540` 读取路径重新建立 row+24 的普通外观关联。客户端 NOTI14 reader `sub_1452E9810` 里没有对应的虚表偏移1472/1480 设置调用，这是当前最直接的时序差异，但尚未证明重复发送完整 mode1 USERINFO 在场景中安全。下一步先追当前客户端重复 mode1 的 existing-actor 分支、寻找原生同类运行向量，不能直接往换装末尾叠完整入场包。

### 2026-09-23 13:48 人工重登对照：确认只在实时刷新丢格

用户按“脱下前 → 脱下 Clone Face 后 → 不换装重登后 → 再穿上 Clone Face 后”提供四张截图：脱下瞬间普通 Avatar 的 Face 格变空，重登后同一格恢复，再穿上后普通 Avatar 格仍正常。最新运行目录 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_134634_853218_next37/` 的 05:48:09 UTC CMD19 为 list3 槽2 的克隆 `513570010` 换至 list1 槽8；同批 NOTI14 list3 槽2 仍发普通外观 `513572726`。05:48:18 UTC 有完整入场链（含 mode1 `entry_addition_sent`）；05:48:32 UTC 再穿回克隆成功。该对照验证存档和入场投影未丢普通外观，故剩余缺陷为脱下瞬间客户端本地格子/关联更新，不能以补数据库行或改入场模板解释。

IDA 对 `sub_1452C1540` 的调用点有 `sub_14563D400` 的 existing-actor 分支，但此分支还处理大量角色状态字段；仅此不足以证明实时换装后重发整个 mode1 USERINFO 安全。未改动当前运行编码、未追加未知包。后续先定位客户端 CMD19/NOTI13/14 对 Avatar/Clone 两页的本地对象更新和原生同槽双装扮换装向量，再决定单点服务端修复。

补充静态线索：NOTI14 `sub_1452E9810` 在已有槽位对象模板与来包模板不同的分支，会调用 `sub_145AD4750` 删除旧对象后再走新对象构建。此次脱 Clone 时 list3 槽2 从克隆模板切为普通外观模板，符合触发这一分支的必要条件；旧对象删除是否同时清掉 mode1 建立的普通外观关联，还需动态命中确认，不能把静态分支等同于最终根因。

### 2026-09-23 实时刷新单点候选（attempt 3/3；后经实机确认）

客户端 CMD19 成功 reader `sub_145283750` 对 list1↔list3 的成功移动调用 `sub_145ACBC90`，该函数会处理本地穿戴对象和角色槽位缓存。其后 NOTI14 在模板变化时可能删除/重建已有对象；mode0 USERINFO 只刷新模型，不能证明重建普通 Avatar 页关联。另一方面 `sub_14563D400` 的 existing-actor 分支会读取 mode1 USERINFO 并调用 `sub_1452C1540`，而前次人工重登证明该 reader 的详细行可以恢复普通外观。

因此隔离候选仅在已观察到的 CMD19 形态（list1 空背包格作 Source，list3 的 0–11 号槽作 Destination，且 DestinationItem 经当前 PVF 目录确认属于 `clear avatar`）时，保留全部既有响应，并在所有 NOTI13、NOTI14、mode0 外观帧之后追加当前角色完整 mode1 `EntryAddition`；重新穿上、普通外观、普通装备、宠物、背包内移动不追加。未改变存档、数据库结构或物品分组。此试验只验证“已知 mode1 reader 能否在实时脱下 Clone 后重建关联”，不把 existing-actor 分支等同于实机安全结论。若出现闪退、技能/UI 其他状态异常或格子仍空，立即恢复启动器的前一候选；不继续叠第四种包。

当前 `513570010.equ` 从 `equipment-full.data` 只读提取确认为 `[item category] clear avatar`。`go test ./...`、`go vet ./...` 均通过，独立程序 `bin/wireprobe-avatar-live-refresh-candidate.exe` SHA-256 为 `95275A922FBB656C857C0C860B39257437C003B64D98822C4363DB9C0A09EB3F`。未覆盖上轮 `wireprobe-avatar-visual-candidate.exe`。测试只需手动重登后从空 Avatar 背包格脱下同一件 Clone Face 一次，立即检查 Avatar/Clone 两页；若异常，停止当前会话并切回上轮候选。

服务进程与 7001 监听均无运行后，已把 `server/launcher.local.json` 的 `server_binary` 切到新隔离候选，并复核目标存在及 SHA-256；之后由用户手动启动并完成实机验收。

### 2026-09-23 实机确认基线

用户确认修复正常：登录后普通 Avatar 页显示普通外观、Clone 页显示克隆装扮；脱下 Clone 部位后，普通 Avatar 对应格不再被清空，Clone 装扮仍可正常穿脱。实时脱下路径只对已识别的 Clone 装备移除请求，在既有 NOTI13/14 与 mode0 刷新之后补发当前角色完整 mode1 `EntryAddition`；其余 CMD19 操作不触发该补发。未改变存档数据、分组规则或数据库结构。该行为作为当前已确认基线。

构建及静态验证：`go test ./...`、`go vet ./...` 均通过。实机运行的隔离候选 SHA-256 为 `95275A922FBB656C857C0C860B39257437C003B64D98822C4363DB9C0A09EB3F`；候选二进制与本地启动配置不纳入版本控制。
