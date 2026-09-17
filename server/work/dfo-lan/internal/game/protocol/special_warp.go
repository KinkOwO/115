package protocol

import "fmt"

// CMD2261 at146d07b8e contains no body. It prepares an animation, not a
// destination authorization. The actual area request follows as CMD36.
func DecodeSpecialWarpPreparation(p []byte) error {
	if len(p) != 0 {
		return fmt.Errorf("special warp preparation requires empty body")
	}
	return nil
}

// NOTI365 at1452e6530: type4, count1, owned actor u16, teleport skin u32.
// Skin0 follows the native default animation fallback at145bf29a0.
func SpecialWarpStart(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid special warp actor")
	}
	return add32(add16([]byte{4, 1}, actor), 0), nil
}
