package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// ValidateSelection 只做「这次选择能不能通过」的判定，不把结果拿去开局。
//
// 捐赠者主线在 session.go 里把它实现成 resolveSelection 的薄包装（那一层不是本仓库的形状），
// 本补丁按上游既有做法调用 Select 并丢弃会话：上游自己的 dungeonaudit / dungeonfull /
// charactercheck 也都是这样校验的。这里只用于军团入场计划（BuildEntryPlan）在启动期核对
// 六个源目的地是否真的可解析——它不落任何存档，也不发任何包。
func ValidateSelection(c catalog.DungeonCatalog, r protocol.DungeonSelection, level byte, accepted map[uint16]bool) error {
	_, e := Select(c, r, level, accepted)
	return e
}
