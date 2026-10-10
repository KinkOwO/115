package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// This walker follows the current client's NOTI14 branches, independently of
// EquipmentPayload: 1459A01E0/1459A0220 always read two blocks for list 1,
// and do so for list 3 only if 145A83FA0 resolves an avatar definition.
// 1452E9B8E..1452E9BA7 reads the period only for a resolved avatar template,
// including when the container is list 1. Empty templates cannot resolve.
func readNativeAvatarUpdate(t *testing.T, p []byte, isAvatar func(uint32) bool) []inventory.BagEquipment {
	t.Helper()
	if len(p) < 3 {
		t.Fatal("truncated NOTI14 header")
	}
	space, count := p[0], binary.LittleEndian.Uint16(p[1:3])
	off := 3
	var rows []inventory.BagEquipment
	for n := uint16(0); n < count; n++ {
		if off+protocol.CurrentItemRecordSize > len(p) {
			t.Fatalf("row %d starts at %d beyond %d-byte packet", n, off, len(p))
		}
		i := inventory.BagEquipment{Slot: binary.LittleEndian.Uint16(p[off:]), Template: binary.LittleEndian.Uint32(p[off+2:]), Durability: binary.LittleEndian.Uint16(p[off+11:])}
		off += protocol.CurrentItemRecordSize
		avatar := isAvatar(i.Template)
		if space == 1 || space == 3 && avatar {
			for _, dst := range []*[]byte{&i.AvatarOptions, &i.AvatarSockets} {
				if off+4 > len(p) {
					t.Fatalf("row %d truncated avatar block length", n)
				}
				size := binary.LittleEndian.Uint32(p[off:])
				off += 4
				if uint64(off)+uint64(size) > uint64(len(p)) {
					t.Fatalf("row %d invalid avatar block length %d at %d", n, size, off)
				}
				*dst = append([]byte(nil), p[off:off+int(size)]...)
				off += int(size)
			}
		}
		if avatar {
			if off+4 > len(p) {
				t.Fatalf("row %d truncated avatar period", n)
			}
			i.Period = binary.LittleEndian.Uint32(p[off:])
			off += 4
		}
		rows = append(rows, i)
	}
	if off != len(p) {
		t.Fatalf("native reader consumed %d of %d bytes; unexpected tail %x", off, len(p), p[off:])
	}
	return rows
}

func TestAvatarEmptySlotNativeLayout(t *testing.T) {
	const avatar = uint32(761001876)
	present := inventory.BagEquipment{Slot: 40, Template: avatar, Durability: 3, Period: 123456,
		AvatarOptions: []byte{1, 2, 3}, AvatarSockets: []byte{4, 5, 6, 7}}
	for _, space := range []byte{1, 3} {
		for _, sentinel := range []uint32{0, 0xffffffff} {
			for _, clearFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("list%d/%08x/clear-first=%t", space, sentinel, clearFirst), func(t *testing.T) {
					item := present
					item.Slot = 3
					clear := inventory.BagEquipment{Slot: 4, Template: sentinel, Period: 86400}
					rows := []inventory.BagEquipment{item, clear}
					if clearFirst {
						rows[0], rows[1] = rows[1], rows[0]
					}
					p, err := inventory.EquipmentPayload(space, rows, false)
					if err != nil {
						t.Fatal(err)
					}
					got := readNativeAvatarUpdate(t, p, func(id uint32) bool { return id == avatar })
					for n := range rows {
						if got[n].Slot != rows[n].Slot || got[n].Template != rows[n].Template || got[n].Durability != rows[n].Durability {
							t.Fatalf("native row %d identity/option corrupted: %+v", n, got[n])
						}
						if rows[n].Template == avatar && (got[n].Period != rows[n].Period || !bytes.Equal(got[n].AvatarOptions, rows[n].AvatarOptions) || !bytes.Equal(got[n].AvatarSockets, rows[n].AvatarSockets)) {
							t.Fatalf("native row %d avatar extensions changed", n)
						}
					}
				})
			}
		}
	}
}

// Includes a real character transaction and the actual gateway packet plan,
// rather than constructing only the expected update in the test.
func exerciseAvatarMoveWire(t *testing.T, store *database.Store, rules inventory.WearService) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "avatar-wire-test")
	if err != nil {
		t.Fatal(err)
	}
	const item = uint32(761001876)
	bag := inventory.Bag{Version: "ordinary-bag-v1", Special: map[byte][]inventory.BagEquipment{1: {
		{Slot: 4, Template: item, Durability: 2, Period: protocol.MaxItemPeriod},
	}}}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":4}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "avatar-test", Profession: 11,
		ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: state}, 24)
	if err != nil {
		t.Fatal(err)
	}
	service := &workflow.WearService{Store: store, WearService: rules}
	w := &worldSession{account: account, store: store, role: role}
	session := &equipmentSession{}
	for n, r := range []protocol.ItemMoveRequest{
		{SourceList: 1, SourceSlot: 4, SourceItem: item, DestinationList: 1, DestinationSlot: 30},
		{SourceList: 1, SourceSlot: 30, SourceItem: item, DestinationList: 3, DestinationSlot: 3},
		{SourceList: 1, SourceSlot: 4, DestinationList: 3, DestinationSlot: 3, DestinationItem: item},
	} {
		r.Count, r.Selection = 1, 0xffffffff
		p := make([]byte, 29)
		p[0], p[11] = r.SourceList, r.DestinationList
		binary.LittleEndian.PutUint16(p[1:], r.SourceSlot)
		binary.LittleEndian.PutUint32(p[3:], r.SourceItem)
		binary.LittleEndian.PutUint32(p[7:], r.Count)
		binary.LittleEndian.PutUint16(p[12:], r.DestinationSlot)
		binary.LittleEndian.PutUint32(p[14:], r.DestinationItem)
		binary.LittleEndian.PutUint32(p[22:], r.Selection)
		plan, err := session.handle(service, w, p, p)
		if err != nil {
			t.Fatalf("move %d: %v", n, err)
		}
		updates := 0
		for _, packet := range plan {
			if packet.ID != 14 || packet.Name != "equipment_slots_updated" {
				continue
			}
			readNativeAvatarUpdate(t, packet.Payload, func(id uint32) bool {
				d, err := rules.Catalog.Definition(id)
				return err == nil && d.IsAvatar()
			})
			updates++
		}
		if updates == 0 {
			t.Fatalf("move %d produced no slot update", n)
		}
	}
	stored, err := store.AdminCharacter(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	final, err := inventory.ReadBag(stored.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Special[1]) != 1 || len(final.Worn) != 0 || final.Special[1][0].Slot != 4 || final.Special[1][0].Template != item || final.Special[1][0].Durability != 2 {
		t.Fatalf("move/equip/unequip lost or copied the avatar: %+v", final)
	}
}

func TestAvatarMoveNativeSourceAndWire(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for current-source avatar sweep")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path,
		ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.Equipment(index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	gear := &inventory.EquipmentCatalog{Source: s.Snapshot(), Full: full}
	jobs, err := s.Characters("")
	if err != nil {
		t.Fatal(err)
	}
	wear, err := inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", s.Snapshot().Checksum)
	if err != nil {
		t.Fatal(err)
	}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	rules := inventory.WearService{Catalog: gear, Professions: jobs, Rules: wear, BagRules: bagRules}
	reported, err := gear.Definition(761001876)
	if err != nil || !reported.IsAvatar() || reported.Fields["[equipment type]"][0].Text != "[coat avatar]" {
		t.Fatalf("reported avatar definition unavailable: %v", err)
	}
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			ctx := context.Background()
			var store *database.Store
			if engine == "postgres" {
				if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
					t.Skip("requires an owned isolated PostgreSQL test database")
				}
				fixture, err := database.OpenTestFixture(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := fixture.Close(); err != nil {
						t.Error(err)
					}
				})
				store = fixture.Storage()
			} else {
				var err error
				store, err = database.Open(ctx, database.Config{Driver: database.DriverSQLite,
					SQLitePath: filepath.Join(t.TempDir(), "avatar.sqlite3"), MaxConnections: 2})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { store.Close() })
			}
			exerciseAvatarMoveWire(t, store, rules)
		})
	}
	var ids []uint32
	for id := range index.Items {
		if full.HasDefinition(id) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	counts := map[string]int{}
	avatars, unresolved := 0, 0
	for _, id := range ids {
		d, err := gear.Definition(id)
		if err != nil {
			unresolved++
			continue
		}
		if !d.IsAvatar() {
			continue
		}
		avatars++
		counts[d.Fields["[equipment type]"][0].Text]++
		body, err := inventory.EquipmentPayload(1, []inventory.BagEquipment{{Slot: 4, Template: 0xffffffff},
			{Slot: 30, Template: id, Period: protocol.MaxItemPeriod}}, false)
		if err != nil {
			t.Fatalf("avatar %d: %v", id, err)
		}
		rows := readNativeAvatarUpdate(t, body, func(n uint32) bool { return n == id })
		if rows[0].Template != 0xffffffff || rows[1].Template != id || rows[1].Slot != 30 {
			t.Fatalf("avatar %d destination corrupted", id)
		}
	}
	if avatars == 0 {
		t.Fatal("avatar sweep unexpectedly empty")
	}
	encoded, _ := json.Marshal(counts)
	t.Logf("source=%s equipment=%d avatars=%d unresolved-equipment=%d families=%s reported-path=%s", s.Snapshot().Checksum, len(ids), avatars, unresolved, encoded, reported.Path)
}
