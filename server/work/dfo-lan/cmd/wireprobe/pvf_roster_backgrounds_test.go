package main

import (
	"dfolan/internal/character"
	"os"
	"testing"
	"time"
)

func TestPVFRosterBackgroundsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete background source parity")
	}
	c, err := preparePVFCoreCatalogs("roster-backgrounds", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := character.EmbeddedRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.installRosterBackgrounds()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := character.CurrentRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.rosterBackgrounds, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Items) != 95 || len(current.Backgrounds) != 63 {
		t.Fatal("background source scope changed")
	}
	checks := 0
	for id, ticket := range old.Items {
		actual, err := character.RosterBackgroundTicketFor(id)
		if err != nil {
			t.Fatal(err)
		}
		moments := []time.Time{time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)}
		if ticket.Until != "" {
			at, err := time.ParseInLocation("2006-01-02 15:04:05", ticket.Until, time.Local)
			if err != nil {
				t.Fatal(err)
			}
			moments = append(moments, at.Add(-time.Second), at, at.Add(time.Second))
		}
		for _, now := range moments {
			want, we := ticket.UnlockAt(now)
			got, ge := actual.UnlockAt(now)
			if got != want || (we == nil) != (ge == nil) {
				t.Fatal("background authorization boundary changed", id, now, got, want, ge, we)
			}
			checks++
		}
	}
	t.Log("native tickets", len(current.Items), "background resources", len(current.Backgrounds), "authorization checks", checks, "resource hash", current.BackgroundSHA256)
}
