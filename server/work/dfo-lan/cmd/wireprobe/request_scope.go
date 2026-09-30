package main

import "dfolan/internal/legion"

// Bounded game-command evidence. Every implemented command must pass this
// gate before its handler, and its body is retained on every occurrence so
// regressions stay diagnosable.
func dungeonRequest(id uint16) bool {
	switch id {
	// 2329 = ENUM_CMDPACKET_MONSTER_HISTORY_LOG：定盘机关每秒上报一次自己的
	// 血量与模板号，scale_death.go 用它判断「玩家已经把它打到血底」。
	case 1722, 16, 37, 38, 39, 40, 42, 43, 45, 46, 69, 70, 71, 72, 117, 132, 449, 450, 1461, 1852, 2015, 2062, 2319, 2320, 2321, 2322, 2323, 2325, 2327, 2329:
		return true
	}
	return false
}

func observedGameRequest(id uint16) bool {
	// Shield deck uploads must never fall through the eight-body sample cap.
	if id == 649 {
		return true
	}
	// 每次选择背景均须解密和保存，不能在第九次点击时落入采样限制。
	if id == 1725 {
		return true
	}
	// 图鉴登记需逐次解密并保留证据，不能在重复登记后落入采样上限。
	if id == 2139 {
		return true
	}
	// 黑鸦每次出发均需解密，不能在第九次点击后失去响应。
	if id == 1852 {
		return true
	}
	// 赤红铁矿仍在按当前客户端取证，逐次保留创建、编队、入场与奖励请求；
	// 这里只取消八次采样限制，不为尚未实现的玩法伪造成功应答。
	if id == 1461 || id == 1462 || id >= 2316 && id <= 2328 {
		return true
	}
	// 原生服务器时钟同步必须逐次校验，不能在八次采样后停止响应。
	if id == 1960 {
		return true
	}
	// 冒险团打开每次发送类型 2、0；重复打开不能受未实现指令八次采样限制。
	if id == 1395 || id == 1406 || id == 2331 || id == 1719 || id == 1811 || id == 2419 || id == 2405 {
		return true
	}
	// 沉月湖（Moon Lake）单人流程的入口命令，必须逐次解密校验：
	//   2284 = NPC「开始攻坚」；2276 = fever 触发；2277 = 特殊怪逃逸报告；
	//   2062 = 守门怪死亡后的洞口换层；1426 与 71 共用「选牌」处理器。
	// 落在 BodySampleLimit(8) 采样门里会让第 9 次起 verified 不再被计算，
	// 玩家表现为「点开始没反应」，与「这条命令根本没实现」一模一样。
	if id == 2284 || id == 2276 || id == 2277 || id == 2062 || id == 1426 {
		return true
	}
	// 强化券的重复使用也必须逐次解密，不受未实现指令的八次采样上限影响。
	if id == 80 {
		return true
	}
	// 205 = 增幅书（打红字）、272 = 附魔宝珠、430 = 锻造：都是**已实现**的命令，必须每次请求都解密校验。
	// 漏登记 205 的后果是：玩家用满 BodySampleLimit(8) 次增幅书之后，服务端不再解密该命令，
	// 请求直接以「明文为空」失败 —— 表现就是增幅书前几次能用、之后毫无反应。
	if id == 205 || id == 272 || id == 430 {
		return true
	}
	// 1722 = 装备继承：已有处理器（见 inherit_flow.go），同样必须每次请求都解密校验。
	// 漏登记的后果与 205 完全相同 —— 玩家用满 BodySampleLimit(8) 次继承之后，服务端
	// 不再解密该命令，请求直接以「明文为空」失败，表现就是「前几次能继承、之后毫无反应」。
	if id == 1722 {
		return true
	}
	// 857 = ENUM_CMDPACKET_OPEN_AURA_SKIN_SLOT（幻化栏窗口点 OK 开启光环/宠物幻化栏）。
	// 包体只有 4 字节且每次都要留证（要核对客户端报的是哪一种窗口）；一旦落进八次采样
	// 门，第 9 次起 verified 不再被计算，表现与「这条命令根本没实现」完全一样。
	if id == 857 {
		return true
	}
	// 开罐和晶体契约选择已有处理器，每次请求都必须解密校验，不能受八次采样限制。
	if id == 681 || id == 527 {
		return true
	}
	// 装备库（装备图鉴）两条命令已经实现：2264 = 收藏（每次都要解密校验，
	// 否则第 9 次起 verified 不再被计算，玩家表现为"点了没反应"，与"没实现"同形）；
	// 2265 = 收录/创建（写侧未实现，但仍要留证明文以定它的请求布局）。
	if id == 2264 || id == 2265 {
		return true
	}
	// 2259 = 装备库「制作 / 变换」（阶段一已接线：解码 + 回 6 字节应答，见 next125 §5）。
	// 与 2264 同理：**已实现**的命令若落进八次采样门，第 9 次起 verified 不再被计算，
	// 分派条件里的 `verified` 恒为 false ⇒ 请求再也不会进处理器，玩家表现与"没实现"一模一样。
	if id == 2259 {
		return true
	}
	// 送礼(806 p[0]=0)和剧情角色染色(806 p[0]=1)共用 CMD806，已有处理器。
	// 不在白名单时只解密前 8 帧，第 9 次起 verified=false 直接不进处理器，
	// 客户端表现为"点击送礼/染色没有任何反应"。每个 806 都必须解密分发。
	if id == 806 {
		return true
	}
	// 2329 = ENUM_CMDPACKET_MONSTER_HISTORY_LOG：定盘机关的每次上报，服务端判死兜底靠它
	// (见 scale_death.go)。它不在这个集合里时会被 BodySampleLimit(8) 截断，此后每条都因为
	// `verified` 从未被计算而被 dungeonRequest 拒掉，而拒绝理由是 `checksum failed` —— 那是
	// 误导：帧本身完全正常，只是我们没解密校验过它。实测 2026-09-27 一场 20 条里 12 条这样丢掉，
	// 恰好让「峰值判据」失效。故它必须每次请求都解密校验、不受采样限制。
	if id == 2329 {
		return true
	}
	// 469 / 495 = 树 1 / 树 2 的「通知已查看」上报。两者都有持久化处理器
	// （MarkCharacterNotice，见 main.go 的 `case 469` / `case 495`），但不在这个集合里时会被
	// BodySampleLimit(8) 截断：第 9 次起 `verified` 不再被计算，请求随即以 `checksum failed` 被
	// dungeonRequest 拒掉 —— 那是误报，帧本身完全正常，只是我们没解密校验过它。
	//
	// 后果：客户端上报的「这条提示我已经看过」丢不掉，于是「点击图标查看可获得奖励」的首次
	// 提示每次进图都弹。只加进白名单，不改 BodySampleLimit、不改任何 handler 逻辑。
	if id == 469 || id == 495 {
		return true
	}
	// 1565 是皮肤仓库「应用」按钮的请求，已有处理器：只解密前八次会让第八次之后的
	// 点击全部分流不进去，实机表现为「第一次能应用，之后换不动字体」。
	if id == 1565 {
		return true
	}
	// 武器幻化复制（CMD1592）已有处理器：包体只有八字节，只解密前八次会让第八次
	// 之后的确认全部分流不进去；每次都要留证以便比对窗口索引到底指向哪个槽位。
	// 1565 已经是共用帧（subtype 区分武器页签与字体页签），无需另加。
	if id == 1592 {
		return true
	}
	// 表情快捷键（CMD1551）已有回包：它只有八字节体，且玩家会连着按。留在采样门里就是
	// 第八次之后 verified 不再被算、回包整个停发，实机表现与「没做这条」一模一样（同 1565
	// 那条坑，见 CHANGELOG 2026-09-27 第二轮）。
	if id == 1551 {
		return true
	}
	if id == 305 || id == 306 || id == 307 || id == 308 {
		return true
	}
	if mailboxRequest(id) {
		return true
	}
	switch id {
	case 18, 21, 22, 26, 27, 38, 40, 41, 63, 64, 102, 160, 173, 393, 449, 450, 451, 467, 483, 507, 777, 1417, 1422, 1438, 1881, 1950, 1951, 2015, 2079, 2177, 2261, 2278, 2346, 2377:
		return true
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 19, 20, 28, 29, 31, 32, 33, 34, 35, 36, 37, 39, 42, 43, 44, 45, 46, 69, 70, 71, 72, 117, 132, 143, 191, 295, 433, 623, 627, 637, 684, 848, 1301, 1418, 1554, 2179:
		return true
	}
	return false
}

// partyEvidenceRequest lists the commands this client sends while a party is
// being formed. None of them is implemented, so they fall under the sampling
// cap below and would only ever yield eight bodies - not enough to rebuild a
// block layout from, and the party layout must come from this build own wire
// format rather than being ported. Every occurrence is kept while the party
// work is in progress.
func partyEvidenceRequest(id uint16) bool {
	switch id {
	case 12, 13, 14, 105, 166, 328, 335, 379, 426, 427, 435, 436, 437, 634, 683, 697, 698, 1523:
		return true
	}
	return false
}

// BodySampleLimit is how many bodies are retained per not-yet-implemented
// command, per connection.
//
// Features that are not implemented are exactly the ones with no recovered
// packet layout, and until 36 their bodies were discarded by the whitelist
// above: a capture could show that the client sent command 407 fifty times
// and nothing about what it contained. Sampling a few of every command turns
// the next ordinary play session into the evidence needed to implement mail,
// avatars, the shop and item use from this client's own wire format instead
// of porting another build's opcodes.
//
// The cap is what keeps that affordable: one run has sent a single telemetry
// command over six thousand times, so retaining every body is not an option.
const BodySampleLimit = 8

// retainRequestBody reports whether this occurrence's plaintext should be
// kept. Implemented commands are always retained; everything else is sampled
// up to the cap and then counted as metadata only.
//
// The legion family is retained wholesale rather than sampled: P1 implements
// only CMD2043, but CMD2044/2045/2046/2354/2355 are the evidence source for
// P2-P5, and every body in the family is short (17-22 bytes) so keeping each
// occurrence costs nothing.
//
// Command 2127 is the telemetry stream the comment above describes: it arrives
// several times a second for as long as a character is in a scene, and its cap
// is deliberately left at BodySampleLimit so it cannot flood the log. Measured
// over one run it sent 257 bodies in 33 seconds while command 35 - the only
// position report this client sends - arrived once per second.
func retainRequestBody(id uint16, seen map[uint16]int) bool {
	if observedGameRequest(id) || partyEvidenceRequest(id) || legion.Requests(id) {
		return true
	}
	if seen == nil || seen[id] >= BodySampleLimit {
		return false
	}
	seen[id]++
	return true
}
