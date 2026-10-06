package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"path/filepath"
	"testing"
)

// 实机 2026-10-04 14:43：662 第二关 CMD16 选图 100004558 被
// `dungeon_request_refused / unsupported dungeon option` 挡死。引导副本在源里是
// `[tutorial dungeon]`，而普通 Select 一律拒绝教程房；SelectGuide 只放开这一条判定。
func TestSelectGuideAcceptsOnlySourceTutorialDungeons(t *testing.T) {
	c, err := catalog.LoadDungeons(filepath.Join("..", "..", "configs", "tutorial-dungeons.current36.json"))
	if err != nil {
		t.Fatal(err)
	}
	var id uint32
	for candidate, d := range c.Dungeons {
		if d.Tutorial {
			id = candidate
			break
		}
	}
	if id == 0 {
		t.Fatal("fixture has no source tutorial dungeon")
	}
	r := protocol.DungeonSelection{ID: id, Party: 65535}
	accepted := map[uint16]bool{}
	if _, err = Select(c, r, 115, accepted); err == nil {
		t.Fatal("ordinary selection accepted a source tutorial dungeon")
	}
	s, err := SelectGuide(c, r, 115, accepted)
	if err != nil {
		t.Fatalf("guide selection refused its own source tutorial dungeon: %v", err)
	}
	if s.Definition.ID != id || !s.Definition.Tutorial || s.RunID == "" || s.Room.Map == 0 {
		t.Fatalf("guide session lost the source tutorial definition or start room: %+v", s.Definition)
	}
	// 放行只针对教程房：拿它进普通副本必须仍然被拒，否则等于绕开选图门禁。
	cleared := c
	cleared.Dungeons = map[uint32]catalog.DungeonDefinition{}
	other := c.Dungeons[id]
	other.Tutorial = false
	cleared.Dungeons[id] = other
	if _, err = SelectGuide(cleared, r, 115, accepted); err == nil {
		t.Fatal("guide selector accepted a non-tutorial dungeon")
	}
	// 其余选项校验不因放行而变松。
	for _, change := range []func(*protocol.DungeonSelection){
		func(r *protocol.DungeonSelection) { r.Party = 2 },
		func(r *protocol.DungeonSelection) { r.Mode = 2 },
		func(r *protocol.DungeonSelection) { r.Event = 1 },
	} {
		bad := protocol.DungeonSelection{ID: id, Party: 65535}
		change(&bad)
		if _, err = SelectGuide(c, bad, 115, accepted); err == nil {
			t.Fatalf("guide selection accepted an unsupported request: %+v", bad)
		}
	}
}
