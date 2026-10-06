# 客户端 mod 分享包（115us）

这个目录里是**可以直接发给别人的客户端 mod 包**（schema 2 四层 mod，用 modkit 安装）。
源码在 [`../../client-patchs/`](../../client-patchs/)（`client-host/`、`chinese-input/`、`auto-confirm/`、`tests/`）。

## 一、包清单（2026-10-06）

| 包 | mod id | 干什么 | 状态 |
| --- | --- | --- | --- |
| `qol.client-host-1.0.0.zip` | `qol.client-host` | **宿主**：占住客户端唯一那个会自动加载的 DLL 槽位，加载 `.115us-mods\*.dll` 里的插件 | 实机在用 |
| `qol.chinese-input-probe-2.0.1.zip` | `qol.chinese-input-probe` | **中文输入**：把客户端「只认 XP 时代传统输入法」的门禁喂过去，并阻止它每次按键取消组字 | **业主实机确认可用** |
| `qol.auto-confirm-1.0.0.zip` | `qol.auto-confirm` | **删角色免打字**：把删角色确认窗编辑框的「取文本」槽按实例替换 | 补丁动作已实机确认；删除流程本身业主未逐项确认 |

校验值（sha256）：

```
1597c0b942a4bda7d61232295f0e731b83f140a9a7a81948282288bfdb03c65e  qol.client-host-1.0.0.zip
2d54219e9b5a17a09ead71431f9a05654b13a50df3de9b2fcd249e7f17a5c851  qol.chinese-input-probe-2.0.1.zip
6679ba3d35953ec8357b7e14c16ef420ba2e4ea356863e6e8119af7a7e99c250  qol.auto-confirm-1.0.0.zip
```

## 二、装（两种方式）

**方式 A：用启动器的 modkit（推荐，可查可卸）**

```powershell
modkit install --client <客户端根目录> --mod qol.client-host-1.0.0.zip
modkit install --client <客户端根目录> --mod qol.chinese-input-probe-2.0.1.zip
modkit install --client <客户端根目录> --mod qol.auto-confirm-1.0.0.zip   # 可选
modkit status  --client <客户端根目录>
```

顺序必须是**宿主先装**（后两个包声明了 `requires: ["qol.client-host"]`，宿主没装会被 plan 挡下）。
升级时先 `uninstall` 再 `install`（`file.add` 不允许覆盖已存在且内容不同的文件）。

**方式 B：手工放文件（没有 modkit 时）**

```
<客户端根>\ChineseLocalization.dll            ← 从 qol.client-host 包里取 client/ChineseLocalization.dll
<客户端根>\.115us-mods\ChineseInputProbe.dll  ← 从 qol.chinese-input-probe 包里取
<客户端根>\.115us-mods\AutoConfirmDelete.dll  ← 可选
```

开关文件（`chinese-input.ini` / `auto-confirm.ini`）**由插件首次运行时自己生成**在 `.115us-mods\` 里，
默认值就是实机验证通过的那套；想改开关改完重启游戏即可（不用重装）。手工方式没有登记，卸载就是删这几个文件。

## 三、卸

```powershell
modkit uninstall --client <客户端根目录> --id qol.chinese-input-probe
modkit uninstall --client <客户端根目录> --id qol.auto-confirm
modkit uninstall --client <客户端根目录> --id qol.client-host     # 插件没卸完会拒绝
```

插件自己生成的 `.115us-mods\*.ini` 与 `*.log`、客户端根的 `client-host.log` 不属于 mod 包，
卸载后按需手工删除。日志写在插件自己所在目录（`client-host.log` 在客户端根，属宿主约定）。

## 四、分享时**必须一起说清**的三件事

1. **只对同一版客户端有效**。补丁里的地址（门禁 `RVA 0x6F22400`、布局写入点 `0x6F23070`、
   Themida 导入槽区间 `0x9186100..0x9187A00`）和期望字节都是**按本机这一版 `DFO.exe`
   （258,972,712 字节）**核对过的。换构建会逐字节核对失败 → 插件**静默跳过并记日志**
   （不会崩，但也不会生效），日志里会出现「期望字节对不上 / 不在已提交内存里」。
2. **需要客户端已有 `dinput8.dll` 汉化代理**。宿主 DLL 靠它被加载；没有这个代理，
   `ChineseLocalization.dll` 不会被自动加载，整个 mod 不生效（这是 115us 整合包自带的）。
3. **客户端带 BlackCipher 反作弊**：本机加载这两个 DLL 未被拦截，**其它环境/其它版本未验证**。
   出问题先卸掉 mod 复现一次，以区分「补丁问题」与「客户端问题」。

## 五、原理一句话（分享时对方可能会问）

客户端其实**天生支持中文输入**，但它只认 XP 时代的传统 IMM32 输入法：
要求键盘布局 ∈ `{E0080404, E0090404, E00E0804}`，且 `ImmGetIMEFileNameW` 返回
`{TINTLGNT, CINTLGNT, MSTCIPHA, PINTLGNT, MSSCIPYA}.IME` 之一 —— 现代 Windows 的 TSF 输入法两条都不满足，
所以它自己把 IME 通路关掉了。这个 mod 把这几个门禁"喂"过去，并阻止客户端每按一键就
`ImmNotifyIME(NI_COMPOSITIONSTR, CPS_CANCEL)` 取消组字，于是拼音能累积、候选窗可用、选字能上屏。

完整取证与实现细节见
[`../../analysis/tasks/chinese-input-probe-20261006.md`](../../analysis/tasks/chinese-input-probe-20261006.md)（§4.13 是实机成功那一节）。
