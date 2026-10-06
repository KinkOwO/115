# client-patchs —— 客户端补丁 / DLL

本目录的规则见 [`AGENTS.md`](AGENTS.md)（默认不启用；只有业主明确要求才动客户端）。
这里放**可直接装进客户端**的东西，全部以 schema 2 mod 包交付，能 `verify / plan / install / uninstall`。

## 一、当前交付（2026-10-06）

| 目录 | mod id | 解决的问题 | 说明 |
| --- | --- | --- | --- |
| [`client-host/`](client-host/) | `qol.client-host` | 客户端只有**一个**自动加载的 DLL 槽位 | 把那个槽位做成插件目录 `<客户端>\.115us-mods\*.dll`，其它客户端 mod 都当插件跑 → [HOST-README](client-host/HOST-README.md) |
| [`chinese-input/`](chinese-input/) | `qol.chinese-input-probe` | **客户端无法输入中文** | 探针（看 IME 消息/调用）+ 门禁修复（把客户端只认 XP 时代输入法的两个门禁喂过去）→ [PROBE-README](chinese-input/PROBE-README.md) |
| [`auto-confirm/`](auto-confirm/) | `qol.auto-confirm` | **删角色要先手打确认短语** | 把删角色通知窗里编辑框的"取文本"槽换成包装，只对该实例返回客户端自己的确认短语 → [AUTO-CONFIRM-README](auto-confirm/AUTO-CONFIRM-README.md) |

三个都装好后的客户端状态：

```
<客户端>\ChineseLocalization.dll            qol.client-host        （宿主）
<客户端>\.115us-mods\ChineseInputProbe.dll  qol.chinese-input-probe
<客户端>\.115us-mods\AutoConfirmDelete.dll  qol.auto-confirm
<客户端>\.115us-mods\*.ini                  各插件首次运行时自己生成
```

## 二、为什么中文输入要这么绕（结论）

静态取证（[任务记录](../analysis/tasks/chinese-input-probe-20261006.md)）：
客户端只在两个条件**同时成立**时才启用它的 IME 通路 ——
键盘布局 HKL ∈ `{E0080404, E0090404, E00E0804}`，且 `ImmGetIMEFileNameW` 返回
`{TINTLGNT.IME, CINTLGNT.IME, MSTCIPHA.IME, PINTLGNT.IME, MSSCIPYA.IME}` 之一。
现代 Windows 的 TSF 输入法两条都不满足（本机 HKL=`08040804`、文件名取不到）⇒ 中文输入整条链路不会被启用。
修复就是把这两个门禁"喂"过去（只对游戏主模块生效，系统输入法框架照实回答）。

## 三、构建 / 打包 / 自测

```powershell
# 逐个补丁：先编 DLL，再打成 mod 包（zip 落在各自 dist\）
cmd /c client-patchs\client-host\build-host.cmd
python client-patchs\client-host\build-mod.py

cmd /c client-patchs\chinese-input\build-probe.cmd
python client-patchs\chinese-input\build-mod.py

cmd /c client-patchs\auto-confirm\build-plugin.cmd
python client-patchs\auto-confirm\build-mod.py
```

自测（**都不碰真客户端**）：

```powershell
# 宿主 + 插件链路（假客户端目录里跑）
cmd /c client-patchs\client-host\smoke-test.cmd
# 探针单独链路（自测宿主按 dinput8 代理的方式加载并调用）
cmd /c client-patchs\chinese-input\smoke-test.cmd
# inline 跳转 / 虚表替换两套危险机制的机制自测（不碰真客户端）
cmd /c client-patchs\tests\build.cmd          # inline 跳转 + 跳板转发
cmd /c client-patchs\tests\build-vtable.cmd   # 虚表槽替换（按实例生效）

# 门禁自测：复刻客户端的 IME 门禁（两步：HKL + ImmGetIMEFileNameW），
# 验证插件把它从"不启用"变成"启用" —— 这是任务（1）在不开客户端时能拿到的最强证据
cmd /c client-patchs\tests\build-gate.cmd client-patchs\chinese-input\dist\ChineseLocalization.dll

# Themida 导入槽端到端自测：造一个带"真槽"的假客户端（镜像撑到扫描区间），
# 装插件后槽被换成包装、通过槽调用拿到传统 IME HKL
cmd /c client-patchs\tests\build-slot.cmd client-patchs\chinese-input\dist\ChineseLocalization.dll
```

> Python 用仓库外便携环境（本机 `C:\Users\Ricar\.dsh\...\python.exe` 或整合包的 Python 3.11）。

## 四、装 / 卸

`modkit.exe` 在启动器仓构建（`go build -o modkit.exe ./cmd/modkit`）。
**注意**：本机沙箱下放在工作区内的可执行文件写不了工作区外 —— 把 `modkit.exe`
复制到 `%TEMP%` 再跑（详见任务记录 §2.2）。

```powershell
$mk = "$env:TEMP\modkit-test.exe"
& $mk verify  --mod client-patchs\client-host\dist\client-host-1.0.0.zip
& $mk plan    --client <客户端根> --mod <zip>
& $mk install --client <客户端根> --mod <zip>
& $mk status  --client <客户端根>
# 卸载（宿主被依赖，必须先卸插件）
& $mk uninstall --client <客户端根> --id qol.chinese-input-probe
& $mk uninstall --client <客户端根> --id qol.auto-confirm
& $mk uninstall --client <客户端根> --id qol.client-host
```

`file.add` 不允许覆盖"已存在且内容不同"的文件：**升级请先 `uninstall` 再 `install`**；
插件开关文件由插件首次运行时自生成，所以包内只有 DLL，升级只换一个文件。

**往返已实测**（2026-10-06）：三个 mod 全部卸载后客户端回到原状
（`ChineseLocalization.dll` 删除、`.115us-mods\` 只剩空目录、原有的 `dinput8.dll` 与 `Script.pvf`
逐字节未变），重装后 `status` 三项 `[一致]` 且现场哈希与包内声明一致。

## 五、实机验证清单（由业主操作）

1. 退出游戏 → 按上面的命令装三个 mod（顺序：宿主 → 探针 → 自动确认）；
2. 正常启动游戏；进到能打字的地方（聊天框），**切中文输入法打几个字**（拼音→候选→上屏），
   再打几个英文字母；
3. 试一次**删角色**：应当**不需要打字**（点「确定」即可；日志里会出现"已把编辑框文本替换为确认短语"）；
4. 退出游戏，把下面三个日志发回：
   - `<客户端>\client-host.log`（两个插件是否都被加载）
   - `<客户端>\.115us-mods\chinese-input-probe.log`（门禁是否被喂过、IME 消息来没来）
   - `<客户端>\.115us-mods\auto-confirm.log`（删角色窗出现时有没有打上补丁）
5. 出问题时把两个 ini 的开关改 `0` 再跑一次（纯观察/全关），以区分是补丁还是客户端问题。
