package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func (a *Archive) decodeScript(raw []byte) string {
	lineCount := len(raw) / 5
	if lineCount == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw) * 2)
	for idx := 0; idx < lineCount; idx++ {
		off := idx * 5
		tokenType := raw[off]
		value := readInt32(raw[off+1 : off+5])
		switch tokenType {
		case 0:
			fmt.Fprintf(&b, "%d ", value)
		case 2:
			fmt.Fprintf(&b, "%.2f ", math.Float32frombits(uint32(value)))
		case 3:
			b.WriteByte('\n')
			b.WriteString(a.resolveString(value))
			b.WriteByte('\n')
		case 5:
			b.WriteByte('\n')
			b.WriteString("{5=``}")
		case 6:
			b.WriteByte('`')
			b.WriteString(a.resolveString(value))
			b.WriteString("` ")
		case 7:
			b.WriteByte('\n')
			b.WriteString("{7=``}")
		}
	}
	return b.String()
}

func (a *Archive) resolveString(magicOffset int) string {
	if magicOffset < 0 {
		return ""
	}
	if a.stringPools != nil {
		return a.stringPools.resolve(magicOffset)
	}
	if magicOffset&1 != 0 {
		return readUTF16String(a.strW, (magicOffset>>1)*2)
	}
	return readUTF8String(a.strA, magicOffset>>1)
}

func readUTF8String(buf []byte, start int) string {
	if start < 0 || start >= len(buf) {
		return ""
	}
	end := bytes.IndexByte(buf[start:], 0)
	if end < 0 {
		return ""
	}
	rawBytes := buf[start : start+end]
	if utf8.Valid(rawBytes) {
		return string(rawBytes)
	}
	raw := string(rawBytes)
	if decoded, err := simplifiedchinese.GB18030.NewDecoder().String(raw); err == nil {
		return decoded
	}
	return raw
}

func readUTF16String(buf []byte, start int) string {
	if start < 0 || start >= len(buf) {
		return ""
	}
	end := start
	for end+1 < len(buf) {
		if buf[end] == 0 && buf[end+1] == 0 {
			break
		}
		end += 2
	}
	if end <= start {
		return ""
	}
	return decodeUTF16LE(buf[start:end])
}

var utf16Scratch = sync.Pool{New: func() any { b := make([]byte, 0, 256); return &b }}

// decodeUTF16LE encodes little-endian UTF-16 to UTF-8 in a single pass. It
// avoids the []uint16 and []rune intermediates of utf16.Decode/string(), which
// dominated the bulk-import profile; a pooled buffer avoids per-string scratch
// allocation. Unpaired surrogates become U+FFFD, matching utf16.Decode.
func decodeUTF16LE(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	bufp := utf16Scratch.Get().(*[]byte)
	buf := (*bufp)[:0]
	ascii := true
	for i := 0; i+1 < len(data); i += 2 {
		if data[i+1] != 0 {
			ascii = false
			break
		}
	}
	if ascii {
		for i := 0; i+1 < len(data); i += 2 {
			buf = append(buf, data[i])
		}
	} else {
		for i := 0; i+1 < len(data); i += 2 {
			u := binary.LittleEndian.Uint16(data[i : i+2])
			switch {
			case u < 0x80:
				buf = append(buf, byte(u))
			case u < 0x800:
				buf = append(buf, 0xC0|byte(u>>6), 0x80|byte(u&0x3F))
			case u >= 0xD800 && u < 0xDC00:
				if i+3 < len(data) {
					v := binary.LittleEndian.Uint16(data[i+2 : i+4])
					if v >= 0xDC00 && v < 0xE000 {
						buf = utf8.AppendRune(buf, rune(u-0xD800)<<10|rune(v-0xDC00)+0x10000)
						i += 2
						continue
					}
				}
				buf = utf8.AppendRune(buf, utf8.RuneError)
			case u >= 0xDC00 && u < 0xE000:
				buf = utf8.AppendRune(buf, utf8.RuneError)
			default:
				buf = append(buf, 0xE0|byte(u>>12), 0x80|byte((u>>6)&0x3F), 0x80|byte(u&0x3F))
			}
		}
	}
	s := string(buf)
	*bufp = buf
	utf16Scratch.Put(bufp)
	return s
}

func zlibBytes(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func compressZlib(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
