package main

// eventInfoTableHex is the 54-byte NOTI108 EVENT_INFO body the
// private-server client accepts: a RAW (uncompressed) table with record
// count 1 and the single "Ispins Legion Open" record (id 776, window
// 2023-02-10..2034-01-04) sliced verbatim from the official capture's
// 6829-byte schedule table (session s1 frame 58 / s4 frame 74,
// 2026-10-02 16:03; source slices in analysis-tools/output/
// gen_108_filtered.py).
//
// Body shape established by the round-11 differential probe experiment
// (next79 §15-§16): the private client REJECTS zlib-compressed bodies
// (probe V2/V5 -> CMD217 ENUM_CMDPACKET_OVERFLOW_INFO, freeze) and rejects
// the 7-record raw table (probe V4 -> same), but accepts this exact body
// (probe V3) AND the gate check then passes - the legion tab opens Ispins
// instead of popping the schedule-locked message. The official client takes
// the zlib stream; this build does not. DO NOT "fix" this back to zlib.
const eventInfoTableHex = "" +
	"0100080301000012000000497370696e73204c6567696f6e204f70656e" +
	"0000000000000000c041f4635f6b6178000000000000000000"

// eventInfoTable decodes the capture above once at startup.
var eventInfoTable = mustHexDecode(eventInfoTableHex)
