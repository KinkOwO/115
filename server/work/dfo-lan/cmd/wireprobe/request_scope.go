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

// Implemented commands and explicitly observed features are decoded on every
// request. Unknown commands retain the existing eight-body sampling limit.
func observedGameRequest(id uint16) bool {
	if mailboxRequest(id) || id >= 2316 && id <= 2328 {
		return true
	}
	switch id {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 18, 19, 20, 21, 22:
		return true
	case 26, 27, 28, 29, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 202:
		return true
	case 43, 44, 45, 46, 63, 64, 69, 70, 71, 72, 80, 102, 117, 132, 143, 160:
		return true
	case 173, 191, 201, 205, 206, 256, 272, 295, 305, 306, 307, 308, 393, 430, 433, 449, 450, 451, 467:
		return true
	case 469, 483, 495, 500, 502, 507, 527, 623, 627, 637, 649, 681, 684, 777, 795, 806, 848:
		return true
	case 857, 1301, 1395, 1406, 1417, 1418, 1421, 1422, 1426, 1438, 1461, 1462, 1551, 1554, 1565, 1592, 1719:
		return true
	case 1722, 1725, 1811, 1852, 1881, 1950, 1951, 1960, 2015, 2047, 2062, 2079, 2139, 2177, 2179, 2258, 2259, 2261:
		return true
	case 2288, 2289:
		// 2289 = 秘宝制作（SOLE_EQUIPMENT_CREATE）：已实现（同一个 sole_flow.go 的 raiseSoleCreate）。
		// 2288 = 秘宝精度提升（SOLE_EQUIPMENT_QUALITY）：已实现（见 cmd/wireprobe/sole_flow.go）。
		// 必须登记：否则第 BodySampleLimit(8) 次之后 verified 不再被计算，请求永远进不了处理器。
		return true
	case 2264, 2265, 2276, 2277, 2278, 2284, 2329, 2331, 2346, 2377, 2405, 2419:
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
