# 频道名称同步修复与确认基线（2026-09-24）

## 用户验收

本机用户截图显示 `ch10.LAN Normal`，随后明确回复“已修复成功”。运行日志记录 NOTI2435 发送所用的服务器/频道：2026-09-24 08:48:37Z 为 1/10，08:50:04Z 为 1/6，08:50:07Z 回到 1/10。

实机验收二进制 SHA256：`2370266cc27814530482e861cd4574493eeb51e4482d863dce7c6cc0a53c3ec5`。该本机构建含其它已有本机修改；本合并申请仅将已验收的频道改动移植到上游 `f760d60`，不将本机整树覆盖上游，也不上传二进制、玩家存档或本机连接配置。因此本申请的重新构建产物不能冒称与实机二进制逐字节相同。

## 实现

- `-channel-identity` 显式启用：由连接监听端口对应的目录项决定服务器号、频道号、频道类型。
- 登录成功后发送 ch0/op2435：四个 little-endian u32 为服务器号、频道号、0、频道类型。
- 每个连接复制 character.Service，设置 ChannelContext。角色基本信息、换装外观、附加信息统一使用该上下文，不修改共享服务实例，不影响同进程其它频道。
- 登录应答按照本项目 `cmd/loginchannel` 已验证的布局修改 **payload[3]**，保留其余内容和原帧头、重算校验；不能照抄参考文档中的其它版本偏移。
- 服务器与频道必须是非零 u8，类型须能放入登录 u8；损坏登录校验拒绝处理。
- `launcher.local.json` 的布尔选项 `channel_identity` 直接传为 DFO_CHANNEL_IDENTITY，再由 channel_probe.py 转为命令行开关。即使一键启动器没有传 --repair-profile，也能生效。本机配置是开关最终来源，默认关闭；profile 里的同名环境项不会覆盖本机选择。
- 停服名单补上独立候选程序名，避免旧进程退出游戏后仍占用17001，导致下次启动失败。

## 部署

在 `server/work/dfo-lan` 执行 `go test ./...`、`go vet ./...`，构建新服务端。先关闭游戏/启动器，备份原配置；本机 `server/launcher.local.json` 设置 `channel_identity: true`，并将 server_binary 指向**包含本提交**的构建。若显式使用 repair profile，其 binary 也应指向同一程序。保留原有配置字段，勿上传本机文件。

例如只需合并如下设置到现有配置，不要用它覆盖整个配置：

```json
{"channel_identity": true, "server_binary": "work/dfo-lan/bin/wireprobe-handoff-source.exe"}
```

验证日志须有 `channel_identity_sent`，同时检查客户端不再显示 ch00.Unknown。停服后确认目录端口释放。回退时关闭游戏后将开关改为false，再恢复旧程序即可；无数据库迁移。

## 验证边界

本合并分支基于 f760d60 重新执行 `go test ./...`、`go vet ./...` 和 `go build -trimpath`，均通过；`python scripts/test_channel_identity_wiring.py` 两项测试通过。新增 channel_identity.go 与实机候选源码 SHA256 一致：`e3578747fb613310f4e728375b8d1bdbecca9f0ee86f63ea899e79c1605eb45e`。

频道目录加密、3模式启动、127.0.0.2游戏监听及CMD3放行均为本机已有逻辑，本次不重写。保留已验收的 first 服务器键和目录命名。测试覆盖登录原样本逐字节往返、频道边界、模式0/1与换装上下文、连接隔离及启动开关/停服名单。

确认范围为频道标签与本次用户反馈；不据此宣称专用频道名望/等级/任务限制、升级特效、游戏内“完成当前等级任务”均已修复。
