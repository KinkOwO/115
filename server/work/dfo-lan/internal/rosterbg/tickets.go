package rosterbg

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed tickets.json
var ticketData []byte

// Ticket 只包含当前PVF的背景券规则，不以物品名称或编号推测解锁目标。
type Ticket struct {
	Background Background `json:"background"`
	Expiration string     `json:"expiration"`
	Days       uint32     `json:"days,omitempty"`
	Until      string     `json:"until,omitempty"`
	Path       string     `json:"path"`
	SHA256     string     `json:"sha256"`
}

type TicketCatalog struct {
	Source           string            `json:"source"`
	Items            map[uint32]Ticket `json:"items"`
	BackgroundPath   string            `json:"background_path,omitempty"`
	BackgroundSHA256 string            `json:"background_sha256,omitempty"`
	Backgrounds      []Background      `json:"backgrounds,omitempty"`
	valid            map[Background]bool
}

func (c *TicketCatalog) ValidBackground(b Background) bool {
	if c == nil {
		return false
	}
	if len(c.Backgrounds) == 0 {
		return legacyBackgroundValid(b)
	}
	return c.valid[b]
}

var loadTickets = sync.OnceValues(func() (*TicketCatalog, error) {
	var data TicketCatalog
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return nil, err
	}
	return NewTicketCatalog(data)
})

func NewTicketCatalog(data TicketCatalog) (*TicketCatalog, error) {
	if b, err := hex.DecodeString(data.Source); err != nil || len(b) != 32 || len(data.Items) == 0 {
		return nil, fmt.Errorf("背景券源规则不完整")
	}
	data.valid = map[Background]bool{}
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

var currentTicketCatalog atomic.Pointer[TicketCatalog]

func EmbeddedTickets() (*TicketCatalog, error) { return loadTickets() }

func CurrentTickets() (*TicketCatalog, error) {
	if c := currentTicketCatalog.Load(); c != nil {
		return c, nil
	}
	return loadTickets()
}

func InstallTickets(source *TicketCatalog) (func(), error) {
	if source == nil || len(source.Backgrounds) == 0 {
		return nil, fmt.Errorf("missing native background resources")
	}
	c := *source
	c.Items = make(map[uint32]Ticket, len(source.Items))
	for k, v := range source.Items {
		c.Items[k] = v
	}
	c.Backgrounds = append([]Background(nil), source.Backgrounds...)
	validated, err := NewTicketCatalog(c)
	if err != nil {
		return nil, err
	}
	previous := currentTicketCatalog.Swap(validated)
	return func() { currentTicketCatalog.Store(previous) }, nil
}

func TicketFor(template uint32) (Ticket, error) {
	tickets, err := CurrentTickets()
	if err != nil {
		return Ticket{}, err
	}
	ticket, ok := tickets.Items[template]
	if !ok {
		return Ticket{}, fmt.Errorf("物品%d不是源定义的选角背景券", template)
	}
	return ticket, nil
}

// 14594BC80：期限1按天加到当前时间，期限2为绝对时间，期限3为永久。
// 1402183F0按有符号32位Unix秒显示到期日期，不能用物品删除时间代替背景授权期限。
func (t Ticket) UnlockAt(now time.Time) (Unlock, error) {
	grant := Unlock{Background: t.Background}
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
