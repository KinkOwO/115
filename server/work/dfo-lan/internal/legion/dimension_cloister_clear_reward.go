package legion

import (
	"encoding/binary"
)

// 次元回廊清关翻牌的两帧奖励（N2252 / N2253）——**本仓自己构造**，不再回放官服原文。
//
// ★ 2026-10-10 业主实机第二轮：「击杀 BOSS 后直接卡死闪退」（`client.log` = `exit=0xC0000005`）。
//
// 上一轮把官服 s30 第 1 界的整段清关序列（#800..#827）**逐字节回放**，其中包括
// 官服 N2252（#811）与 N2253（#820）。这两帧是**官服那次会话的奖励数据**，不是零：
//
//	官服 #811 解压后 @1600 那条 44B 行 = `cb c0 9d 00 14 00 00 00 …` → 模板 10338891 × 20
//	官服 #820 解压后 4 条 40B 行          = 10338891×28 / 10338919×12 / 10338507×67 / 10338508×200
//	两帧的 @7760 是官服的阶段 token `41 3a`
//
// 而本仓的客户端 **不解 zlib**：`IspinsBasicClearReward` 的注释已经把这条钉死了 ——
// 本地 DFO.exe 的 1424FDC30 直接按 0x1e5c 字节读正文（1424FDB60 对 N2253 读 0x965），
// **压缩体会踩到读取器在 146EA0C30 的故意空写**。也就是说：官服那两帧在本客户端上
// 本来就不可能被正确解析，回放它们等于把一段压缩数据喂给一个不做解压的 reader。
//
// 所以这里照同族（伊斯/维纳斯/末世录）已经实机验证过的形状**自己构造**：
// 相同的固定长度、相同的行偏移、相同的 `[flag/item/count/03]` 记录语义、相同的阶段 token，
// 只有内部数据换成本内容自己的奖励。
//
// 奖励行取自**内容总表**（PVF 直读：`contents/2022/dimensioncloister/etc/myresdimensioncloister.etc`），
// 也就是官服那两帧里出现过的同一批模板（导出件见
// `runtime/pvf-dc-etc/00-myresdimensioncloister.etc.txt`）：

// DimCloisterClearRewardRow 是翻牌界面上的一行奖励。
type DimCloisterClearRewardRow struct {
	Template uint32
	Count    uint32
	// Flag 只有 N2253 的 40B 记录用得到：1 = 展示行。官服 4 条里只有第一条是 1。
	Flag byte
}

// DimCloisterBasicClearRewardRows 是 N2252（第一排）要展示的奖励行。
//
// 官服三界的 N2252（#811 / #885 / #968）**解压后逐字节只有这一行不同以外完全相同**
// （`@1600 + 44*i`，行内 `@0` 模板、`@4` 数量）：
//
//	#811 cb c0 9d 00 14 00 00 00 …
//	#885 cb c0 9d 00 14 00 00 00 …
//	#968 cb c0 9d 00 14 00 00 00 …
//
// ⚠️ 模板号一律写**十六进制字面量**，不要写十进制 —— 上一版就是手抄十进制时把
// 0x009DC0CB 抄成了 10338379（多算 3152），客户端去取一个**不存在的模板**，
// 收到这一帧的**瞬间**就崩（业主实机三轮：`exit=0xC0000005`）。
var DimCloisterBasicClearRewardRows = [...]DimCloisterClearRewardRow{
	{Template: 0x009DC0CB, Count: 20}, // 10338507
}

// DimCloisterAdditionalClearRewardRows 是 N2253（第二排）的奖励行。
//
// 官服 #820 / #893 解压后的 4 条（#974 同模板、const 位是 4）：
//
//	rec0 flag=1 模板 cb c0 9d 00 (=10338507) count=28
//	rec1 flag=0 模板 cc c0 9d 00 (=10338508) count=12
//	rec2 flag=0 模板 f3 94 9d 00 (=10327283) count=67
//	rec3 flag=0 模板 f1 94 9d 00 (=10327281) count=200
//
// 同样用十六进制写字面量。
var DimCloisterAdditionalClearRewardRows = [...]DimCloisterClearRewardRow{
	{Template: 0x009DC0CB, Count: 28, Flag: 1}, // 10338507
	{Template: 0x009DC0CC, Count: 12},          // 10338508
	{Template: 0x009D94F3, Count: 67},          // 10327283
	{Template: 0x009D94F1, Count: 200},         // 10327281
}

// DimCloisterBasicClearRewardRaw 构造 7772B 的 N2252（翻牌第一排）**未压缩正文**。
//
// 形状与 `IspinsBasicClearReward` / `VenusBasicClearReward` 同族：
// 全零缓冲 + `@1600 + 44*i` 的奖励行 + `@7760` 的 2B 阶段 token。
// token 取 `DimCloisterEnableClearDungeon` 正文头 2B（官服两帧都是 `41 3a`）——
// 客户端会拿 N31 头与 N2252 尾这一格做一致性校验，两处必须同源。
func DimCloisterBasicClearRewardRaw(stage int, elapsedMS uint64) ([]byte, error) {
	raw := make([]byte, dimCloisterBasicBodySize)
	for i, row := range DimCloisterBasicClearRewardRows {
		if row.Template == 0 || row.Count == 0 {
			continue
		}
		off := 1600 + 44*i
		if off+8 > len(raw) {
			return nil, errDimCloisterRewardRowOverflow
		}
		binary.LittleEndian.PutUint32(raw[off:], row.Template)
		binary.LittleEndian.PutUint32(raw[off+4:], row.Count)
	}
	// @7760 是**一个 u64**：低 2B 是阶段 token（客户端拿它与 N31 头比对），
	// 高 6B 是"通关耗时毫秒"（客户端转 signed32 后按秒显示，0 就是 00:00）。
	// 官服口径 token 与耗时同处这一个 u64，所以不能分两次覆盖。
	if elapsedMS > 0x7fffffff {
		elapsedMS = 0
	}
	token := DimCloisterClearToken(stage)
	elapsed := elapsedMS &^ 0xffff // 低 16 位留给 token
	binary.LittleEndian.PutUint64(raw[7760:], elapsed)
	copy(raw[7760:7762], token)
	return raw, nil
}

// DimCloisterAdditionalClearRewardRaw 构造 2405B 的 N2253（翻牌第二排）**未压缩正文**。
//
// 40B 记录步长，语义照同族：`@0 flag(1=展示)`、`@1 u32 模板`、`@5 u8 数量`、`@9 = 03`。
func DimCloisterAdditionalClearRewardRaw() ([]byte, error) {
	raw := make([]byte, dimCloisterAdditionalBodySize)
	for i, row := range DimCloisterAdditionalClearRewardRows {
		off := 40 * i
		if off+40 > len(raw) {
			return nil, errDimCloisterRewardRowOverflow
		}
		if row.Count > 255 {
			return nil, errDimCloisterRewardRowOverflow
		}
		raw[off] = row.Flag
		binary.LittleEndian.PutUint32(raw[off+1:], row.Template)
		raw[off+5] = byte(row.Count)
		raw[off+9] = 3
	}
	return raw, nil
}

// DimCloisterClearToken 返回第 stage 界用于「N31 头 ↔ N2252 尾」一致性校验的 2B token。
//
// 官服三界的 N31/N2252 尾都是 `41 3a`（见 `DimCloisterEnableClearDungeon` 的正文头），
// 三界同值，所以这里不按界分支；保留 stage 参数是为了让调用点与同族的
// `IspinsDungeonClearEnabled(stage)` 形状一致。
func DimCloisterClearToken(stage int) []byte {
	body := DimCloisterEnableClearDungeon()
	if len(body) < 2 {
		return []byte{0x41, 0x3a}
	}
	return append([]byte(nil), body[:2]...)
}

// DimCloisterClearFrameSkipped 报告官服清关段里的某一帧是否**不能回放**。
//
// 跳过两类（同族 Ispins 早已定下的口径，见 ispins_flow.go 的 §27 注释）：
//
//	op=2  USER_INFO          官服正文内嵌那位玩家的角色数据，必须由本仓按本角色重建；
//	op=14 ITEM_LIST(穿戴)    官服正文是那位玩家的物品行，回放会把别人的物品灌进本角色
//	                         的背包/穿戴视图（同族 v1 明确"不回放"）。
//
// 次元回廊的清关段里 N14 出现 6 次（帧 802/810/815/816/817/818），
// 全部属于第二类。
func DimCloisterClearFrameSkipped(kind byte, id uint16) bool {
	if kind != 0 {
		return false
	}
	return id == 2 || id == 14
}

// 这一族的奖励行数上限由布局定死（7772B 里 @1600 起 140 条 44B 行；
// 2405B 里 5 条 40B 行），越界属于实现错误而不是运行期输入错误。
var errDimCloisterRewardRowOverflow = errDimCloisterReward("次元回廊奖励行超出帧布局")

type errDimCloisterReward string

func (e errDimCloisterReward) Error() string { return string(e) }

// —— 线上形状：**裸正文**（不是 zlib，也不能短 4 字节）——
//
// ★★ 2026-10-10 业主抓包（`D:\zhuabao\captures\20261010-194540`，**能用的伊斯清关**）定案：
//
//	op=2252 s2c  wire len=7792   plain bytes=7776   plain 头 `00 00 00 00`
//	op=2253 s2c  wire len=2424   plain bytes=2408   plain 头 `01 bc b1 9d 00 1c …`
//
// 能用的那一族发的是 **7776B / 2408B 的裸正文**；而客户端 N2252 处理器
// `sub_1424FDC30` 要用 `sub_146EA0BE0(buf, 0x1E5C)` 从**收包缓冲**里取走 7772 字节
// （该函数在缓冲剩余不足时执行 `mov dword ptr ds:0, 0` ⇒ 0xC0000005）。
//
// 关键算术（抓包实测）：
//
//	wire len = plain + 16      （本仓内层帧头 4B + 外层 12B 框）
//	⇒ 客户端解析这一帧时"缓冲剩余" = wire_len − 16 = plain 长度
//	⇒ 要让 7772 字节的读取成功，**plain 必须 ≥ 7772**
//	  官服/伊斯给 7776（多 4 字节余量）；我们此前给 7772 恰好差 4 字节 ⇒ 必崩。
//
// 同时推翻两条早期结论：
//
//	① 官服 s30 抓包里那 64B `78 9c` 是**抓包侧就已经是 zlib**（那份抓包来自另一种
//	   客户端/工具链），本客户端**不解压** —— 它把 64 字节当正文读，必然读不到 7772。
//	② 因此"必须发压缩体"是错的；**本客户端要的是裸正文，压缩体在本客户端上从未成功**。
//
// N2253 同理：官方/伊斯 2408B，我们此前构造 2405B（少 3 字节）。

// dimCloisterBasicBodySize / dimCloisterAdditionalBodySize 是这两帧的**线上正文长度**。
//
// ★★ 定案依据不是抓包工具的输出，而是**同一台服务器上、同一个客户端跑通的那一次**
// （会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_194607_840200_next37`，
// 伊斯清关，`client.log` = `exit=0x0`）。那一场里服务端发的与客户端收到的对应关系是：
//
//	服务端 body=7772  →  客户端 `LEGION_BASIC_CLEAR_REWARD (Size : 7776)`   过关
//	服务端 body=2405  →  客户端 `LEGION_ADDITIONAL_CLEAR_REWARD (Size : 2408)` 过关
//
// 即 **客户端记的 Size = 正文 + 4**（下面那 4 字节是客户端自己在正文里读掉的字段，
// 见本文件开头对 `sub_146EA0BE0` / `sub_1424FDC30` 的说明）。
// 所以正文长度必须与那一场**逐字节一致**：多 4 字节就会把读取游标顶出界，
// 客户端于是走 `mov dword ptr ds:0,0`（0x146EA0C30）—— 实机表现为"击杀 BOSS 瞬间卡住、随后闪退"。
const (
	dimCloisterBasicBodySize      = 7772
	dimCloisterAdditionalBodySize = 2405
)

// DimCloisterBasicClearReward 返回 N2252 的**线上正文**（裸字节，7772B，与跑通的那一场同长）。
func DimCloisterBasicClearReward(stage int, elapsedMS uint64) ([]byte, error) {
	return DimCloisterBasicClearRewardRaw(stage, elapsedMS)
}

// DimCloisterAdditionalClearReward 返回 N2253 的**线上正文**（裸字节，2405B，与跑通的那一场同长）。
func DimCloisterAdditionalClearReward() ([]byte, error) {
	return DimCloisterAdditionalClearRewardRaw()
}

// DimCloisterClearRewardRows 返回本界翻牌要**发放并展示**的奖励行（N2252 第一排 + N2253 第二排）。
//
// 同族（venus/forest）的规矩是「显示与发放同源」：这一张表既喂 N2252/N2253 的展示行，
// 也用来在服务端真实入库（`inventory.Awarder`）。次元回廊此前只展示不发放，是漏项。
func DimCloisterClearRewardRows() []DimCloisterClearRewardRow {
	out := make([]DimCloisterClearRewardRow, 0,
		len(DimCloisterBasicClearRewardRows)+len(DimCloisterAdditionalClearRewardRows))
	out = append(out, DimCloisterBasicClearRewardRows[:]...)
	out = append(out, DimCloisterAdditionalClearRewardRows[:]...)
	return out
}

// DimCloisterOperationRewardBody 返回 N2316（MYRES_DIMENSION_CLOISTER_OPERATION_REWARD）正文。
//
// 官服 s30 #821 是 808B，实测**基本全零**（翻牌奖励的真数据在上面两帧里，
// 这一帧只是"本界奖励已结算"的权威标记）。逐字节照抄官服那一帧的形状（808B 全零，
// 但 @0 是 `01`——官服原文首字节就是 01）。
func DimCloisterOperationRewardBody() []byte {
	body := make([]byte, 808)
	body[0] = 0x01
	return body
}

// DimCloisterMinimalRewardFrame 返回 N2252/N2253 的**最小正文**（16B 全零）。
//
// 为什么需要它（2026-10-10 第十一轮定案）：
//
//	实机崩溃点在长度对齐之后**换了位置**：
//	  前十轮 `0x1424FDD06` ← 空写守卫（"读 7772 字节时缓冲不够"）
//	  本轮   `0x1424FDD35` ← `call qword ptr [r8+0F8h]` 的返回点
//
// 也就是读取已经成功，崩在处理器读完表之后去取本内容的**翻牌/奖励窗口**
// （`qword_14E683C40` → 虚表 `+0xF8`）。本客户端 build 里这个窗口没有可用条目。
//
// 发一个 16B 的最小帧：客户端仍收到该 opcode（不会因为包体过短被分发器丢掉），
// 但处理器读到的表是空的 ⇒ 不去开那个窗口 ⇒ 不崩。
// 代价是翻牌界面不会出现；服务端侧的进度（N2314/N2316）照常推，让流程能继续。
func DimCloisterMinimalRewardFrame() []byte { return make([]byte, 16) }

// NotiDimCloisterStreamPad / DimCloisterStreamPadFrame 是第五轮"补缓冲"方案的遗留物，
// **已不再使用**（第五轮定稿改成照同族"自己构造整条链"，不再回放官服清关段，
// 因此也不需要把被跳过的 N14/N2 的字节量垫回来）。保留导出符号是为了不动既有测试，
// 下一轮清理时一并删除。
//
// 背景（IDA 取证，仍然成立，供后人理解为什么"逐帧回放官服清关段"这条路走不通）：
//
//	客户端 N2252 handler `sub_1424FDC30` **不收包体**，而是从共享收包游标里
//	复制 `0x1E5C`(7772) 字节到自己的栈缓冲；游标不够就 `mov dword ptr ds:0,0`。
const NotiDimCloisterStreamPad = uint16(2168)

// DimCloisterStreamPadFrame 见上（已不再使用）。
func DimCloisterStreamPadFrame() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x54, 0x2b, 0x43, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xcb, 0x5c,
		0x9a, 0xf4, 0x39, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
}
