package protocol

import (
	"encoding/binary"
	"fmt"
)

// BorderRewardInfo 是「调律之边界奖励档位」那一帧的 12 字节载荷 ——
// noti 2756（原生名 FORTIFIED_DISCIPLE_OF_DOOM_REWARD）与 noti 2859
// （BOUNDARY_OF_ATTUNEMENT_REWARD）注册的是**同一个 handler** `0x1406b18d0`，
// 所以两条通知共享这一套载荷语义（见 testdata/border-reward-native.json）。
//
// 原生 handler 把三个 u32 依次写进调律共用模块（`sub_1406B1520` 是它的 init，
// 模块初值 `+0x50=72 / +0x54=72 / +0x58=-1`）的三个位移：
//
//	word0 -> +0x58（十进制 88）  ← 掉落演出的驱动格（实测：`40,43,43` 播最低那一格）
//	word1 -> +0x50（十进制 80） } getHellDungeonItemMaxRarity 取这两者的较大值
//	word2 -> +0x54（十进制 84） }
//
// ⚠️ 三格是**整帧**校验的。客户端四格掉落演出表是
// Unique / Legendary / Epic / Primeval（`sub_140657920` 按这个顺序存入模块），
// 索引 = 值 − 40 ⇒ 合法值域就是 40..43。任何一格落到外面，整帧退化成最后一格
// PrimevalDrop —— 这正是「大深渊每次都播太初动画」的成因。
//
// ⚠️ 所以**不能**把 word0 留在构造初值 -1（原生向量记的 `member58 = -1`）：它
// 不是「不覆盖」，而是越界 ⇒ 整帧兜底。旧版按「word0 语义未定、别乱填」的思路
// 写 -1，结果每次都被兜底成太初（业主 2026-10-08 两次实机复现：
// 不发 / `42,72,72` / `44,43,0` 三种情形都落到太初）。
//
// slot 是**演出表自己的值**（40..43），不是本仓那条 40..45 的档位阶梯 ——
// 换算见 loot.AnimationGrade。本包不认识档位阶梯，只认识这一帧的形状。
func BorderRewardInfo(slot uint32) ([]byte, error) {
	if slot < 40 || slot > 43 {
		return nil, fmt.Errorf("invalid Border drop animation slot %d", slot)
	}
	p := make([]byte, 12)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(p[i*4:], slot)
	}
	return p, nil
}
