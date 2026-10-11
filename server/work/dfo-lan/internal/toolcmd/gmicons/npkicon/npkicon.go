// Package npkicon 从客户端 ImagePacks2 的 NPK 里按需解出物品图标 PNG。
//
// 格式依据（全部为本机实测校准，不是照抄文档）：
//
//	NPK：16 字节魔数 "NeoplePack_Bill\0" + 4 字节 IMG 数量；
//	     每项固定 264 字节：4 字节偏移 + 4 字节长度 + 256 字节异或加密文件名。
//	     文件名密钥 = "puchikon@neople dungeon and fighter" + 1 个空格 + 用 "DNF" 循环填满 + 结尾 \0，共 256 字节。
//	     （第 36 个字节是空格，这一点是靠"全部条目名必须完全可读"穷举定出来的。）
//
//	IMG：客户端混用 V2/V4/V5/V6；普通图片项 36 字节、指向项 8 字节。
//	     V2 头部 32 字节，V4/V6 带单/多调色板，V5 另带 DDS 图集与 64 字节裁切项。
//	     像素：0x10 ARGB8888、0x0F ARGB4444、0x0E ARGB1555；压缩 0x05 原样、0x06 zlib。
//
// 只按需解单帧，不批量解包，落盘缓存由调用方决定。
package npkicon

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	npkHeaderSize  = 20
	npkEntrySize   = 264
	npkNameSize    = 256
	imgEntrySize   = 36
	imgLinkSize    = 8
	imgMagic       = "Neople Img File"
	colorARGB8888  = 0x10
	colorARGB4444  = 0x0F
	colorARGB1555  = 0x0E
	compressNone   = 0x05
	compressZlib   = 0x06
	linkMarker     = 0x11
	maxFramesLimit = 1 << 20
)

var errSpriteMissing = errors.New("贴图条目不存在")

// decodeKey 返回 NPK 文件名解密用的 256 字节密钥。
func decodeKey() []byte {
	const prefix = "puchikon@neople dungeon and fighter"
	key := make([]byte, npkNameSize)
	copy(key, prefix)
	key[len(prefix)] = ' '
	for i := len(prefix) + 1; i < npkNameSize-1; i++ {
		key[i] = "DNF"[(i-len(prefix)-1)%3]
	}
	key[npkNameSize-1] = 0
	return key
}

// Entry 是 NPK 里的一项。
type Entry struct {
	Name   string
	Offset int64
	Size   int64
}

// Archive 是一个已解析索引、可随机读取的 NPK。
type Archive struct {
	path    string
	entries map[string]Entry // 键为小写、正斜杠规范化后的名字
}

// OpenArchive 解析 NPK 索引（只读索引，不读图像数据）。
func OpenArchive(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	head := make([]byte, npkHeaderSize)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(head, []byte("NeoplePack_Bill")) {
		return nil, fmt.Errorf("%s 不是 NPK 文件", filepath.Base(path))
	}
	count := int(binary.LittleEndian.Uint32(head[16:20]))
	if count <= 0 || count > 1<<20 {
		return nil, fmt.Errorf("%s 条目数异常：%d", filepath.Base(path), count)
	}
	table := make([]byte, count*npkEntrySize)
	if _, err := io.ReadFull(f, table); err != nil {
		return nil, err
	}
	key := decodeKey()
	entries := make(map[string]Entry, count)
	for i := 0; i < count; i++ {
		off := i * npkEntrySize
		raw := table[off+8 : off+npkEntrySize]
		name := make([]byte, npkNameSize)
		for j := 0; j < npkNameSize; j++ {
			name[j] = raw[j] ^ key[j]
		}
		if z := bytes.IndexByte(name, 0); z >= 0 {
			name = name[:z]
		}
		entries[Normalize(string(name))] = Entry{
			Name:   string(name),
			Offset: int64(binary.LittleEndian.Uint32(table[off:])),
			Size:   int64(binary.LittleEndian.Uint32(table[off+4:])),
		}
	}
	return &Archive{path: path, entries: entries}, nil
}

// Count 返回条目数。
func (a *Archive) Count() int { return len(a.entries) }

// Names 返回归档内全部条目名（诊断/取素材用；地图迭代顺序随机，调用方自行排序）。
func (a *Archive) Names() []string {
	out := make([]string, 0, len(a.entries))
	for name := range a.entries {
		out = append(out, name)
	}
	return out
}

// Has 判断规范化后的名字是否存在。
func (a *Archive) Has(name string) bool {
	_, ok := a.entries[Normalize(name)]
	return ok
}

// Read 读出某项原始字节。
func (a *Archive) Read(name string) ([]byte, bool) {
	e, ok := a.entries[Normalize(name)]
	if !ok {
		return nil, false
	}
	f, err := os.Open(a.path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	buf := make([]byte, e.Size)
	if _, err := f.ReadAt(buf, e.Offset); err != nil {
		return nil, false
	}
	return buf, true
}

// Normalize 统一路径写法，便于比较。
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimPrefix(s, "/")
	return strings.ToLower(s)
}

// LocateNPK 由图标资源路径推出候选 NPK 文件（返回第一个存在的）。
//
// 例：Item/stackable/consumption_2.img → sprite_item_stackable.NPK
func LocateNPK(imagePacksDir, iconPath string) (string, bool) {
	p := Normalize(iconPath)
	noExt := strings.TrimSuffix(p, filepath.Ext(p))
	dir := strings.TrimSuffix(filepath.ToSlash(filepath.Dir(p)), "/")
	dirFlat := strings.ReplaceAll(dir, "/", "_")
	allFlat := strings.ReplaceAll(noExt, "/", "_")
	cands := []string{
		"sprite_" + dirFlat + ".npk",
		"sprite_" + allFlat + ".npk",
		dirFlat + ".npk",
		allFlat + ".npk",
	}
	for _, c := range cands {
		if c == ".npk" {
			continue
		}
		full := filepath.Join(imagePacksDir, c)
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			return full, true
		}
	}
	return "", false
}

// frame 是一帧的描述（图片项或指向项）。
type frame struct {
	isLink  bool
	link    int
	color   uint32
	comp    uint32
	w, h    uint32
	x, y    int
	length  uint32
	off     int
	blank   bool
	texture *texture
	crop    image.Rectangle
	rotated bool
}

// parseFrames 解析 IMG 索引表；返回帧列表与数据区起点。
func parseFrames(img []byte) ([]frame, int, error) {
	if len(img) < 32 || !bytes.HasPrefix(img, []byte(imgMagic)) {
		return nil, 0, fmt.Errorf("不是 IMG 文件")
	}
	idxLen := int(binary.LittleEndian.Uint32(img[16:20]))
	count := int(binary.LittleEndian.Uint32(img[28:32]))
	if count <= 0 || count > maxFramesLimit {
		return nil, 0, fmt.Errorf("IMG 帧数异常：%d", count)
	}
	version := binary.LittleEndian.Uint32(img[24:28])
	if version == 5 {
		return parseTextureFrames(img, idxLen, count)
	}
	indexStart := 32
	if version == 4 || version == 6 {
		var err error
		indexStart, _, err = indexedPalette(img, 0)
		if err != nil {
			return nil, 0, err
		}
	} else if version != 2 {
		return nil, 0, fmt.Errorf("尚未支持 IMG 版本 %d", version)
	}
	dataBase := indexStart + idxLen
	if dataBase < indexStart || dataBase > len(img) {
		return nil, 0, fmt.Errorf("IMG 数据区起点越界：%d", dataBase)
	}

	frames := make([]frame, 0, count)
	pos := 0
	o := indexStart
	for i := 0; i < count; i++ {
		if o+imgLinkSize > dataBase {
			return nil, 0, fmt.Errorf("第 %d 帧索引不完整", i)
		}
		k0 := binary.LittleEndian.Uint32(img[o:])
		k1 := binary.LittleEndian.Uint32(img[o+4:])
		isImage := k0 == colorARGB8888 || k0 == colorARGB4444 || k0 == colorARGB1555
		// 指向项在两种版本里都只占 8 字节，只是字段摆放相反：
		//   V2：类型 0x11 在 +0、目标帧号在 +4（实测 11 00 00 00 18 02 00 00）；
		//   V5：目标帧号在 +0、类型 0x11 在 +4（实测 8C 00 00 00 11 00 00 00）。
		// 校验：某张 V2 表 idxLen=118120、count=3285，36x+8y=118120 且 x+y=3285 恰好 y=5。
		if !isImage && (k0 == linkMarker || k1 == linkMarker) {
			target := int(k1)
			if k0 != linkMarker {
				target = int(k0)
			}
			frames = append(frames, frame{isLink: true, link: target})
			o += imgLinkSize
			continue
		}
		if !isImage {
			return nil, 0, fmt.Errorf("第 %d 项类型未知：0x%02X", i, k0)
		}
		if o+imgEntrySize > dataBase {
			return nil, 0, fmt.Errorf("第 %d 帧索引不完整", i)
		}
		f := frame{
			color:  k0,
			comp:   k1,
			w:      binary.LittleEndian.Uint32(img[o+8:]),
			h:      binary.LittleEndian.Uint32(img[o+12:]),
			length: binary.LittleEndian.Uint32(img[o+16:]),
			off:    pos,
			x:      int(int32(binary.LittleEndian.Uint32(img[o+20:]))),
			y:      int(int32(binary.LittleEndian.Uint32(img[o+24:]))),
		}
		if f.w == 0 || f.h == 0 || f.w > 4096 || f.h > 4096 {
			return nil, 0, fmt.Errorf("第 %d 项尺寸异常：%dx%d", i, f.w, f.h)
		}
		// 源包以画布右边界外的 1×1 帧表示空白；其四字节内容不一定是完整 zlib。
		f.blank = f.w == 1 && f.h == 1 && f.y == 0 && f.x == int(binary.LittleEndian.Uint32(img[o+28:]))
		// V2 未压缩帧的 length 会残留为 4；物理像素按实际色深占用 2 或 4 字节。
		if f.comp == compressNone && version == 2 {
			f.length = f.w * f.h
			if f.color == colorARGB8888 {
				f.length *= 4
			} else {
				f.length *= 2
			}
		}
		if int(f.length) > len(img)-dataBase-pos {
			return nil, 0, fmt.Errorf("第 %d 帧数据不完整", i)
		}
		pos += int(f.length)
		o += imgEntrySize
		frames = append(frames, f)
	}
	if o != dataBase || pos != len(img)-dataBase {
		return nil, 0, fmt.Errorf("IMG 索引或数据长度不符")
	}
	return frames, dataBase, nil
}

// indexedPalette 读取 V4 单调色板和 V6 多调色板，返回索引表起点及指定 RGBA 色表。
// V4 的色数位于 +32；V6 的 +32 是色表数量，每张色表另带一个色数。
func indexedPalette(img []byte, selected int) (int, []byte, error) {
	if len(img) < 36 {
		return 0, nil, fmt.Errorf("IMG 调色板头部不完整")
	}
	version := binary.LittleEndian.Uint32(img[24:28])
	if version == 5 {
		if len(img) < 44 {
			return 0, nil, fmt.Errorf("V5 色表头部不完整")
		}
		colors := int(binary.LittleEndian.Uint32(img[40:]))
		if selected != 0 || colors > 256 || 44+colors*4 > len(img) {
			return 0, nil, fmt.Errorf("V5 调色板越界")
		}
		return 44 + colors*4, img[44 : 44+colors*4], nil
	}
	if version != 4 && version != 6 {
		return 0, nil, nil
	}
	count := 1
	pos := 32
	if version == 6 {
		count = int(binary.LittleEndian.Uint32(img[32:36]))
		pos = 36
	}
	if count < 1 || count > 4096 {
		return 0, nil, fmt.Errorf("IMG 调色板数量异常：%d", count)
	}
	if selected < 0 || selected >= count {
		return 0, nil, fmt.Errorf("调色板编号 %d 越界（共 %d 套）", selected, count)
	}
	var palette []byte
	for i := 0; i < count; i++ {
		if pos+4 > len(img) {
			return 0, nil, fmt.Errorf("IMG 调色板越界")
		}
		colors := int(binary.LittleEndian.Uint32(img[pos:]))
		pos += 4
		if colors < 0 || colors > 256 || colors > (len(img)-pos)/4 {
			return 0, nil, fmt.Errorf("IMG 调色板色数异常：%d", colors)
		}
		if i == selected {
			palette = img[pos : pos+colors*4]
		}
		pos += colors * 4
	}
	return pos, palette, nil
}

// DecodeFrame 解出 IMG 里第 frame 帧的像素（自动跟随指向项）。
func DecodeFrame(img []byte, want int) (image.Image, error) {
	return decodeFramePalette(img, want, 0)
}

func decodeFramePalette(img []byte, want, palette int) (image.Image, error) {
	frames, dataBase, err := parseFrames(img)
	if err != nil {
		return nil, err
	}
	if want < 0 || want >= len(frames) {
		return nil, fmt.Errorf("帧号 %d 越界（共 %d 帧）", want, len(frames))
	}
	f := frames[want]
	for i := 0; f.isLink && i < 16; i++ {
		if f.link < 0 || f.link >= len(frames) {
			return nil, fmt.Errorf("指向项目标越界：%d", f.link)
		}
		f = frames[f.link]
	}
	if f.isLink {
		return nil, fmt.Errorf("指向链条过深")
	}
	if f.blank {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil
	}
	if f.texture != nil {
		return decodeTextureFrame(img, f)
	}
	pixels, err := framePixels(img, dataBase, f)
	if err != nil {
		return nil, err
	}
	return renderPixels(img, palette, f, pixels)
}

func framePixels(img []byte, dataBase int, f frame) ([]byte, error) {
	start := dataBase + f.off
	if start < 0 || start+int(f.length) > len(img) {
		return nil, fmt.Errorf("帧数据越界")
	}
	payload := img[start : start+int(f.length)]

	var pixels []byte
	switch f.comp {
	case compressZlib:
		zr, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("zlib 头无效：%w", err)
		}
		defer zr.Close()
		pixels, err = io.ReadAll(io.LimitReader(zr, int64(f.w)*int64(f.h)*4+1))
		if err != nil {
			return nil, fmt.Errorf("zlib 解压失败：%w", err)
		}
	case compressNone:
		pixels = payload
	default:
		return nil, fmt.Errorf("压缩方式未知：0x%02X", f.comp)
	}
	return pixels, nil
}

func renderPixels(img []byte, palette int, f frame, pixels []byte) (image.Image, error) {
	out := image.NewNRGBA(image.Rect(0, 0, int(f.w), int(f.h)))

	// 索引色使用 IMG 自带的 RGBA 色表；按 BGRA 读取会把肤色变成蓝色。
	if len(pixels) == int(f.w)*int(f.h) {
		_, pal, err := indexedPalette(img, palette)
		if err != nil {
			return nil, err
		}
		if len(pal) > 0 {
			for y := 0; y < int(f.h); y++ {
				for x := 0; x < int(f.w); x++ {
					idx := int(pixels[y*int(f.w)+x])
					if idx*4+3 >= len(pal) {
						return nil, fmt.Errorf("像素索引 %d 超出调色板", idx)
					}
					out.SetNRGBA(x, y, color.NRGBA{
						R: pal[idx*4],
						G: pal[idx*4+1],
						B: pal[idx*4+2],
						A: pal[idx*4+3],
					})
				}
			}
			return out, nil
		}
	}

	switch f.color {
	case colorARGB8888:
		need := int(f.w) * int(f.h) * 4
		if len(pixels) < need {
			return nil, fmt.Errorf("ARGB8888 像素不足：%d < %d", len(pixels), need)
		}
		for y := 0; y < int(f.h); y++ {
			for x := 0; x < int(f.w); x++ {
				i := (y*int(f.w) + x) * 4
				out.SetNRGBA(x, y, color.NRGBA{R: pixels[i+2], G: pixels[i+1], B: pixels[i], A: pixels[i+3]})
			}
		}
	case colorARGB4444, colorARGB1555:
		need := int(f.w) * int(f.h) * 2
		if len(pixels) < need {
			return nil, fmt.Errorf("16 位像素不足：%d < %d", len(pixels), need)
		}
		for y := 0; y < int(f.h); y++ {
			for x := 0; x < int(f.w); x++ {
				i := (y*int(f.w) + x) * 2
				// 小端 16 位：背景字节 `42 88` 读作 0x8842（最高位=1 → 不透明）。
				// 实测 DFO 的物品图标本身就是"满幅不透明"的图块，不是透明底，
				// 所以判据是"多数像素不透明"，而不是"背景透明"。
				v := binary.LittleEndian.Uint16(pixels[i:])
				var c color.NRGBA
				if f.color == colorARGB4444 {
					c = color.NRGBA{
						R: uint8((v>>8)&0xF0 | (v>>12)&0x0F),
						G: uint8((v>>4)&0xF0 | (v>>8)&0x0F),
						B: uint8(v&0xF0 | (v>>4)&0x0F),
						A: uint8((v & 0x0F) * 17),
					}
				} else {
					a := uint8(0)
					if v&0x8000 != 0 {
						a = 255
					}
					// 5 位色扩展到 8 位要左移 3 位并补低位，不能写成 (v>>10)&0xF8。
					r5 := (v >> 10) & 0x1F
					g5 := (v >> 5) & 0x1F
					b5 := v & 0x1F
					c = color.NRGBA{
						R: uint8(r5<<3 | r5>>2),
						G: uint8(g5<<3 | g5>>2),
						B: uint8(b5<<3 | b5>>2),
						A: a,
					}
				}
				out.SetNRGBA(x, y, c)
			}
		}
	default:
		return nil, fmt.Errorf("色彩系统未知：0x%02X", f.color)
	}
	return out, nil
}

// Cache 按目录缓存已解析的 NPK 索引，供反复取图使用。
type Cache struct {
	dir string

	mu        sync.Mutex
	archives  map[string]*Archive
	used      map[string]uint64
	clock     uint64
	locations map[string]string
	indexOnce sync.Once
	index     []spriteLocation
	packages  []string
	indexErr  error
}

// 索引只保留路径指纹和包编号，不常驻全部条目名或贴图数据。
type spriteLocation struct {
	hash uint64
	pack uint32
}

func spritePathHash(name string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.TrimPrefix(Normalize(name), "sprite/")))
	return h.Sum64()
}

// 仅在常规定位失败时建立跨包索引；每个客户端缓存实例最多扫描一次。
func (c *Cache) indexSprites() {
	files, err := os.ReadDir(c.dir)
	if err != nil {
		c.indexErr = err
		return
	}
	for _, file := range files {
		if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".npk") {
			continue
		}
		path := filepath.Join(c.dir, file.Name())
		a, err := OpenArchive(path)
		if err != nil {
			if c.indexErr == nil {
				c.indexErr = fmt.Errorf("读取资源包索引 %s：%w", file.Name(), err)
			}
			continue
		}
		pack := uint32(len(c.packages))
		c.packages = append(c.packages, path)
		for name := range a.entries {
			c.index = append(c.index, spriteLocation{hash: spritePathHash(name), pack: pack})
		}
	}
	sort.Slice(c.index, func(i, j int) bool {
		if c.index[i].hash == c.index[j].hash {
			return c.index[i].pack < c.index[j].pack
		}
		return c.index[i].hash < c.index[j].hash
	})
}

// NewCache 为某个 ImagePacks2 目录建缓存。
func NewCache(imagePacksDir string) *Cache {
	return &Cache{dir: imagePacksDir, archives: make(map[string]*Archive), used: make(map[string]uint64)}
}

// Dir 返回所属 ImagePacks2 目录。
func (c *Cache) Dir() string { return c.dir }

func (c *Cache) archive(path string) (*Archive, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clock++
	if a, ok := c.archives[path]; ok {
		c.used[path] = c.clock
		return a, nil
	}
	a, err := OpenArchive(path)
	if err != nil {
		return nil, err
	}
	// 只缓存最近使用的资源包索引，切换职业不会永久累积所有 NPK。
	if len(c.archives) >= 24 {
		oldest := ""
		for key := range c.archives {
			if oldest == "" || c.used[key] < c.used[oldest] {
				oldest = key
			}
		}
		delete(c.archives, oldest)
		delete(c.used, oldest)
	}
	c.archives[path] = a
	c.used[path] = c.clock
	return a, nil
}

// PNG 解出图标并编码为 PNG 字节。
func (c *Cache) PNG(iconPath string, frameIndex int) ([]byte, error) {
	b, _, _, err := c.PNGWithOffset(iconPath, frameIndex)
	return b, err
}

// PNGWithOffset 同时返回帧在原始画布中的裁切坐标，供角色图层对齐。
func (c *Cache) PNGWithOffset(iconPath string, frameIndex int) ([]byte, int, int, error) {
	return c.PNGWithPalette(iconPath, frameIndex, 0)
}

// readSprite 只区分条目不存在与读取失败，损坏资源不能被当成配色变体回退。
func (c *Cache) readSprite(iconPath string) ([]byte, error) {
	key := Normalize(iconPath)
	c.mu.Lock()
	known := c.locations[key]
	c.mu.Unlock()
	preferred, _ := LocateNPK(c.dir, iconPath)
	tried := map[string]bool{}
	read := func(path string) ([]byte, bool, error) {
		if path == "" || tried[path] {
			return nil, false, nil
		}
		tried[path] = true
		raw, found, err := c.readArchiveSprite(path, iconPath)
		if found && err == nil {
			c.mu.Lock()
			if c.locations == nil || len(c.locations) >= 2048 {
				c.locations = map[string]string{}
			}
			c.locations[key] = path
			c.mu.Unlock()
		}
		return raw, found, err
	}
	for _, path := range []string{known, preferred} {
		if raw, found, err := read(path); found || err != nil {
			return raw, err
		}
	}
	// 包名不等于 IMG 所在目录：桐人、女格斗等整身皮肤实际放在同职业 coat 包。
	// 只补查同一职业、同一资源族的时装包，并核对完整 IMG 路径，不能串用其它职业。
	if parent, _, avatar := strings.Cut(strings.TrimPrefix(key, "sprite/"), "/avatar/"); avatar {
		prefix := "sprite_" + strings.ReplaceAll(parent, "/", "_") + "_avatar_"
		paths, err := filepath.Glob(filepath.Join(c.dir, prefix+"*"))
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			if !strings.EqualFold(filepath.Ext(path), ".npk") {
				continue
			}
			if raw, found, err := read(path); found || err != nil {
				return raw, err
			}
		}
	}
	// 特效可能打包到其它职业的 NPK，包名只能作为快速查找线索。
	// 指纹仅筛选候选包，read 仍核对完整 IMG 路径，哈希碰撞不会串图。
	c.indexOnce.Do(c.indexSprites)
	hash := spritePathHash(iconPath)
	start := sort.Search(len(c.index), func(i int) bool { return c.index[i].hash >= hash })
	for i := start; i < len(c.index) && c.index[i].hash == hash; i++ {
		if raw, found, err := read(c.packages[c.index[i].pack]); found || err != nil {
			return raw, err
		}
	}
	if c.indexErr != nil {
		return nil, fmt.Errorf("无法完整核对贴图 %s：%w", iconPath, c.indexErr)
	}
	return nil, fmt.Errorf("客户端资源包里没有 %s：%w", iconPath, errSpriteMissing)
}

// 同名资源优先，其次才是已有的 (tn) 变体；读取失败与条目缺失保持区分。
func (c *Cache) readArchiveSprite(npkPath, iconPath string) ([]byte, bool, error) {
	a, err := c.archive(npkPath)
	if err != nil {
		return nil, false, err
	}
	name := iconPath
	if !a.Has(name) {
		name = "sprite/" + Normalize(iconPath)
	}
	if !a.Has(name) && strings.Contains(Normalize(iconPath), "/avatar/") {
		// 部分时装仅保存带 (tn) 前缀的同名资源；普通条目存在时始终优先。
		if slash := strings.LastIndex(name, "/"); slash >= 0 {
			alternate := name[:slash+1] + "(tn)" + name[slash+1:]
			if a.Has(alternate) {
				name = alternate
			}
		}
	}
	if !a.Has(name) {
		return nil, false, nil
	}
	raw, ok := a.Read(name)
	if !ok {
		return nil, true, fmt.Errorf("读取贴图失败：%s", iconPath)
	}
	return raw, true, nil
}

// ResolveVariant 优先保留独立配色文件；缺少该文件时，按 V6 内置调色板解析配色。
// 例如 gg_hair0101d 不独立存放，而是 gg_hair0100d 的第 1 号调色板。
func (c *Cache) ResolveVariant(sprite, paletteSprite string, palette int) (string, int, error) {
	_, err := c.readSprite(sprite)
	if err == nil {
		return sprite, 0, nil
	}
	if !errors.Is(err, errSpriteMissing) || paletteSprite == "" || palette <= 0 {
		return "", 0, err
	}
	raw, readErr := c.readSprite(paletteSprite)
	if readErr != nil {
		return "", 0, readErr
	}
	if len(raw) < 36 || binary.LittleEndian.Uint32(raw[24:28]) != 6 {
		return "", 0, fmt.Errorf("%s 不含 V6 配色变体：%w", paletteSprite, err)
	}
	if _, _, paletteErr := indexedPalette(raw, palette); paletteErr != nil {
		return "", 0, fmt.Errorf("%s 缺少独立贴图，备用配色不可用（%v）：%w", sprite, paletteErr, err)
	}
	return paletteSprite, palette, nil
}

// PNGWithPalette 使用指定调色板解码，同时保留图层裁切坐标。
func (c *Cache) PNGWithPalette(iconPath string, frameIndex, palette int) ([]byte, int, int, error) {
	raw, err := c.readSprite(iconPath)
	if err != nil {
		return nil, 0, 0, err
	}
	im, err := decodeFramePalette(raw, frameIndex, palette)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%s（第 %d 帧，调色板 %d）：%w", iconPath, frameIndex, palette, err)
	}
	frames, _, err := parseFrames(raw)
	if err != nil {
		return nil, 0, 0, err
	}
	f := frames[frameIndex]
	for f.isLink {
		f = frames[f.link] // DecodeFrame 已校验指向链与帧号。
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), f.x, f.y, nil
}
