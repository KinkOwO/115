// Command dump342 is a throwaway forensics helper (next79 §17): it walks the
// official s2c capture, decrypts every NOTI342 frame with the session key,
// inflates the zlib body and dumps the completed-quest list structure.
package dump342

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"dfolan/internal/game/wire"
)

func Run() {
	keyPath, binPath := os.Args[1], os.Args[2]
	keys, err := os.ReadFile(keyPath)
	if err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(binPath)
	if err != nil {
		panic(err)
	}
	off := 0
	for off+16 <= len(raw) {
		id := binary.LittleEndian.Uint16(raw[off+1 : off+3])
		ln := int(binary.LittleEndian.Uint32(raw[off+3 : off+7]))
		if ln < 16 || off+ln > len(raw) {
			fmt.Printf("frame at %d malformed (id=%d ln=%d)\n", off, id, ln)
			return
		}
		if id == 342 {
			body := raw[off+16 : off+ln]
			plain, err := wire.DecryptPayload(keys, id, body)
			if err != nil {
				fmt.Printf("342 at %d decrypt: %v\n", off, err)
				return
			}
			fmt.Printf("frame @%d id=%d body=%d plain[0:8]=%s\n", off, id, len(body), hex.EncodeToString(plain[:8]))
			zr, err := zlib.NewReader(bytes.NewReader(plain))
			if err != nil {
				fmt.Println("not zlib:", err)
				fmt.Println(hex.Dump(plain[:64]))
				return
			}
			out, err := io.ReadAll(zr)
			if err != nil {
				fmt.Println("inflate:", err)
				return
			}
			fmt.Printf("inflated len=%d head=%s\n", len(out), hex.EncodeToString(out[:48]))
			n := binary.LittleEndian.Uint32(out[:4])
			fmt.Println("count field:", n)
			if len(out)%4 == 0 && int(4+4*n) <= len(out) {
				fmt.Printf("=> %d ids; first 16:", n)
				for i := 0; i < 16; i++ {
					fmt.Printf(" %d", binary.LittleEndian.Uint32(out[4+4*i:]))
				}
				fmt.Println()
				hit := map[uint32]bool{}
				for i := 0; i < int(n); i++ {
					hit[binary.LittleEndian.Uint32(out[4+4*i:])] = true
				}
				for _, q := range []uint32{13763, 13714, 13748, 13762} {
					fmt.Printf("quest %d in official 342: %v\n", q, hit[q])
				}
				fmt.Println("total parsed bytes used:", 4+4*n, "of", len(out))
			} else {
				fmt.Println("count does not match body length — alternate layout")
				fmt.Println(hex.Dump(out[:96]))
			}
			return
		}
		off += ln
	}
	fmt.Println("no 342 frame found")
}
