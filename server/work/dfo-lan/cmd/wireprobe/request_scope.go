package main

// Bounded game-command evidence. Every implemented command must pass this
// gate before its handler, and its body is retained on every occurrence so
// regressions stay diagnosable.
func observedGameRequest(id uint16) bool {
	switch id {
	case 18, 21, 22, 26, 38, 40, 41, 63, 64, 102, 160, 173, 451, 507, 1417, 2177, 2261:
		return true
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 16, 19, 28, 29, 31, 32, 33, 34, 35, 36, 37, 39, 42, 43, 44, 45, 46, 69, 70, 71, 72, 117, 132, 143, 191, 433, 623, 627, 637, 684, 848, 1301, 1554:
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
// Command 2127 is the telemetry stream the comment above describes: it arrives
// several times a second for as long as a character is in a scene, and its cap
// is deliberately left at BodySampleLimit so it cannot flood the log. Measured
// over one run it sent 257 bodies in 33 seconds while command 35 - the only
// position report this client sends - arrived once per second.
func retainRequestBody(id uint16, seen map[uint16]int) bool {
	if observedGameRequest(id) || partyEvidenceRequest(id) {
		return true
	}
	if seen == nil || seen[id] >= BodySampleLimit {
		return false
	}
	seen[id]++
	return true
}
