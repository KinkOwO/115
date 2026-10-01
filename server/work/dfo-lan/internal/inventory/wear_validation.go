package inventory

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

func (s *WearService) ValidateApplyAmplifyGrimoire(r protocol.AmplifyOptionRequest) error {
	if !AmplifyGrimoiresLoaded() {
		return fmt.Errorf("增幅书规则未装载")
	}
	if r.Type < amplifyTypeVitality || r.Type > amplifyTypeIntelligence {
		return fmt.Errorf("次元属性类型 %d 不在 1..4 范围内", r.Type)
	}
	return nil
}

func (s *WearService) ValidateApplyAmplifyTicket(r protocol.ReinforcementRequest) error {
	if r.Mode != 1 {
		return fmt.Errorf("增幅券请求的 mode 必须是 1，收到 %d", r.Mode)
	}
	if !AmplifyTicketsLoaded() {
		return fmt.Errorf("增幅券规则未装载")
	}
	return nil
}

func (s *WearService) ValidateApplyAmplifyUpgrade(r protocol.ReinforcementRequest) error {
	if r.Mode != 1 {
		return fmt.Errorf("增幅请求的 mode 必须是 1，收到 %d", r.Mode)
	}
	if !AmplifyUpgradeRulesLoaded() {
		return fmt.Errorf("增幅规则未装载")
	}
	return nil
}

func (s *WearService) ValidateApplyEnchantByBead(r protocol.EnchantByBeadRequest) error {
	if !EnchantBeadsLoaded() {
		return fmt.Errorf("附魔宝珠规则未装载")
	}
	if r.BeadSpace != 0 {
		return Refuse(RefusalItems, "附魔宝珠容器 %d 不支持（仅背包）", r.BeadSpace)
	}
	return nil
}

func (s *WearService) ValidateApplyRefine(r protocol.RefineRequest) error {
	if !RefineRulesLoaded() {
		return fmt.Errorf("锻造规则未装载")
	}
	return nil
}

func (s *WearService) ValidateApplyInherit(r protocol.InheritRequest) error {
	if len(r.Entries) == 0 {
		return fmt.Errorf("装备继承请求里没有有效的继承记录")
	}
	return nil
}

func (s *WearService) ValidateReinforceWithMaterial(r protocol.ReinforcementRequest) error {
	if !GoldRulesLoaded() {
		return fmt.Errorf("金币强化规则未装载")
	}
	return nil
}
