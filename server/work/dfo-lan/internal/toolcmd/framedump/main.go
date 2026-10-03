// Command framedump is a one-off forensics helper: it decrypts a single
// server frame from a wiredecode-captured session stream using the private
// server's wire package, so captured official frames can be compared
// byte-for-byte with replayed constants.
package framedump

import (
	"fmt"
	"os"

	"dfolan/internal/game/wire"
)

func Run() {
	key, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	stream, err := os.ReadFile(os.Args[3])
	if err != nil {
		panic(err)
	}
	var off, size int
	header := 16
	if len(os.Args) > 5 {
		if _, err := fmt.Sscanf(os.Args[5], "%d", &header); err != nil {
			panic(err)
		}
	}
	if os.Args[4] == "all" {
		// Walk the whole stream and dump every frame's full plaintext, one
		// line per frame: idx off kind id size plain=<complete hex>.
		for o, idx := 0, 0; o+header <= len(stream); idx++ {
			s := int(stream[o+3]) | int(stream[o+4])<<8 | int(stream[o+5])<<16 | int(stream[o+6])<<24
			if s < header || o+s > len(stream) {
				fmt.Printf("frame=%d off=%d TRUNCATED rem=%d\n", idx, o, len(stream)-o)
				break
			}
			fid := int(stream[o+1]) | int(stream[o+2])<<8
			plain, err := wire.DecryptPayload(key, uint16(fid), stream[o+header:o+s])
			if err != nil {
				// The channel key packet (id=1) rides unencrypted; fall back
				// to raw bytes so the dump still preserves it in full.
				fmt.Printf("frame=%d off=%d kind=%d id=%d size=%d plain=RAW(%v):%x\n", idx, o, stream[o], fid, s, err, stream[o+header:o+s])
			} else {
				fmt.Printf("frame=%d off=%d kind=%d id=%d size=%d plain=%x\n", idx, o, stream[o], fid, s, plain)
			}
			o += s
		}
		return
	}
	if _, err := fmt.Sscanf(os.Args[4], "%d:%d", &off, &size); err != nil {
		var id int
		if _, err := fmt.Sscanf(os.Args[4], "%d", &id); err != nil {
			panic(err)
		}
		// Scan the stream for a server frame with this id.
		for o := 0; o+16 <= len(stream); {
			s := int(stream[o+3]) | int(stream[o+4])<<8 | int(stream[o+5])<<16 | int(stream[o+6])<<24
			if s < 16 || o+s > len(stream) {
				break
			}
			fid := int(stream[o+1]) | int(stream[o+2])<<8
			if fid == id {
				off, size = o, s
				break
			}
			o += s
		}
		if size == 0 {
			panic("frame not found")
		}
	}
	frame := stream[off : off+size]
	fmt.Printf("offset=%d size=%d id=%d\n", off, size, int(frame[1])|int(frame[2])<<8)
	plain, err := wire.DecryptPayload(key, uint16(frame[1])|uint16(frame[2])<<8, frame[header:])
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(plain); i += 32 {
		e := i + 32
		if e > len(plain) {
			e = len(plain)
		}
		for _, b := range plain[i:e] {
			fmt.Printf("%02x", b)
		}
		fmt.Println()
	}
}
