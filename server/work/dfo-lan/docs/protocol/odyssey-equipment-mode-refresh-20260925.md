# 奥德赛换装备后的角色模式刷新（confirmed baseline，2026-09-25）

## 现象与根因

奥德赛角色在城镇更换装备后，客户端会把角色识别为剧情模式，造成副本信息错误并可能进入错误副本。重新选择角色，或进出副本触发完整入场刷新后，模式才恢复。

CMD19 装备变更后的 `AppearanceProbe` 会重发 mode0 userinfo。此前它构造的 `CharacterRow` 没有设置 `Odyssey`，协议编码器因此使用默认普通模式 `0`；登录/进城路径则使用真实角色模式，奥德赛为 `5`。

## 修复与确认范围

`internal/character/service.go` 的 `AppearanceProbe` 现在调用 `s.IsOdyssey(role)`，并将结果写入 `CharacterRow.Odyssey`。该判断与 `EntryBasicProbe` 一致，毕业角色等既有角色规则继续由 `OdysseyRole` 处理。普通剧情角色仍编码为 `0`，奥德赛角色编码为 `5`；装备外观和宠物字段保留。

用户已实机确认：奥德赛模式下城镇更换装备后不再切换为剧情模式，副本信息正确。该行为作为当前已确认基线。

## 回归与构建

- `TestAppearanceProbePreservesCharacterMode` 覆盖奥德赛字节 `5`、剧情字节 `0`，并校验外观刷新模式与入场模式一致。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `server/Build-Server.ps1` 已构建源码候选版；归档版 `wireprobe-dungeon39.exe` 未覆盖。
- 未修改数据库结构、角色存档格式或客户端补丁。
