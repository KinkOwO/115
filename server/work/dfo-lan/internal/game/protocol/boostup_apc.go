package protocol

import "fmt"

// 140C13B00 writes one u8=1 for CMD2333; live AES body is padded to16.
func DecodeBoostAPCRequest(p []byte) error {
	if len(p) == 0 || p[0] != 1 {
		return fmt.Errorf("Boost同伴请求标志无效")
	}
	return padding(p[1:], 16)
}
