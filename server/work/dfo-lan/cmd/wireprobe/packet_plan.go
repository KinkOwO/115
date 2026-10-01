package main

// Encode and send one packet at a time, preserving partial-send behaviour.
func sendPacketPlan(plan []outboundPacket, send func(byte, uint16, []byte) error, sent func(outboundPacket)) error {
	for _, packet := range plan {
		if err := send(packet.Kind, packet.ID, packet.Payload); err != nil {
			return err
		}
		if sent != nil {
			sent(packet)
		}
	}
	return nil
}
