package rosterbg

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sync"
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

var loadTickets = sync.OnceValues(func() (map[uint32]Ticket, error) {
	var data struct {
		Source string            `json:"source"`
		Items  map[uint32]Ticket `json:"items"`
	}
	if err := json.Unmarshal(ticketData, &data); err != nil {
		return nil, err
	}
	if len(data.Source) != 64 || len(data.Items) == 0 {
		return nil, fmt.Errorf("背景券源规则不完整")
	}
	for id, ticket := range data.Items {
		if id == 0 || ticket.Background.Category != 1 || !ticket.Background.Valid() || ticket.Path == "" || len(ticket.SHA256) != 64 {
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
	return data.Items, nil
})

func TicketFor(template uint32) (Ticket, error) {
	tickets, err := loadTickets()
	if err != nil {
		return Ticket{}, err
	}
	ticket, ok := tickets[template]
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
