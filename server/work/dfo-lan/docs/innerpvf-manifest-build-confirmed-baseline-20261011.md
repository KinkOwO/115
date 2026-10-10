# Python内层PVF清单兼容测试：构建确认基线

业主确认原话：“确认能编译通过，可以提交”。本基线只确认编译和自动测试，不新增游戏实机验收。

## 问题与修复

原 `TestDecideOnThePythonWrittenManifest` 直接读取本机忽略目录中的旧清单、归档及活动launcher.local.json，错误地假设当前客户端必须与旧产物同源。合并上游后的构建日志 `runtime/build-game/20261011-035537-893/build.log` 记录此项失败，独立执行同名测试也复现。

只读核对发现当前DFO.exe的SHA256与旧清单不同，sk.dat与Script.pvf相同。`decideInnerManifest` 此时要求重建是正确行为，不能为了让测试通过而放宽生产校验或改写用户运行资源。

测试改为读取 `internal/launcher/testdata/inner-manifest-python.json`，对 `buildInnerTree` 创建的临时三件套及13字节 `inner-archive` 验证复用。975字节夹具由Python hashlib.sha256和json.dumps(ensure_ascii=False, indent=2)生成，保留格式、decoder、路径、size、hash和cache结构；mtime_ns固定0，强制走指纹慢路径。测试执行无需Python，也不依赖任何本机运行产物。

另逐项修改临时DFO.exe/sk.dat/Script.pvf，确认每项都要求重建。生产 `innerpvf.go` 不变；夹具仅供测试，不是PVF内容或运行配置的第二来源。

## 验证与产物

专项覆盖Python清单同源复用、三种输入变化、清单解析、渲染及门禁分支，全部通过。Go1.26.0原构建入口 `pwsh -NoProfile -File scripts/build-game.ps1`：

- `go build ./...`：退出0。
- `go vet ./...`：退出0。
- `go test ./... -count=1`：退出0，失败集合空；launcher 3.740秒。
- 启动器与源码候选真实编译、模块身份检查通过。

日志及旧件备份：`runtime/build-game/20261011-035945-351/`。源码候选 `bin/wireprobe-handoff-source.exe` 为31386624字节、身份 `dfolan/cmd/wireprobe`、SHA256 `82af37d170a2a924ada534fd8a812f785d4cab6b2d7eb0b5d3cfb642ccdc4fef`；启动器 `bin/dfolauncher.exe` 为16979968字节、身份 `dfolan/cmd/dfolauncher`、SHA256 `0d9301cdb26c6f2d105725c29ff8cee76ca40dcee5bef941ed7db9a6a7b1f228`。

## 提交与边界

当前树已经合并gud/main；提交前fetch实际远端成功，无待合并更新。本机无origin/fork，未擅自新增远端。仅提交测试、合成夹具、CHANGELOG及本文；上一项登录后台生命周期修复分开提交。日志、二进制、缓存、玩家存档及其它工作区文件不入库。

本轮未重建本机内层PVF、未修改客户端或运行清单、未启动客户端，schema与存档格式不变，无迁移需求。默认程序未发布，实机回归未执行。
