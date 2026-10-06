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

// maxBoostRosterMarkerBytes 是 NOTI2639（BOOST_UP_MODE_ALL_CHARAC_INFO）明文在本仓**自定**的
// 防御上限：编码按 `4 + 7×行数` 定长（protocol.BoostRosterRowBytes115），一行一个角色位次、
// 按 32 槽计 ⇒ `4 + 7×32 = 228` 字节。
//
// ⚠️ 本常量**不是**客户端实测上限。原先记的「9 字节」来自一次**误读**：
//
//	观察（2026-10-07 实机 client_trace）：1 行（5 字节行宽 ⇒ 明文 9 字节）可用；
//	2 行（明文 14 字节）→ `PACKET OVERFLOW : ENUM_NOTIPACKET_BOOST_UP_MODE_ALL_CHARAC_INFO`
//	→ CMD217 → 之后本地把所有外发包 IGNORE。
//
//	结论（当时的推理）：**太长**。实际是**太短** —— 该客户端的读取链 handler `sub_140C131D0`
//	按游标读 `u32 行数 + 每行 7 字节`（`{u8 轨道, u32 名单位次, u8 状态, u8 保留}`），
//	游标函数带 `n4 < 4 / n4 < 1` 的**剩余长度判断** ⇒ **正文短于 `4+7N` 就是越界读**。
//	14 字节只够 1 行 + 2 字节，第 2 行读不满 ⇒ 这才是 CMD217 的来源。
//
// 改用 7 字节行宽后实机复测：2 行 18 字节、3 行 25 字节、4 行 32 字节**全部被接受、CMD217 归零**
// （会话 `…_20261006_183702_434609_next37` / `…_190433_177502_next37`）。官服抓包同样印证：
// 2639 共 11 帧、**正文恒 16 字节** = `4 + 7×1 + 5 字节尾`。
//
// 完整证据链（官服帧 / IDA 读取链 / 实机前后对比）见
// `docs/protocol/noti2639-row-width-authoritative-20261007.md`。
//
// 因此这里保留常量只为**防畸形输入**（正常情况下任何 ≤32 位的名单都不会触发），
// 不再表达「客户端能吃多大」。真正的约束是**下界** `len >= 4 + 7×行数`。
const maxBoostRosterMarkerBytes = 4 + protocol.BoostRosterRowBytes115*32

// boostRosterFrame 把 NOTI2639 的 marker 包成出站包；超出本仓自定防御上限时按 emptyFallback 处理。
//
// emptyFallback 的语义（业主 2026-10-07 实机 bug：选角名单整份被拒）：
//   - **true** = 改发**空标记**（`{0,0,0,0}`，即编码器注释里"空账号"那个已实机验证的形态）。
//     **选角路径必须传 true**：`client_dispatch_character.go:624` 要求这次快照恰好 2 个包
//     （NOTI2639 标记 + CMD2 名单），少一个就报 `boost roster snapshot incomplete` 并把
//     **名单整份丢弃** —— 玩家重启后看不到任何角色（现场表现就像"存档丢了"，其实 7 个角色都在）。
//   - false = 超限就不发这一帧（培养胶囊/毕业路径没有"包数"契约，宁缺勿错）。
func boostRosterFrame(name string, marker []byte, emptyFallback bool) (outboundPacket, bool) {
	if len(marker) > maxBoostRosterMarkerBytes {
		rows := 0
		if len(marker) >= 4 {
			rows = (len(marker) - 4) / protocol.BoostRosterRowBytes115
		}
		if !emptyFallback {
			log.Printf("[boostup] 抑制 NOTI2639（%s）：%d 行 / 明文 %d 字节 > 自定防御上限 %d 字节"+
				"（畸形标记。该帧编码按 `4+7×行数` 定长、32 槽上限，正常名单到不了这一步；"+
				"客户端读取链与实测边界见 docs/protocol/noti2639-row-width-authoritative-20261007.md）",
				name, rows, len(marker), maxBoostRosterMarkerBytes)
			return outboundPacket{}, false
		}
		empty, e := protocol.BoostRoster115(nil)
		if e != nil {
			log.Printf("[boostup] NOTI2639（%s）连空标记都编不出来（%v）—— 这一帧不发", name, e)
			return outboundPacket{}, false
		}
		log.Printf("[boostup] NOTI2639（%s）降级为**空标记**：%d 行 / 明文 %d 字节 > 自定防御上限 %d 字节。"+
			"为什么不是直接不发：选角快照必须是 2 个包，少一个会被判 incomplete 而丢掉整份选角名单"+
			"（玩家重启后看不到角色）。代价：本次登录不显示培养标记。",
			name, rows, len(marker), maxBoostRosterMarkerBytes)
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
