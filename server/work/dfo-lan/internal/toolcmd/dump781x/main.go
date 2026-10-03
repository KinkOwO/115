// 一次性工具：解密 s1/s4 官服抓包中的 op781/782 帧完整明文（next79 作战次数调查）。
package dump781x

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"

	"dfolan/internal/game/wire"
)

func dump(tag, keyPath, binPath string) {
	keys, err := os.ReadFile(keyPath)
	if err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(binPath)
	if err != nil {
		panic(err)
	}
	off := 0
	n := 0
	for off+16 <= len(raw) {
		id := binary.LittleEndian.Uint16(raw[off+1 : off+3])
		size := int(binary.LittleEndian.Uint32(raw[off+3 : off+7]))
		if size < 16 || off+size > len(raw) {
			fmt.Printf("[%s] frame %d @%#x: bad size %d, stop\n", tag, n, off, size)
			return
		}
		if id == 781 || id == 782 || id == 706 {
			body := raw[off+16 : off+size]
			plain, e := wire.DecryptPayload(keys, id, body)
			if e != nil {
				fmt.Printf("[%s] frame %d op=%d: decrypt error: %v\n", tag, n, id, e)
			} else {
				fmt.Printf("[%s] frame %d @%#x op=%d len=%d\n%s\n", tag, n, off, id, len(plain), hex.EncodeToString(plain))
			}
		}
		off += size
		n++
	}
	fmt.Printf("[%s] total frames parsed: %d, bytes: %d/%d\n", tag, n, off, len(raw))
}

func Run() {
	base := `D:\115us\analysis-tools\output\official_20261002-160349_decoded`
	dump("s1", base+`\session_s1_key.bin`, base+`\session_s1_s2c.bin`)
	fmt.Println()
	dump("s4", base+`\session_s4_key.bin`, base+`\session_s4_s2c.bin`)
}
