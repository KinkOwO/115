# DFO 115 服务端合并记录

- 合并时间：2026-09-24
- 合并来源：`D:\115us-merge-src\115-fix`（21 项 Go 服务端修复包）
- 验证方式：`D:\115us-merge-test\` 隔离环境完整测试通过

---

## 一、替换的文件清单

### 1. Go 源码（覆盖 `D:\115us\server\work\dfo-lan\`）
- `cmd\`、`internal\`、`docs\`、`go.mod`、`go.sum`
- 共 594 个 .go 文件
- 在生产目录重新 `go build`，产出 `bin\wireprobe-dungeon39.exe`

### 2. configs（覆盖 `D:\115us\server\work\dfo-lan\configs\`）
- 从修复包复制，共 89 个文件（原 38 个）
- **排除** `channel.local34.json`（保留生产版 `127.0.0.1:7001`）
- 新增关键 catalog：shop-purchase-pilot.json、itemshop-candidate.json、selection-boxes-candidate.json、items.index.json、booster-catalog.json、randomoption.current37.json、apocalypse.generated.json、dungeons.odyssey-scenes-release.json 等

### 3. channel_probe.py
- 生产：`D:\115us\server\work\dfo_probe_tools\channel_probe.py`
- 从旧版 12,753 字节 → 修复版 29,466 字节
- 支持 catalog 自动检测、odyssey tag 分支

### 4. wireprobe exe
- 当前生产：`bin\wireprobe-dungeon39.exe` = **18,699,776 字节**（area-fix 版）
- SHA256：`D3790F25BC7236FB92AEEF8FFFC5997C857062BFB0352C08901B9987FA4EED69`

### 5. service.go area-fix 补丁
- 文件：`internal\world\service.go` L182
- 改动：`if strict || !permissive` → `if strict`
- 作用：普通走路/副本回城/NPC 传送不再被 area_refused 阻塞；strict=true（奥德赛跨城）仍校验 portal edge

---

## 二、绝对未动的文件

- `D:\115us\client\`（DFO.exe、Script.pvf、sk.dat 全部只读）
- `D:\115us\server\work\dfo_probe_tools\probe.exe`（584,704 字节，SHA256 未变）
- `D:\115us\server\work\dfo-lan\runtime\storage\pgdata\`（存档未动）
- `D:\115us\server\work\dfo-lan\runtime\storage\local.json`
- `D:\115us\server\work\dfo-lan\runtime\storage\redis.conf`
- `D:\115us\server\launcher.local.json`
- `D:\115us\tools\`
- `D:\115us\启动游戏.cmd` / `停止游戏环境.cmd`
- `scripts\launch_local.py`、`scripts\bootstrap_local.py`（保留生产版管理员检查）
- `configs\channel.local34.json`（保留 Listen=127.0.0.1:7001）

---

## 三、备份位置（D:\115us-backup\）

| 文件 | 大小 | 用途 |
|---|---|---|
| wireprobe-dungeon39.exe.before-merge | 12,012,032 | 最早原版（回滚到合并前） |
| wireprobe-dungeon39.exe.before-area-fix | 18,704,384 | area-fix 前 |
| service.go.before-area-fix | 8,974 | area-fix 前源码 |
| channel_probe.py.before-merge / .before-merge-2 | 12,753 | 旧版 channel_probe |
| launch_local.py.before-merge | 4,997 | 旧版启动脚本 |
| local.json.before-merge | 496 | 旧版 local.json |
| dfo-lan-before-merge-20260923\ | 421 文件 | 生产源码全量备份 |
| configs-before-merge-20260923\ | 38 文件 | 旧 configs 全量备份 |

### 回滚方法
1. 停游戏（`D:\115us\停止游戏环境.cmd`）
2. `bin\wireprobe-dungeon39.exe` 改名为 `.broken`
3. 把 `bin\wireprobe-dungeon39.exe.old` 改回 `wireprobe-dungeon39.exe`
4. `robocopy D:\115us-backup\configs-before-merge-20260923 D:\115us\server\work\dfo-lan\configs /MIR`
5. `copy /Y D:\115us-backup\channel_probe.py.before-merge-2 D:\115us\server\work\dfo_probe_tools\channel_probe.py`
6. 从 `D:\115us-backup\dfo-lan-before-merge-20260923\` 复制 `internal\world\service.go` 回去
7. 重启游戏

---

## 四、已验证通过的功能

- 普通登录、选角、进城镇
- 商城道具购买（Cera 扣减、物品到账、delivery 协议）
- 仓库打开、装备穿脱
- 技能栏、背包、角色信息
- 奥德赛模式登录、Status Board
- 商城 UI 打开（49 ordinary products loaded）
- item shop（527 shops）、selection boxes（2975）、booster catalog（42301）加载

---

## 五、已知限制

1. **教程副本撤离黑屏**：hua1 回 town area 0 时，客户端在副本视图不处理 area_change_sent。ESC 菜单回城可绕过。
2. **mirkwood1.map 客户端崩溃**（0xC0000005）：进 dungeon 3（mirkwood）时客户端 Access Violation。避开 mirkwood 副本。
3. **旧版 Sky Tower（Lv15 West Coast）副本链未接**：dungeon selection 无副本。这是旧内容遗留，不是合并引入。
4. **时装购买 pilot 白名单**：shop-purchase-pilot.json 里只有普通道具，时装购买会 reject。需要后续加白名单。
5. **DFO_SHOP_RELEASE / DFO_VAULT_PURCHASE_RELEASE 未启用**：保守模式，正式开关后续再开。
6. **奥德赛武器属性**：手动 SQL 塞的武器属性不正确（Used Lyra Bow，物攻 13），奥德赛成长机制未完整走通。
7. **area-fix 副作用**：普通走路不再校验 portal edge，理论上可能穿墙，但实测未发现。
