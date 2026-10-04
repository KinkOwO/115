# 增量更新包工具（scripts/incremental-package）

把**某一次提交里改动的文件**打成增量更新包，包内自带安装脚本（备份原文件 + 写入增量文件）与
一键还原脚本（还原到安装前状态）。用于把服务端/文档的小改动交付到另一份 115us 安装目录，
不必整包重发，也不会碰到 `tools/`、`runtime/`、数据库与存档。

## 1. 打包

在仓库根目录执行（默认打**最后一次提交**）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\incremental-package\build-incremental.ps1
```

也可以双击 `scripts\打包增量更新.cmd`，或指定提交/输出目录：

```powershell
# 指定提交
... build-incremental.ps1 -Commit HEAD~1
... build-incremental.ps1 -Commit f07aa402
# 指定输出目录与包名
... build-incremental.ps1 -Commit f07aa402 -OutDir D:\out -Name DFO115US-增量更新-测试
# 保留暂存目录（排查用）
... build-incremental.ps1 -KeepStaging
```

产出：`<OutDir>\DFO115US-增量更新-<提交短号>-<yyyyMMdd>.zip`（默认 `<OutDir>` = 仓库上一级目录）。

特性：

- 只从 **git 对象库**（`git archive`）导出提交内容，**不读工作区**，因此其它未提交改动、其他 agent 正在改的文件都不会被打进包；
- 二进制文件（exe/dll/pvf 等）原样导出；
- 提交里被**删除**的文件记为 `D`，安装时先备份再删除，还原时恢复；
- `manifest.json` 记录每个文件更新前后的 **SHA256**，安装脚本据此预检目标机版本并逐文件校验写入结果。

## 2. 包的内部结构

```
DFO115US-增量更新-<sha>-<date>/
├─ 安装增量更新.cmd        双击安装（备份 + 更新 + 校验）
├─ 还原上一版本.cmd        双击一键还原
├─ 查看备份记录.cmd        列出目标目录下的历史备份
├─ 使用说明.md             面向使用者的说明（含本次文件清单）
├─ manifest.json           提交信息 + 逐文件 SHA256/size/status
├─ payload/…               新版本文件，保持仓库相对路径
└─ _update/
   ├─ update.ps1           安装/还原引擎（-Mode Apply|Restore|List）
   └─ common.ps1           公共函数
```

## 3. 使用（目标机）

1. 把 zip 解压到游戏根目录（含 `scripts\启动游戏.cmd`、`server\` 的目录），会生成一个包文件夹；
2. 进入包文件夹双击 `安装增量更新.cmd`：默认目标目录 = 包文件夹的上一级，回车确认即可；
3. 备份写入 `<游戏根目录>\_update-backup\<时间戳>_<提交短号>\`（`journal.json` + `files\` 原文件）；
4. 需要回滚时双击 `还原上一版本.cmd`；只回滚该次更新涉及的文件。

非交互用法：

```powershell
... _update\update.ps1 -Mode Apply   -Target "C:\Game\dof\115us\115" -Yes
... _update\update.ps1 -Mode Restore -Target "C:\Game\dof\115us\115" -Yes
... _update\update.ps1 -Mode Restore -Backup 20261004-180000_f07aa402
... _update\update.ps1 -Mode Apply   -DryRun            # 只预检不写盘
... _update\update.ps1 -Mode Apply   -Force             # 忽略基线不一致，直接覆盖
```

目标目录解析顺序：`-Target` 参数 > 包内 `target.txt`（可选，第一行非 `#` 内容）> 解压目录的上一级 > 交互输入。

## 4. 约定与边界

- 安装/还原只影响 `manifest.json` 列出的文件；不涉及数据库结构、`pgdata`、存档与运行环境；
- 包内 `.ps1` 必须以 **UTF-8 with BOM** 保存（Windows PowerShell 5.1 才会正确显示中文），
  `.cmd` 一律纯 ASCII；`build-incremental.ps1` 只做复制，不改写模板编码；
- 安装过程中任何一步失败都会用本次备份**自动回滚**；
- 同一次更新重复安装不会出错：会再生成一条备份记录，还原按时间倒序取最近一条；
- `_update-backup\` 已被根 `.gitignore` 忽略，不会污染版本库。
