package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/game/protocol"
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
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	info, err := w.adventureInfoForRoles(profile, roles)
	if err != nil {
		return nil, err
	}
	identity := w.characters.ChannelContext
	response, err := protocol.AdventureResponse(protocol.AdventureRequest{Target: w.role.WireID, Kind: 1}, identity[0], uint32(identity[1]), info)
	if err != nil {
		return nil, err
	}
	detail := response[1:]
	collection := protocol.AdventureCollectionGuide(profile.Data.CollectionEquipment)
	// 对实际下发内容取签名，包含职业/觉醒/等级/主力与角色总数。
	// 账号经验封顶、角色重排或GM升级不能让职业收藏永久停留在旧快照。
	// 迷雾进度另有2799/2858，不参与此窗口签名。
	h := sha256.New()
	h.Write(detail)
	h.Write(collection)
	var signature [32]byte
	copy(signature[:], h.Sum(nil))
	var packets []outboundPacket
	if signature != w.adventureSnapshot {
		rules, err := adventure.Current()
		if err != nil {
			return nil, err
		}
		experience := protocol.AdventureExperience(w.role.WireID, profile.Level, profile.Experience, rules.Experience[profile.Level+1])
		packets = append(packets, outboundPacket{"冒险团经验同步", 0, 1337, experience}, outboundPacket{"冒险团资料同步", 0, 1331, detail})
		packets = append(packets, outboundPacket{"图鉴装备登记恢复", 0, 2425, collection})
		w.adventureSnapshot = signature
	}
	elite, err := adventureElitePayloadForRoles(profile, roles, w.role.ID)
	if err != nil {
		return nil, err
	}
	elite, err = w.boostTrainingAPCMerge(elite)
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
