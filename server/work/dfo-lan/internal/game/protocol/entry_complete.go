package protocol

// EnterGameworldComplete is NOTI 124 in this build. Native 145301370 reads
// one u32 at 1453013b7, finishes world initialization, and enables normal
// inventory/settings input through 1459b47f0 at 1453023df. The scalar is
// consumed only by optional UI 0xab5 (+0x658); its business meaning remains
// unresolved, so the probe supplies an explicit zero without granting content.
func EnterGameworldComplete() []byte { return add32(nil, 0) }
