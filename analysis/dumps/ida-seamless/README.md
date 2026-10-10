# 小深渊「无缝续刷」客户端逆向资产（2026-10-07）

## 怎么复跑

```bash
# 副本（原库 client/DFO.exe.i64 是权威资产，绝不在原库上跑）
mkdir -p server/work/dfo-lan/runtime/ida-seamless
cp client/DFO.exe.i64 server/work/dfo-lan/runtime/ida-seamless/copy.i64
# 单轮只做一件事；输出走绝对路径
"/d/tools/ida94/idat.exe" -A \
  -L"D:\115us\server\work\dfo-lan\runtime\ida-seamless\idaN.log" \
  -S"D:\115us\analysis\dumps\ida-seamless\ida-probeN.py" \
  "D:\115us\server\work\dfo-lan\runtime\ida-seamless\copy.i64"
```

成本实测：打开 486 MB IDB ≈ 40 s；只反编译 1 个函数 ≈ 36 s；全库 3,662 万条指令的位移扫描 ≈ 6 分 20 秒。
（技能 `ida-headless-survey` 的成本纪律：一轮一个目标；含反斜杠的脚本先写 `.py` 再执行。）

## 这一轮的四个探针

| 探针 | 做什么 | 产物 |
| --- | --- | --- |
| 1 | 12 个字符串地址 → 代码引用点（含对照组） | `probe1-out.txt` |
| 2 | 反编译 `sub_145243AF0`（CMD2062 注册）+ `sub_146D1AC90`（Proc_SeamlessLoading） | `probe2-out.txt` |
| 3 | 全库扫 `+508h`（模块状态字段）的读写 | `probe3-out.txt` |
| 4 | 反编译 `sub_146D19550`（Proc_FastLoading）+ `sub_146D29700` | `probe4-out.txt` |

字符串表（**先查它、不必开 IDB**）：`analysis/dumps/xorstr_addr_to_text.json`，
取法 `json.load(...)` 后 key 是 16 进制地址字符串，值是解密后的文本。

## 结论（L0 级证据）

1. **客户端有无缝加载子系统**：
   `CNSelectDungeonModule::Proc_FastLoading` → `Proc_SeamlessLoading`
   + `onEnterModule_SeamlessLoading` / `onExitModule_SeamlessLoading` / `CNTownModule::onStartSeamlessLoading`。
2. **状态字段 = `CNSelectDungeonModule + 0x508`（1288）**，取值 1/2/3/4；
   `Proc_SeamlessLoading` 的第一句就是 `if ( *(this+1288) == 3 )`。
   全模块区只有 6 个函数写它，**把 3 写进去的只有一处**：
   `Proc_FastLoading`（`sub_146D19550`，源文件行 10395；同处把 `+0x52C(1324)` 写成 4）。
   ⇒ 无缝加载是**客户端自己的状态机**推进的，不是某个服务端通知直接置位。
   我们的重开**已经在走这条路**（客户端 trace 里就是它的日志
   `[SelectDungeon] cutSceneState = %d, loadingstate = %d, dungeon index = %d`，`loadingstate` = `+0x860(2144)`）。
3. **`.dgn` 键解析器**：`[direct eplp on clear dungeon]`、`[direct move keep state]`、
   `[keep buff and summons]` 三者都在同一个函数 `sub_147444B30` 里读
   ⇒ 客户端对「直进」有明确的**状态延续策略**，小深渊的 `.dgn` 写的是
   `[keep buff and summons] 2`（＝延续 buff 与召唤物）。
4. `is direct move area` / `direct move next phase guarantee delay time` 在 `sub_1476E0120`
   —— 那是**军团数据集**（semi-raid）解析器，不是副本 `.dgn`。
5. `[ENTRY] state : %s` 出自 `CNEntryDungeonPartyManager::setEntryState`
   —— 是**队伍进入/同步**状态，**不是**「进本方式」，别拿它当判据（本轮先误读过一次）。

## 由此得到的修复

`dungeon_buff_enhancement_restored`（NOTI1361）在本仓就是「增益强化注册」，
本仓三处测试断言写着 `buff registration must bind the new actor` ——
它的作用就是**把注册的 buff 重新绑到重建出来的角色上**。
实机 `events.jsonl` 显示首次进本与**无缝重开各发一次**（11:08:42 / 11:08:54，载荷 `28 00 00`）
⇒ 这就是玩家看到的「自动上 buff」。
`.dgn` 要求的是**延续**，所以无缝续刷这一帧必须跳过（`seamlessRetry` 标志，见 `next173 §10`）。
