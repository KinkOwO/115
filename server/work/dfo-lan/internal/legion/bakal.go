// Bakal raid assault (巴卡尔攻坚战, contents/2022/bakalraid) wire contract —
// the RAID command family.
//
// Bakal does NOT speak the legion 2043/2290/2655 family of Venus/apocalypse:
// it enters through its own create-raid / start-raid pair (CMD656/CMD2089) and
// drives the whole run over a script channel (N2286/N2288/N572/N570) plus the
// shared dungeon direct-move (CMD2062). cmd and noti share one 16-bit id space
// with different meanings per direction: id 13 is the s2c reward inventory
// and the c2s reward claim; id 2 is the party actor snapshot; id 9 the
// subparty-created record.
//
// Evidence sources (do not reinterpret; extend only from new captures):
//   - Handover binary full clear run, runtime session
//     roles_..._20261005_200137_561796_next37/events.jsonl (12:29:29 start →
//     12:39:42 settlement, 24 map loads, every frame plain_hex; fixtures in
//     testdata/bakal_wire_fixtures.json).
//   - Failure runs in testdata/bakal_refusals.json: exact refusal reasons
//     ("PVF raid waiting room is not bound", "normal Bakal start requires its
//     owned real waiting-room member and native phase rules", "Bakal scripted
//     warp outside loaded owned combat", "invalid native Bakal camp return")
//     and the legal retreat path (N2285 retreat_to_camp).
//   - PVF rule source contents/2022/bakalraid/etc/bakal.etc (waiting room
//     town 152 area 2, 17 dungeon indexes 100003149..100003165, member max
//     12, start delay 3s, phase time over 9999, weekly clear 1, monster max
//     HP 10000). Parsed by internal/catalog; the constants below mirror the
//     values the wire carries so the wire layer can gate without the catalog.
//
// Unimplemented branches stay explicit refusals upstream — never invented
// bodies. The official-server anger commands (CMD582/CMD2290) have no
// handover-binary counterpart and are not implemented here.
package legion

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// C2S opcodes of the Bakal RAID family (ENUM_CMDPACKET_*).
//
// CMD12 (subparty create), CMD13 (reward claim; same id as the s2c reward
// inventory), CMD2062 (dungeon direct move) and CMD22/707 (portals) are shared
// with other subsystems — dispatch must gate on active Bakal world state, the
// id alone never identifies the packet.
const (
	CmdBakalCreateRaid   uint16 = 656  // create the raid party (waiting room)
	CmdBakalRaidUpdate   uint16 = 657  // raid party update / leave (silent)
	CmdBakalStartRaid    uint16 = 2089 // START_RAID — begin the vote/start flow
	CmdBakalBattleReport uint16 = 2069 // kill / damage report from a combat room
	CmdBakalScriptWarp   uint16 = 2070 // scripted warp inside the owned combat
	CmdBakalLoadingDone  uint16 = 2073 // map load complete (0B)
	CmdBakalCampReturn   uint16 = 2074 // native retreat to camp
	CmdBakalGiveup       uint16 = 2261 // observed once pre-raid, 0B; refused upstream
	CmdBakalFinalConfirm uint16 = 1134 // confirm the final-dungeon move
)

// S2C notification ids of the Bakal run. 574/581/572/584 are the shared RAID
// state family; 2343/2344 the vote pair; 2285/2286/2288 the run channel;
// 2194/570 the in-room monster/symbol channel; 27 the final selection panel
// (36B zero body — NOT the generic area-select N27); 588 the clear result.
const (
	NotiBakalVoteStart      uint16 = 2343
	NotiBakalVoteState      uint16 = 2344
	NotiBakalMemberAssigned uint16 = 578
	NotiBakalRaidState      uint16 = 574 // 0100 preparing / 0200 active / 0000 phase_ended
	NotiBakalCountdown      uint16 = 581 // 0B
	NotiBakalSubpartyCreate uint16 = 9
	NotiBakalPartyLocation  uint16 = 2285
	NotiBakalMonsters       uint16 = 2286
	NotiBakalBuffInventory  uint16 = 2288
	NotiBakalDungeonStates  uint16 = 572
	NotiBakalRemaining      uint16 = 584
	NotiBakalPortalReady    uint16 = 2281
	NotiBakalSourceBoss     uint16 = 2194
	NotiBakalSourceSymbol   uint16 = 570
	NotiBakalFinalSelection uint16 = 27
	NotiBakalClearResult    uint16 = 588
)

// Raid-state bytes of N574 across the run: preparing at start, active after
// the +3s solo vote window, phase_ended after the settlement burst.
const (
	BakalRaidStatePreparing = "\x01\x00"
	BakalRaidStateActive    = "\x02\x00"
	BakalRaidStateEnded     = "\x00\x00"
)

// World anchors from bakal.etc: the waiting room is town 152 area 2 and the
// raid runs on channel type 82 (internal raid_created_waiting events).
const (
	BakalWaitroomTown uint32 = 152
	BakalWaitroomArea uint32 = 2
	BakalChannelType  uint32 = 82
)

// Dungeon indexes of the 17 opened maps, straight from bakal.etc
// [DUNGEON INFO] and the s2c 572 open list (0x05F5ED4D..0x05F5ED5D). 149 is
// the Bakal dungeon itself, 150..152 thedragon lords skasa/sparazzi/hisma,
// 153..164 the normal maps, 165 the last Bakal dungeon whose PVF map is
// BakalFinalMap (the 1134/2281 target).
const BakalFinalMap uint32 = 100005068 // captured post-settlement CMD1134 map anchor

// Refusal reasons, byte-identical to the handover binary's event log
// (testdata/bakal_refusals.json). The weekly-clear limit has no captured
// refusal text — the binary symbol is ErrBakalWeeklyClearLimit — so it keeps
// the A-layer name until a live capture pins the exact string.
var (
	ErrBakalWaitingRoomNotBound = errors.New("PVF raid waiting room is not bound")
	ErrBakalStartRequiresMember = errors.New("normal Bakal start requires its owned real waiting-room member and native phase rules")
	ErrBakalWarpOutsideCombat   = errors.New("Bakal scripted warp outside loaded owned combat")
	ErrBakalInvalidCampReturn   = errors.New("invalid native Bakal camp return")
	ErrBakalWeeklyClearLimit    = errors.New("Bakal weekly clear limit reached")
)

// BakalRequests reports whether the id belongs to the Bakal-exclusive command
// set. Shared ids (12/13/22/2062/707) are intentionally absent: the caller
// gates those on active Bakal world state.
func BakalRequests(id uint16) bool {
	switch id {
	case CmdBakalCreateRaid, CmdBakalRaidUpdate, CmdBakalStartRaid,
		CmdBakalBattleReport, CmdBakalScriptWarp, CmdBakalLoadingDone,
		CmdBakalCampReturn, CmdBakalGiveup, CmdBakalFinalConfirm:
		return true
	default:
		return false
	}
}

// --- c2s decoders ---------------------------------------------------------

// BakalCreateRaidRequest is the decoded CMD656 body. Both observed shapes
// carry a null-terminated character name at offset 5 ("Lansmt" 32B named
// form; "1" 16B form); bytes 0-1 are a client counter, byte 14 the constant
// 07. The name identifies the creating member.
type BakalCreateRaidRequest struct {
	Name string
}

func DecodeBakalCreateRaid(p []byte) (BakalCreateRaidRequest, error) {
	if len(p) < 16 {
		return BakalCreateRaidRequest{}, fmt.Errorf("bakal create raid payload %d bytes, want at least 16", len(p))
	}
	end := 5
	for end < len(p) && p[end] != 0 {
		end++
	}
	return BakalCreateRaidRequest{Name: string(p[5:end])}, nil
}

// DecodeBakalStartRaid validates the CMD2089 START_RAID body: bytes 8-12 are
// the constant fe ff ff ff ff marker on every captured instance (clear and
// refused runs alike); bytes 0-3 are a client-side counter that must not be
// pinned.
func DecodeBakalStartRaid(p []byte) error {
	if len(p) < 32 {
		return fmt.Errorf("bakal start raid payload %d bytes, want at least 32", len(p))
	}
	if p[8] != 0xfe || !(p[9] == 0xff && p[10] == 0xff && p[11] == 0xff && p[12] == 0xff) {
		return fmt.Errorf("bakal start raid marker % x", p[8:13])
	}
	return nil
}

// BakalSubpartyRequest is the decoded CMD12 body: u32 name length @2, name @6
// ("Party : 1"), capacity byte 04 after the name, ffffffff, then the 05 type
// byte. Same family as the Venus standby party request but a distinct layout
// — the shared decoder refuses it, so Bakal owns this copy.
type BakalSubpartyRequest struct {
	Name      string
	Capacity  byte
	PartyType byte
}

func DecodeBakalSubparty(p []byte) (BakalSubpartyRequest, error) {
	if len(p) < 6 {
		return BakalSubpartyRequest{}, fmt.Errorf("bakal subparty payload %d bytes, want at least 6", len(p))
	}
	n := int(binary.LittleEndian.Uint32(p[2:6]))
	if n < 1 || 6+n > len(p) {
		return BakalSubpartyRequest{}, fmt.Errorf("bakal subparty name length %d out of range", n)
	}
	req := BakalSubpartyRequest{Name: string(p[6 : 6+n])}
	if at := 6 + n; at+1 < len(p) {
		req.Capacity = p[at]
	}
	if at := 6 + n + 1 + 4; at < len(p) { // capacity, ffffffff, then the type byte
		req.PartyType = p[at]
	}
	return req, nil
}

// BakalBattleReport is the decoded CMD2069 body: a kill/damage report from the
// combat room. The stable anchors are the current dungeon id u32 @17 and the
// defeated monster id u32 @21 (the 100007xxx PVF id space, e.g. 0x05F5FE49);
// bytes 1-12 are client counters/timestamps and the 0x65+ tail carries
// per-monster damage pairs.
type BakalBattleReport struct {
	Dungeon     uint32
	Map         uint32
	DamageTotal uint32
	DamageHits  uint32
}

func DecodeBakalBattleReport(p []byte) (BakalBattleReport, error) {
	if len(p) < 25 {
		return BakalBattleReport{}, fmt.Errorf("bakal battle report payload %d bytes, want at least 25", len(p))
	}
	report := BakalBattleReport{
		Dungeon: binary.LittleEndian.Uint32(p[17:21]),
		Map:     binary.LittleEndian.Uint32(p[21:25]),
	}
	if len(p) >= 69 {
		report.DamageTotal = binary.LittleEndian.Uint32(p[65:69])
	}
	if len(p) >= 137 {
		report.DamageHits = binary.LittleEndian.Uint32(p[133:137])
	}
	return report, nil
}

// DecodeBakalLoadingDone reads the CMD2073 load-complete: an empty body.
func DecodeBakalLoadingDone(p []byte) error {
	if len(p) != 0 {
		return fmt.Errorf("bakal loading done payload %d bytes, want 0", len(p))
	}
	return nil
}

// DecodeBakalScriptWarp reads the CMD2070 scripted warp. The captured bodies
// are 48B with two coordinate groups; the server-side gate is world state
// (inside loaded owned combat), not payload shape, so only the length is
// validated here.
func DecodeBakalScriptWarp(p []byte) error {
	_, err := DecodeBakalRoomWarp115(p)
	return err
}

type BakalRoomWarp struct {
	Grid   [2]byte
	Record [18]byte
}

// Handover raid_bakal.go:14/17/24/28 and captured CMD2070 vectors.
func DecodeBakalRoomWarp115(p []byte) (BakalRoomWarp, error) {
	var r BakalRoomWarp
	if len(p) < 39 || len(p) > 48 {
		return r, fmt.Errorf("invalid native Bakal room warp size")
	}
	for i := range r.Grid {
		v := binary.LittleEndian.Uint32(p[13+4*i:])
		if v > 255 {
			return r, fmt.Errorf("invalid native Bakal room grid")
		}
		r.Grid[i] = byte(v)
	}
	copy(r.Record[:], p[21:39])
	if r.Record[0] != 1 || r.Record[1] != 0 || r.Record[2] != 0 || r.Record[3] != 0 {
		return r, fmt.Errorf("invalid native Bakal warp token")
	}
	for _, v := range p[39:] {
		if v != 0 {
			return r, fmt.Errorf("nonzero Bakal warp padding")
		}
	}
	return r, nil
}

// BakalCampReturnRequest is the decoded CMD2074 retreat: 32B, u64 zeros,
// ffffffff, 00, then the option byte @13 (04 and 03 captured) — likely the
// retreat target selection (camp vs waiting room).
type BakalCampReturnRequest struct {
	Option byte
}

func DecodeBakalCampReturn(p []byte) (BakalCampReturnRequest, error) {
	if len(p) < 32 {
		return BakalCampReturnRequest{}, fmt.Errorf("bakal camp return payload %d bytes, want at least 32", len(p))
	}
	return BakalCampReturnRequest{Option: p[13]}, nil
}

// BakalFinalConfirmRequest is the decoded CMD1134: u32 1, the final map id
// u32 @4 (always 0x05F5F4CC = 100007116 on the captured instance), u32 14,
// u32 0x1801, u32 0xcd.
type BakalFinalConfirmRequest struct {
	FinalMap uint32
}

func DecodeBakalFinalConfirm(p []byte) (BakalFinalConfirmRequest, error) {
	if len(p) < 32 {
		return BakalFinalConfirmRequest{}, fmt.Errorf("bakal final confirm payload %d bytes, want at least 32", len(p))
	}
	return BakalFinalConfirmRequest{FinalMap: binary.LittleEndian.Uint32(p[4:8])}, nil
}

// DecodeBakalRewardClaim reads the CMD13 claim: 8 zero bytes on every
// captured instance.
func DecodeBakalRewardClaim(p []byte) error {
	if len(p) != 8 {
		return fmt.Errorf("bakal reward claim payload %d bytes, want 8", len(p))
	}
	return nil
}

// --- s2c constructors ----------------------------------------------------

// BakalVoteStartFrame builds the N2343 vote window: 181B = 13 zero bytes then
// 42 u32 slots [1, 0, 2, 1, 0, 3, 0, 3, …, 3] (solo capture, three vote
// options). The slot values are recorded, not interpreted.
func BakalVoteStartFrame() []byte {
	return bakalVoteFrame(0)
}

// BakalVoteStateFrame builds the N2344 vote state: identical to the vote
// start with slot 1 set to 1 — the solo member's vote registered
// immediately.
func BakalVoteStateFrame() []byte {
	return bakalVoteFrame(1)
}

func bakalVoteFrame(voteSlot byte) []byte {
	p := make([]byte, 13, 181)
	slots := make([]uint32, 0, 42)
	slots = append(slots, 1, uint32(voteSlot), 2, 1, 0)
	for i := 0; i < 18; i++ {
		slots = append(slots, 3, 0)
	}
	slots = append(slots, 3)
	for _, v := range slots {
		p = binary.LittleEndian.AppendUint32(p, v)
	}
	return p
}

// BakalMemberAssignedFrame builds the N578 real-member record (87B): u16 1,
// the waitroom channel 0x0152, u32 3, party byte 1, 2, name length, 3 pad
// bytes, the name, then the captured constants — 0x31, 1, 0xff, 115 (the
// client version marker), 2.
func BakalMemberAssignedFrame(name string) []byte {
	p := make([]byte, 0, 87)
	p = append(p, 1, 0)
	p = append(p, 0x52, 0x01) // waitroom channel (town 152)
	p = binary.LittleEndian.AppendUint32(p, 3)
	p = append(p, 1)
	p = append(p, 2)
	p = append(p, 0, 0) // two pad bytes before the name length
	p = append(p, byte(len(name)))
	p = append(p, 0, 0, 0)
	p = append(p, name...)
	p = append(p, 0)
	p = append(p, 0x31)
	p = append(p, 1)
	p = append(p, make([]byte, 5)...)
	p = append(p, 0xff)
	p = append(p, 115)
	p = append(p, make([]byte, 7)...)
	p = append(p, 2)
	return append(p, make([]byte, 87-len(p))...)
}

// BakalSubpartyCreatedFrame builds the N9 record (116B): u16 1, u16 9999
// (0x0F27), u16 1, the waitroom channel, 6 pad bytes, u32 name length, name,
// 2 pad bytes, capacity byte 4, ffffffff, party type 5, 20 zeros, 07×8, 15
// zeros, 1, 0, 2. The name and capacity come from the client's CMD12.
func BakalSubpartyCreatedFrame(name string, capacity byte, partyType byte) []byte {
	p := make([]byte, 0, 116)
	p = append(p, 1, 0)
	p = binary.LittleEndian.AppendUint16(p, 9999)
	p = append(p, 1, 0)
	p = append(p, 0x01, 0x52) // channel pair — byte order differs from N578's 52 01
	p = append(p, make([]byte, 6)...)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, name...)
	p = append(p, 0, 0)
	p = append(p, capacity)
	p = binary.LittleEndian.AppendUint32(p, 0xffffffff)
	p = append(p, partyType)
	p = append(p, make([]byte, 20)...)
	p = append(p, 7, 7, 7, 7, 7, 7, 7, 7)
	p = append(p, make([]byte, 15)...)
	p = append(p, 1, 0, 2)
	return append(p, make([]byte, 116-len(p))...)
}

// BakalPartyLocation is one entry of the N2285 20-slot table: the location
// index (bakal.etc [LOCATION INFO], 1..56; 25 marks an unused slot) and the
// opaque counter the capture carries next to it.
type BakalPartyLocation struct {
	Index   uint32
	Counter uint32
}

// bakalUnusedPartyLocation is the 25/0 filler of the N2285 table.
const bakalUnusedPartyLocation = 25

// BakalPartyFrame builds the N2285 party/room-location snapshot (173B): u8 1,
// u8 1, u16 0, u8 0, u32 current location @5, u32 0, then exactly 20
// (index, counter) pairs. The same shape serves the open burst, the per-room
// location updates, the legal retreat and the settlement return-to-camp.
func BakalPartyFrame(location uint32, entries []BakalPartyLocation) []byte {
	p := make([]byte, 13, 173)
	p[0], p[1] = 1, 1
	binary.LittleEndian.PutUint32(p[5:], location)
	for i := 0; i < 20; i++ {
		var entry BakalPartyLocation
		if i < len(entries) {
			entry = entries[i]
		} else {
			entry.Index = bakalUnusedPartyLocation
		}
		p = binary.LittleEndian.AppendUint32(p, entry.Index)
		p = binary.LittleEndian.AppendUint32(p, entry.Counter)
	}
	return p
}

// BakalMonster is one 19B row of the N2286 channel: slot/index u32, location
// u32, count u32, max-HP u32, then the 3-byte ffffffff tail. The initial
// frame carries the 8 boss slots (bakal.etc [CREATE MONSTER] locations 7, 12,
// 23, 24, 25, 26, 40, 47) at full HP; script frames carry spawn rows (HP
// 10000) and defeat rows (HP 0).
type BakalMonster struct {
	Buffs    [3]byte
	HasBuffs bool
	Slot     uint32
	Location uint32
	Count    uint32
	MaxHP    uint32
}

// BakalMonstersFrame builds an N2286 body: u8 row count, then count × 19B
// rows. 153B for the 8-row opening roster, 20/39/58/77/96B for the script
// spawn, health and defeat rows.
func BakalMonstersFrame(rows []BakalMonster) []byte {
	p := make([]byte, 0, 1+19*len(rows))
	p = append(p, byte(len(rows)))
	for _, r := range rows {
		p = binary.LittleEndian.AppendUint32(p, r.Slot)
		p = binary.LittleEndian.AppendUint32(p, r.Location)
		p = binary.LittleEndian.AppendUint32(p, r.Count)
		p = binary.LittleEndian.AppendUint32(p, r.MaxHP)
		if r.HasBuffs {
			p = append(p, r.Buffs[:]...)
		} else {
			p = append(p, 0xff, 0xff, 0xff)
		}
	}
	return p
}

// BakalBuffInventoryFrame builds the N2288 buff panel: 5 per-slot counts
// (bakal.etc [ADD BAKAL RAID BUFF 0..4], each 2 at open), the 0x19 constant,
// then a 7-byte tail — 00 00 00 ff ff ff ff at open, all zeros on the script
// updates that follow a buff grant.
func BakalBuffInventoryFrame(counts [5]byte, tail [7]byte) []byte {
	p := make([]byte, 0, 13)
	p = append(p, counts[:]...)
	p = append(p, 0x19)
	return append(p, tail[:]...)
}

// Bakal open-burst N2288 constants.
var (
	BakalBuffTailOpen   = [7]byte{0, 0, 0, 0xff, 0xff, 0xff, 0xff}
	BakalBuffTailScript = [7]byte{}
)

// BakalOpenDungeonsFrame builds the opening N572 (91B): u8 0, u8 17, then
// 17 × (dungeon index u32, 0), then u32 0.
func BakalOpenDungeonsFrame(dungeons []uint32) []byte {
	p := make([]byte, 0, 6+5*len(dungeons))
	p = append(p, 0, byte(len(dungeons)))
	for _, id := range dungeons {
		p = binary.LittleEndian.AppendUint32(p, id)
		p = append(p, 0)
	}
	return binary.LittleEndian.AppendUint32(p, 0)
}

// BakalDungeonStateFrame builds the 11B N572 single-dungeon update: u8 0,
// u8 1, dungeon index u32, then five zero bytes.
func BakalDungeonStateFrame(dungeon uint32) []byte {
	p := make([]byte, 0, 11)
	p = append(p, 0, 1)
	p = binary.LittleEndian.AppendUint32(p, dungeon)
	return append(p, make([]byte, 5)...)
}

// BakalRemainingFrame builds the N584 (9B): u8 0, u32 9999
// (bakal.etc [PHASE TIME OVER]), u32 0.
func BakalRemainingFrame(remaining uint32) []byte {
	p := make([]byte, 0, 9)
	p = append(p, 0)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	return binary.LittleEndian.AppendUint32(p, 0)
}

// BakalPortalReadyFrame builds the N2281 portal-ready (38B all zero) — also
// the final-move body.
func BakalPortalReadyFrame() []byte { return make([]byte, 38) }

// BakalSourceBossFrame builds the 34B N2194 in-room boss row: 1, slot, kind
// (0 boss-room row, 5 on the Bakal row), the room counter u32, the run
// counter u32, then 00 03 00, the 100 monster-family marker (the generic
// N2194 row carries the same marker), the HP pair, ffffffff, 0. The captured
// instances:
//
//	slot 3 kind 0 room 0x105c run 0x067f6dcf HP 650/250
//	slot 4 kind 0 room 0x103c run 0x067f6dd1 HP 650/250
//	slot 1 kind 5 room 0x1001 run 0x067f6dd3 HP 945/311
func BakalSourceBossFrame(gridX, gridY byte, entity, template, x, y uint32) []byte {
	p := make([]byte, 0, 34)
	p = append(p, 1, gridX, gridY)
	p = binary.LittleEndian.AppendUint32(p, entity)
	p = binary.LittleEndian.AppendUint32(p, template)
	p = append(p, 0, 3, 0)
	p = binary.LittleEndian.AppendUint32(p, 100)
	p = binary.LittleEndian.AppendUint32(p, x)
	p = binary.LittleEndian.AppendUint32(p, y)
	p = binary.LittleEndian.AppendUint32(p, 0xffffffff)
	return binary.LittleEndian.AppendUint32(p, 0)
}

// N570 is count=1, symbol ID u32, signed value i32. This matches both the
// native RaidGlobalSymbol115 builder and the captured 01c900000010270000.
func BakalSourceSymbolFrame(symbol uint32, value int32) []byte {
	p := make([]byte, 0, 9)
	p = append(p, 1)
	p = binary.LittleEndian.AppendUint32(p, symbol)
	return binary.LittleEndian.AppendUint32(p, uint32(value))
}

// BakalFinalSelectionFrame builds the N27 final-selection panel: 36B all
// zero. This is the Bakal body, not the generic area-select N27.
func BakalFinalSelectionFrame() []byte { return make([]byte, 36) }

// BakalClearResultFrame builds the N588 settlement result (10B): 00 00, u16
// 610 (the Bakal content tag), u32 0, then the bytes 00 01.
func BakalClearResultFrame() []byte {
	p := make([]byte, 0, 10)
	p = append(p, 0, 0)
	p = binary.LittleEndian.AppendUint16(p, 610)
	p = binary.LittleEndian.AppendUint32(p, 0)
	return append(p, 0, 1)
}

// BakalAck builds the family's 1B success replies (N2089 start ack, N2073
// loading ack, N2070 script-warp ack).
func BakalAck() []byte { return []byte{1} }
