package protocol

// CharacterBuffDungeon 是 NOTI475 ENUM_NOTIPACKET_CHARACTER_BUFF_DUNGEON 的载荷：
// 「角色 buff（副本）」那条记录。官服在**每次进图**时都发它（冷进场与无缝再次挑战都有）。
//
// ## 取证（两处独立官服抓包，字节完全相同）
//
//   - 小深渊 `official_20261008-012831`（analysis/tasks/next178 §14/§17）：3 次，8 字节
//   - 伊斯大陆 replay 夹具 `internal/legion/ispins_replay_frames.generated.go` 里的
//     `character_buff_dungeon`，同样 8 字节
//
// 两处都是 `0000a51f257d3f00`，目前只观测到这一个取值。**各字段语义尚未取证** ——
// 这里只钉住「官方进图会发、本仓此前一帧都不发」这一点，载荷按官服字节原样发，不构造。
//
// ⚠️ 本仓此前只在伊斯 replay 那条诊断路径里发过它（`cmd/wireprobe/ispins_flow.go` 的
// appendIspinsReplays），正常进图路径一帧都不发 —— 那正是「再次挑战时 buff 没有服务端依据、
// 客户端自己重推一遍」的缺口。
var characterBuffDungeonBody = []byte{0x00, 0x00, 0xa5, 0x1f, 0x25, 0x7d, 0x3f, 0x00}

// CharacterBuffDungeon 返回一帧新的载荷（调用方可以安全改写）。
func CharacterBuffDungeon() []byte {
	return append([]byte(nil), characterBuffDungeonBody...)
}

