# vendor/ —— 第三方原件（不入库）

本目录放**别人编好的二进制原件**，只作打包输入；`.gitignore` 的 `*.dll` 规则
已覆盖本目录，所以这些文件不会进 Git。重打包时若本目录是空的，脚本会回退到
`~/Downloads/dinput8.dll`；两个都没有就**不打** `dinput8.dll` 那条 op（只警告，不失败）。

## dinput8.dll —— 扩展按钮 / 商城标签页

| 项 | 值 |
|---|---|
| 文件 | `vendor/dinput8.dll` |
| 大小 | 855,040 B |
| sha256 | `5b4a90994d2ba59441f7f96d3dc6bc37a40c3f8e9267db38e43b41fbc64b1147` |
| PE 时间戳 | 2026-10-08 17:14:33（x64，GUI 子系统） |
| 来源 | 业主下载（原始位置 `~/Downloads/dinput8.dll`） |

### 它是什么

一个 **dinput8 代理 + 客户端增强** DLL：

- 导出 6 个函数，与客户端现有的 `dinput8.dll`（"DFO localization proxy v2"，107,008 B）
  **完全一致**：`DirectInput8Create`、`DllCanUnloadNow`、`DllGetClassObject`、
  `DllRegisterServer`、`DllUnregisterServer`、`GetdfDIJoystick`；
- 自己转发系统 DirectInput（实测 `DirectInput8Create` 返回 `S_OK`）；
- **主动 `LoadLibrary("ChineseLocalization.dll")`** 并调用它的 `StartLocalization`
  —— 也就是它认得我们的 `qol.client-host` 共用入口，装上后插件链继续工作；
- 自己的日志写到**自身所在目录**的 `115-shop-tabs.log`（符合 AGENTS §0.9）。

### 它加了什么（来自自身字符串）

| 字符串 | 含义 |
|---|---|
| `115-shop-tabs.log` | 日志名 → 商城标签页 / 扩展按钮 |
| `INSTALL rejected image layout` | 只认特定 `DFO.exe`；不匹配就**只记一行日志、不挂钩** |
| `INSTALL rejected vtable slot=%d` / `BEAST rejected actor vtable` | 虚表替换：商城页签、怪物变身 |
| `AVATAR_MARKER skipped vtable mismatch` | 头像标记 |
| `EQUIPMENT_TITLE ready: … first reinforced Chinese title only` | 装备中文强化标题 |
| `HUD_DIAG dispatch slot=…` | HUD 诊断输出 |
| `PROXY failed system dinput8 loading` | 系统 dinput8 转发失败时的告警 |

### 落位与回滚

- 落位：client 层 `file.replace` → 目标 **`<客户端根>\dinput8.dll`**（客户端唯一会被
  自动加载的槽位），会顶掉现有 localization proxy v2。
- 回滚：modkit `file.replace` 会备份原文件，卸载逐字节还原。
- 依赖：`requires: ["qol.client-host"]` —— 它要加载的 `ChineseLocalization.dll`
  正是该 mod 落位的宿主。

### 实测记录（2026-10-11，非游戏进程内）

用受控加载器（`.tmp/extbtn/loader.c`）加载它：

| 场景 | 结果 |
|---|---|
| 宿主进程名不是 `DFO.exe` | `INSTALL skipped non-DFO host` + `CLIENT_HOST load skipped error=126` |
| 宿主改名 `DFO.exe` 且同目录有 `ChineseLocalization.dll` | `INSTALL rejected image layout` + `CLIENT_HOST started result=0` |
| 直接调 `DirectInput8Create` | `S_OK`（转发正常） |

`error=126` = `ERROR_MOD_NOT_FOUND`（当时目录里没放 `ChineseLocalization.dll`）；
`rejected image layout` 是因为测试进程不是真的 `DFO.exe` —— **两者都符合预期**，
说明它有"不盲改内存"的自我保护，不会在陌生进程里乱挂钩。

> 在 `115\` 树下直接 `LoadLibrary` 它会拿到 `err=5`（该树的可执行文件加载限制），
> 这是**测试环境**的限制，不是 DLL 的问题；装到客户端根由 `DFO.exe` 加载不受影响。
