package rosterbg

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"slices"
)

// ImportTickets 从当前 PVF 归档读取选角背景券与背景类别表。
//
// 归属：选角背景的 PVF 投影由 rosterbg 拥有（曾经的 catalog.ImportRosterBackgroundTickets），
// 使纯基础设施 catalog 不再反向依赖领域。
func ImportTickets(a *pvf.Archive, index catalog.ItemIndex) (*TicketCatalog, error) {
	if a == nil || index.Source.Checksum == "" || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("roster background PVF/index source mismatch")
	}
	const resourcePath = "etc/selectcharacterver2/selectcharacterver2.etc"
	resource, err := catalog.ReadScript(a, resourcePath)
	if err != nil {
		return nil, err
	}
	backgrounds, err := parseRosterBackgrounds(resource.Cells)
	if err != nil {
		return nil, err
	}
	c := TicketCatalog{Source: a.Snapshot().Checksum, Items: map[uint32]Ticket{}, BackgroundPath: resourcePath, BackgroundSHA256: resource.SHA256, Backgrounds: backgrounds}
	type entry struct {
		item catalog.ItemIndexEntry
		file int
	}
	entries := []entry{}
	for _, item := range index.Items {
		if item.Kind != "stackable" {
			continue
		}
		file, ok := a.FindFile(item.Path)
		if !ok {
			return nil, fmt.Errorf("missing roster background source item %d", item.ID)
		}
		entries = append(entries, entry{item, file.Index})
	}
	slices.SortFunc(entries, func(a, b entry) int { return a.file - b.file })
	for i, row := range entries {
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
		cells, err := a.Tokens(row.item.Path)
		if err != nil {
			return nil, err
		}
		action := rosterFirstSection(cells, "[action type]")
		if len(action) == 0 || action[0].Type != 6 || action[0].Text != "[change bg select character]" {
			continue
		}
		ticket, err := parseRosterTicket(cells)
		if err != nil {
			return nil, fmt.Errorf("background ticket %d: %w", row.item.ID, err)
		}
		script, err := catalog.ReadScript(a, row.item.Path)
		if err != nil {
			return nil, err
		}
		ticket.Path, ticket.SHA256 = script.Path, script.SHA256
		c.Items[row.item.ID] = ticket
	}
	return NewTicketCatalog(c)
}

func rosterFirstSection(cells []pvf.Token, tag string) []pvf.Token {
	for i, t := range cells {
		if t.Type == 3 && t.Text == tag {
			end := i + 1
			for end < len(cells) && cells[end].Type != 3 {
				end++
			}
			return cells[i+1 : end]
		}
	}
	return nil
}

func parseRosterTicket(cells []pvf.Token) (Ticket, error) {
	var ticket Ticket
	action := rosterFirstSection(cells, "[action type]")
	if len(action) != 3 || action[0].Type != 6 || action[0].Text != "[change bg select character]" || action[1].Type != 0 || action[1].Value != 1 || action[2].Type != 0 || action[2].Value < 0 || action[2].Value > math.MaxUint16 {
		return ticket, fmt.Errorf("invalid background ticket action")
	}
	ticket.Background = Background{Category: 1, ID: uint16(action[2].Value)}
	expiration := rosterFirstSection(cells, "[action expiration info]")
	if len(expiration) == 0 || expiration[0].Type != 6 {
		return ticket, fmt.Errorf("missing background authorization expiration")
	}
	ticket.Expiration = expiration[0].Text
	switch ticket.Expiration {
	case "[unlimit]":
		if len(expiration) != 1 {
			return ticket, fmt.Errorf("invalid permanent background authorization")
		}
	case "[period]":
		if len(expiration) != 2 || expiration[1].Type != 0 || expiration[1].Value <= 0 {
			return ticket, fmt.Errorf("invalid period background authorization")
		}
		ticket.Days = uint32(expiration[1].Value)
	case "[date]":
		if len(expiration) != 2 || expiration[1].Type != 6 {
			return ticket, fmt.Errorf("invalid dated background authorization")
		}
		ticket.Until = expiration[1].Text
	default:
		return ticket, fmt.Errorf("unsupported background authorization type")
	}
	return ticket, nil
}

func parseRosterBackgrounds(cells []pvf.Token) ([]Background, error) {
	var out []Background
	inside := false
	group := -1
	imageOpen := false
	seen := map[Background]bool{}
	for i, t := range cells {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[background image]":
			if inside {
				// Inside an image this same tag names the large image resource;
				// only the outer occurrence owns the groups.
				if !imageOpen {
					return nil, fmt.Errorf("nested background image section")
				}
				continue
			}
			inside = true
		case "[/background image]":
			if group >= 0 || imageOpen {
				return nil, fmt.Errorf("unclosed background group")
			}
			inside = false
		case "[group]":
			if !inside {
				continue
			}
			if group >= 0 || i+1 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value < 0 || cells[i+1].Value > 1 {
				return nil, fmt.Errorf("invalid native background group")
			}
			group = int(cells[i+1].Value)
		case "[/group]":
			if !inside {
				continue
			}
			if group < 0 || imageOpen {
				return nil, fmt.Errorf("unclosed background image")
			}
			group = -1
		case "[image]":
			if !inside {
				continue
			}
			if group < 0 || imageOpen || i+1 >= len(cells) || cells[i+1].Type != 0 || cells[i+1].Value < 0 || cells[i+1].Value > math.MaxUint16 {
				return nil, fmt.Errorf("invalid native background image")
			}
			b := Background{Category: uint8(group), ID: uint16(cells[i+1].Value)}
			if seen[b] {
				return nil, fmt.Errorf("duplicate native background")
			}
			seen[b] = true
			out = append(out, b)
			imageOpen = true
		case "[/image]":
			if !inside {
				continue
			}
			if !imageOpen {
				return nil, fmt.Errorf("background image close without opening")
			}
			imageOpen = false
		}
	}
	if inside || group >= 0 || imageOpen || len(out) == 0 {
		return nil, fmt.Errorf("incomplete native background resource")
	}
	return out, nil
}
