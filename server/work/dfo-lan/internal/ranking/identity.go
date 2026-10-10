package ranking

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// UserKey supplies the local read-only ranking identity. The native NOTI2631
// reader (1452DF690) copies exactly 33 bytes, and the URL builder treats them
// as a NUL-terminated filename component. The hash is a server-owned opaque
// identifier, not a claimed native cipher or an authentication credential.
func UserKey(account int64) ([]byte, error) {
	if account <= 0 {
		return nil, fmt.Errorf("ranking identity requires an account")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("115us:local-ranking:account:%d", account)))
	body := make([]byte, 33)
	hex.Encode(body[:32], sum[:16])
	return body, nil
}
