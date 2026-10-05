// Command wiredecodedump decrypts a captured DFO 2.38.2.34 stream and dumps the
// FULL plaintext of every frame (the prebuilt wiredecode.exe truncated plain= at
// 37 bytes, which hid the bodies we need for the Azure Main packet layouts).
//
// Read-only forensic tool: it never touches the game path or any storage.
//
//	wiredecodedump <key.bin> <stream.bin> c2s|s2c [out.txt]
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"dfolan/internal/game/wire"
)

// deobf inverts the Channel Info packet body obfuscation: ROR2(c) ^ 0xB5.
func deobf(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		out[i] = (((c << 6) | (c >> 2)) & 0xFF) ^ 0xB5
	}
	return out
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: wiredecodedump <key.bin> <stream.bin> c2s|s2c [out.txt]")
		os.Exit(2)
	}
	key, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read key:", err)
		os.Exit(1)
	}
	if len(key) != wire.SessionKeyBytes {
		fmt.Fprintf(os.Stderr, "session key bytes: got %d, need %d\n", len(key), wire.SessionKeyBytes)
		os.Exit(1)
	}
	stream, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read stream:", err)
		os.Exit(1)
	}
	mode := strings.ToLower(os.Args[3])

	var sink io.Writer = os.Stdout
	if len(os.Args) > 4 {
		f, err := os.Create(os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, "create out:", err)
			os.Exit(1)
		}
		defer f.Close()
		sink = f
	}
	w := bufio.NewWriter(sink)
	defer w.Flush()

	fmt.Fprintf(w, "# key=%s (%d bytes)\n", os.Args[1], len(key))
	fmt.Fprintf(w, "# stream=%s mode=%s bytes=%d\n", os.Args[2], mode, len(stream))

	off, idx := 0, 0
	for off < len(stream) {
		switch mode {
		case "c2s":
			if len(stream)-off < wire.ClientHeaderSize {
				fmt.Fprintf(w, "# tail %d bytes too short for a c2s header\n", len(stream)-off)
				off = len(stream)
				continue
			}
			typ := stream[off]
			id := binary.LittleEndian.Uint16(stream[off+1 : off+3])
			size := int(binary.LittleEndian.Uint32(stream[off+3 : off+7]))
			sum := stream[off+7]
			seq := binary.LittleEndian.Uint16(stream[off+11 : off+13])
			if size < wire.ClientHeaderSize || size > wire.MaxPacketSize || off+size > len(stream) {
				fmt.Fprintf(w, "# !! bad c2s frame #%d at 0x%06X size=%d remain=%d\n", idx, off, size, len(stream)-off)
				off = len(stream)
				continue
			}
			body := stream[off+wire.ClientHeaderSize : off+size]
			plain, derr := wire.DecryptPayload(key, id, body)
			status := "OK"
			if derr != nil {
				status = "ERR(" + derr.Error() + ")"
				plain = nil
			} else {
				chk := make([]byte, 2+len(plain))
				binary.LittleEndian.PutUint16(chk, seq)
				copy(chk[2:], plain)
				if wire.Checksum(chk) != sum {
					status = "BADCS"
				}
			}
			fmt.Fprintf(w, "c2s %4d @0x%06X type=%d id=%-6d size=%-6d body=%-6d seq=%-5d slot=%-2d cs=%-6s plain=%s\n",
				idx, off, typ, id, size, len(body), seq, int(id)%14, status, hex.EncodeToString(plain))
			off += size

		case "s2c":
			if len(stream)-off < wire.ServerHeaderSize {
				fmt.Fprintf(w, "# tail %d bytes too short for an s2c header\n", len(stream)-off)
				off = len(stream)
				continue
			}
			kind := stream[off]
			id := binary.LittleEndian.Uint16(stream[off+1 : off+3])
			size := int(binary.LittleEndian.Uint32(stream[off+3 : off+7]))
			sum := stream[off+11]
			if size < wire.ServerHeaderSize || size > wire.MaxPacketSize || off+size > len(stream) {
				fmt.Fprintf(w, "# !! bad s2c frame #%d at 0x%06X size=%d remain=%d\n", idx, off, size, len(stream)-off)
				off = len(stream)
				continue
			}
			body := stream[off+wire.ServerHeaderSize : off+size]
			if kind == 0 && id == 1 {
				fmt.Fprintf(w, "s2c %4d @0x%06X kind=%d id=%-6d size=%-6d body=%-6d slot=%-2d cs=KEY  plain=%s\n",
					idx, off, kind, id, size, len(body), int(id)%14, hex.EncodeToString(deobf(body)))
				off += size
				idx++
				continue
			}
			plain, derr := wire.DecryptPayload(key, id, body)
			status := "OK"
			if derr != nil {
				status = "ERR(" + derr.Error() + ")"
				plain = nil
			} else if wire.Checksum(body) != sum {
				status = "BADCS"
			}
			fmt.Fprintf(w, "s2c %4d @0x%06X kind=%d id=%-6d size=%-6d body=%-6d slot=%-2d cs=%-6s plain=%s\n",
				idx, off, kind, id, size, len(body), int(id)%14, status, hex.EncodeToString(plain))
			off += size

		default:
			fmt.Fprintln(os.Stderr, "mode must be c2s or s2c")
			os.Exit(2)
		}
		idx++
	}
	fmt.Fprintf(w, "# frames=%d\n", idx)
}
