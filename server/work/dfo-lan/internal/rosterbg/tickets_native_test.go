package rosterbg

import (
	"errors"
	"testing"
)

func TestNativeTicketsAvoidEmbeddedReadAndProtectSource(t *testing.T) {
	old, err := EmbeddedTickets()
	if err != nil {
		t.Fatal(err)
	}
	source := *old
	source.BackgroundPath = "etc/selectcharacterver2/selectcharacterver2.etc"
	source.BackgroundSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for category := 0; category <= 1; category++ {
		for id := 0; id <= 65535; id++ {
			b := Background{Category: uint8(category), ID: uint16(id)}
			if old.ValidBackground(b) {
				source.Backgrounds = append(source.Backgrounds, b)
			}
		}
	}
	restore, err := InstallTickets(&source)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := CurrentTickets()
	if err != nil {
		t.Fatal(err)
	}
	original := loadTickets
	loadTickets = func() (*TicketCatalog, error) { return nil, errors.New("embedded tickets unavailable") }
	defer func() { loadTickets = original }()
	for id, ticket := range old.Items {
		got, err := TicketFor(id)
		if err != nil || got != ticket {
			t.Fatal("native lookup changed", id, err)
		}
		current.Items[id] = Ticket{}
		if old.Items[id] != ticket {
			t.Fatal("native ticket table shares source map")
		}
		break
	}
	if !(Background{Category: 1, ID: 51}).Valid() || (Background{Category: 1, ID: 52}).Valid() {
		t.Fatal("native resource eligibility changed")
	}
	bad := source
	bad.Backgrounds = nil
	if _, err := InstallTickets(&bad); err == nil {
		t.Fatal("missing resources installed")
	}
	after, err := CurrentTickets()
	if err != nil || after != current {
		t.Fatal("failed install damaged native resources", err)
	}
}
