// Code generated from the official capture (event_info_official.plain, 2026-10-02
// session; slices via reference/analysis-tools/gen_event_info_gate_table.py); DO NOT
// EDIT by hand - regenerate with that script.
package main

// eventInfoTableHex is the RAW (uncompressed) NOTI108 EVENT_INFO body the
// private-server client accepts, now carrying ALL legion/raid gate records:
// count(2) + 19 official gate records + 0x00 empty tail. Every record is
// sliced verbatim from the official capture with the v2.38.2.34 layout
// (u16 id, u8 flag1, u8 f1, u8 f2, str x3, u32 start, u32 end, str x2, u8
// flag2; the layout closes exactly against all 77 official records via
// reference/analysis-tools/walk_event_info.py).
//
// Gate-record set (2026-10-04, jun tuan / gong jian zhan tab "cannot enter"
// live investigation): the tab entries are unlocked by the event-table bits
// (IDA: bit-1007 consumers own the apocalypse tab lock; probe V3 proved
// record 776 opens the Ispins tab). Official area keys alone did NOT unlock
// the other raids (2026-10-04 live test) - only Ispins (776, the previous
// single record) opened. These 19 fixed-shape records carry no URL/banner
// payload, so the full-table select-screen crash (official banner records
// reference 3.25-era resources; see internal/legion/event_info_min.go)
// cannot trigger. Windows keep the official start/end values (every end is
// 2033..2034, so now is in-window).
//
// WIRE FORMAT IS RAW ONLY: probe V2/V5 showed zlib bodies are rejected
// (CMD217 ENUM_CMDPACKET_OVERFLOW_INFO, freeze); probe V3 accepted this raw
// framing. An earlier 7-record raw probe (V4) also froze, but its
// construction is lost; this body is closed-form against the verified
// layout and keeps the empty tail. If the client ever freezes at
// select-screen again, roll back to the previous binary
// (wireprobe-pvf.exe.previous-20261004-0940-8006be60) or trim this table
// back to the single 0x0308 record.
//
// Records, in official-table order (id: name):
//
//	0x0991 FiendWarEnterDungeonEvent (45)          0x019E PreyRaidEnterDungeonEvent (50)
//	0x01CE DefaultEvent(ENTER_SIROCO_RAID) (67)    0x01E2 DefaultEvent(ENTER_OZMA_RAID) (76)
//	0x0308 Ispins Legion Open (81/86/87)           0x0280 DefaultEvent(ENTER_PRE_BAKAL_RAID) (83)
//	0x0281 DefaultEvent(ENTER_BAKAL_RAID) (82)     0x026D (84, official record name empty)
//	0x0282 Dusky Island Open (91)                  0x02A8 Forest of Awakening Normal (96)
//	0x02A9 Forest of Awakening Extreme (96)        0x024E ENTER_PRE_ASRAHAN_RAID (92)
//	0x0272 DefaultEvent(ENTER_ASRAHAN_RAID) (93)   0x0273 Venus Open (99)
//	0x0274 Semi Raid Bidding Open (85/9)           0x03D1 ENTER_ARTIFICIAL_GOD_RAID (98/107)
//	0x032C DefaultEvent(ENTER_INAE_DUSK_WAR) (111) 0x0383 DefaultEvent(ENTER_DELEZIE_RAID) (112/120)
//	0x03EF Apocalypse Channel (119/30/31/32)
const eventInfoTableHex = "" +
	"13009109010000190000004669656e64576172456e74657244756e67656f6e4576656e740000" +
	"00000000000010792a6a0e2561780000000000000000009e0101000019000000507265795261" +
	"6964456e74657244756e67656f6e4576656e74000000000000000010792a6a0e256178000000" +
	"000000000000ce010100001f00000044656661756c744576656e7428454e5445525f5349524f" +
	"434f5f5241494429000000000000000010792a6a0e256178000000000000000000e201010000" +
	"1d00000044656661756c744576656e7428454e5445525f4f5a4d415f52414944290000000000" +
	"00000010792a6a0e256178000000000000000000080301000012000000497370696e73204c65" +
	"67696f6e204f70656e0000000000000000c041f4635f6b617800000000000000000080020100" +
	"002200000044656661756c744576656e7428454e5445525f5052455f42414b414c5f52414944" +
	"29000000000000000010792a6a0e25617800000000000000000081020100001e000000446566" +
	"61756c744576656e7428454e5445525f42414b414c5f5241494429000000000000000010792a" +
	"6a0e2561780000000000000000006d02010000000000000000000000000000803c75640f2561" +
	"780000000000000000008202010000110000004475736b792049736c616e64204f70656e0000" +
	"00000000000080d5f8650f256178000000000000000000a8020100001a000000466f72657374" +
	"206f66204177616b656e696e67204e6f726d616c000000000000000010584e670f2561780000" +
	"00000000000000a9020100001b000000466f72657374206f66204177616b656e696e67204578" +
	"7472656d65000000000000000010584e670f2561780000000000000000004e020100002c0000" +
	"004576656e745072654173726168616e456e74657228454e5445525f5052455f415352414841" +
	"4e5f5241494429000000000000000010792a6a0e256178000000000000000000720201000020" +
	"00000044656661756c744576656e7428454e5445525f4153524148414e5f5241494429000000" +
	"000000000010792a6a0e25617800000000000000000073020100000a00000056656e7573204f" +
	"70656e0000000000000000801610680f25617800000000000000000074020100001600000053" +
	"656d6920526169642042696464696e67204f70656e0000000000000000801610680f25617800" +
	"0000000000000000d1030101002700000044656661756c744576656e7428454e5445525f4152" +
	"544946494349414c5f474f445f5241494429000000000000000010792a6a0e25617800000000" +
	"00000000002c030101002100000044656661756c744576656e7428454e5445525f494e41455f" +
	"4455534b5f57415229000000000000000010792a6a0e25617800000000000000000083030101" +
	"002000000044656661756c744576656e7428454e5445525f44454c455a49455f524149442900" +
	"0000000000000010792a6a0e256178000000000000000000ef030101001200000041706f6361" +
	"6c79707365204368616e6e656c00000000000000008015966afdf76178000000000000000000" +
	"00"

// eventInfoTable decodes the capture above once at startup.
var eventInfoTable = mustHexDecode(eventInfoTableHex)
