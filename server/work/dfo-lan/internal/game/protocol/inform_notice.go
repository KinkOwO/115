package protocol

// InformNoticeSeen builds the NOTI402 / NOTI426 payload: one count byte
// followed by each read notice id as a single byte. The empty set is the
// single byte 0 — the client clears its own read red-black tree and stops
// re-popping the teaching frames (the third-awakening
// "Skill Evolve/Enhance Option Selection Guide" rides tree 2, notice id 62).
func InformNoticeSeen(ids []uint16) []byte {
	n := len(ids)
	if n > 255 {
		n = 255
	}
	p := make([]byte, 1+n)
	p[0] = byte(n)
	for i := 0; i < n; i++ {
		p[1+i] = byte(ids[i])
	}
	return p
}
