package npkicon

import (
	"encoding/binary"
	"fmt"
)

// ValidateFrames 使用预览的实际像素解码器检查指定帧，不编码 PNG、不落盘。
func (c *Cache) ValidateFrames(sprite string, palette int, indices []int) error {
	raw, err := c.readSprite(sprite)
	if err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, index := range indices {
		if seen[index] {
			continue
		}
		seen[index] = true
		if _, err := decodeFramePalette(raw, index, palette); err != nil {
			return fmt.Errorf("%s 第 %d 帧：%w", sprite, index, err)
		}
	}
	return nil
}

// FrameCount 读取实际帧表，供外观引用检查使用，不生成 PNG。
func (c *Cache) FrameCount(sprite string) (int, error) {
	raw, err := c.readSprite(sprite)
	if err != nil {
		return 0, err
	}
	frames, _, err := parseFrames(raw)
	return len(frames), err
}

// AuditIMG 使用实际预览解析器核对全帧、指向链、像素长度及全部配色索引。
// 不输出数百万张 PNG，避免全量检查占满磁盘。
func AuditIMG(img []byte) (int, error) {
	frames, base, err := parseFrames(img)
	if err != nil {
		return 0, err
	}
	_, palette, err := indexedPalette(img, 0)
	if err != nil {
		return 0, err
	}
	colors := len(palette) / 4
	if binary.LittleEndian.Uint32(img[24:]) == 6 {
		for i := 1; i < int(binary.LittleEndian.Uint32(img[32:])); i++ {
			_, p, e := indexedPalette(img, i)
			if e != nil {
				return 0, e
			}
			if len(p)/4 < colors {
				colors = len(p) / 4
			}
		}
	}
	textures := map[*texture]bool{}
	for i, f := range frames {
		if f.isLink {
			for hops := 0; f.isLink; hops++ {
				if hops >= 16 || f.link < 0 || f.link >= len(frames) {
					return i, fmt.Errorf("第 %d 帧指向链无效", i)
				}
				f = frames[f.link]
			}
			continue
		}
		if f.blank {
			continue
		}
		if f.texture != nil {
			if !textures[f.texture] {
				if _, err := decodeTextureFrame(img, f); err != nil {
					return i, fmt.Errorf("第 %d 帧：%w", i, err)
				}
				textures[f.texture] = true
			}
			continue
		}
		pixels, err := framePixels(img, base, f)
		if err != nil {
			return i, fmt.Errorf("第 %d 帧：%w", i, err)
		}
		n := int(f.w) * int(f.h)
		if colors > 0 && len(pixels) == n {
			for _, index := range pixels {
				if int(index) >= colors {
					return i, fmt.Errorf("第 %d 帧调色板索引越界", i)
				}
			}
		} else {
			if f.color == colorARGB8888 {
				n *= 4
			} else {
				n *= 2
			}
			if len(pixels) != n {
				return i, fmt.Errorf("第 %d 帧像素长度不符：%d/%d", i, len(pixels), n)
			}
		}
	}
	return len(frames), nil
}
