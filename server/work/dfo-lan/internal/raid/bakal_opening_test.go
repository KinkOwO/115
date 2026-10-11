package raid

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
	"time"
)

func TestNativeBakalOpeningAdmissionAndCountdown(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &d)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[82]
	now := time.Unix(1700000000, 0)
	for _, members := range [][]Member{nil, {{0, 0}}, {{9, 0}, {9, 1}}, {{9, 0}, {10, 0}}, {{9, 12}}} {
		if _, err = PrepareBakalOpening(entry, members, now); err == nil {
			t.Fatalf("invalid roster accepted: %+v", members)
		}
	}
	s, err := PrepareBakalOpening(entry, []Member{{9, 1}}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{211, 212, 213, 214} {
		if s.SymbolValues()[id] != 1 {
			t.Fatalf("native road monsters self-destruct without presence symbol %d", id)
		}
	}
	if s.SymbolValues()[208] != 0 {
		t.Fatal("reserved Swan incorrectly announced as already present")
	}
	slot := entry.Bakal.Slots[24]
	if len(slot.Maps) == 0 {
		t.Fatal("missing native Bakal map")
	}
	if _, err = s.AuthorizeSlot(24, slot.Dungeon, slot.Maps[0], now); err == nil {
		t.Fatal("entry before start")
	}
	if err = s.Activate(now.Add(2 * time.Second)); err == nil {
		t.Fatal("countdown bypass")
	}
	start := now.Add(3 * time.Second)
	if err = s.Activate(start); err != nil {
		t.Fatal(err)
	}
	if err = s.Activate(start); err == nil {
		t.Fatal("replayed activation")
	}
	if _, err = s.AuthorizeSlot(24, slot.Dungeon, slot.Maps[0], start); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][3]uint32{{24, slot.Dungeon + 1, slot.Maps[0]}, {24, slot.Dungeon, 1}, {52, slot.Dungeon, slot.Maps[0]}, {99, slot.Dungeon, slot.Maps[0]}} {
		if _, err = s.AuthorizeSlot(ids[0], ids[1], ids[2], start); err == nil {
			t.Fatalf("foreign destination admitted: %v", ids)
		}
	}
	m, ok := s.InitialMonster(24)
	if !ok || m.ID != 109014482 {
		t.Fatalf("wrong native boss: %+v", m)
	}
	copyMembers := s.Members()
	copyMembers[0].Actor = 999
	if s.Members()[0].Actor != 9 {
		t.Fatal("caller changed real roster")
	}
	if s.Remaining(start) != 9999 || s.Remaining(start.Add(9999*time.Second)) != 0 {
		t.Fatal("native raid deadline mismatch")
	}
	// Preview must wake the three living dragons before map ACT starts,
	// while failed/abandoned preparation cannot mutate the real battlefield.
	for _, pair := range [][2]uint32{{100003151, 205}, {100003150, 206}, {100003152, 207}} {
		if s.SymbolValues()[pair[1]] != 0 {
			t.Fatal("dragon already awake")
		}
		preview, err := s.EnterDungeon(pair[0], start, false)
		if err != nil || preview[pair[1]] != 1 {
			t.Fatalf("native dragon entry did not wake %v: %v", pair, err)
		}
		if s.SymbolValues()[pair[1]] != 0 {
			t.Fatal("preview changed real raid")
		}
		values, err := s.EnterDungeon(pair[0], start, true)
		if err != nil || values[pair[1]] != 1 || s.SymbolValues()[pair[1]] != 1 {
			t.Fatal("loading did not commit awakening")
		}
		// Repeated loading acknowledgements are idempotent.
		if _, err := s.EnterDungeon(pair[0], start, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.EnterDungeon(1, start, true); err == nil {
		t.Fatal("foreign event accepted")
	}
	if _, err = s.AuthorizeSlot(24, slot.Dungeon, slot.Maps[0], start.Add(9999*time.Second)); err == nil {
		t.Fatal("expired raid admitted dungeon")
	}
}
