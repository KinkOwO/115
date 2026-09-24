# 赛丽亚房间返程基线（2026-09-24）

## 已确认行为

- 从西海岸进入赛丽亚房间后，靠近房间出口可以正常离开并返回西海岸；此项由用户手动实机确认。
- CMD 1418 `PREV_VILLAGE` 空请求使用角色进房时保存的 Return 戳发起返程，成功回显请求 opcode，并复用 CMD 36 的切图通知顺序。
- CMD 36 离开赛丽亚房间时，服务端忽略客户端给出的动画落点，恢复 Return 戳中的 Town、Area、X、Y。
- 没有 Return 戳时拒绝 CMD 1418；不在 ReturnWarpBounds 出门范围内时拒绝返程。
- 房内同区域移动保留原 Return 戳；数据库结构及 `WorldReturn` 存档格式未变。

## 实现与验证

- `cmd/wireprobe/main.go` 将 CMD 1418 纳入世界请求分发。
- `cmd/wireprobe/request_scope.go` 保留 CMD 1418 请求体用于诊断。
- `cmd/wireprobe/world_flow.go` 增加空请求、城镇状态和 Return 戳校验；成功后发送 ACK 1418、NOTI 23 和 NOTI 24。
- `internal/world/service.go` 按 Return 戳验证和落点，保留出门光圈距离校验；同区重定位不清除戳。
- `cmd/wireprobe/odyssey_teleport.go` 允许赛丽亚出口请求进入 Return 戳迁移校验。
- 回归测试覆盖 CMD 1418 拒绝条件、通用/无效客户端目的地、多个来源城镇、出口距离和同区移动。
- `go test ./...`：21 个测试包通过；`go vet ./...` 与 `go build ./...` 通过。
- 候选程序：`bin/wireprobe-handoff-source.exe`，SHA-256 `D16CF21ACF5658EAA75C03BA6007477A10FF286894E4098E439B6E21E577F201`。

## 验收边界

已实机确认西海岸单来源往返。阿法利亚、亨顿玛尔、艾尔文等其他来源仍由单元测试覆盖，尚未逐一实机验证。客户端由用户手动操作；归档版 `wireprobe-dungeon39.exe` 未替换。
