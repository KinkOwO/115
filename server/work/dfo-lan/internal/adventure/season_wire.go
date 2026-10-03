package adventure

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// SeasonLevelHistory 对应 NOTI2799。
//
// 0x1405674C0读取28字节记录；0x140458072/0x1404580D0消费次数/经验。
// +0、+20和末尾填充不参与当前reader的展示，保留零，不借用为其它业务字段。
//
// 归属：赛季报文由 adventure 拥有（曾经的 protocol.SeasonLevelHistory），
// protocol 不再反向依赖本领域。
func SeasonLevelHistory(s SeasonState) []byte {
	p := make([]byte, 12+28*len(s.History))
	binary.LittleEndian.PutUint32(p, uint32(len(s.History)))
	for i, h := range s.History {
		row := p[4+i*28:]
		binary.LittleEndian.PutUint32(row[4:], h.ID)
		binary.LittleEndian.PutUint32(row[8:], h.Category)
		binary.LittleEndian.PutUint32(row[12:], h.Count)
		binary.LittleEndian.PutUint32(row[16:], h.Experience)
		if h.CSOnly {
			row[24] = 1
		}
	}
	tail := p[4+28*len(s.History):]
	binary.LittleEndian.PutUint32(tail, s.Experience)
	binary.LittleEndian.PutUint32(tail[4:], s.RewardMask)
	return p
}

// SeasonOathHistory 对应 NOTI2858。
//
// 0x1402889B0读取数量、长度前缀角色名、模板和Unix时间。
// 即使从未兑换也要发送0数量，让原生经理把可兑换阶段初始化为100。
func SeasonOathHistory(rows []OathAcquisition) ([]byte, error) {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint32(p, uint32(len(rows)))
	for _, row := range rows {
		name, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(row.Name))
		if err != nil || len(name) >= 30 || row.Time < 0 || row.Time > 0x7fffd27f {
			return nil, fmt.Errorf("誓约获取记录名称或时间超出客户端范围")
		}
		prefix := make([]byte, 4)
		binary.LittleEndian.PutUint32(prefix, uint32(len(name)))
		p = append(p, prefix...)
		p = append(p, name...)
		tail := make([]byte, 8)
		binary.LittleEndian.PutUint32(tail, row.Template)
		binary.LittleEndian.PutUint32(tail[4:], uint32(row.Time))
		p = append(p, tail...)
	}
	return p, nil
}
