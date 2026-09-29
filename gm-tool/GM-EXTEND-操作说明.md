# GM-EXTEND 任务管理功能扩展 — 操作说明

## 功能概述

在 GM 工具「任务管理」页新增 **「5. 设为可提交」** 功能：
- 把已接取任务的 `progress` 改为 0，使其在游戏内显示为「可提交」
- 可选：同时把任务所需物品补发到背包
- 支持单个任务、批量任务、全部已接任务

## 修改文件

| 文件 | 改动 |
|---|---|
| `dashboard/gm_dashboard_proxy.py` | 新增 `load_quests_catalog()`、`quest_objective_items()`、`grant_to_bag()`、`quest_ready()` 4 个函数；新增 `/api/quest/ready` 路由 |
| `dashboard/index.html` | 任务管理页新增「5. 设为可提交」卡片和事件绑定 |

## 备份位置

`D:\115us\server\.ai_backup\GM-EXTEND\`
- `gm_dashboard_proxy.py.orig`（原始版本）
- `index.html.orig`（原始版本）

## 使用方法

1. 启动 GM 工具（Start-GMWeb.cmd 或 gmweb.py）
2. 浏览器打开 `http://127.0.0.1:28081/`
3. 在「角色管理」中选择目标角色
4. 切到「任务管理」页
5. 滚动到「5. 设为可提交」卡片：
   - **任务 ID**：留空 = 全部已接任务；填 ID（逗号分隔）= 指定任务
   - **勾选「同时补发任务所需物品到背包」**：[seeking] 类任务需要收集物品时勾选
   - 点击「设为可提交」
6. **重要**：操作后请 **切地图或重登角色**，游戏内任务 UI 才会刷新

## 技术原理

- **设为可提交**：`UPDATE character_quests SET progress=0 WHERE character_id=X AND quest_id=Y AND status='accepted' AND progress<>0`
- **补发物品**：读 `characters.state` jsonb → 解析 `inventory.items` → 合并/新增物品 → 写回 DB
- **服务端不需要重启**：服务端每次 `settleProximityObjectives()` 都从 DB 读任务状态
- **config_version 不需要改**：DB 现有记录的 checksum 与当前 catalog 一致

## 注意事项

- 只处理 `status='accepted'` 的任务；未接取的任务不会自动接取
- 操作前请把角色 **切到别的角色或退回选人界面**
- 补发物品直接写入背包，需重登才能在游戏内看到
- 跳过的任务（已完成、未接取、progress 已是 0）会在返回消息中列出

## 回滚方法

```powershell
Copy-Item "D:\115us\server\.ai_backup\GM-EXTEND\gm_dashboard_proxy.py.orig" "D:\115us\gm-tool\dashboard\gm_dashboard_proxy.py" -Force
Copy-Item "D:\115us\server\.ai_backup\GM-EXTEND\index.html.orig" "D:\115us\gm-tool\dashboard\index.html" -Force
```
重启 GM 工具代理即可。
