package character

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cloneReattachFixtureCatalog(t *testing.T) *inventory.EquipmentCatalog {
	t.Helper()
	path := filepath.Join(t.TempDir(), "equipment.json")
	data, err := json.Marshal(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "fixture"},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "clear avatar"}},
			}},
			{ID: 112500000, Path: "default.equ", SHA256: strings.Repeat("b", 64)},
			{ID: 517502726, Path: "look.equ", SHA256: strings.Repeat("c", 64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestCloneReattachPacketsDetachThenAssignCover(t *testing.T) {
	for _, tc := range []struct {
		name, worn string
		cover      uint32
	}{
		{"clone only", `[{"slot":3,"template":517500000}]`, 112500000},
		{"clone and look", `[{"slot":3,"template":517500000},{"slot":3,"template":517502726,"group":1}]`, 517502726},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"version":"ordinary-bag-v1","worn":` + tc.worn + `}}`)
			before := append([]byte(nil), state...)
			role := Character{WireID: 503, Profession: 11, State: state}
			s := &Service{DetailedWornCandidate: true, Equipment: cloneReattachFixtureCatalog(t)}
			baseline, err := s.EntryAddition(role)
			if err != nil {
				t.Fatal(err)
			}
			reset, full, ok, err := s.CloneReattachPackets(role)
			if err != nil || !ok {
				t.Fatalf("reattach unavailable: ok=%v err=%v", ok, err)
			}
			empty, err := protocol.DetailedEquipment(nil)
			if err != nil {
				t.Fatal(err)
			}
			// 下发视图现在会按 PVF 默认孔给时装**就地补孔**（上游 90896789 在
			// `entryAdditionWithStats` 里加了 `len(dw.AvatarOptions)==0` 时填
			// `Equipment.DefaultAvatarSockets(item.Template)`）。期望值必须带上同样的孔，
			// 否则比的是"补孔前"的形状 —— 这是期望值过时，不是实现坏了。
			attached, err := protocol.DetailedEquipment([]protocol.DetailedWorn{{
				Slot: 3, Template: 517500000, HeaderTemplateA: tc.cover,
				AvatarOptions: s.Equipment.DefaultAvatarSockets(517500000),
			}})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(reset, empty) || bytes.Contains(reset, attached) || !bytes.Contains(full, attached) {
				t.Fatal("mode-1 pair did not remove then rebuild the Clone with its cover")
			}
			if bytes.Equal(baseline, full) && tc.name == "clone only" {
				t.Fatal("clone-only candidate did not add its native default cover")
			}
			if !bytes.Equal(state, before) {
				t.Fatal("reattach changed persisted state")
			}
		})
	}
}

func TestCloneReattachPacketsRequireClone(t *testing.T) {
	s := &Service{DetailedWornCandidate: true, Equipment: cloneReattachFixtureCatalog(t)}
	role := Character{WireID: 503, Profession: 11, State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":517502726,"group":1}]}}`)}
	reset, full, ok, err := s.CloneReattachPackets(role)
	if err != nil || ok || reset != nil || full != nil {
		t.Fatalf("ordinary look enabled Clone reattach: ok=%v err=%v", ok, err)
	}
}
