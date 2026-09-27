# 穿时装城镇模型不实时刷新 —— Placeholder 只投影了武器槽（2026-09-26）

## 症状（实机）

在 Avatar 窗（UJ）穿戴/脱下时装（CMD19 list1→list3），城镇模型不变化；
重选角色后才显示。服务器侧其实已发出完整刷新集：CMD19 应答、NOTI13 背包/
穿戴重同步、NOTI14 槽位与穿戴窗、mode0 外观刷新（`equipment_appearance_refreshed`）。

## 取证链

1. 会话 `roles_..._20260926_183016`：每次 avatar 穿戴后服务端按序发出
   id19 → id13(包) → id13(worn) → id14(space1 sentinel + space3 新模板) →
   id14(worn window) → **id2 mode0**，但画面不动 → 问题在 id2 的外观段内容。
2. `internal/character/service.go` `wornAppearance`：2026-09-25 的武器互换修复
   （attempt 2/3）只给槽 12/24 填 `Placeholder`，其余槽位（含时装 0..11）留 0，
   注释明言"此次只修正主副手槽，保留其他部位及克隆装扮的现有投影"。
3. 协议定案（`protocol/entry_userinfo.go` 外观块结构注释，指令级）：
   条目 `+1 u32 Placeholder` 保存到 `slot*8+0x30`，`145BEFD60 → 145BD63D0 →
   145BEE6C0` 用它做**城镇模型的装备模板查找**。填 0 = 查找失败 = 模型不更新。

## 修复

`wornAppearance` 对**所有** worn 槽（≤25，Group1 覆盖 Group0 的既有规则不变）
统一 `row.Placeholder = model` —— 与进城路径客户端从 worn 物件自填的值同形
（2026-09-18 实机验证过该自填行为）。武器互换路径行为不变（原本就填）。

## 验证

- `go build ./...`、`go vet ./...` 通过；`go test ./internal/character/
  ./cmd/wireprobe/` 通过（appearance_inventory / equipment_avatar_refresh /
  equipment_appearance_policy 等既有断言不涉及 Placeholder，未改动即通过）。
- 实机（2026-09-26 用户确认）：Avatar 窗穿/脱时装，城镇模型立即更新，
  无需重选角色；称号/装备/装扮混穿无异常。
- 部署二进制 SHA-256 `e819b12032ed8f2c392b39ed4a219a4bf913ed4b4c75428a1f9a2206141a104a`
  （与商城发货分类修复同一候选版）。
