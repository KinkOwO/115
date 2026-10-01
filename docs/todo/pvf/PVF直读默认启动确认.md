# PVF直读默认启动确认

用户确认“已确认，将pvf模式作为默认启动项”，本批54选择项/63类源投影升级为confirmed baseline。默认profile为server/work/dfo-lan/configs/pvf-default.json，正式程序为server/work/dfo-lan/bin/wireprobe-pvf.exe，SHA256 a3ea388ac9a2966f0368e6ede552f3d8559fc10bfba08f24f5158bb583e2d98c，与已确认的全量隔离程序逐字节相同。来源仍为server/work/client-build/Script.inner.pvf及精确7ef SHA256，不别名、不改写玩家存档。确认依据用户反馈，本轮未新增实机会话日志，不扩大为逐项客户端动态命中。

启动服务端.cmd、启动游戏.cmd、启动游戏-奥德赛.cmd通过共用launch_local默认加载全量PVF配置；分别保留仅服务端、剧情模式0及奥德赛模式1。默认JSON基线审计关闭，源身份/策略校验仍强制。独立profile显式保持既有奥德赛武器奖励发布值1，使挂载不依赖旧武器盒导出JSON是否存在。客户端路径、channel identity、存储配置及其它玩法开关保持；客户端/存储单独模式不要求本地PVF。

编排按选中的characters/dungeons领域跳过旧JSON告警或副本文件门禁，其余JSON模式/未选领域继续原检查。PVF选中而程序不自报pvf-catalogs能力时拒绝启动，不能静默退回JSON。外层等待从30秒调整为PVF 210秒，覆盖内层180秒源准备；JSON等待仍30秒。停止环境识别wireprobe-pvf.exe。Build-Server首次构建补齐默认PVF程序，已有确认程序在普通构建时保留；-UpdatePVFDefault显式发布后续已验收构建。

显式--repair-profile仍可选择逐批隔离/修复配置；--json-mode使用launcher.local.json原server_binary并清除继承的DFO_PVF_*，与repair-profile互斥。--source-build在默认PVF模式下改用源码程序并保留源配置，需要重建有全部选择项的版本；--source-build --json-mode为旧JSON源码路径。旧39程序、原源码程序、各批隔离程序和JSON文件均保留。GM启动入口仍保持独立选择，不在本次三个入口切换范围。

全量go test ./...及go vet ./...通过；启动/profile/inner-PVF/channel identity共25项Python检查与5项GM检查通过。模拟就绪在40秒到达仍成功，缺失旧JSON与不支持PVF程序拒绝/跳过边界均验证。默认服务端、剧情、奥德赛及显式JSON四组实际--check均通过，均不启动服务或客户端、不操作玩家数据库。

## 日常使用

直接使用根目录三个启动脚本，无需额外PVF参数。原生inner位于server/work/client-build/Script.inner.pvf；不要替换为另一个checksum的客户端资源。

只检查依赖，不启动环境：

```powershell
tools/python/python.exe server/work/dfo-lan/scripts/launch_local.py --check --server-only
```

显式回退旧JSON模式：

```powershell
./启动服务端.cmd --json-mode
./启动游戏.cmd --json-mode
./启动游戏-奥德赛.cmd --json-mode
```

此选项沿用本机launcher.local.json的旧程序和完整JSON，失败时不会自动在两种数据源之间切换。
