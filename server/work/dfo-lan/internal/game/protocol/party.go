package protocol

// SoloPartyInfo uses the same current full-info N9 reader as ordinary parties.
// Native 146d78070 and 146d77f50 both consume empty u32 lengths; the member
// count is at69, with one 17-byte seat row and all eight remaining seat updates.
// Context must match the actor introduced by N2, including on dungeon entry.
func SoloPartyInfo(actor uint16, context [2]byte) ([]byte, error) {
	// The native capacity is the invitation limit, not the member count.
	// Keep one occupied seat and four available ordinary-party seats.
	return PartyRosterSeats115(1, context, [4]uint16{actor}, actor, 4)
}
