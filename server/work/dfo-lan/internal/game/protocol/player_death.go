package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD40 sender 145d29bbc writes x, y and a mode-specific actor word.
// Ordinary solo reports zero for the latter (145d29dd8). Slot1 pads to16.
func DecodePlayerDeath(p []byte) ([2]uint16, error) {
	var xy [2]uint16
	if len(p) != 16 {
		return xy, fmt.Errorf("player death requires 16-byte solo record")
	}
	for _, b := range p[4:] {
		if b != 0 {
			return xy, fmt.Errorf("unsupported player death mode or padding")
		}
	}
	xy[0], xy[1] = binary.LittleEndian.Uint16(p), binary.LittleEndian.Uint16(p[2:])
	return xy, nil
}

// NOTI32 at1452aa080 reads u16 actor, u8 state, u8 flag, u16 hp, u16 mp.
// State0 marks the actor dead and enters the native resurrection UI path.
// Sending only an ACK40 never runs that path. No currency is changed here.
func PlayerDeathState(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid death actor")
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, actor)
	return p, nil
}

// deathStateReviveHPMP 是「解除死亡态」那一帧里 hp/mp 的满值。
//
// 官服抓包（E:/迅雷下载/20261003-214424/decoded/F16-s2c.txt）里 NOTI32 的体是
// `9a100001e803e803527fd63744000000`（seq563）与 `9a100701e803e803bb1af83545000000`（seq564）：
// actor u16、state u8、flag u8、**hp = mp = 0x03e8 = 1000**、再跟 5B nonce。
// 客户端把 1000 渲染成满格，0 渲染成 0% —— 私服此前只改 state、把 hp/mp 留 0，
// 玩家回到城镇就是 0% 的虚弱态（实机 2026-10-04）。
const deathStateReviveHPMP uint16 = 1000

// PlayerReviveState 是「解除死亡幽灵态」那一帧（NOTI32，state=1）。
//
// 与 PlayerDeathState 的唯一差别是 state=1 且 hp/mp 填满值 —— 少了 hp/mp，
// 角色会以 0% 血蓝回到城镇，客户端只好去弹收费的 Stamina Recovery 服务。
func PlayerReviveState(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid death actor")
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, actor)
	p[2] = 1
	binary.LittleEndian.PutUint16(p[4:], deathStateReviveHPMP)
	binary.LittleEndian.PutUint16(p[6:], deathStateReviveHPMP)
	return p, nil
}

// NOTI 33 (0x0021, ENUM_NOTIPACKET_FAIL_CLEAR_DUNGEON), native reader 1452af830.
// Payload is a single u8 reason byte (0 = default defeat/death, 100 = timeout).
// Sets dungeon state to 3 (DUNGEON_STATE_FAIL_CLEAR), plays failure BGM (146cce880),
// and triggers the native player death scene and failure settlement.
func DungeonFailClear(reason byte) []byte {
	return []byte{reason}
}

// AzureMainFinishFighting 是 NOTI249（ENUM_NOTIPACKET_FINISH_VILLAGE_MONSTER_FIGHTING）
// 的蔚蓝号收尾形态。
//
// 官服尾段在 CMD72 的应答之后补了它（F16-s2c.txt #698，16B
// `0000000000aeb8a08638000000000000`：中间是一段未解语义的 5B 值，其余补零）。
// 那段 5B 属于本仓在 N29 尾部 / N38 尾部已经验证「可省略」的同一类字段，所以这里留 0。
// 私服全仓此前没有任何 opcode 249 —— 少了它客户端不认「本内容结束」这段收尾。
func AzureMainFinishFighting() []byte {
	return make([]byte, 16)
}
