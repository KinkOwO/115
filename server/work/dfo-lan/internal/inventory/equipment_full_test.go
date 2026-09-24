package inventory

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFullCatalogIntegrity(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "full")
	sum := strings.Repeat("1", 64)
	script := catalog.ScriptRecord{Path: "equipment/test.equ", SHA256: sum, Cells: []pvf.Token{{Type: 3, Text: "[equipment type]"}, {Type: 6, Text: "[oath]"}}}
	raw, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	z := zlib.NewWriter(&packed)
	z.Write(raw)
	z.Close()
	data := packed.Bytes()
	index := FullEquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, IndexSHA256: sum, Records: map[uint32]equipmentLocation{123: {Size: len(data), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}}}
	raw, err = json.Marshal(&index)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prefix+".index.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prefix+".data", data, 0600); err != nil {
		t.Fatal(err)
	}
	if c, e := OpenFullEquipmentCatalog(prefix, "wrong"); e == nil {
		c.Close()
		t.Fatal("source mismatch accepted")
	}
	c, err := OpenFullEquipmentCatalog(prefix, sum)
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.Definition(123)
	if err != nil || d.Fields["[equipment type]"][0].Text != "[oath]" {
		t.Fatal(d, err)
	}
	if _, err = c.Definition(456); err == nil {
		t.Fatal("missing ID accepted")
	}
	c.Close()
	changed := append([]byte(nil), data...)
	changed[0] ^= 1
	os.WriteFile(prefix+".data", changed, 0600)
	c, err = OpenFullEquipmentCatalog(prefix, sum)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Definition(123); err == nil {
		t.Fatal("tampered record accepted")
	}
	c.Close()
	os.WriteFile(prefix+".data", data[:len(data)-1], 0600)
	if c, err = OpenFullEquipmentCatalog(prefix, sum); err == nil {
		c.Close()
		t.Fatal("truncation accepted")
	}
}

func TestEquipmentPayloadEmptyAvatarAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		space   byte
		slot    uint16
		id      uint32
		restore bool
		size    int
	}{
		{1, 0, 0, false, 192}, {1, 0, 123, false, 196}, {1, 0, 123, true, 198},
		{3, 0, 0, false, 184}, {3, 0, 123, false, 196}, {3, 47, 123, true, 188}, {7, 0, 123, true, 184},
	} {
		p, e := EquipmentPayload(tc.space, []BagEquipment{{Slot: tc.slot, Template: tc.id}}, tc.restore)
		if e != nil || len(p) != tc.size {
			t.Fatalf("%+v bytes=%d error=%v", tc, len(p), e)
		}
	}
	record := bytes.Repeat([]byte{0x53}, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(record[2:], 123)
	item := BagEquipment{Slot: 47, Template: 123, Durability: 12, Record: record}
	row := EquipmentRow(item)
	if !bytes.Equal(row[13:], record[13:]) || binary.LittleEndian.Uint16(row[:]) != 47 || binary.LittleEndian.Uint16(row[11:]) != 12 {
		t.Fatal("instance bytes lost")
	}
	item.Template = 456
	if _, e := EquipmentPayload(3, []BagEquipment{item}, false); e == nil {
		t.Fatal("identity mismatch accepted")
	}
}

func TestFullCatalogWearFamiliesIntegration(t *testing.T) {
	if os.Getenv("EQUIPMENT_INTEGRATION") != "1" {
		t.Skip("set EQUIPMENT_INTEGRATION=1 for imported catalog")
	}
	svc, role := wearFixture(t)
	original, e := LoadEquipmentCatalog("../../configs/equipment.current37.json", role.ConfigVersion)
	if e != nil {
		t.Fatal(e)
	}
	full, e := OpenFullEquipmentCatalog("../../configs/equipment-full", role.ConfigVersion)
	if e != nil {
		t.Fatal(e)
	}
	defer full.Close()
	if len(full.Records) != 424216 || len(full.Errors) != 0 {
		t.Fatal("incomplete catalog")
	}
	expanded := *original
	expanded.Full = full
	svc.Catalog = &expanded
	svc.Rules, e = LoadWearRules("../../configs/equipment-wear.full-candidate.json", role.ConfigVersion)
	if e != nil {
		t.Fatal(e)
	}
	pool := append([]EquipmentDrop(nil), original.DropPool()...)
	ids := []uint32{500050217, 1600057, 63495, 500100209, 500150206, 500200205, 500250208, 500300663, 500310712, 100610096, 500320704, 100333257, 500340547, 500350540, 100360301, 100380071, 500390522, 400400071, 100620011, 500990173, 500990174, 64506, 55502, 517040006, 56723, 517600108, 57116, 52346, 53547, 57915, 55899, 58262, 101590528, 113370040}
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			d, e := full.Definition(id)
			if e != nil {
				t.Fatal(e)
			}
			kind := d.Fields["[equipment type]"][0].Text
			slot, ok := svc.Rules.Slots[kind]
			if !ok {
				t.Fatal("missing slot", kind)
			}
			found := false
			for profession, j := range svc.Professions.Professions {
				for _, allowed := range d.Fields["[usable job]"] {
					if allowed.Text == "[all]" || allowed.Text == j.Job {
						role.Profession = profession
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Fatal("missing profession", kind)
			}
			grow := byte(0)
			if g := d.Fields["[usable grow type]"]; len(g) > 0 && g[0].Value >= 0 {
				grow = byte(g[0].Value)
			}
			record := bytes.Repeat([]byte{0x53}, 181)
			binary.LittleEndian.PutUint32(record[2:], id)
			item := BagEquipment{Slot: 9, Template: id, Record: record, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}, Period: 123}
			bag := Bag{Version: "ordinary-bag-v1"}
			space := EquipmentBagSpace(kind)
			if space == 0 {
				bag.Equipment = []BagEquipment{item}
			} else {
				bag.Special = map[byte][]BagEquipment{space: {item}}
			}
			role.State, e = SaveBag(json.RawMessage(fmt.Sprintf(`{"level":115,"advancement":%d}`, grow)), bag)
			if e != nil {
				t.Fatal(e)
			}
			move := protocol.ItemMoveRequest{SourceList: space, SourceSlot: 9, SourceItem: id, DestinationList: 3, DestinationSlot: slot, Count: 1, Selection: 0xffffffff}
			role.State, e = svc.MoveOrdinary(role, move)
			if e != nil {
				t.Fatal(kind, e)
			}
			worn, e := ReadBag(role.State)
			if e != nil || len(worn.Worn) != 1 {
				t.Fatal(worn, e)
			}
			expected := item
			expected.Slot = slot
			if !reflect.DeepEqual(worn.Worn[0], expected) {
				t.Fatal("instance metadata changed")
			}
			if _, e = svc.MoveOrdinary(role, move); e == nil {
				t.Fatal("stale source replay accepted")
			}
			move.SourceItem = 0
			move.DestinationItem = id
			role.State, e = svc.MoveOrdinary(role, move)
			if e != nil {
				t.Fatal(e)
			}
			restored, e := ReadBag(role.State)
			if e != nil {
				t.Fatal(e)
			}
			rows := restored.Equipment
			if space != 0 {
				rows = restored.Special[space]
			}
			if len(restored.Worn) != 0 || len(rows) != 1 || !reflect.DeepEqual(rows[0], item) {
				t.Fatal("unequip failed to conserve instance")
			}
		})
	}
	if !reflect.DeepEqual(pool, original.DropPool()) || !reflect.DeepEqual(pool, expanded.DropPool()) {
		t.Fatal("drop pool changed")
	}
	weapons := map[string]uint32{
		"[swordman]": 101000013, "[demonic swordman]": 101000013, "[at swordman]": 101000013, "[knight]": 101000013,
		"[fighter]": 102000039, "[at fighter]": 102000039, "[gunner]": 104000032, "[at gunner]": 104000032,
		"[mage]": 106000029, "[at mage]": 106000029, "[creator mage]": 106000029, "[priest]": 108000029,
		"[at priest]": 108000029, "[thief]": 109000029, "[demonic lancer]": 114000018, "[gun blader]": 116000025, "[archer]": 117000075,
	}
	for _, slot := range []uint16{36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46} {
		b := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 100401595}}}
		role.State, e = SaveBag(json.RawMessage(`{"level":115,"advancement":1}`), b)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := svc.MoveOrdinary(role, protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 100401595, DestinationList: 3, DestinationSlot: slot, Count: 1, Selection: 0xffffffff})
		if e != nil {
			t.Fatal("primer slot", slot, e)
		}
		if _, e = ReadBag(raw); e != nil {
			t.Fatal(e)
		}
	}
	for job, id := range weapons {
		d, e := full.Definition(id)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = original.Definition(id); e == nil {
			t.Fatalf("weapon %d unexpectedly in baseline", id)
		}
		kind := d.Fields["[equipment type]"][0].Text
		if e = WearableBy(d.Fields, kind, job, 0, 115); e != nil {
			t.Fatal(job, id, e)
		}
		if e = WearableBy(d.Fields, kind, job, 0, 1); e == nil {
			t.Fatal("low level accepted", id)
		}
		if e = WearableBy(d.Fields, kind, "[invalid job]", 0, 115); e == nil {
			t.Fatal("wrong job accepted", id)
		}
	}
	t.Logf("high-level weapons cover %d professions; wrong level/job rejected", len(weapons))
	t.Logf("catalog=%d reward=%d drop=%d families=%d", len(full.Records), len(original.Rows), len(pool), len(ids))
}
