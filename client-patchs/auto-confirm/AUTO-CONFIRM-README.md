# 自动确认（删角色）——AutoConfirmDelete.dll

## 它改什么

客户端删角色之前有一个**必须手打确认短语**的弹窗
（`CharacterDeleteNotiWindow`，`UI/CharacterDeleteNoti/Character_Delete.xui`，
里面是编辑框 `edit_delete`）。短语取自本地化表 **DSTR id 100086308**：

```
英文原文：delete character      ← 汉化包里会变成中文 ⇒ 必须中文输入法才打得出来
```

静态取证（`analysis/tasks/chinese-input-probe-20261006.md` §6）还发现：**发给服务端的角色名
不来自输入框**，而是窗口成员 `window+0x958`（名字）/`window+0x978`（槽位）——
所以"跳过输入校验"不会发错名字。

本插件把该弹窗里**那一个编辑框实例**的「取文本」虚表槽（`vtable+0x2A8`）换成包装：

- 只对这个实例返回客户端自己的确认短语（运行时通过客户端自己的 `0x14723C170(id)` 取，
  所以汉化后自动是中文）；
- 其它任何编辑框一律原样转发（按实例判定，不会波及其它输入框）。

于是校验直接通过、**不需要打任何字**，玩家点一次「确定」即可完成删除。

## 开关

`auto-confirm.ini`（**由插件首次运行时自动生成**在插件同目录）：

| 开关 | 默认 | 作用 |
| --- | --- | --- |
| `auto_confirm_delete` | 1 | `1` = 打补丁（不用打字，仍需点一次「确定」）；`0` = 完全关掉 |
| `phrase_id` | `5F73224` | 确认短语的 DSTR id（换客户端版本可能要改） |

## 装 / 卸

```powershell
# 需要先有宿主 qol.client-host
modkit install --client <客户端根> --mod auto-confirm-1.0.0.zip --root <启动器根>
modkit uninstall --client <客户端根> --id qol.auto-confirm --root <启动器根>
```

## 边界与风险

- 只改**内存里的虚表槽**，不动 `DFO.exe`、不改报文、不自己发包；卸载即删除本 DLL。
- 装钩子前后都做自检：目标地址必须可读、开头字节必须与取证时一致，否则**跳过并记日志**
  （换客户端版本时不会崩）。
- 它**不会**替玩家按「确定」：删除角色仍是玩家的一次明确点击。
  "连确定也自动按"（直接发 `CMD 6`）需要调用客户端的 `0x11F7C90`，那必须在 UI 线程上做，
  本轮**未实现**（留作后续，见任务记录 §6.4 ③）。
- 日志：`<客户端目录>\.115us-mods\auto-confirm.log`。
