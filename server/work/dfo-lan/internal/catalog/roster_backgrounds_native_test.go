package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestRosterTicketUsesAuthorizationDate(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[expiration date]"}, {Type: 6, Text: "2020-04-16 06:00:00"}, {Type: 3, Text: "[action type]"}, {Type: 6, Text: "[change bg select character]"}, {Type: 0, Value: 1}, {Type: 0, Value: 0}, {Type: 3, Text: "[/action type]"}, {Type: 3, Text: "[action expiration info]"}, {Type: 6, Text: "[date]"}, {Type: 6, Text: "2020-09-03 06:00:00"}}
	ticket, err := parseRosterTicket(cells)
	if err != nil || ticket.Until != "2020-09-03 06:00:00" {
		t.Fatal(ticket, err)
	}
	cells = cells[:7]
	if _, err := parseRosterTicket(cells); err == nil {
		t.Fatal("item deletion date used as authorization fallback")
	}
}

func TestRosterBackgroundsRequireClosedOwnerGroups(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[background image]"}, {Type: 3, Text: "[group]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[image]"}, {Type: 0, Value: 500}, {Type: 3, Text: "[background image]"}, {Type: 6, Text: "background.img"}, {Type: 0, Value: 19}, {Type: 3, Text: "[/image]"}, {Type: 3, Text: "[/group]"}, {Type: 3, Text: "[/background image]"}}
	backgrounds, err := parseRosterBackgrounds(cells)
	if err != nil || len(backgrounds) != 1 || backgrounds[0].ID != 500 {
		t.Fatal(backgrounds, err)
	}
	cells[8].Text = "[name]"
	if _, err := parseRosterBackgrounds(cells); err == nil {
		t.Fatal("unclosed image accepted")
	}
}
