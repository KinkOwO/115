// RosterBackgroundTicketCatalog contains the background ticket rules and PVF projection.
package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed roster_background_tickets.json
var ticketData []byte

// RosterBackgroundTicket 只包含当前PVF的背景券规则，不以物品名称或编号推测解锁目标。
type RosterBackgroundTicket struct {
	Background RosterBackground `json:"background"`
	Expiration string           `json:"expiration"`
	Days       uint32           `json:"days,omitempty"`
	Until      string           `json:"until,omitempty"`
	Path       string           `json:"path"`
	SHA256     string           `json:"sha256"`
}

type RosterBackgroundTicketCatalog struct {
	Source           string                            `json:"source"`
	Items            map[uint32]RosterBackgroundTicket `json:"items"`
	BackgroundPath   string                            `json:"background_path,omitempty"`
	BackgroundSHA256 string                            `json:"background_sha256,omitempty"`
	Backgrounds      []RosterBackground                `json:"backgrounds,omitempty"`
	valid            map[RosterBackground]bool
}

func (c *RosterBackgroundTicketCatalog) ValidRosterBackground(b RosterBackground) bool {
	if c == nil {
		return false
	}
	if len(c.Backgrounds) == 0 {
		return legacyBackgroundValid(b)
	}
	return c.valid[b]
}

var loadTickets = sync.OnceValues(func() (*RosterBackgroundTicketCatalog, error) {
	var data RosterBackgroundTicketCatalog
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return nil, err
	}
	return NewRosterBackgroundTicketCatalog(data)
})

func NewRosterBackgroundTicketCatalog(data RosterBackgroundTicketCatalog) (*RosterBackgroundTicketCatalog, error) {
	if b, err := hex.DecodeString(data.Source); err != nil || len(b) != 32 || len(data.Items) == 0 {
		return nil, fmt.Errorf("背景券源规则不完整")
	}
	data.valid = map[RosterBackground]bool{}
	if len(data.Backgrounds) > 0 {
		if data.BackgroundPath != "etc/selectcharacterver2/selectcharacterver2.etc" {
			return nil, fmt.Errorf("invalid background resource path")
		}
		if b, err := hex.DecodeString(data.BackgroundSHA256); err != nil || len(b) != 32 {
			return nil, fmt.Errorf("invalid background resource hash")
		}
		for _, background := range data.Backgrounds {
			if background.Category > 1 || data.valid[background] {
				return nil, fmt.Errorf("invalid or duplicate native background")
			}
			data.valid[background] = true
		}
	}
	for id, ticket := range data.Items {
		valid := legacyBackgroundValid(ticket.Background)
		if len(data.Backgrounds) > 0 {
			valid = data.valid[ticket.Background]
		}
		if id == 0 || ticket.Background.Category != 1 || !valid || ticket.Path == "" || len(ticket.SHA256) != 64 {
			return nil, fmt.Errorf("背景券%d的源规则无效", id)
		}
		switch ticket.Expiration {
		case "[unlimit]":
			if ticket.Days != 0 || ticket.Until != "" {
				return nil, fmt.Errorf("永久背景券%d含有期限", id)
			}
		case "[period]":
			if ticket.Days == 0 || ticket.Days > math.MaxInt32/86400 || ticket.Until != "" {
				return nil, fmt.Errorf("背景券%d的天数无效", id)
			}
		case "[date]":
			if _, err := time.ParseInLocation("2006-01-02 15:04:05", ticket.Until, time.Local); err != nil || ticket.Days != 0 {
				return nil, fmt.Errorf("背景券%d的固定期限无效", id)
			}
		default:
			return nil, fmt.Errorf("背景券%d的期限类型尚未支持", id)
		}
	}
	return &data, nil
}

var currentTicketCatalog atomic.Pointer[RosterBackgroundTicketCatalog]

func EmbeddedRosterBackgroundTickets() (*RosterBackgroundTicketCatalog, error) { return loadTickets() }

func CurrentRosterBackgroundTickets() (*RosterBackgroundTicketCatalog, error) {
	if c := currentTicketCatalog.Load(); c != nil {
		return c, nil
	}
	return loadTickets()
}

func InstallRosterBackgroundTickets(source *RosterBackgroundTicketCatalog) (func(), error) {
	if source == nil || len(source.Backgrounds) == 0 {
		return nil, fmt.Errorf("missing native background resources")
	}
	c := *source
	c.Items = make(map[uint32]RosterBackgroundTicket, len(source.Items))
	for k, v := range source.Items {
		c.Items[k] = v
	}
	c.Backgrounds = append([]RosterBackground(nil), source.Backgrounds...)
	validated, err := NewRosterBackgroundTicketCatalog(c)
	if err != nil {
		return nil, err
	}
	previous := currentTicketCatalog.Swap(validated)
	return func() { currentTicketCatalog.Store(previous) }, nil
}

func RosterBackgroundTicketFor(template uint32) (RosterBackgroundTicket, error) {
	tickets, err := CurrentRosterBackgroundTickets()
	if err != nil {
		return RosterBackgroundTicket{}, err
	}
	ticket, ok := tickets.Items[template]
	if !ok {
		return RosterBackgroundTicket{}, fmt.Errorf("物品%d不是源定义的选角背景券", template)
	}
	return ticket, nil
}

// 14594BC80：期限1按天加到当前时间，期限2为绝对时间，期限3为永久。
// 1402183F0按有符号32位Unix秒显示到期日期，不能用物品删除时间代替背景授权期限。
func (t RosterBackgroundTicket) UnlockAt(now time.Time) (RosterBackgroundUnlock, error) {
	grant := RosterBackgroundUnlock{RosterBackground: t.Background}
	var end int64
	switch t.Expiration {
	case "[unlimit]":
		return grant, nil
	case "[period]":
		end = now.Unix() + int64(t.Days)*86400
	case "[date]":
		date, err := time.ParseInLocation("2006-01-02 15:04:05", t.Until, time.Local)
		if err != nil {
			return grant, err
		}
		end = date.Unix()
	default:
		return grant, fmt.Errorf("背景授权期限类型无效")
	}
	if end <= now.Unix() || end > math.MaxInt32 {
		return grant, fmt.Errorf("该背景授权已过期或超出原生时间范围")
	}
	grant.ExpiresAt = uint32(end)
	return grant, nil
}

// ImportRosterBackgroundTickets 从当前 PVF 归档读取选角背景券与背景类别表。
//
// 归属：选角背景的 PVF 投影由 character 领域拥有（曾经的 catalog.ImportRosterBackgroundTickets），
// 使纯基础设施 catalog 不再反向依赖领域。
func ImportRosterBackgroundTickets(a *pvf.Archive, index catalog.ItemIndex) (*RosterBackgroundTicketCatalog, error) {
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
	c := RosterBackgroundTicketCatalog{Source: a.Snapshot().Checksum, Items: map[uint32]RosterBackgroundTicket{}, BackgroundPath: resourcePath, BackgroundSHA256: resource.SHA256, Backgrounds: backgrounds}
	type entry struct {
		item catalog.ItemIndexEntry
		file int
	}
	entries := []entry{}
	missing := 0
	for _, item := range index.Items {
		if item.Kind != "stackable" {
			continue
		}
		file, ok := a.FindFile(item.Path)
		if !ok {
			// devpack 基线差异：缺失源脚本的堆叠物品不可能是背景券，跳过。
			missing++
			continue
		}
		entries = append(entries, entry{item, file.Index})
	}
	if missing > 0 {
		log.Printf("PVF roster backgrounds: %d stackable scripts missing (devpack baseline gap)", missing)
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
	return NewRosterBackgroundTicketCatalog(c)
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

func parseRosterTicket(cells []pvf.Token) (RosterBackgroundTicket, error) {
	var ticket RosterBackgroundTicket
	action := rosterFirstSection(cells, "[action type]")
	if len(action) != 3 || action[0].Type != 6 || action[0].Text != "[change bg select character]" || action[1].Type != 0 || action[1].Value != 1 || action[2].Type != 0 || action[2].Value < 0 || action[2].Value > math.MaxUint16 {
		return ticket, fmt.Errorf("invalid background ticket action")
	}
	ticket.Background = RosterBackground{Category: 1, ID: uint16(action[2].Value)}
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

func parseRosterBackgrounds(cells []pvf.Token) ([]RosterBackground, error) {
	var out []RosterBackground
	inside := false
	group := -1
	imageOpen := false
	seen := map[RosterBackground]bool{}
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
			b := RosterBackground{Category: uint8(group), ID: uint16(cells[i+1].Value)}
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
