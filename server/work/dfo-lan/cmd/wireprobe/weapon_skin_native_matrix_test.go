package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// Real-source coverage checks every profession and representative weapon family.
// It verifies packet ordering and save preservation, not rendered attack frames.
func TestWeaponSkinNativeProfessionAndWeaponFamilyMatrix(t *testing.T) {
	archive := os.Getenv("DFO_WEAPON_SKIN_NATIVE_MATRIX")
	if archive == "" {
		t.Skip("set DFO_WEAPON_SKIN_NATIVE_MATRIX to the current archive")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	chars, err := s.Characters("")
	if err != nil {
		t.Fatal(err)
	}
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.Equipment(index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	ids := []uint32{}
	for id, r := range index.Items {
		if strings.Contains(strings.ToLower(r.Path), "/weapon/") {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	families := map[string]inventory.EquipmentDefinition{}
	avatars := map[byte]uint32{}
	for _, id := range ids {
		path := strings.ToLower(index.Items[id].Path)
		parts := strings.SplitN(path, "/weapon/", 2)
		family := parts[0] + "/" + strings.Split(parts[1], "/")[0]
		if _, ok := families[family]; ok {
			continue
		}
		d, e := full.Definition(id)
		if e != nil {
			continue
		}
		kind := d.Fields["[equipment type]"]
		if len(kind) == 0 {
			continue
		}
		if kind[0].Text == "[weapon]" {
			families[family] = d
		}
		if kind[0].Text == "[weapon avatar]" {
			for pid, prof := range chars.Professions {
				if avatars[pid] == 0 && inventory.UsableByJob(d.Fields, prof.Job, 0) == nil {
					avatars[pid] = id
				}
			}
		}
	}
	covered := map[byte]bool{}
	cases := 0
	townProjections := 0
	for pid, prof := range chars.Professions {
		for family, def := range families {
			if inventory.UsableByJob(def.Fields, prof.Job, 0) != nil {
				continue
			}
			covered[pid] = true
			for _, inDungeon := range []bool{false, true} {
				for _, applied := range []bool{false, true} {
					for _, takingOff := range []bool{false, true} {
						skin := uint32(0)
						if applied {
							skin = def.ID
						}
						bag := inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: skin, WeaponSkins: []uint32{def.ID}, Worn: []inventory.BagEquipment{{Slot: 12, Template: def.ID}}}
						if avatars[pid] != 0 && !takingOff {
							bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: 10, Template: avatars[pid]})
						}
						raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), bag)
						if e != nil {
							t.Fatal(e)
						}
						w := &worldSession{characters: &character.Service{Catalog: chars}, role: database.Character{WireID: 1, Name: "SkinMatrix", Profession: pid, State: raw}}
						if inDungeon {
							w.activeDungeon = &dungeon.Session{}
						}
						move := protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3, DestinationSlot: 10}
						if takingOff {
							move = protocol.ItemMoveRequest{SourceList: 3, SourceSlot: 10, DestinationList: 1}
						}
						base := []outboundPacket{{"worn", 0, 13, []byte{1}}, {"avatar appearance", 0, 2, []byte{2}}}
						if !inDungeon {
							for _, project := range []func(character.Character, [2]byte) ([]byte, error){w.characters.EntryBasicProbe, w.characters.AppearanceProbe} {
								body, e := project(w.role, [2]byte{})
								if e != nil {
									t.Fatal(e)
								}
								if got := binary.LittleEndian.Uint32(body[5+0x6c:]); got != skin {
									t.Fatalf("town override profession=%d family=%s: %d want %d", pid, family, got, skin)
								}
								base[1].Payload = body
								townProjections++
							}
						}
						plan, e := appendDungeonWeaponSkinAfterMove(base, w, move)
						if e != nil {
							t.Fatal(e)
						}
						want := 2
						if applied {
							want = 4
						}
						if len(plan) != want || !bytes.Equal(raw, w.role.State) {
							t.Fatalf("save/order mismatch: %d %s", pid, family)
						}
						if applied {
							last := plan[len(plan)-1]
							if last.Kind != 1 || last.ID != 1565 || !bytes.Equal(last.Payload, protocol.WeaponSkinSelectionEcho(skin)) {
								t.Fatal("skin override not last")
							}
						}
						cases++
					}
				}
			}
		}
	}
	if len(covered) != len(chars.Professions) {
		t.Fatalf("profession coverage %d/%d", len(covered), len(chars.Professions))
	}
	familyIDs := map[string]uint32{}
	for family, definition := range families {
		familyIDs[family] = definition.ID
	}
	professionJobs := map[byte]string{}
	for id, profession := range chars.Professions {
		professionJobs[id] = profession.Job
	}
	out := map[string]any{"source": s.Snapshot(), "professions": len(chars.Professions), "profession_jobs": professionJobs, "weapon_families": len(families), "family_template_ids": familyIDs, "weapon_avatar_professions": len(avatars), "weapon_avatar_templates": avatars, "packet_and_save_cases": cases, "town_header_projections": townProjections, "rendered_animation_tested": false, "scope": "all native professions; representative source weapon families; town/dungeon; applied/cleared; avatar equip/remove; town entry and refresh native header"}
	b, e := json.MarshalIndent(out, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if path := os.Getenv("DFO_WEAPON_SKIN_MATRIX_REPORT"); path != "" {
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log(fmt.Sprintf("native matrix: professions=%d families=%d avatars=%d cases=%d", len(chars.Professions), len(families), len(avatars), cases))
}
