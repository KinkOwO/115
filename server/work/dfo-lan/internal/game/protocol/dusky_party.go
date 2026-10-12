package protocol

// Current US 142ac1950 descriptor at149b97420 binds channel91, dungeon type32,
// content103 and party type16; its +1c8/+1d8 maps bind party<->content.
// The common native standby writer/reader retains the shared C12/N9 layout.
const DuskyStandbyPartyType byte = 16

func DecodeDuskyStandbyParty(p []byte) ([]byte, error) {
	return decodeLegionStandbyParty(p, DuskyStandbyPartyType)
}

func DuskyStandbyPartyReply(name []byte, actor uint16, channel [2]byte) ([]byte, error) {
	return legionStandbyPartyReply(name, actor, channel, DuskyStandbyPartyType)
}
