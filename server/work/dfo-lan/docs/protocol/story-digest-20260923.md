# Story digest 进度修复：attempt 1/3

状态：用户已确认实机验收成功。源码通过 `go test ./...`、`go vet ./...`。

依据：本项目客户端 opcode 表将 CMD1438 标为 `STORY_DIGEST_UPDATE`，NOTI1370 标为 `STORY_DIGEST_INFO`。用户提供的《片头剧情重复播放修复-纯提示词分享版》记录了同版本服务端的明文载荷与存档观察。实现后用户确认实机验收成功。

改动：从 `characters.state.story_digest_level` 恢复四字节小端等级，紧随 NOTI1352 发送 NOTI1370；接收空载荷 CMD1438 时，以当前角色等级在行锁事务中单调推进存档。旧存档缺键按 0 处理，其他状态字段保留；没有数据库结构迁移。

实机验收：用户确认修复成功。代码侧覆盖缺键默认 0、115 与高位小端编码、入场顺序、白名单接收，以及存档只增不减并保留其它角色状态字段。后续若再次出现重播，先检查 `story_digest_restored`、`story_digest_saved` 与 CMD1438 会话日志；若现象不符，停止叠包，保留日志并核对当前客户端 reader 和实际明文。
