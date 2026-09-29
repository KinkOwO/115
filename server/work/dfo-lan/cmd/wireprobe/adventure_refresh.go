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
	if w.characters == nil || w.characters.Store == nil || w.fatigue == nil || !w.adventureReady {
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
	elite, err := w.adventureElitePayload(ctx, profile)
	if err != nil {
		return nil, err
	}
	eliteSignature := sha256.Sum256(elite)
	if eliteSignature != w.adventureEliteSnapshot {
		packets = append(packets, outboundPacket{"精锐角色设置恢复", 0, 1754, elite})
		w.adventureEliteSnapshot = eliteSignature
	}
	seasonPackets, err := w.refreshSeason(ctx)
	if err != nil {
		return nil, err
	}
	packets = append(packets, seasonPackets...)
	return packets, nil
}
