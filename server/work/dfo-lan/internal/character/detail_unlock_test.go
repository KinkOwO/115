package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"strings"
	"testing"
)

func unlockState(t *testing.T, flags byte) json.RawMessage {
	t.Helper()
	base := `{"source_sha256":"` + strings.Repeat("a", 64) +
		`","level":84,"attributes":{"[hp max]":100,"[mp max]":50}}`
	state, e := inventory.SaveBag(json.RawMessage(base), inventory.Bag{Version: "ordinary-bag-v1", ExpandEquipFlags: flags})
	if e != nil {
		t.Fatal(e)
	}
	return state
}

// 登录时 armoury 只能从 USERINFO1 的开槽字节恢复解锁状态，所以存档里的
// expand_equip_flags 必须投影到该字节上；缺字段的老存档读作 0。
func TestEntryAdditionProjectsSavedUnlockByte(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	role := func(flags byte) storage.Character {
		return storage.Character{WireID: 3, Name: "LanTest01", Profession: 0, State: unlockState(t, flags)}
	}

	locked, e := s.EntryAddition(role(0))
	if e != nil {
		t.Fatal(e)
	}
	if locked[360] != 0 {
		t.Fatalf("legacy/empty save unlock byte: %d", locked[360])
	}
	for _, flags := range []byte{1, 2, 16, 1 | 2 | 16} {
		got, e := s.EntryAddition(role(flags))
		if e != nil {
			t.Fatal(e)
		}
		if got[360] != flags {
			t.Fatalf("saved flags %d reached the payload as %d", flags, got[360])
		}
		if len(got) != len(locked) {
			t.Fatalf("flags %d changed the payload length", flags)
		}
	}
}
