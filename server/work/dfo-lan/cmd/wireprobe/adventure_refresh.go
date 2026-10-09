package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/game/protocol"
	"encoding/json"
)

// 用现有串行会话轮询发送账号成长变化，签名未变化时不重复刷新界面。
func (w *worldSession) refreshAdventure(ctx context.Context) ([]outboundPacket, error) {
	if w.characters == nil || w.store == nil || w.fatigue == nil || !w.adventureReady {
		return nil, nil
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	// 迷雾进度有专用2799/2858通知，不因每次经验变化重建整个冒险团窗口。
	snapshot := profile
	snapshot.Data.SeasonLevel = adventure.SeasonState{}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	raw = append(raw, byte(w.role.WireID), byte(w.role.WireID>>8))
	signature := sha256.Sum256(raw)
	var packets []outboundPacket
	if signature != w.adventureSnapshot {
		detail, err := w.adventureDetail(ctx)
		if err != nil {
			return nil, err
		}
		rules, err := adventure.Current()
		if err != nil {
			return nil, err
		}
		experience := protocol.AdventureExperience(w.role.WireID, profile.Level, profile.Experience, rules.Experience[profile.Level+1])
		packets = append(packets, outboundPacket{"冒险团经验同步", 0, 1337, experience}, outboundPacket{"冒险团资料同步", 0, 1331, detail})
		packets = append(packets, outboundPacket{"图鉴装备登记恢复", 0, 2425, protocol.AdventureCollectionGuide(profile.Data.CollectionEquipment)})
		w.adventureSnapshot = signature
	}
	elitePackets, err := w.refreshAdventureEliteSelections(ctx, profile)
	if err != nil {
		return nil, err
	}
	packets = append(packets, elitePackets...)
	seasonPackets, err := w.refreshSeason(ctx)
	if err != nil {
		return nil, err
	}
	packets = append(packets, seasonPackets...)
	return packets, nil
}

// forceAdventureRefresh 在**通关结算**时无条件重发冒险团三帧
// （NOTI1337 经验 / NOTI1331 资料 / NOTI2425 图鉴），并刷新签名缓存。
//
// 为什么需要「无条件」：`refreshAdventure` 用资料快照签名去重，而军团本翻牌
// 的奖励只进背包、不改冒险团资料 ⇒ 签名不变 ⇒ 三帧被静默跳过。
//
// 证据（2 号权威抓包 D:\zhuabao\captures\20261008-184805，105.85s 清关后）：
//
//	idx=641 105.85s N2252 LEGION_BASIC_CLEAR_REWARD
//	idx=642 105.86s N2253 LEGION_ADDITIONAL_CLEAR_REWARD
//	idx=644 107.26s N1337 ADVENTURE_LV_EXP            ← 本实现从不发
//	idx=645 107.27s N1331 ADVENTURE_INFO              ← 本实现从不发
//	idx=646 107.27s N2425 ADVENTURE_COLLECTION        ← 本实现从不发
//
// 这三帧驱动客户端的结算面板：业主截图（2026-10-08 2 号）显示结算窗顶部是
// 「更新账号纪录! **01分20秒**」（= 本次通关耗时，对应 N1337 的经验/纪录行），
// 下面是装备与星蕴石清单。缺这三帧时面板就是空的。
func (w *worldSession) forceAdventureRefresh(ctx context.Context) ([]outboundPacket, error) {
	if w == nil || w.characters == nil || w.store == nil || !w.adventureReady {
		return nil, nil
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	detail, err := w.adventureDetail(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := adventure.Current()
	if err != nil {
		return nil, err
	}
	experience := protocol.AdventureExperience(w.role.WireID, profile.Level, profile.Experience, rules.Experience[profile.Level+1])
	packets := []outboundPacket{
		{"apocalypse_adventure_experience", 0, 1337, experience},
		{"apocalypse_adventure_info", 0, 1331, detail},
		{"apocalypse_adventure_collection", 0, 2425, protocol.AdventureCollectionGuide(profile.Data.CollectionEquipment)},
	}
	// 刷新签名缓存，避免紧接着的 tick 再重复发一遍。
	snapshot := profile
	snapshot.Data.SeasonLevel = adventure.SeasonState{}
	raw, err := json.Marshal(snapshot)
	if err == nil {
		raw = append(raw, byte(w.role.WireID), byte(w.role.WireID>>8))
		w.adventureSnapshot = sha256.Sum256(raw)
	}
	return packets, nil
}
