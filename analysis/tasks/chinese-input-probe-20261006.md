# 中文输入（IME）取证任务 —— 2026-10-06

目标：查清 115US 客户端「无法输入中文」断在哪一环，并交付一个可装/可卸的 schema 2 mod。

## 1. 已确证的静态事实（可直接引用）

### 1.1 客户端自带的 `dinput8.dll` 是汉化代理（`DFO localization proxy v2`）

`C:\Game\dof\115us\DFO\dinput8.dll`（107,008 B，非系统 dinput8）导出 6 个 DirectInput 转发函数，
**不导出** `StartLocalization`。它的加载逻辑（`dumpbin /disasm` 逐条读过，RVA 0x1000 起的加载函数）：

```
GetModuleHandleExW(PIN|FROM_ADDRESS, 自身地址) → 取自己的模块路径
wcsrchr(path, '\\') → 把文件名部分替换成 L"ChineseLocalization.dll"（.rdata 0xF3A0）
LoadLibraryExW(<客户端目录>\ChineseLocalization.dll, NULL, 0x900)
GetProcAddress(h, "StartLocalization")（.rdata 0xF3D0，ASCII）
call 它（**无参数**），返回值即代理线程退出码
```

代理自己的失败码：`1` InitOnce/GetModuleHandleEx 失败、`2` GetModuleFileNameW 失败、
`3` 路径里没有反斜杠、`4` LoadLibrary 失败、`5` GetProcAddress 失败。

> ⚠️ 早期一次字节扫描把 `GetdfDIJoystick\0` 尾字节 `k` 与后面的 UTF-16 `Chinese…` 连成了
> `kChineseLocalization.dll` —— 那是**跨边界假命中**。逐字节 hexdump（`proxy_rdata_hex.txt`）
> 证明真实文件名是 **`ChineseLocalization.dll`**（无 `k`）。

**该文件当前在客户端目录里不存在** → 代理每次启动都停在 `4`，等于汉化/输入扩展只装了一半。

### 1.2 客户端自己带 IME 代码

`DFO.exe` 的真实导入表每个 DLL 只有一个代表函数（`DINPUT8.dll!DirectInput8Create`、
`IMM32.dll!ImmReleaseContext` …），其余 API 全靠运行时按名字解析（Themida 壳 + 自定义
加载器，xorstr 表里有 `IMM32.DLL`）。`.rdata` 里有一组 IME 相关明文：

```
\imm32.dll + ImmLockIMC / ImmUnlockIMC / ImmLockIMCC / ImmUnlockIMCC
TINTLGNT.IME / CINTLGNT.IME / MSTCIPHA.IME / PINTLGNT.IME / MSSCIPYA.IME
Default / UnKonwn、GetReadingString、ShowReadingWindow
software\microsoft\windows\currentversion\、MSTCIPH、TINTLGNT、Keyboard Mapping
```

这是微软《Using an IME in a game》样例（SDL `SDL_windowskeyboard.c` 同源）的 IME 实现：
**直接锁 IMC/IMCC 读组字缓冲**，并且带一份 XP 时代的 IME 白名单。
⇒ 客户端有 IME 通路，但**是否被启用、走哪条分支，必须实测**。

### 1.3 本机已无 IDA

`D:\tools\ida94\idat.exe` 已不存在（全盘搜 `ida*.exe` / `idat.exe` 均无命中），
`D:\ida-work-primer\` 只剩 IDB 副本与日志。⇒ 本轮静态分析只能靠 `dumpbin` + 自己写的 PE 解析，
函数级逆向**暂时做不到**，改走动态取证。

## 2. 本轮交付：诊断探针（mod `qol.chinese-input-probe`）

- 源码 `client-patchs/chinese-input/src/chinese-input-probe.c`（C，MSVC x64、`/MT`）；
- 构建 `build-probe.cmd`（cl + vcvars64），自测 `smoke-test.cmd`（自测宿主按代理的方式加载并调用）；
- 打包 `build-mod.py` → `dist/chinese-input-1.0.0.zip`（client 层 `file.add ChineseLocalization.dll`）。

探针做的事（**只观察，包装后原样转发**）：

1. 记录键盘布局 / IME 名称 / 是否 IME；
2. **EAT 钩子**（导出地址表）挂在 `imm32.dll` 23 个 IME API、`user32` 的
   `CreateWindowExW/A`、`DestroyWindow`、`kernel32!GetProcAddress` 上，逐条记调用与调用栈；
3. 子类化本进程窗口，记 `WM_CHAR` / `WM_KEY*` / `WM_IME_*`；
4. 每 200 ms 轮询前台窗口的输入法状态（开启状态 + 组字串），变化即记。

### 2.1 EAT 钩子的正确做法（踩过的坑）

EAT 里存的是 **32 位 RVA**，`GetProcAddress` 做 `base + RVA`。
第一版把 64 位函数指针直接写进 4 字节槽 → 写坏相邻表项、算出的地址也是错的，
自测宿主直接 `0xC0000005`。
正确做法（现版本）：在目标模块 ±2GB 内 `VirtualAlloc` 一页，写 14 字节绝对跳转桩
（`FF 25 00000000` + 8 字节目标地址），再把 EAT 的 RVA 指向桩；改完用
`GetProcAddress` **自检**（日志里每条钩子都有 `GetProcAddress=桩 OK`）。

### 2.2 沙箱坑（做别的任务也会撞到）

在 DSH 沙箱里，**可执行文件放在工作区内时，它写工作区外会被拒**（`mkdir/open … Access is denied`），
而 `pwsh` / `cmd` / `go run` 的临时产物写同一路径没问题。
⇒ 需要写工作区外的辅助程序（modkit 等）**要放到 `%TEMP%` 再跑**。

### 2.3 宿主 / 插件（两个客户端 mod 的落位问题）

客户端**只有那一个**可自动加载的 DLL 槽位，而 modkit 规定「同目标路径两个 mod 冲突即拒绝」，
所以第二个客户端 mod 没有地方落位。本轮把那个槽位做成插件目录：

```
<客户端>\ChineseLocalization.dll      ← 宿主（mod id: qol.client-host）
<客户端>\.115us-mods\*.dll            ← 客户端 mod 插件（file.add 落位）
```

- 宿主 `client-patchs/client-host/src/client-host.c`：按文件名字典序枚举插件，每个插件起一个线程
  调它的 `ModStart`（也认 `StartLocalization`），失败不影响其它插件；插件一律 `PIN` 到进程结束。
- 插件 ABI 与边界写在 `client-patchs/client-host/HOST-README.md`。
- 探针据此改成插件（v1.1.0，`requires: ["qol.client-host"]`，目标 `.115us-mods/ChineseInputProbe.dll`），
  同时保留 `StartLocalization` 导出，因此仍可单独占槽位调试。
- 端到端自测：`client-patchs/client-host/smoke-test.cmd` 造一个假客户端目录，
  宿主 → 加载插件 → 插件装钩子 → 两份日志分别落在各自目录，进程 0 退出。
- `modkit plan` 在宿主未装时对探针给出 `冲突：依赖未安装：qol.client-host`（退出码 2），
  装上宿主后 0 阻断 —— 依赖门禁本身也验证过了。

当前客户端已装状态（`modkit status` 三项 `[一致]`）：

| 文件 | 大小 | 所属 mod |
| --- | --- | --- |
| `C:\Game\dof\115us\DFO\ChineseLocalization.dll` | 166,400 | `qol.client-host` v1.0.0 |
| `C:\Game\dof\115us\DFO\.115us-mods\ChineseInputProbe.dll` | 186,368 | `qol.chinese-input-probe` v1.5.0 |
| `C:\Game\dof\115us\DFO\.115us-mods\AutoConfirmDelete.dll` | 162,304 | `qol.auto-confirm` v1.0.0 |
| `.115us-mods\*.ini` | — | **各插件首次运行时自己生成**（不随包落位，见 §4.6） |

### 2.4 mod 往返验证（第 11 轮：install → uninstall → install）

按"可 verify/plan/install"的要求实测了一遍完整往返：

1. **卸载三个**（插件先、宿主后）：每条都是 `还原 0 / 删除 1`，`modkit status` → `没有已安装的 mod`；
2. **现场回到原状**：`ChineseLocalization.dll` 已删除；`.115us-mods\` 剩一个空目录（modkit 只删文件，
   不删它建的目录 —— 无害，记录在此免得以后误以为有残留）；
   两个**不属于本 mod** 的关键文件逐字节未变：`dinput8.dll` = `71E31AF1…`、`Script.pvf` = `BCF02F20…`；
3. **重装三个**（宿主 → 探针 → 自动确认）：各自 `1 步执行`，`status` 三项 `[一致]`；
4. **现场哈希 = 包内声明**：`ChineseLocalization.dll`、`ChineseInputProbe.dll`、`AutoConfirmDelete.dll`
   三者逐个核对 `一致 ✔`；
5. 顺带复证依赖门禁：宿主没装时装探针会被 `plan` 挡下（`冲突：依赖未安装：qol.client-host`）。

⇒ 这套交付确实可逆、可复现，且不会波及客户端里原有的 `dinput8.dll` 代理与 PVF。

## 3. 下一步（等实机日志）

> **第 3 轮更新**：已静态定位到门禁根因并交付修复开关（见 §4）。
> 这一轮跑游戏**同时**验证两件事：观察日志 + 门禁修复是否让中文输入生效。
> 日志路径也变了（插件在 `.115us-mods\` 下）：

1. 用启动器起游戏，切中文输入法在能打字的地方打几个字（拼音→候选→上屏），再打几个英文字母；
2. 退出游戏，取 `C:\Game\dof\115us\DFO\.115us-mods\chinese-input-probe.log`
   与 `C:\Game\dof\115us\DFO\client-host.log`；
3. `python client-patchs/chinese-input/analyze-log.py <log>` 出摘要，按四种判读选修法：
   - 收不到 IME 消息、也不调 imm32 → 主动给窗口挂 IME 上下文 + 把组字喂进客户端；
   - 自己调 imm32 但没有 IME 消息 → 它走的是直读 IMC 缓冲那条路，重点看调用者是不是 DFO.exe；
   - 有 IME 消息但无非 ASCII 上屏 → 客户端收到组字却没消费（过滤/未提交）；
   - 已有非 ASCII 字符到窗口 → 问题在更后面的渲染/提交环节。

## 4. 静态取证：门禁根因（2026-10-06 第 3 轮）

### 4.1 方法（没有 IDA 也能做）

1. 先证明**静态分析与 IDB 可以对上**：`DFO.exe` 与 `DFO.ida-analysis.exe` 逐块比对，
   **258,972,712 字节里只有 12 字节不同，且都在前 64 KB（PE 头）** ⇒ .text/.rdata 完全一致，
   IDB/分析 Dump 里的地址对出货的 EXE 一一对应。
2. `dumpbin /disasm` 全量反汇编（3.1 GB 文本），按地址区间抽取。
3. 自己写了个 **RIP 相对引用扫描器**（`lea/mov reg,[rip+disp32]` 的字节模式），
   把引用 IME 字符串的指令全找出来（120 处，全部集中在 RVA 0x6F1A700–0x6F31000 一个模块里）。

### 4.2 证据链（RVA 0x6F22400 起的函数）

```
mov rbx, rcx                     ; rcx = IME 管理器对象
mov rcx, [rcx+68h]               ; ← 该对象里的键盘布局（HKL）
cmp rcx, 0E0080404h ; je ok      ; 只认这三个
cmp rcx, 0E0090404h ; je ok
cmp rcx, 0E00E0804h ; jne 拒绝    ; 其它一律拒绝
ok:
  rcx=[rbx+68h]; rdx=&buf; r8d=3FFh; call ImmGetIMEFileNameW     ; ← 取输入法文件名
  test eax,eax ; je 拒绝                                          ; 返回 0 也算拒绝
  cmp qword [rbx+98h],0 ; jne 接受                                 ; 已经有函数指针就直接用
  ; 逐个和 5 个名字比：CompareStringW(0x409, NORM_IGNORECASE, buf, -1, 名字, -1)（返回值 2 = CSTR_EQUAL）
  "TINTLGNT.IME" / "CINTLGNT.IME" / "MSTCIPHA.IME" / "PINTLGNT.IME" / "MSSCIPYA.IME"
  都不等 → 拒绝
拒绝（0x6F226A0）：直接恢复栈帧返回，`[obj+98h]` 保持 0 —— **IME 子系统整条不被启用**。
```

⇒ **客户端只认 3 个 XP 时代传统 IME 的 HKL + 5 个传统输入法文件名。**

### 4.3 本机实测（决定性）

用 `GetKeyboardLayoutList` + `ImmGetIMEFileNameW` + `ImmIsIME` 直接查本机：

```
count=1  foreground=08040804
hkl=08040804 isIME=True ime="" (r=0) desc="" (r=0)
```

两个门禁**同时过不去**：HKL 不在那三个里，`ImmGetIMEFileNameW` 直接返回 0
（TSF 输入法没有传统 IME 文件）。⇒ **中文输入链路永远不会被启用**，与用户现象一致。

### 4.4 交付的修复（`chinese-input.ini` 控制，可随时关掉）

| 开关 | 作用 |
| --- | --- |
| `fix_layout=1` | `GetKeyboardLayout`（**含 Themida 重建 IAT 后的调用**——EAT 钩子与解析方式无关）对中文布局改报传统 IME HKL：`0x0804→E00E0804`、`0x0404→E0080404` |
| `fix_ime_name=1` | `ImmGetIMEFileNameW` 在客户端拿不到白名单名字时，写入 `ime_name`（默认 `CINTLGNT.IME`）并返回长度 |
| `post_layout=1` | 给**每个新建的顶层窗口**各补一条 `WM_INPUTLANGCHANGE(wParam=传统 HKL)`（微软样例就是在消息里更新布局字段的；不知道游戏用哪个窗口收发，就都发一次） |

> 第 6 轮把 `post_layout` 从"只发一次"改成"每个顶层窗口发一次"并默认打开：
> 静态分析**无法确定**客户端是从 `GetKeyboardLayout` 还是从 `WM_INPUTLANGCHANGE` 取那个 HKL
> （客户端多数 API 由 Themida 重建 IAT 解析，文件里没有可枚举的描述符表——全盘只找到 2 个
> `\模块.dll` 描述符，且只有 imm32 那个带名字）。三条路一起覆盖，代价是多几条无害消息。

**只对游戏主模块生效**：MSCTF（TSF 输入法框架）在同一进程里也走 imm32 导出，
把它的布局/文件名换掉会真的把输入法搞坏，所以包装里用
`GetModuleHandleExW(FROM_ADDRESS)` 判断调用方模块，系统模块照实转发（日志里标 `[系统模块]`）。

验证（`client-patchs/client-host/smoke-test.cmd`，自测宿主 + 宿主 + 插件全链路）：

```
配置 …：fix_layout=1 fix_ime_name=1 post_layout=0 ime_name=CINTLGNT.IME zhcn=0xe00e0804 zhtw=0xe0080404
GetKeyboardLayout(tid=0) -> 0000000008040804  ← 改报传统 IME HKL 00000000E00E0804 [主模块]
ImmGetIMEFileNameW(hkl=00000000E00E0804, cap=128) -> 12 "CINTLGNT.IME" [主模块]  ← 已替换成白名单名字
GetKeyboardLayout(tid=0) -> 0000000008040804 [系统模块]     ← 系统调用不动
```

（踩到的坑：ini 里 HKL 是十六进制，第一版用 `atoi` 解析成 0，会让 `GetKeyboardLayout`
返回 NULL —— 现在用 `ini_hex`，且解析为 0 时回落到默认值。）

### 4.6 顺带做出来的两件事（第 3 轮尾）

1. **C2S 发包追踪开关**（`chinese-input.ini` 的 `trace_c2s`，默认 0）：
   按 `analysis/dumps/CLIENT-MECHANICS.md` §2 的发包链，对
   RVA `0x6D746E0`（写 opcode）与 `0x6D75B10`（追加 body）做 **5/6 字节 inline 跳转**。
   - 这两处开头是位置无关的完整指令（`40 55 56 57 41 56` / `48 89 5C 24 08`，
     已用脚本对出货 EXE 逐字节核对通过）；发送函数 `0x6D75AF0` 首字节含 rel32 call，搬不动，**不钩**。
   - 装钩子前先逐字节比对现场，对不上就跳过（不盲改）。
   - 用途：任务（2）需要知道"删角色"到底发了哪个 opcode、body 里的名字是什么。
2. **开关文件改成插件自生成**：modkit 的 `file.add` 不允许覆盖「已存在且内容不同」的文件，
   把 ini 放进包里会让 v1.2.0 → v1.3.0 的升级**被 plan 阻断**
   （`目标已存在且内容不同（file.add 不允许覆盖）`，退出码 2，即使 plan 同一行还写着"将被视为升级"）。
   这是引擎升级路径与 `file.add` 语义的冲突（**框架级发现**，值得反馈给 modkit）：
   只含 `file.add` 的 mod **无法原地升级被改过的文件**。
   绕法：先 `uninstall` 再 `install`；根治办法是本 mod 采用的——开关由插件首次运行时生成，
   包内只留 DLL，升级只换一个文件。

### 4.7 inline 钩子机制的独立自测（第 5 轮，抓到两个真 bug）

`trace_c2s` 要用 inline 跳转改客户端代码，是整套东西里最危险的部分，所以单独写了个
**不碰客户端的机制自测**（`client-patchs/tests/inline_test.c`）：在本进程里手写一段
机器码 `mov eax,ecx; add eax,5; ret`（入口字节完全确定，不依赖编译器），对它装钩子、
调用、核对返回值与包装调用次数。

```
目标入口字节：8b c1 83 c0 05
装上：目标 00007FF7CF380000，桥 00007FF7CF390040，跳板 00007FF7CF390000（覆盖 5 字节）
f(10)=15 f(100)=105 包装调用=2 次
PASS：inline 跳转 + 跳板转发 + 返回值都正确
```

自测一次就暴露了两个**会在真机上崩**的缺陷，都已修：

| 缺陷 | 现象 | 修法 |
| --- | --- | --- |
| 桥（近地址桩）与目标的距离没检查 | 目标离主模块远时 rel32 越界 → 跳飞 → `0xC0000005` | 装钩子前算 `|bridge-target|`，超 ±2GB 就跳过并记日志 |
| 读目标字节前没验证内存可读 | 换一个客户端构建时 RVA 不在镜像里 → `memcmp` 直接崩 | 先 `VirtualQuery` 确认已提交、可读、长度够，否则跳过 |

第二条是用"负面测试"验的：把插件装到 `test-host.exe`（不是 DFO.exe）上并要求 `trace_c2s=2`，
日志应当出现跳过而不是崩：

```
inline 钩子[writeOpcode] 跳过：目标 00007FF6E0B846E0 不在已提交内存里（客户端版本不同？）
inline 钩子[appendBody]  跳过：目标 00007FF6E0B85B10 不在已提交内存里（客户端版本不同？）
```

（真机上这两个 RVA 在 DFO.exe 的 .text 里，且期望字节已逐字节核对通过，所以会正常装上。）

### 4.8 门禁复刻自测（第 10 轮）：不开客户端的最强证据

`client-patchs/tests/gate_test.c` 在本进程里**复刻客户端那道门禁**（两条判据、同样的
接受集合），然后加载插件再跑一遍。关键点：所有 API 都用 `GetProcAddress` 现取，
这样才会拿到插件装在导出表里的桩。

```
=== 装插件之前 ===
  第一步 键盘布局 = 0x08040804 → 不过（客户端会直接放弃 IME 通路）
门禁 = 不启用

=== 装插件之后 ===
  第一步 键盘布局 = 0xe00e0804 → 通过
  第二步 ImmGetIMEFileNameW → 12 "CINTLGNT.IME"
门禁 = 启用

PASS：插件把客户端形状的 IME 门禁从未通过变成通过
```

⇒ 门禁逻辑本身已被证明能翻；剩下唯一变量是"客户端那个字段是不是从 `GetKeyboardLayout` 取的"
（已在 §4.8 第 3 条登记，探针日志可判）。

另：第 10 轮还把 `client-patchs` 下所有 `.cmd` 归一化成 **纯 ASCII + CRLF + 无 BOM**
（自查发现新写的脚本是裸 LF、`build-gate.cmd` 里混了中文 —— 都违反根 §0.4.2 的 `.cmd` 环境要求，
已按规范修正并复跑全部自测）。

### 4.10 ★ 第一次实机（2026-10-06 15:32）否掉了第 3 轮的修法，找到了真凶

用户实机跑了一次（进程存活 15:32:00–15:33:00），三个日志都产出了。结论：

**（1）框架部分全部正常**

- 宿主加载了 2 个插件（`client-host.log`），**BlackCipher 没有拦**；
- 自动确认插件在真客户端上装上了 inline 钩子（`delNotiInit 已装：00000001411F3EC0`），
  并取到确认短语 **`"删除角色"`**（汉化后确实是中文，与 §6.3 推断一致），
  还把编辑框文本替换掉了（`★ 已把编辑框文本替换为确认短语 "删除角色"`）。

**（2）中文输入仍然不行，而且原因和我第 3 轮的判断不同**

探针日志 687 行里最关键的几条：

| 现象 | 数据 |
| --- | --- |
| 组字确实在发生 | `WM_IME_STARTCOMPOSITION` / `WM_IME_COMPOSITION` / `WM_IME_ENDCOMPOSITION` 各 **35 次**，`WM_IME_NOTIFY` 287 次 |
| 但每次只组**一个字母**就结束 | 探针读到的组字串是 `'s'`×11、`'d'`×9、`'a'`×7、`'f'`×3…，**从不累积**成拼音 |
| 从没有结果串 | **`RESULTSTR` 出现 0 次**，也没有任何中文 `WM_CHAR`（只有 `0x8` 退格、`0xd` 回车、以及 ASCII 字母） |
| **客户端一次都没进我们的钩子** | 运行期 `Imm*` 调用记录 **0 条** |
| `GetKeyboardLayout` 35 次全是系统模块调的 | `-> 0000000008040804 [系统模块]`，我们的 `fix_layout` 一次都没生效 |

**（3）真凶：客户端是 Themida 加壳的，它调 user32/imm32 走自己的导入桩**

静态复查门禁函数（RVA `0x6F22400`，`.pdata` 边界 `0x6F22400..0x6F226C6`）时发现它调
`ImmGetIMEFileNameW` 的指令是 `call 0000000148A6EDCA` —— 而那里是：

```
0x148A6EDCA:  FF 25 38 74 71 00    jmp qword ptr [0x149186208]
```

即 **Themida 自己的导入桩**（`FF 25` + 槽）。静态扫全 .text：这类桩共 **482 条**，
其中 442 条指向 `.rdata`，槽集中在 RVA **`0x9186160..0x9187988`（297 个）**。
槽里存的是 **Themida 解包时提前解析好的真实函数地址**。

⇒ 我们改的是**导出表（EAT）**，而客户端根本不走导出表 —— 所以第一次实机里
客户端一次都没进钩子，门禁读到的仍是真实 HKL `0x08040804`，直接被拒，
客户端从不处理输入法组字（组字被它自己取消/忽略）。

**（4）v1.6.0 的修法（两条，都已在本机自测）**

| 开关 | 做法 |
| --- | --- |
| `fix_slots=1` | `fixup_import_slots()`：在 `0x9186100..0x9187A00` 逐 8 字节**精确匹配**我们的原函数地址（`原 == spec[i].orig`）→ 换成我们的包装。因为槽是解包时才填的，主循环每 200 ms 重扫一次（幂等，扫完就没了）。只扫主模块镜像内（用 PE `SizeOfImage` 兜底，换小 EXE 时整段跳过） |
| `fix_gate=1` | `try_install_gate()`：inline 钩住门禁本体 **RVA `0x6F22400`**（9 字节 = `push rbp; sub rsp,450h`，期望字节 `40 55 48 81 EC 50 04 00 00`，已对出货 EXE 逐字节核对）。入口把 `mgr->[0x68]` 强制改成 `0xE00E0804`，并记录原值、`mgr->[0x98]`。字节对不上（Themida 未解包）就静默等下一轮，不刷日志 |

**（5）v1.7.0 再加一层：布局**写入点**（第 12 轮尾）**

静态搜「谁写 `[obj+0x68]`」（在 IME 区域按 `48 89 /r 68` 扫，9 个命中里排除 7 个栈写入）定位到
**RVA `0x6F23070`**（`.pdata` 边界 `0x6F23070..0x6F2336B`）：

```
rbx = r9;                     // 第 4 个参数 = HKL
rdi = rcx;                    // 第 1 个参数 = IME 管理器
*(u64 *)(rcx + 0x68) = rbx;   // ★ 门禁读的那个字段，就是在这里被写进去的
*(u16 *)(rcx + 0x70) = (u16)rbx;
call 0x6F222F0(lang);         // 按语言 ID 分发，后面接跳转表
```

⇒ 客户端拿 HKL 的路子（`GetKeyboardLayout` 或 `WM_INPUTLANGCHANGE` 的 wParam）最后都落到这个函数。
于是加第三个开关 `fix_gate` 的下半部分：inline 钩 `0x6F23070`（5 字节 `48 89 5C 24 18`），
**只把中文布局**（低 16 位 = `0x0804`/`0x0404`）换成传统 IME HKL，其它语言原样转发。

三层保险各管一种时序：

| 层 | 覆盖的情况 |
| --- | --- |
| 槽修正 | 客户端**之后**还会调 `GetKeyboardLayout`/`ImmGetIMEFileNameW`（走它自己的导入桩） |
| 布局写入点 | 客户端从**任何**来源拿 HKL 写进管理器（含 `WM_INPUTLANGCHANGE`） |
| 门禁本体 | 门禁被调用时兜底改字段（哪怕前面两层都错过） |

另外这一版开始记录「**插件进场时客户端进程已运行多少秒**」——用来判断门禁是不是在我们来之前就跑过了
（第一次实机里，探针 0 ms 时游戏窗口就已经存在，说明我们进场偏晚）。

**（6）v1.8.0：把两条备用路先做好（默认关），省一次来回**

下次实机日志无论指向哪一边，都不用再改代码、只需要改 ini：

| 开关 | 默认 | 用在什么证据下 |
| --- | --- | --- |
| `fix_cancel` | 0 | 日志里若出现客户端**主动取消组字**（`ImmNotifyIME(himc, action=0x15, idx=0x4, …)`，即 `NI_COMPOSITIONSTR`+`CPS_CANCEL`），说明组字是被客户端自己掐断的 ⇒ 打开它把这一调用吞掉 |
| `ime_bridge` | 0 | 门禁修好、组字也能上屏，但客户端**自己不读结果串**（看不到 `ImmGetCompositionStringW` 的 RESULTSTR 调用）⇒ 打开它，由我们把结果串逐字投成 `WM_CHAR`（客户端自己的文本输入是吃 `WM_CHAR` 的：第一次实机里 ASCII 字母就是这样进去的） |

两个都默认关，是因为它们都可能与客户端自身行为重复（尤其 `ime_bridge` 会导致字插两遍）。
两个开关都只在真客户端里才可能生效（槽修正之后客户端才会走我们的 imm32 包装）。

门禁函数的完整逻辑（本轮重新反出来的，比 §4.4 更准）：

```
mgr->[0x88] = 0;
hkl = mgr->[0x68];
if (hkl ∉ {E0080404, E0090404, E00E0804})            goto REJECT;
if (ImmGetIMEFileNameW(hkl, buf, 0x3FF) == 0)        goto REJECT;
if (mgr->[0x98] != 0)                                goto ACCEPT;   // 已有 IME 对象
for (5 个白名单名字)  if (strcmp(buf, name) == 0)     goto ACCEPT;
REJECT: return;            // IME 通路不启用
ACCEPT: 转换文件名、建立 IME 对象…
```

即：**HKL 那条由门禁钩子兜底，文件名那条由槽修正后的 `ImmGetIMEFileNameW` 包装负责。**

### 4.12 Themida 导入槽的端到端自测（第 14 轮）：机制已被证明可用

第 12 轮判定"三层保险"里最关键的一层是**槽修正**（客户端只从它自己的导入桩调 API），
但那一层一直没被真正跑过。于是造了一个**假客户端**（`client-patchs/tests/slot_test.c`）来验它：

1. 用一个 `0x9187000` 字节的 `.bss` 占位数组把镜像 `SizeOfImage` 撑到 `0x91ad000`，
   使插件的扫描区间 `0x9186100..0x9187A00` 真的落在本进程镜像里（`.bss` 不占文件体积；
   数组必须被引用，否则 `/O2` 会把它消掉）；
2. 在 RVA `0x9186200` 放一个"槽"，槽值 = 加载插件**之前**取到的真 `GetKeyboardLayout`
   —— 这正是 Themida 桩表里存的东西；
3. 按 dinput8 代理的方式加载插件（`LoadLibraryExW` + `StartLocalization`），等一个扫描周期；
4. 再从槽里读函数指针并调用。

```
镜像大小=0x91ad000，槽 00007FF60ACC6200 落在 [00007FF60ACC6000, 00007FF60ACE7000) 区间内
镜像基址=00007FF601B40000 槽=…（RVA 0x9186200）槽值=真 GetKeyboardLayout 00007FFAAF3281C0
装插件前，通过槽调用 -> 0x08040804
装插件后，槽值=00007FFA8B911B80（已被换成包装）
装插件后，通过槽调用 -> 0xe00e0804
PASS：客户端那条导入桩被接管，中文布局拿到了传统 IME HKL
```

⇒ **槽修正这条链在本机被端到端证明**：能认出来、能换成包装、包装返回的 HKL 正是门禁要的那三个之一。
配合 §4.8 的门禁复刻自测，"客户端的 HKL 门禁能不能过"在离线状态下已经全部有据。

（这个自测写的时候还踩了一次自己的坑：`.bss` 数组没人引用被 `/O2` 消掉，导致槽地址不可写而崩；
已在测试里加"镜像大小 + `VirtualQuery` 可写"两道自检，并把它记在这里免得以后重复踩。）

### 4.13 ★★ 实机成功（2026-10-06 16:02–16:03，业主确认「已经好了」）

在 v2.0.0（`fix_cancel=1` + `ime_bridge=1`，其余层沿用）上的一次实机运行，日志（`chinese-input-probe.log`，
1,543 行）显示链路完整打通：

```
★ 吞掉 ImmNotifyIME(CPS_CANCEL) ×43                  ← 客户端不再每次按键取消组字
组字串开始累积：'a' → "a's" → "a's'd" → "a's'da"      ← 拼音能连着打了
WM_IME_COMPOSITION … flags=0x1c00 RESULTSTR
ImmGetCompositionStringW(idx=0x800, cap=6) -> 6 "模拟器"   ← 选字上屏，客户端自己把中文读走
★ ime_bridge：0 条                                    ← 兜底一次都没触发（防重复判定生效）
转换模式=0x401（中文/原生）
```

**结论**：`fix_cancel` 是最后一块拼图 —— 客户端在"门禁已过、已在读组字串"之后，仍然每按一键就
`ImmNotifyIME(NI_COMPOSITIONSTR, CPS_CANCEL)`，所以拼音永远只有一个字母、候选窗用不了；
把这个调用吞掉，组字就能跨按键累积，候选窗正常，上屏的中文由**客户端自己的代码**读走并插入
（不需要我们投 `WM_CHAR`，所以 `ime_bridge` 保持兜底身份、默认值虽为 1 但实际不介入）。

**开箱即用的默认配置**（v2.0.1 起写进编译期默认值与自动生成的 ini）：

| 开关 | 值 |
| --- | --- |
| `fix_layout` / `fix_ime_name` / `post_layout` | 1 |
| `fix_slots` / `fix_gate` | 1 |
| `fix_cancel` | **1**（实机必需） |
| `ime_bridge` | **1**（兜底；实机未触发） |

### 4.13.1 第二次实机（16:32）：门禁/组字都通了，但 `ImmIsIME` 拆台

同一套配置（`fix_cancel=1 ime_bridge=1`）再跑一次，日志（1,562 行）显示：

```
配置 …：fix_layout=1 fix_ime_name=1 post_layout=1 fix_slots=1 fix_gate=1 fix_cancel=1 ime_bridge=1
★ 吞掉 ImmNotifyIME(CPS_CANCEL) ×37
组字累积：'k' → 'ku' → 'kuan' → 'kuang'          ← 拼音完全正常（上次只有单字母）
转换模式=0x401（中文/原生）                        ← 输入法确实在中文模式
ImmGetCompositionStringW(idx=0x800) -> 10 "kuang"  ← ★ 上屏的是原始字母，不是汉字
ImmIsIME(hkl=00000000E00E0804) -> 0                ← ★ 客户端问"这布局是输入法吗"，Windows 答"不是"
```

⇒ 门禁与组字这两层已经确认可用；新的矛盾点在于**我们假装了 HKL、假装了 IME 文件名，却让
`ImmIsIME` 如实回答"不是输入法"** —— 客户端据此把输入当普通字母处理（commit 出 `kuang`）。

**修法（v2.0.2，`fix_isime=1` 默认开）**：`h_ImmIsIME` 对**游戏主模块**的调用、且 HKL 属于
我们报出去的那三个传统布局（或语言 ID 为 `0x0804/0x0404`）时，把 `FALSE` 改报 `TRUE` 并记日志；
其它调用者/其它布局照实回答。这样客户端拿到的"布局是传统 IME、IME 文件名在白名单里、这个布局
确实是输入法"三个答案是自洽的。

### 4.14 还没闭环的

0. **v1.9.0 补了一条关键诊断**：每个新窗口都记录 `himc / 开启 / 转换模式`，并且前台窗口的
   转换模式一变就记一行 —— `转换模式` 的 bit0（`IME_CMODE_NATIVE`）就是"中文/英文"开关。
   第一次实机组字串永远是单个字母、且没有任何中文上屏，有一个可能是**输入法当时其实在英文模式**；
   这条日志能直接判定（显示"中文/原生"还是"英文/字母数字"），不用再猜。
   同时它也会告诉我们 **`DNF_WND_CHAT`（聊天框子窗口）有没有输入法上下文**（`himc=NULL` 就是重点怀疑对象）。

1. **第二次实机验证（v1.6.0）**：槽修正 + 门禁钩子是否让客户端真的进 IME 分支。
   下一次日志要重点看四件事：
   - `Themida 槽修正[user32/imm32]：N 处`（N>0 才算真的把客户端那条路接管了）；
   - `★ 门禁钩子已装` 与 `★ 门禁[gate] …mgr+0x68 从 0x… 改成 0xE00E0804`；
   - 客户端有没有开始调 `Imm*`（组字处理）——第一次实机是 **0 条**；
   - 组字串是否开始**累积**（不再是单字母）、`RESULTSTR` 是否出现。
2. 客户端进入分支后还会尝试按 IME 文件名去拿 `GetReadingString`/`ShowReadingWindow`
   （RVA 0x6F23219/0x6F2322D 附近）；名字是假的，这一步大概率失败——按微软样例的写法
   失败应当只是"没有联想串"而不影响组字，需实机确认。
3. 若门禁没被调用过（日志里没有 `门禁[gate]` 行），说明 IME 管理器在插件加载前就已初始化完：
   那时要改成**直接改管理器对象字段**，需要先定位持有 mgr 的全局（用 `refscan.py call 0x6F22400` 找调用者）。

## 5. 卸载/回滚

```powershell
# modkit 要放在 %TEMP% 之类的**工作区外**路径再跑（见 §2.2 沙箱坑）
& "$env:TEMP\modkit-test.exe" uninstall --client C:\Game\dof\115us\DFO --id qol.chinese-input-probe
& "$env:TEMP\modkit-test.exe" uninstall --client C:\Game\dof\115us\DFO --id qol.client-host
```

（两个都是 `file.add` 新增文件，卸载即删除；不改 `DFO.exe`、不改内存里的函数体。
宿主被依赖时 modkit 会拒绝卸载，必须先卸插件。）

## 6. 任务（2）「去掉需要输入文字的确认」——静态取证

### 6.1 服务端侧（不可动）

- C2S `CMD 6`（`protocol.DecodeDeleteCharacter`）：`u16 roster slot` + `确认名字`（wide string）；
- `cmd/wireprobe/client_dispatch_character.go:566` 直接调 `gameStore.DeleteCharacter(slot, name)`；
- `internal/database/character_delete.go:26`：**服务端强制 `row.Name == name`**，注释写明这是刻意的
  防重放设计（「同一 slot + 精确确认名」防止旧请求删掉刚补位上来的邻居）。

⇒ **放宽服务端校验不是可选项**（拆掉一条有意的存档安全保证）。"自动确认"只能做在客户端。

### 6.2 客户端侧：确认框确实要打字，位置已定位

用同一套方法（xorstr 池 + RIP 相对引用扫描 + 全量反汇编）：

| 发现 | 证据 |
| --- | --- |
| 删除角色确认窗 | `UI/TemplatePopupWindow/Contents/CharacterDeleteConfirmWindow.xui`（VA `0x14B0C5570`），类 `CNRDCharacterDeleteConfirmWindow`，资源 id `0x957`；唯一文案是「Are you sure that you want to delete this character?…」 |
| **带输入框的删除窗** | `CharacterDeleteNotiWindow`：`UI/CharacterDeleteNoti/Character_Delete.xui`（`0x14965A6C0`）、弹窗类型 `POPUP_WINDOW_TYPE_CHARACTER_DELETE_NOTI`（`0x149634910`）、**编辑框控件名 `edit_delete`（`0x14965AAA0`）**、`CharacterDeleteNotiWindow::on_complete`（`0x14965A820`） |
| 控件绑定 | 该窗 init 里两次 `call 0x146E8C7D0`（xorstr 解密）→ `call 0x145F6E4E0`（UI 框架 `GetControl(this, &out, 名字)`）：第一次存到 `this+0x680`，**`edit_delete` 存到 `this+0x690`** |
| 客户端另有一类"输固定短语"确认 | DSTR：`Input "Delete Now" to delete.` / `Delete Now` / `If you want to remove, please enter "Delete Now."`（公会驻地删除等） |

⇒ 用户说的"输入角色名删角色"在本客户端**成立**：链路是
删除确认（`CNRDCharacterDeleteConfirmWindow`）之后还有一步 `CharacterDeleteNotiWindow`，
里面是 `edit_delete` 输入框，校验通过才发 `CMD 6`。

### 6.3 全链路已闭环（第 7 轮）：要打的是固定短语，名字来自窗口

用两个新工具拿到最终答案：
`refscan.py`（numpy 一次扫完 `.text`，找**指向任意地址的 RIP 相对访存与 rel32 调用**）
+ `find_opcode_sender.py`（在"写 opcode"的 3,313 个调用点里，按 `mov edx, <opcode>` 过滤）。

```
opcode 6（DELETE_CHARACTER）的发送点共 7 处，其中 VA 0x1411F7D52 在 UI 模块里，
与 CharacterDeleteNotiWindow 的代码同区（0x11Fxxxx）。
```

那个函数（**入口 0x1411F7C90**（`.pdata` 边界 0x11F7C90..0x11F7E62），`rcx` = 删除通知窗对象）逐条读出来是：

```
rcx = this->[0x690]                      ; ← edit_delete 编辑框
call [rax+0x2A8]                         ; 取编辑框文本
ecx = 0x5F73224 ; call 0x14723C170       ; ← 取本地化文案（就是汉化 DLL 钩的那个 UI 查找）
逐字符比较（手写循环，非 wcscmp）          ; 相等 → edi = 0
call [rax+0x18](dl)                      ; 用比较结果设置"确定"按钮可用状态
...
test edi,edi ; jne 不发送                 ; 文本不匹配 → 不发送
test al,al   ; jne 发送                   ; 另有 0x146EFD3B0 / 0x146ECFD90 两个放行条件
发送：
  call 0x146D74000                        ; 取 writer
  mov edx, 6 ; call 0x146D746E0           ; 写 opcode = CMD 6
  movzx edx, word [this+0x978] ; call 0x146D76180   ; 写 u16 slot
  lea rdx, [this+0x958]        ; call 0x146D76080   ; 写 wstring 名字
  call 0x146D75AF0                        ; 发送
  …清两个 busy 标志（0x14023DFC0/0x14023DFD0）
```

**结论（三条，全部有据）：**

1. 要输入的**不是角色名，而是一个固定短语**：`0x5F73224` 在
   `analysis/dumps/dstr_id_to_text.json` 里 = id `100086308` → **`"delete character"`**。
   汉化包会把这条翻译成中文 ⇒ **玩家必须用中文输入法打这句短语**；
   这正是"客户端无法输入中文"（任务 1）与"删角色删不掉"（任务 2）连在一起的原因。
2. 发给服务端的**名字不来自输入框**，而是窗口成员 `this+0x958`（std::wstring）、槽位 `this+0x978`（u16）
   —— 也就是说：**跳过输入校验不会发错名字**，客户端本来就用自己存的名字。
3. 组包用的是客户端自己的 writer（`0x146D74000` / `0x146D746E0` / `0x146D76180` / `0x146D76080` /
   `0x146D75AF0`），我们**不需要重实现协议或加密**。

### 6.4 实现（第 8 轮已交付：mod `qol.auto-confirm`）

`client-patchs/auto-confirm/`（插件 `AutoConfirmDelete.dll`，走 `qol.client-host` 插件通道）：

| 步骤 | 状态 | 做法 |
| --- | --- | --- |
| ① 抓到窗口对象 | ✅ | inline 钩住窗口 init **RVA `0x11F3EC0`**（`.pdata` 边界 `0x11F3EC0..0x11F4496`；开头 5 字节 `48 8B C4 55 57` = `mov rax,rsp; push rbp; push rdi`，位置无关、可整段搬走）。进入时 `rcx` = window，**返回后**读 `[window+0x690]` 拿到 `edit_delete` 控件 |
| ② 让校验通过 | ✅ | 读控件 vtable，把槽 `+0x2A8`（取文本）换成包装：**只有 `this` 等于记下的那个控件实例**时返回确认短语，其它实例原样转发。短语在 init 钩子里通过客户端自己的 `0x14723C170(phrase_id)` 取（默认 `0x5F73224`），汉化后自动是中文 |
| ③ 连"确定"也自动按 | ⛔ 未做 | 需要调 `0x1411F7C90` 且**必须在 UI 线程上**（它直接操作 UI 对象）；现在仍需玩家点一次「确定」（但不用打字）。开关位置已留（`auto_confirm_delete=2`） |

**验证（都可复跑）**

1. 期望字节自检：`0x11F3EC0` → `48 8b c4 55 57`、`0x11F7C90` → `48 89 5c 24 08`，与出货 EXE 逐字节一致；
2. **虚表槽替换独立自测**（`client-patchs/tests/vtable_test.c`）：假控件对象 + 假虚表，
   验证"盯住的实例返回替换值、其它实例不受影响" → `PASS`；
3. inline 跳转机制自测见 §4.7（同一套代码）；
4. **负面测试**：把两个插件一起装到 `test-host.exe`（不是 DFO.exe）上跑，
   自动确认插件安全跳过并记日志，进程 0 退出：

```
没有 …\auto-confirm.ini → 已生成默认开关（auto_confirm_delete=1）
inline 钩子[delNotiInit] 跳过：00007FF6DB003EC0 不在已提交内存里（客户端版本不同？）
初始化钩子没装上：本插件不会生效（客户端版本不同？）
```

5. `modkit verify/plan/install` 全通过；当前三个 mod（host / 中文输入探针 / 自动确认）全部 `[一致]`。

### 6.5 还没闭环的

1. 任务（1）与任务（2）的**实机验证**（都要等用户跑一次游戏）；
2. ③「连确定也自动按」：需要 UI 线程 + `0x11F7C90` 调用，另需弄清
   `0x146EFD3B0` / `0x146ECFD90` 两个放行条件的语义；
3. ②若在真机上表现为"按钮仍未点亮"，则需要再找**触发校验的那个函数**
   （它是本窗类的虚表成员，位于 vtable `0x140965A238` 一族里）并主动调一次。

