# 中文输出支持（ChineseInputProbe.dll）— mod id `qol.chinese-input-probe`

这是一个**只观察、不修改**客户端行为的诊断插件：用来查清「115US 客户端无法输入中文」
到底断在哪一环。它以**插件**形式由 `qol.client-host`（`<客户端>\ChineseLocalization.dll`）
加载，不需要改 `DFO.exe`，也不占那个唯一的 DLL 槽位。

## 它做什么

1. 记录进程环境：键盘布局、IME 名称、输入法是否可用；
2. **EAT 钩子**包装 `imm32.dll` 的 IME API（`ImmGetContext` / `ImmAssociateContext` /
   `ImmGetCompositionStringW` / `ImmLockIMC*` / `ImmNotifyIME` …）与 `user32` 的
   `CreateWindowExW`，**包装后原样转发**，只记调用与调用栈；
3. 子类化本进程的窗口，记录 `WM_CHAR` / `WM_KEY*` / `WM_IME_*` 消息；
4. 每 200 ms 轮询前台窗口的输入法状态（开启状态 + 组字串），一变就记一行。

**它不改任何返回值、不改报文、不改窗口过程语义**；唯一的副作用是在 DLL 自己所在目录
写一个日志文件。

## 日志

``<客户端目录>\.115us-mods\chinese-input-probe.log`（插件自己所在目录，见 AGENTS 的 DLL 日志硬规则，宿主日志另在 `client-host.log`）。`（DLL 所在目录，见 AGENTS 的 DLL 日志硬规则）。

## 怎么用（人工操作）

1. 退出游戏；
2. 先装宿主，再装本插件（顺序不能反，清单里写了 `requires: ["qol.client-host"]`）：

   ```powershell
   modkit install --client <客户端根> --mod client-host-1.0.0.zip --root <启动器根>
   modkit install --client <客户端根> --mod chinese-input-1.1.0.zip --root <启动器根>
   ```

3. 启动游戏，进到能打字的界面（聊天框 / 角色名输入框）；
4. **切到中文输入法，打几个字**（拼音→候选→上屏），再打几个英文字母；
5. 退出游戏，把 `.115us-mods\chinese-input-probe.log` 发回；
6. 卸载：先卸插件再卸宿主（被依赖时 modkit 会拒绝卸载宿主）。

## 门禁修复（本 mod 从 1.2.0 起）

静态取证发现客户端**只认 3 个 XP 时代传统 IME 的键盘布局（HKL）和 5 个传统输入法文件名**
（`TINTLGNT.IME` / `CINTLGNT.IME` / `MSTCIPHA.IME` / `PINTLGNT.IME` / `MSSCIPYA.IME`），
否则整条 IME 通路不会被启用。本机布局是 `0x08040804`、`ImmGetIMEFileNameW` 返回 0，
两个门禁都过不去。

`chinese-input.ini`（**由插件首次运行时自动生成**在插件同目录，之后不再改动）控制行为：

| 开关 | 默认 | 作用 |
| --- | --- | --- |
| `fix_layout` | 1 | 中文布局改报传统 IME HKL（`0x0804→E00E0804`、`0x0404→E0080404`） |
| `fix_ime_name` | 1 | `ImmGetIMEFileNameW` 返回 `ime_name` 指定的白名单名字 |
| `post_layout` | 1 | 给每个新建的顶层窗口补一条 `WM_INPUTLANGCHANGE`（覆盖"从消息取 HKL"那条路径） |
| `fix_slots` | 1 | **修正 Themida 导入槽**：客户端是加壳的，它调 user32/imm32 走自己的导入桩（`FF 25` → 槽里存着提前解析好的真实地址），改导出表对它无效。打开后精确匹配到我们的原函数地址就换成我们的包装 |
| `fix_gate` | 1 | **钩住客户端门禁本体**（RVA `0x6F22400`）+ **布局写入点**（RVA `0x6F23070`）：入口把 `mgr+0x68` 改成传统 IME 的 HKL / 把要写进去的中文布局换掉。不管客户端从哪拿到真实 HKL，门禁都会过 |
| `fix_isime` | **1** | `ImmIsIME` 对我们报出去的传统布局改报「是输入法」（实机 16:32：客户端问到 0 就把拼音当字母上屏） |
| `fix_cancel` | **1** | 备用：吞掉客户端主动取消组字（`ImmNotifyIME`：`NI_COMPOSITIONSTR`+`CPS_CANCEL`）。日志若显示客户端一直在取消组字（组字串永远单个字母）才打开 |
| `ime_bridge` | **1** | 备用：客户端自己不处理组字时，把已上屏的结果串（`GCS_RESULTSTR`）逐字当 `WM_CHAR` 投给它。客户端也会处理组字时字会重复，所以默认关 |
| `trace_c2s` | 0 | 客户端 C2S 发包追踪：`1` 只记 opcode、`2` 连 body 前 48 字节（inline 跳转，装前逐字节核对现场） |

**为什么开关文件不随包落位**：modkit 的 `file.add` 不允许覆盖「已存在且内容不同」的文件，
所以把开关放进包里会让**下一次升级被阻断**。改成插件自己生成一次，升级就只替换 DLL。

**只对游戏主模块生效**：MSCTF 等系统模块照实转发（日志里标 `[系统模块]`），否则会破坏输入法框架。
**想退回纯观察**：把开关都改成 0（或删掉 ini 让它重新生成后再改），重启游戏即可，不需要重装。

## 边界

- 探针在 `imm32` / `user32` / `kernel32` 的导出地址表里放 14 字节跳转桩（不改函数体）；
  包装后**原样转发**，不改任何返回值语义。
- 卸载即逐字节删除（`.115us-mods\ChineseInputProbe.dll` 是新增文件）。
- 若游戏启动异常，先卸载本插件（有必要连宿主一起卸）再复现一次，以区分探针问题与客户端问题
  —— 两份日志（`client-host.log` 与插件日志）能直接看出插件有没有被加载、加载到哪里失败。
