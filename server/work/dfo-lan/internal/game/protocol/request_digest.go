package protocol

import (
	"bytes"
	"crypto/md5"
	"fmt"
)

// Current send flush146d73a70 ->1456b9700 appends a four-byte folded MD5
// for selected commands whose semantic body is1..400 bytes. The ordinary
// request writer does not include this suffix. This is separate from the
// encrypted frame's existing checksum, which the dispatcher also verifies.
func requestDigest(body []byte) []byte {
	d := md5.Sum(body)
	return []byte{d[13] ^ d[8] ^ d[10] ^ d[0] ^ 0x81, d[12] ^ d[5] ^ d[9] ^ d[1] ^ 0x78, d[14] ^ d[4] ^ d[6] ^ d[2] ^ 0x1a, d[15] ^ d[7] ^ d[11] ^ d[3] ^ 0xbf}
}

func digestRequestTail(p []byte, semantic, alignment int) error {
	if semantic < 1 || len(p) < semantic {
		return fmt.Errorf("unsupported digest request length")
	}
	// Unpadded direct native-writer fixtures stop before send flush.
	if len(p) == semantic {
		return nil
	}
	if semantic > 400 {
		return padding(p[semantic:], alignment)
	}
	if len(p) != (semantic+4+alignment-1)/alignment*alignment || !bytes.Equal(p[semantic:semantic+4], requestDigest(p[:semantic])) {
		return fmt.Errorf("request body digest mismatch")
	}
	return padding(p[semantic+4:], alignment)
}
