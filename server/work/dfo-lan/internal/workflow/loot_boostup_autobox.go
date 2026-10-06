package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
)

// boostAutoStep 编排「毕业一步自动开盒」的账户材料事务（§7.2 E13）；
// 展开与发放纯计算在 loot.PrepareBoostAutoStep。
//
// 事务复用既有的 CommitAccountMaterialEvent：它已经把角色行与同账号的
// account_material_storage 行一起 FOR UPDATE，事件键 boostup-reward:<step>
// 就是幂等闸门，重放不会二次发放，也不会让两个角色的材料入账互相覆盖。
// 背包放不下时整步回滚（不发邮寄），清包后再点一次即可。
func (s *LootService) boostAutoStep(ctx context.Context, role database.Character, c *boostup.Catalog, step byte) (database.Character, loot.BoostStepReceipt, bool, error) {
	var receipt loot.BoostStepReceipt
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, receipt, false, fmt.Errorf("auto step storage missing")
	}
	key := fmt.Sprintf("boostup-reward:%d", step)
	next, _, applied, e := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-step-v1",
		func(cur database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			state, materials, out, err := s.Loot.PrepareBoostAutoStep(LootRole(cur), c, step, counts)
			if err != nil {
				return nil, nil, err
			}
			receipt = out
			return state, materials, nil
		})
	if e != nil {
		return role, receipt, false, e
	}
	if applied {
		if receipt.Step != step || !receipt.Claim || !receipt.AutoOpened || len(receipt.Rewards) == 0 || len(receipt.Granted) == 0 {
			return role, receipt, false, fmt.Errorf("auto step landing receipt mismatch")
		}
		next.WireID = role.WireID
		return next, receipt, true, nil
	}
	// 重放：这一步早就提交过了。发放明细不从历史回执里猜（旧树的回执形状与
	// 本树不同），只如实报告「已领」，由调用方按整包 inventory bootstrap 回给
	// 客户端，账户材料以存储里的实际内容为准。
	next.WireID = role.WireID
	return next, loot.BoostStepReceipt{Step: step, Claim: true, AutoOpened: true}, false, nil
}
