package npkicon

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

type texture struct{ off, length, size, w, h int }

// V5 在颜色表之后保存 28 字节纹理描述，再保存混合的 36/64/8 字节帧索引。
// 纹理负载为 zlib 包裹的 DDS；普通像素帧接在全部纹理负载之后。
func parseTextureFrames(img []byte, idxLen, count int) ([]frame, int, error) {
	if len(img) < 44 {
		return nil, 0, fmt.Errorf("V5 头部不完整")
	}
	u := func(at int) int { return int(binary.LittleEndian.Uint32(img[at:])) }
	n, colors := u(32), u(40)
	if n > maxFramesLimit || colors > 256 || 44+colors*4+n*28 > len(img) {
		return nil, 0, fmt.Errorf("V5 纹理表越界")
	}
	start := 44 + colors*4 + n*28
	base := start + idxLen
	if base < start || base > len(img) {
		return nil, 0, fmt.Errorf("V5 帧表越界")
	}
	textures := make([]texture, n)
	data := base
	for i := range textures {
		at := 44 + colors*4 + i*28
		t := texture{data, u(at + 12), u(at + 16), u(at + 20), u(at + 24)}
		if u(at+8) != i || t.w < 1 || t.h < 1 || t.w > 4096 || t.h > 4096 || t.length > len(img)-data || t.size > t.w*t.h*4+128 {
			return nil, 0, fmt.Errorf("V5 纹理 %d 描述无效", i)
		}
		textures[i] = t
		data += t.length
	}
	frames := make([]frame, 0, count)
	pos := start
	for i := 0; i < count; i++ {
		if pos+8 > base {
			return nil, 0, fmt.Errorf("V5 帧 %d 索引不完整", i)
		}
		k, comp := u(pos), u(pos+4)
		if k == linkMarker || comp == linkMarker {
			target := comp
			if k != linkMarker {
				target = k
			}
			frames = append(frames, frame{isLink: true, link: target})
			pos += 8
			continue
		}
		if pos+36 > base {
			return nil, 0, fmt.Errorf("V5 图像索引不完整")
		}
		f := frame{color: uint32(k), comp: uint32(comp), w: uint32(u(pos + 8)), h: uint32(u(pos + 12)),
			length: uint32(u(pos + 16)), x: int(int32(u(pos + 20))), y: int(int32(u(pos + 24))), off: data - base}
		if f.w == 0 || f.h == 0 || f.w > 4096 || f.h > 4096 {
			return nil, 0, fmt.Errorf("V5 帧尺寸异常")
		}
		f.blank = f.w == 1 && f.h == 1 && f.y == 0 && f.x == u(pos+28)
		if k >= 18 && k <= 20 && comp == 7 {
			if pos+64 > base || u(pos+40) >= n || u(pos+60) > 1 {
				return nil, 0, fmt.Errorf("V5 纹理帧引用无效")
			}
			f.texture = &textures[u(pos+40)]
			f.crop = image.Rect(u(pos+44), u(pos+48), u(pos+52), u(pos+56))
			f.rotated = u(pos+60) == 1
			cw, ch := f.crop.Dx(), f.crop.Dy()
			if f.rotated {
				cw, ch = ch, cw
			}
			if cw != int(f.w) || ch != int(f.h) || !f.crop.In(image.Rect(0, 0, f.texture.w, f.texture.h)) {
				return nil, 0, fmt.Errorf("V5 纹理裁切越界")
			}
			pos += 64
		} else if k >= colorARGB1555 && k <= colorARGB8888 && (comp == compressNone || comp == compressZlib) {
			if int(f.length) > len(img)-data {
				return nil, 0, fmt.Errorf("V5 像素负载越界")
			}
			data += int(f.length)
			pos += 36
		} else {
			return nil, 0, fmt.Errorf("V5 帧类型不支持：%d/%d", k, comp)
		}
		frames = append(frames, f)
	}
	if pos != base || data != len(img) {
		return nil, 0, fmt.Errorf("V5 索引或数据长度不符")
	}
	return frames, base, nil
}

func decodeTextureFrame(img []byte, f frame) (image.Image, error) {
	t := f.texture
	zr, err := zlib.NewReader(bytes.NewReader(img[t.off : t.off+t.length]))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	dds, err := io.ReadAll(io.LimitReader(zr, int64(t.size)+1))
	if err != nil {
		return nil, err
	}
	if len(dds) != t.size {
		return nil, fmt.Errorf("DDS 解压长度不符")
	}
	atlas, err := decodeDDS(dds, t.w, t.h)
	if err != nil {
		return nil, err
	}
	out := image.NewNRGBA(image.Rect(0, 0, int(f.w), int(f.h)))
	for y := 0; y < int(f.h); y++ {
		for x := 0; x < int(f.w); x++ {
			sx, sy := f.crop.Min.X+x, f.crop.Min.Y+y
			if f.rotated {
				sx, sy = f.crop.Max.X-1-y, f.crop.Min.Y+x
			}
			out.SetNRGBA(x, y, atlas.NRGBAAt(sx, sy))
		}
	}
	return out, nil
}

func decodeDDS(raw []byte, w, h int) (*image.NRGBA, error) {
	if len(raw) < 128 || string(raw[:4]) != "DDS " || binary.LittleEndian.Uint32(raw[4:]) != 124 ||
		int(binary.LittleEndian.Uint32(raw[12:])) != h || int(binary.LittleEndian.Uint32(raw[16:])) != w {
		return nil, fmt.Errorf("DDS 头部与纹理描述不符")
	}
	format := string(raw[84:88])
	blockSize := 16
	if format == "DXT1" {
		blockSize = 8
	} else if format != "DXT3" && format != "DXT5" {
		return nil, fmt.Errorf("DDS 格式未支持：%s", format)
	}
	if len(raw)-128 < ((w+3)/4)*((h+3)/4)*blockSize {
		return nil, fmt.Errorf("DDS 像素块不足")
	}
	color565 := func(v uint16) color.NRGBA {
		r, g, b := byte(v>>11), byte((v>>5)&63), byte(v&31)
		return color.NRGBA{r<<3 | r>>2, g<<2 | g>>4, b<<3 | b>>2, 255}
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	pos := 128
	for by := 0; by < h; by += 4 {
		for bx := 0; bx < w; bx += 4 {
			block := raw[pos : pos+blockSize]
			pos += blockSize
			colorBlock := block
			if blockSize == 16 {
				colorBlock = block[8:]
			}
			c0, c1 := binary.LittleEndian.Uint16(colorBlock), binary.LittleEndian.Uint16(colorBlock[2:])
			cs := [4]color.NRGBA{color565(c0), color565(c1)}
			mix := func(a, b color.NRGBA, na, nb, div int) color.NRGBA {
				return color.NRGBA{byte((int(a.R)*na + int(b.R)*nb) / div), byte((int(a.G)*na + int(b.G)*nb) / div), byte((int(a.B)*na + int(b.B)*nb) / div), 255}
			}
			if c0 > c1 || format != "DXT1" {
				cs[2] = mix(cs[0], cs[1], 2, 1, 3)
				cs[3] = mix(cs[0], cs[1], 1, 2, 3)
			} else {
				cs[2] = mix(cs[0], cs[1], 1, 1, 2)
			}
			indices := binary.LittleEndian.Uint32(colorBlock[4:])
			var alpha [8]byte
			var alphaBits uint64
			if format == "DXT5" {
				alpha[0], alpha[1] = block[0], block[1]
				for i := 0; i < 6; i++ {
					alphaBits |= uint64(block[2+i]) << uint(i*8)
				}
				if alpha[0] > alpha[1] {
					for i := 2; i < 8; i++ {
						alpha[i] = byte(((8-i)*int(alpha[0]) + (i-1)*int(alpha[1])) / 7)
					}
				} else {
					for i := 2; i < 6; i++ {
						alpha[i] = byte(((6-i)*int(alpha[0]) + (i-1)*int(alpha[1])) / 5)
					}
					alpha[6], alpha[7] = 0, 255
				}
			}
			for i := 0; i < 16; i++ {
				x, y := bx+i%4, by+i/4
				if x >= w || y >= h {
					continue
				}
				c := cs[(indices>>uint(2*i))&3]
				if format == "DXT3" {
					c.A = ((block[i/2] >> uint(4*(i%2))) & 15) * 17
				}
				if format == "DXT5" {
					c.A = alpha[(alphaBits>>uint(3*i))&7]
				}
				out.SetNRGBA(x, y, c)
			}
		}
	}
	return out, nil
}
