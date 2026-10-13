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
// ⚠️ **word1/word2 不是演出格，而是「本场最高奖励品质」，走的是 40..45 那条档位阶梯**
// （normal..primeval）。依据有两处，都在我们自己的源码里：
//
//   - 原生 handler 的位移与 `getHellDungeonItemMaxRarity` 取两者的**较大值**（上面那张表）；
//   - 本仓早期版本（桌面快照 `yuanma`，2026-09 那条线）就是这么发的，而且把客户端的走法
//     写在注释里：「44 史诗：**球结束即掉落**；45 太初：**诺克斯之后再回到球阶段**」
//     —— 副本里那颗**球**的阶段分支读的就是这个数。
//
// 2026-10-08 为了修「三格越界 ⇒ 整帧兜底成太初」，把**三格统一填成演出格**（40..43）。
// 那一步修对了 word0，却把 word1/word2 从「本场档位」压成了「演出格」：
// `getHellDungeonItemMaxRarity` 于是永远最多读到 43 = legendary，**45 从来没有上过线**
// （实机 2026-10-12：业主在 大深渊 拿到太初装备，球仍是史诗那一档）。
// 现在两个值分开：word0 填演出格、word1 填本场档位、word2 保持早期版本的常量下界 40。
//
// ⚠️ 三格仍然**各自都要落在自己的值域里**。客户端四格掉落演出表是
// Unique / Legendary / Epic / Primeval（`sub_140657920` 按这个顺序存入模块），
// 索引 = 值 − 40 ⇒ 演出格合法值域就是 40..43。任何一格落到外面，整帧退化成最后一格
// PrimevalDrop —— 这正是「大深渊每次都播太初动画」的成因。
//
// ⚠️ 所以**不能**把 word0 留在构造初值 -1（原生向量记的 `member58 = -1`）：它
// 不是「不覆盖」，而是越界 ⇒ 整帧兜底。旧版按「word0 语义未定、别乱填」的思路
// 写 -1，结果每次都被兜底成太初（业主 2026-10-08 两次实机复现：
// 不发 / `42,72,72` / `44,43,0` 三种情形都落到太初）。
//
// `slot` 是**演出表自己的值**（40..43），不是本仓那条 40..45 的档位阶梯 ——
// 换算见 loot.AnimationGrade；`maxRarity` 才是档位阶梯上的值。
// 本包不认识档位阶梯的语义，只认识这一帧的形状与两段的值域。
func BorderRewardInfo(slot, maxRarity uint32) ([]byte, error) {
	if slot < 40 || slot > 43 {
		return nil, fmt.Errorf("invalid Border drop animation slot %d", slot)
	}
	if maxRarity < 40 || maxRarity > 45 {
		return nil, fmt.Errorf("invalid Border reward grade %d", maxRarity)
	}
	p := make([]byte, 12)
	binary.LittleEndian.PutUint32(p[0:4], slot)
	binary.LittleEndian.PutUint32(p[4:8], maxRarity)
	binary.LittleEndian.PutUint32(p[8:12], 40)
	return p, nil
}

// RewardBoostTingInfo 是 noti 2755（REWARD_BOOST_DUNGEON_TING_INFO）的 4 字节载荷。
//
// 客户端 handler `sub_145310880` 用 `sub_146EA0BE0(&v4, 4)` **定长读 4 字节**，交给
// `RewardBoostDungeonManager::onRecvRewardBoostDungeonTingInfo`（`sub_142180BD0`）。
// 那里对这个数做两件事，两件的形状都指向**副本号**：
//
//	① `sub_14520CFB0(<数>)` 取副本管理器（同一个函数在 `sub_14661E5E0` 里按
//	   100002511 / 100002512 / 500000048 这些副本号分支构造），再比它的 `+2740`
//	   是否落在 **204 / 225** —— 正好是调律之边界与灾厄深渊两条线共用的那一个模块
//	   （`sub_1406B1520` 里硬编码 `+228=100004768`、`+232=100005068`）。
//	② `sub_1406B1A40` 拿它去当前副本的 5 槽列表（`manager+640+3332`，即 dgn 的
//	   `[difficulty another dungeon] 100005066 100005067 100005068 0 0`）里**定位次**，
//	   位次写进共用模块 `+92` 并刷新 UI 窗口 3842（`sub_1406B2980`）。
//
// ⚠️ 「它是不是那条事件横幅」还没闭环：仓内唯一可用的官服样本（cap43）里没有 2755 帧，
// 而 4 字节也装不下截图里那句文案。载荷取值按上面 ① ② 定为副本号，语义待实机对照。
func RewardBoostTingInfo(dungeon uint32) []byte {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint32(p, dungeon)
	return p
}
