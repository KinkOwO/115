package main

// 隐藏 BOSS（奥尔泰尔）的「通关保底」。
//
// 为什么需要它：客户端那套阶梯是**确定性**的 —— 只要 `oath_max == 45`，每次通关都会
// 召唤奥尔泰尔（docs/protocol/endkeeper-of-order-primer-20260926.md §32.1/§32.2）。
// 也就是说「稀有」这件事只能由服务端表达：这里按通关场次攒保底，攒满了才给一次 45。
//
// 为什么不再用「按穿戴誓约装备算档位」：客户端**脱不下誓约槽**（实机 4 条
// equipment_move_committed 全是穿进、无一次被拒）⇒ 穿上 primeval 誓约就永久 45、
// 场场出隐藏 BOSS。旧规则退化成诊断通道（-oath-grades-from-gear）。
//
// 保底只在**通关时**归零、不在进本时：掉线或退出不该吞掉已经攒下的场次。

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// oathGradePrimeval 是唯一会召唤隐藏 BOSS 的档位（§32.1）。取值与
	// internal/inventory/oath_grade.go 的稀有度映射一致（rarity 8 → 45）。
	oathGradePrimeval uint16 = 45
	// oathDefaultProgressClears 是保底场次的默认值：通关这么多场后，下一场必出。
	oathDefaultProgressClears = 5
	// oathDefaultProgressDungeons 是计入保底的副本，默认只有小深渊「调律之边界」。
	oathDefaultProgressDungeons = "100005014"
)

// oathProgressDungeon 返回本次进本的副本号；没有活跃副本时是 0（不参与保底）。
func (w *worldSession) oathProgressDungeon() uint32 {
	if w == nil || w.activeDungeon == nil {
		return 0
	}
	return w.activeDungeon.Definition.ID
}

// oathProgressEnabled 说明这个副本是否计入保底。
func (w *worldSession) oathProgressEnabled(dungeonID uint32) bool {
	if w == nil || w.oathProgressDungeons == nil {
		return false
	}
	return w.oathProgressDungeons[dungeonID]
}

// oathProgressDue 判断保底是否到期：该角色在这个副本上的通关数已达到阈值。
//
// 读错误**往上抛**，不回落成「不到期」：静默吞掉读错误会让隐藏 BOSS 无声消失，
// 那是比启动时直接报错难查得多的行为变更。
func (w *worldSession) oathProgressDue(dungeonID uint32) (bool, error) {
	if w == nil || w.service == nil || w.oathProgressClears <= 0 || !w.oathProgressEnabled(dungeonID) {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clears, err := w.service.Store.OathProgressClears(ctx, w.role.ID, int64(dungeonID))
	if err != nil {
		return false, fmt.Errorf("oath progress read: %w", err)
	}
	return clears >= w.oathProgressClears, nil
}

// noteOathProgressClear 在通关时推进保底，返回 (推进前, 推进后) 的场次。
//
// 到期就归零（这次通关把保底兑现掉），否则 +1。
func (w *worldSession) noteOathProgressClear(dungeonID uint32) (int, int, error) {
	if w == nil || w.service == nil || w.oathProgressClears <= 0 || !w.oathProgressEnabled(dungeonID) {
		return 0, 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before, after, err := w.service.Store.BumpOathProgress(ctx, w.role.ID, int64(dungeonID), w.oathProgressClears)
	if err != nil {
		return 0, 0, fmt.Errorf("oath progress bump: %w", err)
	}
	return before, after, nil
}

// parseOathProgressDungeons 解析逗号分隔的副本号；空串 = 一个都不计入。
func parseOathProgressDungeons(spec string) (map[uint32]bool, error) {
	out := map[uint32]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseUint(part, 10, 32)
		if err != nil || v == 0 {
			return nil, fmt.Errorf("bad oath progress dungeon %q: want a non-zero dungeon id", part)
		}
		out[uint32(v)] = true
	}
	return out, nil
}
