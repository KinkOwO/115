package protocol

import (
	"encoding/binary"
	"fmt"
)

// 活动 662 的 NOTI108 活动清单（`ENUM_NOTIPACKET_EVENT_INFO`）。
//
// 只含本活动自己的行，不携带频道开放门（channel open events）：进城时的 108
// 由本树的固定 EVENT_INFO 表独占（cmd/wireprobe/event_info_generated.go +
// entry_flow.go），这份快照只在选角名单（CMD8）与回选角（CMD7）下发，
// 两者不重叠，也就不存在"第二条 108 抹掉已有门参数"的问题。
//
// 日期由调用方给出：662/665 用官服抓包同 id 记录里的窗口（boostup.EventStart/EventEnd，
// 对应两份 .evt 的 `[event period]`），见 cmd/wireprobe/event_info_variant_test.go 的
// 逐字节比对 —— 本编码器不再自行决定日期。
func BoostOpeningEvents115(start, end uint32, challenge ...bool) ([]byte, error) {
	if len(challenge) > 1 {
		return nil, fmt.Errorf("ambiguous challenge activity setting")
	}
	if end <= start {
		return nil, fmt.Errorf("invalid boost event dates")
	}
	ids := []uint16{10017, 10018, 662}
	if len(challenge) == 1 && challenge[0] {
		ids = append(ids, 665)
	}
	count := uint16(0)
	body := []byte{0, 0}
	text := func(s string) { body = add32(body, uint32(len(s))); body = append(body, []byte(s)...) }
	for _, id := range ids {
		body = add16(body, id)
		body = append(body, 1, 2, 4)
		text("Sky of a Thousand Seas Boost Up")
		text("")
		text("")
		body = add32(body, start)
		body = add32(body, end)
		calendar := ""
		if id == 10017 || id == 10018 {
			calendar = fmt.Sprintf("event gift window/0/%d", id)
		}
		text(calendar)
		text("")
		flag := byte(0)
		if id == 10017 {
			flag = 1
		}
		body = append(body, flag)
		count++
	}
	binary.LittleEndian.PutUint16(body, count)
	return append(body, 0), nil
}
