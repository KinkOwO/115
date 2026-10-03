package gamedata

import (
	"dfolan/internal/catalog"
	"log"
)

// 频道目录的**直读**准备（业主 2026-10-02：规则层一律以 PVF 为准，不必要的 JSON 逐步废弃）。
//
//	etc/clientchannelinfo.etc   每个 [channelType] 的城镇/副本/军团标记（频道属性）
//	etc/channelslotinfo.etc     特殊频道进哪个 F7 面板（[attach panel]）
//	etc/channel_info.etc        普通频道的 ID/Type/Area/11 个 SourceValues + [dungeon] 区域表
//	list/town.lst + .twn + .map 特殊频道**自己的城镇**（[seriaRoomTown]）与可行走区域
//
// 三份投影在启动期准备好，供 main 里 channelrefresh.Config.Resolve 补全
// configs 中声明的 {ID, Name}，并让"切到特殊频道"能把角色投到该频道的专属城镇。
func preparePVFChannels(c *Catalogs, s *Source) error {
	dir, err := s.ChannelDirectory()
	if err != nil {
		return err
	}
	info, err := s.ChannelInfo()
	if err != nil {
		return err
	}
	c.ChannelDirectory, c.ChannelInfo = dir, info

	// 特殊频道各有**自己的城镇**：征讨/军团这类频道里角色只能在门口的专属城镇活动，
	// 落点必须从该城镇的地图直读，不能沿用普通城镇。
	towns := map[uint32]catalog.TownArea{}
	for _, channelType := range dir.Types() {
		a, ok := dir.Attributes(channelType)
		if !ok || a.Town <= 0 {
			continue
		}
		if !(a.IsLegion || a.IsRaid || a.IsPreRaid || a.IsSemiRaid) {
			continue
		}
		// 源里 [seriaRoomArea] 给 0，但月湖那种赛丽亚房间的 walkable 解析不出来
		// （town.go 只认 map 里的可行走矩形）。逐个探测、取第一个能读出可行走
		// 区域的 —— 不猜，也不为了让它过而放宽校验。
		candidates := []uint32{}
		if a.TownArea >= 0 {
			candidates = append(candidates, uint32(a.TownArea))
		}
		for _, extra := range []uint32{1, 2} {
			dup := false
			for _, seen := range candidates {
				if seen == extra {
					dup = true
				}
			}
			if !dup {
				candidates = append(candidates, extra)
			}
		}
		var (
			picked  catalog.TownArea
			lastErr error
		)
		for _, areaID := range candidates {
			got, e := s.TownArea(uint32(a.Town), areaID)
			if e != nil {
				lastErr = e
				continue
			}
			if len(got.Walkable) == 0 {
				lastErr = nil
				continue
			}
			picked = *got
			break
		}
		if len(picked.Walkable) == 0 {
			log.Printf("PVF channel %d: town %d 没有可用的可行走区域（最后错误：%v）", channelType, a.Town, lastErr)
			continue
		}
		towns[channelType] = picked
	}
	c.ChannelTowns = towns

	log.Printf("PVF channel directory: %d type(s) from %s (%d slot symbol(s)); ordinary rows + [dungeon] from %s; %d channel town(s) from list/town.lst",
		len(dir.ByType), catalog.ClientChannelInfoPath, len(dir.SlotSymbols), catalog.ChannelInfoPath, len(towns))
	for _, channelType := range dir.Types() {
		t, ok := towns[channelType]
		if !ok {
			continue
		}
		x, y := t.Spawn()
		log.Printf("  channel %d -> town %d/%d %s (spawn %d,%d, walkable=%d)", channelType, t.TownID, t.AreaID, t.MapPath, x, y, len(t.Walkable))
	}
	return nil
}
