package boostup

import (
	"fmt"
)

type PointRequirement struct {
	Contracts []byte
	Minimum   uint32
}

func (s Step) PointRequirement() (*PointRequirement, error) {
	if s.Mission != "partset point only equip item and contract" {
		return nil, nil
	}
	r := &PointRequirement{}
	seen := map[byte]bool{}
	for _, t := range values(s.MissionCells, "[condition]") {
		if t.Type != 0 || t.Value <= 0 || t.Value > 255 || seen[byte(t.Value)] {
			return nil, fmt.Errorf("invalid boost contract requirement")
		}
		seen[byte(t.Value)] = true
		r.Contracts = append(r.Contracts, byte(t.Value))
	}
	if len(r.Contracts) == 0 {
		return nil, fmt.Errorf("empty boost contract requirement")
	}
	var err error
	r.Minimum, err = number(s.MissionCells, "[condition2]", ^uint32(0))
	if err != nil || r.Minimum == 0 {
		return nil, fmt.Errorf("invalid boost set point requirement")
	}
	return r, nil
}
func (r *PointRequirement) Satisfied(points map[int32]uint32, contracts map[uint8]bool) bool {
	if r == nil {
		return false
	}
	for _, id := range r.Contracts {
		if !contracts[id] {
			return false
		}
	}
	for _, n := range points {
		if n >= r.Minimum {
			return true
		}
	}
	return false
}
// 第三关「技能进化点」的完成门槛读自 [mission][condition]：源里写的是 3，
// 客户端引导条也是「使用 3 点以上」。本地规则原来要求五点全花完，比源更严，
// 只加 3 点的玩家会卡在第三关不刷新。
func (s Step) VPPointCondition() (uint32, error) {
	if s.Mission != "skill vp option" {
		return 0, nil
	}
	min, err := number(s.MissionCells, "[condition]", ^uint32(0))
	if err != nil || min == 0 {
		return 0, fmt.Errorf("invalid boost VP point condition")
	}
	return min, nil
}

// ValidatePointMissions 只做源条款校验，不解析套装积分表：积分总量来自
// character.FameBreakdown.SetPointTotals（名望侧唯一的 setpointinfo.cos 解析器）。
func (c *Catalog) ValidatePointMissions() error {
	for _, s := range c.Steps {
		if _, err := s.PointRequirement(); err != nil {
			return err
		}
	}
	return nil
}
