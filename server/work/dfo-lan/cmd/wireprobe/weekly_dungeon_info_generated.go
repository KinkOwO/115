// Code generated from the official capture (session_s1, 2026-10-02 16:03,
// frame 78); DO NOT EDIT.
package main

// weeklyDungeonInfoTableHex is the verbatim NOTI706 WEEKLY_DUNGEON_INFO body
// (1464 bytes) the official server pushed in the channel-entry announce.
// The s4 announce (the connection the CMD2043 click was fired from) does not
// repeat it, but s1/s4 are sequential connections of one client process, so
// the table primed by s1 plausibly persisted into s4 - and it is the only
// frame in the capture literally named WEEKLY_DUNGEON_INFO. Field semantics
// unknown; replayed as-is (project rule: bytes come from official evidence).
//
// 0x2de 语义修正（2026-10-03，next79 作战次数调查第二轮）：本表生成自 s1
//（2026-10-02 16:03 登录连接，帧 78）——yan55 当周已完成 1 场并领奖、仍能
// 创建队伍进作战（s4 全程作战抓包为证），s1 表 0x2de=0x00。switch 抓包
//（2026-10-03，已完成 2 场）的 706 表与 s1 逐字节 diff 仅 0x2de 一处：
// s1=0x00 → switch=0x01。即 0x2de=1 与「本周完成 ≥2 场」关联，并非「奖励
// 可领」标志。早前一轮曾按「68 个 0x01 奖励槽」的猜测把它改成 0x01，方向
// 反了：把「完成 2 场」状态回放给从未作战的私服角色，实测（05:07 会话）
// 待机区创建队伍仍弹「本周奖励已全部领取」。现恢复 s1 原值 0x00，其余
// 字节保持 verbatim。
const weeklyDungeonInfoTableHex = "" +
	"ffffffff01010500000079616e35359cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff0000000000000000000000009c" +
	"ffffff9cffffff000000009cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9c" +
	"ffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff01" +
	"0101010101010101010101010101010101010101010101010101010101010101" +
	"0101010101010101010101010101010101010101010101010101010101010001" +
	"01010101010101010101010101010101019cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff000000009cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff010000009cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffffff9cffff" +
	"ff8c000004000300030000009600000001000000c9f4f5050000000000000000" +
	"000000000000000000000000000000000000000075e50000"

// weeklyDungeonInfoTable decodes the capture above once at startup.
var weeklyDungeonInfoTable = mustHexDecode(weeklyDungeonInfoTableHex)

func mustHexDecode(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := range b {
		var v byte
		c := s[2*i]
		if c >= '0' && c <= '9' {
			v = c - '0'
		} else if c >= 'a' && c <= 'f' {
			v = c - 'a' + 10
		} else {
			panic("bad hex byte")
		}
		v <<= 4
		c = s[2*i+1]
		if c >= '0' && c <= '9' {
			v |= c - '0'
		} else if c >= 'a' && c <= 'f' {
			v |= c - 'a' + 10
		} else {
			panic("bad hex byte")
		}
		b[i] = v
	}
	return b
}
