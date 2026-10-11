package protocol

import "fmt"

// CMD656 sender1450701f0 writes its fifth option via14506f4b0.
// Native Bakal UI choices1/2 produce0/1;2/3 belong to other raid variants.
func (r RaidCreateRequest) BakalMode115() (byte, error) {
	if r.Kind != 8 || r.Options[4] > 1 {
		return 0, fmt.Errorf("unsupported native Bakal variant")
	}
	return r.Options[4], nil
}
