package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	_ "embed"
	"encoding/json"
	"sync"
)

// Source COS hash d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e.
// Journal nodes are ordered. Every prior node must be complete, matching
// the client's 1402cd7a0 membership check, rather than a level-only unlock.
//
//go:embed odyssey_journal_routes.json
var odysseyJournalRoutes []byte

type odysseyJournalNode = catalog.OdysseyJournalNode

var loadEmbeddedOdysseyJournalRoutes = sync.OnceValues(func() (*catalog.OdysseyJournalRoutes, error) {
	var nodes []odysseyJournalNode
	if err := json.Unmarshal(odysseyJournalRoutes, &nodes); err != nil {
		return nil, err
	}
	return catalog.NewOdysseyJournalRoutes(catalog.OdysseyJournalRoutes{Source: pvf.ArchiveSnapshot{Checksum: catalog.OdysseySource}, Path: catalog.OdysseyJournalPath, SHA256: catalog.OdysseyChaptersJournalSHA, Nodes: nodes})
})

func EmbeddedOdysseyJournalRoutes() (*catalog.OdysseyJournalRoutes, error) {
	return loadEmbeddedOdysseyJournalRoutes()
}

func (s *ProgressionService) OdysseyJournalTeleport(role storage.Character, r protocol.AreaChangeRequest) bool {
	// [MERGE-20260928-JOURNAL-TAILFLAGS] 原来这里是 `r.TailFlags != [2]byte{}`，只接受
	// 全零的尾部标志。但客户端报的尾部标志并非只有全零一种：实机 2026-09-28 从魔界
	// (31,2) 回捷尔瓦的请求带的是 [0,2]（TailFlags[1]=2），不是地图选择器（那一位是
	// 5），却因为「非零」被这道门挡掉，请求于是掉到 strict 的门户检查、被
	// "no authorized source portal to destination" 拒绝，客户端卡在传送门上。
	// 该排除的是地图选择器（TailFlags 里出现 5，见 world.service 与 areaTransition
	// 的 isMapTeleport），不是任何非零值。
	if s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != s.Odyssey.Source || r.Flag != 5 ||
		r.TailFlags[0] == 5 || r.TailFlags[1] == 5 {
		return false
	}
	ids, err := s.odysseyCompleted(role)
	if err != nil {
		return false
	}
	completed := map[uint32]bool{}
	for _, id := range ids {
		completed[id] = true
	}
	routes := s.JournalRoutes
	if routes == nil {
		var err error
		routes, err = loadEmbeddedOdysseyJournalRoutes()
		if err != nil {
			return false
		}
	}
	if routes.Source.Checksum != s.Odyssey.Source {
		return false
	}
	// [MERGE-20260928-JOURNAL-LANDING] 原来这里是 `node.Destination == target`，把落点
	// 坐标也一起比。但客户端用地图选择器/传送门出发时报的是**它自己的**默认落点：
	// 实机 2026-09-28 从根特 6/1（gent2ant1）「前往天界」，请求报 town=12 area=0
	// 落在 (363,229)，而路由表这一站记的是 (257,250) —— 像素级不等就永远匹配不上，
	// 请求于是掉到 strict 的门户检查，客户端收到 area_refused 后卡在门上。
	// 站点的身份是 (town, area)，落点由服务端自己决定，所以只比城镇与区域。
	target := [2]uint32{r.Town, r.Area}
	for _, node := range routes.Nodes {
		if node.Destination[0] == target[0] && node.Destination[1] == target[1] {
			return true
		}
		for _, id := range node.Dungeons {
			if !completed[id] {
				return false
			}
		}
	}
	return false
}
