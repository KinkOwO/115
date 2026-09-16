package protocol

import "fmt"

// EmptyMercenaryInfo answers the initial account's empty mercenary collection.
// Request 0x143d5c070: u8 count, count bytes, then slot-13 block padding.
// Success 0x143d59f80: u8 collection count; zero skips all nested records.
func EmptyMercenaryInfo(request []byte) ([]byte, error) {
	if len(request) == 0 || int(request[0])+1 > len(request) {
		return nil, fmt.Errorf("short mercenary request")
	}
	if err := padding(request[1+int(request[0]):], 8); err != nil {
		return nil, err
	}
	return []byte{1, 0}, nil
}

// ProbeRosterCounters is the development account's zeroed counter fixture.
// 0x145258d00 consumes exactly 19 bytes after success. 0x14022af70 uses
// the first u32; remaining meanings are unresolved. No paid/event slots are
// granted. This is kept separate from the data-driven character capacity.
func ProbeRosterCounters(request []byte) ([]byte, error) {
	if len(request) != 0 {
		return nil, fmt.Errorf("roster counter request must be empty")
	}
	return append([]byte{1}, make([]byte, 19)...), nil
}
