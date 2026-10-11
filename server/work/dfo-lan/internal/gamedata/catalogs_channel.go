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
// 决斗场（PKC）频道：客户端 clientchannelinfo/channeluiinfo 里的
// CHANNEL_INTEGRATED_PVP（8，无双/排位）与 CHANNEL_INTEGRATED_FREEPVP（13，自由练习场），
// 城镇都是 Town/Fair_PVP.twn（town 10）。这两条 [channelType] **不在**内层 PVF 的
// clientchannelinfo.etc 里（源表只有 41 个团本/特殊类型），所以上面的 dir.Types() 循环
// 补不到它们。城镇与地图本身随客户端一起发布（list/town.lst + map/fair_pvp/*.map），
// 这里按同一套候选探测补落点；否则切进决斗场频道会沿用普通频道的共享城镇，
// 而 channelWorldIsolated 又会把位置隔离，玩家会落在没有对应地图的坐标上。
const (
	pvpCourtChannelTypeIntegrated = 8
	pvpCourtChannelTypeFreePvp    = 13
	pvpCourtTownID                = 10
)

var pvpCourtChannelTypes = [...]uint32{pvpCourtChannelTypeIntegrated, pvpCourtChannelTypeFreePvp}

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
	c.RaidEntrances, err = s.RaidEntrances(dir)
	if err != nil {
		return err
	}

	// 特殊频道各有**自己的城镇**：征讨/军团这类频道里角色只能在门口的专属城镇活动，
	// 落点必须从该城镇的地图直读，不能沿用普通城镇。
	// 2026-10-04 起不再按 legion/raid 标记过滤：110（SemiRaid 竞拍）、112/120
	//（瘟疫之迪瑞吉）这类未打标记但有专属城镇的频道同样需要落点（通用落点表）。
	towns := map[uint32]catalog.TownArea{}
	for _, channelType := range dir.Types() {
		a, ok := dir.Attributes(channelType)
		if !ok || a.Town <= 0 {
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
	// 决斗场频道（type 8/13）的城镇本地补充：源表没有它们的 [channelType]，
	// 城镇固定是 Town/Fair_PVP.twn（town 10）。候选探测与上面同口径 —— 取第一个
	// 能读出可行走区域的 area，读不出就不补（切进该频道会沿用普通城镇，不静默猜坐标）。
	for _, courtType := range pvpCourtChannelTypes {
		if _, exists := towns[courtType]; exists {
			continue
		}
		var (
			courtTown catalog.TownArea
			courtErr  error
		)
		for _, areaID := range []uint32{0, 1, 2} {
			got, e := s.TownArea(pvpCourtTownID, areaID)
			if e != nil {
				courtErr = e
				continue
			}
			if len(got.Walkable) == 0 {
				continue
			}
			courtTown = *got
			break
		}
		if len(courtTown.Walkable) == 0 {
			log.Printf("PVF channel %d: Fair_PVP town %d 没有可用的可行走区域（最后错误：%v）", courtType, pvpCourtTownID, courtErr)
			continue
		}
		towns[courtType] = courtTown
		courtX, courtY := courtTown.Spawn()
		log.Printf("  channel %d -> town %d/%d %s (spawn %d,%d, walkable=%d) [决斗场本地补充]", courtType, courtTown.TownID, courtTown.AreaID, courtTown.MapPath, courtX, courtY, len(courtTown.Walkable))
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
