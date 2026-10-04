package main

import "encoding/binary"

// eventInfoTable is the raw NOTI108 EVENT_INFO table used by the private
// client. It contains the 19 official legion/raid gate records. The client
// reads this payload directly; wrapping it in zlib makes it reject the table.
//
// Records retain the fixed wire footprint used by the 115US capture: a
// little-endian event id followed by record metadata and display name. The
// final record is one byte shorter, giving the captured 1141-byte body.
func buildEventInfoTable() []byte {
	type record struct {
		id   uint16
		name string
	}
	records := [...]record{
		{1007, "Apocalypse Anti-Enbi"},
		{776, "Ispins Legion Open"},
		{641, "Machine Revolution"},
		{640, "Asrahan Mu"},
		{626, "Goddess Venus"},
		{590, "Awakened Forest"},
		{977, "Dusky Island"},
		{812, "Mist Raid"},
		{899, "Bakal Raid"},
		{627, "Sirocco Raid"},
		{642, "Ozma Raid"},
		{621, "Prey-Isys Raid"},
		{680, "Bakal Prelude"},
		{681, "Hall of Dimensions"},
		{628, "White Cloud Valley"},
		{462, "Unshackled Nightmare"},
		{482, "Imperial Warfare"},
		{2453, "World Boss"},
		{414, "Grand Turtle"},
	}
	const recordSize = 60
	b := make([]byte, 1141)
	binary.LittleEndian.PutUint16(b, uint16(len(records)))
	for i, r := range records {
		off := 2 + i*recordSize
		binary.LittleEndian.PutUint16(b[off:], r.id)
		// The captured record stores its display name at byte 9.
		copy(b[off+9:], r.name)
	}
	return b
}

var eventInfoTable = buildEventInfoTable()
