# PVF直读默认启动确认

## 当前确认基线：第三批首段（2026-10-01）

用户反馈“已确认，可以提交，按进度继续吧”，地图按需读取升级confirmed baseline。正式wireprobe-pvf.exe与源码程序均核对为450843870979c753949a740b093152d6c8123d550520c101d429afc6bebd4a97，三个根入口直接使用确认版。现行profile采用来源自动派生，保留同期合入的存档处理；用户启动后的当前内层为be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0，manifest已生成。确认依据用户反馈，未新增新归档逐项实机取证；此前原生完整对照与0.93GiB/36.80秒性能样本属于7ef源，不混同轮次。第三批后续与第四批继续实施，详见PVF启动与内存优化实施计划.md。

## 历史确认基线：第二批后段（2026-10-01）

用户反馈“已确认，按进度继续吧”，复杂物品联合扫描升级confirmed baseline。正式wireprobe-pvf.exe与源码程序均核对为2e4bd343012d5821160f27770a07c382172818d6c3b2bf15d7edae1a808c0678，为-trimpath发布构建；上一bbce是未使用-trimpath的采样构建。三个根入口直接使用当前确认版；54选择项/63投影、源身份及存档source保持。确认依据用户反馈，未新增逐项实机取证；第三批按授权继续实施，详见PVF启动与内存优化实施计划.md。

## 历史确认基线：启动与内存优化（2026-10-01）

后续第二批后段已形成新源码候选（bbce6de8…），详见PVF启动与内存优化实施计划.md；尚未升级默认程序。下段源码身份指该次已验收构建，当前源码入口已用于后续候选验证。

用户对优化候选反馈“确认没有问题”。正式server/work/dfo-lan/bin/wireprobe-pvf.exe与源码程序均已核对为SHA256 59e14ec18f498f07004b9a797c73e2cdba5f3f8af2245e1401d1953b3105dad3，默认三个根入口直接使用该优化版。54选择项/63类源投影、精确7ef来源、存档source及启动模式保持。

本次确认覆盖紧凑归档索引、固定文件按块读取、有界缓存、目录筛选和四类物品联合扫描。全量Go测试/vet及真实原生对照通过；本机单次只读准备约44.52秒、峰值工作集约3.40GiB。确认依据用户反馈，未新增实机会话日志；采样不作为严格冷盘或进城稳定内存。实施和剩余计划见PVF启动与内存优化实施计划.md。

## 历史记录：全量PVF成为默认入口

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
