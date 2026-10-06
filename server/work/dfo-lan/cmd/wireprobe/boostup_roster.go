package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
	"log"
	"time"
)

// maxVerifiedBoostRosterBytes 是 NOTI2639（BOOST_UP_MODE_ALL_CHARAC_INFO）**明文**在已实测
// 可用范围内的最大长度。
//
// 口径与证据（2026-10-07 实机，客户端 client_trace 原文）：
//
//	1 行（明文 9 字节）→ 客户端正常吃下；
//	2 行（明文 14 字节）→ 客户端 `PACKET OVERFLOW : ENUM_NOTIPACKET_BOOST_UP_MODE_ALL_CHARAC_INFO`
//	                     → 发 CMD217 OVERFLOW_INFO → 之后**本地把所有外发包 IGNORE**
//	                     （`EndPacket(...) : IGNORE (made after exit packet)`）。
//	                     现场表现：选角界面没有角色、点「检查角色名重复」毫无反应（看着像存档丢了）。
//
// 编码器自己也写明这一帧「未过 IDA 门禁」（见 protocol.BoostRoster115 注释）。所以这里以
// **已证实可用**的 9 字节为上限：超过就**不发这一帧**（宁少推一次进度，也不能让客户端自锁）。
// 等 IDA 把该帧真实布局与客户端缓冲上限查清后，改这个常量即可放开。
const maxVerifiedBoostRosterBytes = 9

// boostRosterFrame 把 NOTI2639 的 marker 包成出站包；超出已实测上限时按 emptyFallback 处理。
//
// emptyFallback 的语义（业主 2026-10-07 实机 bug：选角名单整份被拒）：
//   - **true** = 改发**空标记**（`{0,0,0,0}`，即编码器注释里"空账号"那个已实机验证的形态）。
//     **选角路径必须传 true**：`client_dispatch_character.go:624` 要求这次快照恰好 2 个包
//     （NOTI2639 标记 + CMD2 名单），少一个就报 `boost roster snapshot incomplete` 并把
//     **名单整份丢弃** —— 玩家重启后看不到任何角色（现场表现就像"存档丢了"，其实 7 个角色都在）。
//   - false = 超限就不发这一帧（培养胶囊/毕业路径没有"包数"契约，宁缺勿错）。
func boostRosterFrame(name string, marker []byte, emptyFallback bool) (outboundPacket, bool) {
	if len(marker) > maxVerifiedBoostRosterBytes {
		rows := 0
		if len(marker) >= 4 {
			rows = (len(marker) - 4) / 5
		}
		if !emptyFallback {
			log.Printf("[boostup] 抑制 NOTI2639（%s）：%d 行 / 明文 %d 字节 > 客户端已实测上限 %d 字节"+
				"（发出去会让客户端 PACKET OVERFLOW 并本地自锁，选角界面失效）；"+
				"这是编码布局未过 IDA 门禁期间的护栏，见 cmd/wireprobe/boostup_roster.go",
				name, rows, len(marker), maxVerifiedBoostRosterBytes)
			return outboundPacket{}, false
		}
		empty, e := protocol.BoostRoster115(nil)
		if e != nil {
			log.Printf("[boostup] NOTI2639（%s）连空标记都编不出来（%v）—— 这一帧不发", name, e)
			return outboundPacket{}, false
		}
		log.Printf("[boostup] NOTI2639（%s）降级为**空标记**：%d 行 / 明文 %d 字节 > 客户端已实测上限 %d 字节。"+
			"为什么不是直接不发：选角快照必须是 2 个包，少一个会被判 incomplete 而丢掉整份选角名单"+
			"（玩家重启后看不到角色）。代价：本次登录不显示培养进度标记。",
			name, rows, len(marker), maxVerifiedBoostRosterBytes)
		return outboundPacket{name, 0, 2639, empty}, true
	}
	return outboundPacket{name, 0, 2639, marker}, true
}

func selectionRosterPackets(ctx context.Context, s *character.Service, account int64, fatigue *character.FatigueService, activity *boostup.Catalog) ([]outboundPacket, error) {
	if s == nil || s.Store == nil || account <= 0 {
		return nil, fmt.Errorf("owned roster service required")
	}
	var packets []outboundPacket
	if activity != nil {
		roles, e := s.Store.Characters(ctx, account)
		if e != nil {
			return nil, e
		}
		for _, r := range roles {
			if r.AccountID != account {
				return nil, fmt.Errorf("foreign boost roster role")
			}
		}
		marker, e := boostRosterForRoles(roles, database.Character{})
		if e != nil {
			return nil, e
		}
		if pkt, ok := boostRosterFrame("boost_roster_restored", marker, true); ok {
			packets = append(packets, pkt)
		}
	}
	p, e := s.ListWithFatigue(ctx, account, fatigue, time.Now())
	if e != nil {
		return nil, e
	}
	return append(packets, outboundPacket{"character_roster_snapshot", 0, 2, p}), nil
}

func boostLoginRoster(ctx context.Context, s *database.Store, account int64, activity *boostup.Catalog) ([]outboundPacket, error) {
	if activity == nil {
		return nil, nil
	}
	if s == nil || account <= 0 {
		return nil, fmt.Errorf("boost login roster owner missing")
	}
	roles, e := s.Characters(ctx, account)
	if e != nil {
		return nil, e
	}
	marker, e := boostRosterForRoles(roles, database.Character{})
	if e != nil {
		return nil, e
	}
	if pkt, ok := boostRosterFrame("boost_roster_restored", marker, true); ok {
		return []outboundPacket{pkt}, nil
	}
	return nil, nil
}
