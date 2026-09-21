package protocol

import "fmt"

// CMD1881 (ENUM_CMDPACKET_CHANGE_GROW_TYPE) and CMD777
// (ENUM_CMDPACKET_RE_GROWUP_CHANGE) are the in-town job-change family: 1881 is
// the first advancement (base->job), 777 the free re-change (job->job). Both
// senders initialize only byte 13 of a 14-byte stack body with the target
// packed grow-type (confirmed live: 1881->04 dragon knight, 777->01/03); the
// leading bytes are uninitialized stack data and are not trusted. 1881 rides
// cipher slot 1881%14 == 5 (skipjack), 777 slot 777%14 == 7 (blowfish), both
// 8-byte aligned, so a decrypted body is 14 bytes or 16 with zero padding.
//
// The response handlers 0x14524fd70 (1881) and 0x1452aff30 (777) sit behind
// the shared [result:u8][code:u16] CMD-response dispatch (0x1459a2d70). A
// nonzero result is success and the new grow-type is read back from the
// already-resynced character object, so the success body carries no grow-type
// of its own.
func DecodeChangeGrowType(p []byte) (advancement byte, err error) {
	if len(p) != 14 && len(p) != 16 {
		return 0, fmt.Errorf("change grow type request length")
	}
	if err := padding(p[14:], 8); err != nil {
		return 0, err
	}
	// 0x14563eee3 splits the packed grow-type byte: the low nibble is the
	// advancement branch and bits 4..6 the awakening stage. A job change
	// always lands unawakened, so only the advancement branch is honoured.
	advancement = p[13] & 0xF
	if advancement == 0 {
		return 0, fmt.Errorf("change grow type target is the base profession")
	}
	return advancement, nil
}

// ChangeGrowTypeSuccess is [result:u8][code:u16] with result == 1.
func ChangeGrowTypeSuccess() []byte { return []byte{1, 0, 0} }
