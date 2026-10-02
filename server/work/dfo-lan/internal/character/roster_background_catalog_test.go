package character

import (
	"errors"
	"testing"
)

func TestNativeTicketsAvoidEmbeddedReadAndProtectSource(t *testing.T) {
	old, err := EmbeddedRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	source := *old
	source.BackgroundPath = "etc/selectcharacterver2/selectcharacterver2.etc"
	source.BackgroundSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for category := 0; category <= 1; category++ {
		for id := 0; id <= 65535; id++ {
			b := RosterBackground{Category: uint8(category), ID: uint16(id)}
			if old.ValidRosterBackground(b) {
				source.Backgrounds = append(source.Backgrounds, b)
			}
		}
	}
	restore, err := InstallRosterBackgroundTickets(&source)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := CurrentRosterBackgroundTickets()
	if err != nil {
		t.Fatal(err)
	}
	original := loadTickets
	loadTickets = func() (*RosterBackgroundTicketCatalog, error) { return nil, errors.New("embedded tickets unavailable") }
	defer func() { loadTickets = original }()
	for id, ticket := range old.Items {
		got, err := RosterBackgroundTicketFor(id)
		if err != nil || got != ticket {
			t.Fatal("native lookup changed", id, err)
		}
		current.Items[id] = RosterBackgroundTicket{}
		if old.Items[id] != ticket {
			t.Fatal("native ticket table shares source map")
		}
		break
	}
	if !(RosterBackground{Category: 1, ID: 51}).Valid() || (RosterBackground{Category: 1, ID: 52}).Valid() {
		t.Fatal("native resource eligibility changed")
	}
	bad := source
	bad.Backgrounds = nil
	if _, err := InstallRosterBackgroundTickets(&bad); err == nil {
		t.Fatal("missing resources installed")
	}
	after, err := CurrentRosterBackgroundTickets()
	if err != nil || after != current {
		t.Fatal("failed install damaged native resources", err)
	}
}
