package legion

import (
	"encoding/binary"
	"fmt"
)

// 次元回廊：让**本会话的怪物表**与客户端实际持有的怪物用同一个编号。
//
// ★ 2026-10-10 业主实机「击败 BOSS 后不出横幅、不出翻牌界面」的根因就是这个编号错位。
//
// 实机证据（会话 `..._20261010_182920_601678_next37`，`client.log` 是 `exit=0x1`，
// 也就是客户端**一直停在 BOSS 房**，不是崩溃）：
//
//	client_trace.txt  [ETC] sendDieMonsterPacket start monster uniqueId :2957, idx :109015480
//	events.jsonl      {"id":39,"kind":"client_frame","plain_hex":"8d0b0000…"}       ← CMD39 实体 = 0x0b8d = 2957
//	events.jsonl      {"id":39,"kind":"dungeon_request_refused","reason":"monster absent from current source room"}
//	events.jsonl      {"id":117,"kind":"dungeon_request_refused","reason":"boss check target is not a source boss in this room"}
//
// 而 `internal/dungeon` 的会话怪物表是 `.dgn` 脚本自己编的号（`fixedMonsters` 从 4096 起，
// 三界的 BOSS 恰好都是 `entity=4096 template=109014935/109014930/109015480 rank=3 team=100`），于是：
//
//	CMD39 → ConfirmDeath 查不到 2957 → 返回错误
//	      → client_dispatch_world.go 里那条给次元回廊开的 `if e == nil` 兜底不触发
//	      → monsterDeath 中途返回，completeDungeon 那一支也走不到（`Completed()` 恒假）
//	      → 清关链（N31 / N2059 / N2252 / N2253 / N2316）**一帧都没发出去**
//	        ⇒ 业主看到的「BOSS 死了却没有横幅、没有翻牌界面、也没有下一界」
//	CMD117 → BossCheck 同样找不到 2957 → 被拒（第二条 refused 就是它）
//
// 客户端手上那只 BOSS 的编号来自**我们自己下发的 N29（START_MAP）**正文 —— 本族的怪物
// 完全由回放的官服帧列产生（`.dgn` 刷新出来的会话怪物在客户端并不存在，三界实测都是
// `entity=4096` 一只 rank3/team100 的领主）。所以正确做法是
// **把会话怪物的编号对齐到 N29 里的编号**，而不是另发明一张映射表：
//
//	官服 s30 三帧 N29 第 1 条记录的实体（s2c #786 / #860 / #942）= 0x09ff / 0x07e7 / 0x0b8d
//	官服 s30 三帧 CMD39 的实体  （c2s #489 / #520 / #558）= 0x09ff / 0x07e7 / 0x0b8d（同值）
//	官服 s30 三帧 N115 的身份   （s2c #827 / #900 / #984）= 0x09ff / 0x07e7 / 0x0b8d（同值）
//
// 三处同值即「N29 的实体就是客户端的怪物编号」，下面这套函数的依据就是这个闭环。

// N29（START_MAP）正文里怪物记录区的偏移与记录长度。
//
// 与 `protocol.StartMap` 的构造一一对应（internal/game/protocol/dungeon.go:231）：
//
//	@0  u8  position.x       @1  u8  position.y       @2  u8  层图标记
//	@3  u32 seed             @7  u8  hellPartyMode    @8  u8  0
//	@9  u32 ffffffff
//	@13 18B 换图记录（默认 00 00 00 00 ff×8 00 00 00 00 00 00）
//	@31 u8  1（初始化怪物）   @32 u32 地图号           @36 u8  怪物条数
//	@37 起每条 15B：`u16 spawnOrder | u32 sourceIndex | u16 entity | u32 template |
//	                  u8 level | u8 rank | u8 createTrigger | u8 hidden | u8 255 | u32 team | u8 0`
//	  即 entity 在记录内 +6、template +8、level +12、rank +13、team +17。
//	末尾 4B：`00 00 00 ff`
//
// 官方进图帧列里的 N29（88B 明文含 13B 信封 ⇒ 正文 75B）逐字节对得上：
//
//	#786 条数=1 spawnOrder=0 sourceIndex=0 entity=0x09ff template=109014935 level=0x8c rank=0x03 team=0x64
//	#860 条数=1 spawnOrder=0 sourceIndex=0 entity=0x07e7 template=109014930 level=0x8c rank=0x03 team=0x64
//	#942 条数=1 spawnOrder=0 sourceIndex=0 entity=0x0b8d template=109015480 level=0x8c rank=0x03 team=0x64
//
// 三界的模板号与 `.dgn` 直读出来的会话怪物**逐个相同**（109014935 / 109014930 / 109015480），
// 也就是「同一批怪」的直接证据。
const (
	// DimCloisterStartMapID 是 N29（START_MAP）的 opcode。
	DimCloisterStartMapID = uint16(29)

	n29MonsterCountAt = 36
	n29RecordFrom     = 37
	n29RecordSize     = 15
	n29EntityAt       = 6
	n29BodyMin        = n29RecordFrom + 4
	n29MonsterMax     = 255
)

// DimCloisterStartMapEntities 从**这一关要下发的进图帧列**里读出客户端会持有的怪物编号。
//
// 返回的顺序就是 N29 记录的顺序，也是客户端 `sendDieMonsterPacket` 认的顺序。
func DimCloisterStartMapEntities(vectors []DimCloisterVector) ([]uint16, error) {
	out := make([]uint16, 0, 4)
	seen := false
	for _, v := range vectors {
		if v.Kind != 0 || v.ID != DimCloisterStartMapID {
			continue
		}
		seen = true
		body := v.Body
		if len(body) < n29BodyMin {
			return nil, fmt.Errorf("次元回廊 N29 正文只有 %d 字节（至少 %d）", len(body), n29BodyMin)
		}
		count := int(body[n29MonsterCountAt])
		if count == 0 || count > n29MonsterMax {
			return nil, fmt.Errorf("次元回廊 N29 怪物条数 %d 越界", count)
		}
		if n29RecordFrom+count*n29RecordSize > len(body) {
			return nil, fmt.Errorf("次元回廊 N29 正文 %d 字节装不下 %d 条记录", len(body), count)
		}
		for i := 0; i < count; i++ {
			at := n29RecordFrom + i*n29RecordSize
			entity := binary.LittleEndian.Uint16(body[at+n29EntityAt:])
			if entity == 0 || entity == 65535 {
				return nil, fmt.Errorf("次元回廊 N29 第 %d 条怪物编号 %d 无效", i, entity)
			}
			out = append(out, entity)
		}
	}
	if !seen {
		return nil, fmt.Errorf("次元回廊进图帧列里没有 N29（START_MAP %d）", DimCloisterStartMapID)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("次元回廊 N29 里没有任何怪物记录")
	}
	return out, nil
}

// DimCloisterArenaRoster 是「本会话自己的怪物表」的最小接口。//
// 只要求读写编号这一列，是为了让对齐逻辑留在本包（数据真源一侧），
// 而 `cmd/wireprobe` 那一侧既能直接用 `*dungeon.Session` 调用，也能在测试里断言。
type DimCloisterArenaRoster interface {
	LegionMonsterEntities() []uint16
	SetLegionMonsterEntities([]uint16) error
}

// DimCloisterAlignArenaEntities 把会话怪物表的编号按**位置顺序**对齐到客户端持有的编号。
//
// 为什么按位置而不是按模板编号：客户端那只 BOSS 的编号来自 N29，服务端这条怪物来自 `.dgn`
// 的 [monster] 表 —— 两者是**同一个房间的同一批怪**（本族的怪物全部由官服帧列产生，
// 会话这份只用来记「死了没有、算不算通关」）。三界实测都是「一只 rank3/team100 的 BOSS」，
// 位置是两岸唯一可靠的对应关系；对应不上就**不做任何改动**并返回错误，让上层退化到旧行为，
// 而不是错配到别的怪身上。
//
// `fallbackBase` 是造新号时的起点（调用方传 `Session.NextEntity`）：官服号落在 0..4095，
// 本仓 `internal/dungeon` 的怪物号从 4096 起，两边天然不撞。
func DimCloisterAlignArenaEntities(roster DimCloisterArenaRoster, entities []uint16, fallbackBase uint16) ([]uint16, error) {
	if roster == nil {
		return nil, fmt.Errorf("次元回廊编号对齐缺少会话怪物表")
	}
	current := roster.LegionMonsterEntities()
	if len(current) == 0 {
		return nil, fmt.Errorf("次元回廊会话怪物表为空")
	}
	if len(entities) != len(current) {
		return nil, fmt.Errorf("次元回廊 N29 怪物 %d 只、会话怪物 %d 只，无法一一对应",
			len(entities), len(current))
	}
	applied := make([]uint16, len(current))
	taken := make(map[uint16]bool, len(current))
	for i, entity := range entities {
		if entity == 0 || entity == 65535 {
			return nil, fmt.Errorf("次元回廊第 %d 只怪物的官服编号 %d 无效", i, entity)
		}
		if taken[entity] {
			return nil, fmt.Errorf("次元回廊第 %d 只怪物的官服编号 %d 重复", i, entity)
		}
		taken[entity] = true
		applied[i] = entity
	}
	// 官服号优先；官服帧里没给编号的位置（多出来的怪物）才另造一个不撞的号。
	next := fallbackBase
	if next == 0 {
		next = 4096
	}
	for i := range applied {
		if applied[i] != 0 {
			continue
		}
		for next == 0 || next == 65535 || taken[next] {
			next++
			if next == 0 {
				return nil, fmt.Errorf("次元回廊没有可用的怪物编号")
			}
		}
		applied[i] = next
		taken[next] = true
	}
	if err := roster.SetLegionMonsterEntities(applied); err != nil {
		return nil, err
	}
	return applied, nil
}
