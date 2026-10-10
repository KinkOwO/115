# 登录后台发送生命周期：构建确认基线

业主确认原话：“确认能编译通过，可以提交”。确认范围为本轮编译和自动测试；游戏实机行为仍沿用既有确认范围。

## 修复与证据

- 原构建在 `connectionOutput.send(nil)` 的互斥锁访问处panic，来源为 `ispinsPostSelection` 保活首个30秒tick；新增虚拟时间回归在合并上游前的原实现上复现相同调用栈。
- 当前上游已经有goroutine内部的nil保护。本提交在启动前冻结输出，缺输出时不启动后台保活；伊斯频道仍挂起pending账本。保活和普通/次元回廊延迟登录推送收到连接done后退出，次元回廊日志冻结频道身份。
- 30秒间隔、16字节N1719体、1.1秒延迟以及N2254/781/782发包顺序保持。`ispins_wiring_test.go` 覆盖正常保活帧、无输出不崩、断连取消保活及两类延迟登录；既有次元回廊登录分流测试通过。

## 验证与程序身份

Go1.26.0，`GOTOOLCHAIN=local`，通过原入口 `pwsh -NoProfile -File scripts/build-game.ps1`：

- `go build ./...`：退出0。
- `go vet ./...`：退出0。
- `go test ./... -count=1`：退出0，失败集合空；wireprobe 64.782秒，launcher 3.740秒。
- 两个程序真实编译和模块身份检查通过；本次完整工作树包含另行提交的Python清单固定夹具修复，该项见对应确认基线。

日志及旧件备份：`runtime/build-game/20261011-035945-351/`。

| 产物 | 字节数 | SHA256 |
| --- | ---: | --- |
| `bin/wireprobe-handoff-source.exe` | 31386624 | `82af37d170a2a924ada534fd8a812f785d4cab6b2d7eb0b5d3cfb642ccdc4fef` |
| `bin/dfolauncher.exe` | 16979968 | `0d9301cdb26c6f2d105725c29ff8cee76ca40dcee5bef941ed7db9a6a7b1f228` |

## 提交与边界

当前基线已合并gud/main；提交前成功fetch并确认无落后。本机没有origin/fork，两个名字的fetch返回128，不猜测或新增远端。只提交本修复的源码、测试、CHANGELOG及本文；二进制、日志、存档、缓存及其它工作区文件不入库。

未修改存档格式或schema，无迁移需求；未启动客户端或操作玩家存档。默认程序未发布，未声称完成游戏实机回归。
