// Package channelrefresh implements the current client's separate channel
// directory socket. It never creates accounts or changes the game session keys.
package channelrefresh

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// Channel 是本地配置里的一个频道行。**配置文件只写 ID 与 Name** ——
// Type / Area / SourceValues 是内层 PVF 的规则，由 Resolve 从直读结果填入，
// 不再写进 configs（业主 2026-10-02：不必要的 JSON 逐步废弃）。
type Channel struct {
	// ID 同时是发布用的频道号，也按"ID = type"约定关联到 clientchannelinfo 的 [channelType]。
	ID uint32
	// Name 是本地显示名（PVF 里只有图标 iconIndex，没有名字）。
	Name string

	// 以下三项由 Resolve 填充。只有**在内层 PVF 里没有 [channelType] 的本地频道**
	// （目前是 63 Oculus / 73 Purgatory / 74 Revelation：类型来自客户端 exe 144DA9020
	// 的特殊分类，不在 clientchannelinfo.etc）才需要在配置里写出来。
	Type         uint32
	Area         string
	SourceValues []int32
}

// ChannelAttributes 是**直读**来的频道属性。channelrefresh 自己不碰 PVF ——
// 调用方（cmd/wireprobe）从内层归档解析后注入，保持本包对 PVF 无依赖。
type ChannelAttributes struct {
	Type         uint32
	Area         string
	SourceValues []int32
}
type Config struct {
	Listen, ServerKey, SourceSHA256 string
	ServerID                        uint32
	MaxUsers                        uint32
	// 当前客户端目录显式启用，确保登录和角色刷新使用所选频道身份。
	SynchronizeIdentity bool
	Dungeons            map[string][]uint32
	Channels            []Channel
}

func Load(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	h, _, e := net.SplitHostPort(c.Listen)
	// An empty host is the wildcard bind; a concrete host must still be a
	// numeric address. Loopback is no longer required so the directory can be
	// reached by clients on other machines.
	if e != nil || (h != "" && net.ParseIP(h) == nil) || len(c.SourceSHA256) != 64 || len(c.Channels) == 0 || len(c.Channels) > 128 || len(c.ServerKey) > 19 || c.MaxUsers == 0 {
		return c, fmt.Errorf("invalid local channel configuration")
	}
	if c.SynchronizeIdentity && (c.ServerID == 0 || c.ServerID > 255) {
		return c, fmt.Errorf("频道身份同步要求服务器编号在 1～255 之间")
	}
	seen := make(map[uint32]bool, len(c.Channels))
	for _, ch := range c.Channels {
		if ch.ID == 0 || len(ch.Name) == 0 || len(ch.Name) > 18 || strings.ContainsAny(ch.Name, "`\r\n\x00") {
			return c, fmt.Errorf("invalid source channel row")
		}
		if seen[ch.ID] {
			return c, fmt.Errorf("频道编号重复：%d", ch.ID)
		}
		seen[ch.ID] = true
		if c.SynchronizeIdentity && (ch.ID > 255 || ch.Type > 255) {
			return c, fmt.Errorf("频道 %d 的编号或类型超出身份字段范围", ch.ID)
		}
	}
	return c, nil
}

// Resolve 用**直读**来的频道属性补全每个频道的 Type / Area / SourceValues。
//
// 属性必须存在：缺失说明"这个 ID 在内层 PVF 的 clientchannelinfo.etc 里没有对应的
// [channelType]"，直接报错 —— 不静默跳过、也不猜类型。这是业主 2026-10-02 定的口径：
// 配置文件只声明"发布哪些频道、叫什么名"，规则一律以直读为准，写错就在启动期炸掉。
func (c *Config) Resolve(attrs func(id uint32) (ChannelAttributes, bool)) error {
	if attrs == nil {
		return fmt.Errorf("频道属性解析器缺失")
	}
	for i := range c.Channels {
		ch := &c.Channels[i]
		a, ok := attrs(ch.ID)
		if !ok {
			// 源里没有这个 [channelType]：只接受**配置完整给出**的本地频道
			// （63/73/74，类型来自客户端 exe 的特殊分类）。Type 必填 —— 缺它说明
			// 这个 ID 既不是合法 type、也没被本地定义，直接报错。
			if ch.Type == 0 {
				return fmt.Errorf("频道 %d 在内层 PVF 的 clientchannelinfo.etc 里没有对应的 [channelType]（要么按 type 给出 ID，要么在配置里写本地 Type）", ch.ID)
			}
			if ch.Area == "" {
				ch.Area = "[none]"
			}
			if len(ch.SourceValues) == 0 {
				ch.SourceValues = make([]int32, 11)
			}
			continue
		}
		if len(a.SourceValues) != 11 {
			return fmt.Errorf("频道 %d 的属性带 %d 个 SourceValue，源要求 11 个", ch.ID, len(a.SourceValues))
		}
		// Type 已在配置里写明的，视为**本地覆盖**（目前只有 ID 10：源里是 type 0，
		// 客户端不认；本地用 22 让它成为普通区域频道 —— 业主 2026-10-02 受控实验判定）。
		// Area 与 SourceValues 仍然一律以直读为准。
		if ch.Type == 0 {
			ch.Type = a.Type
		}
		ch.Area = a.Area
		ch.SourceValues = a.SourceValues
	}
	return nil
}

func (c Config) Script() []byte {
	var b strings.Builder
	// Local display names are explicit configuration; source area keys/IDs and
	// the eleven channel rule scalars are retained from this client's PVF.
	for _, ch := range c.Channels {
		ids := c.Dungeons[ch.Area]
		if len(ids) == 0 {
			continue
		} // Source [none] entrance has no dungeon block.
		fmt.Fprintf(&b, "[dungeon]\n`%s` `%s`", ch.Area, strings.Trim(ch.Area, "[]"))
		for _, id := range ids {
			fmt.Fprintf(&b, " %d", id)
		}
		b.WriteString("\n[/dungeon]\n")
	}
	fmt.Fprintf(&b, "[server]\n%d\n", c.ServerID)
	for _, ch := range c.Channels {
		fmt.Fprintf(&b, "%d `%s` %d `%s`", ch.ID, ch.Name, ch.Type, ch.Area)
		for _, n := range ch.SourceValues {
			fmt.Fprintf(&b, " %d", n)
		}
		b.WriteString(" ``\n")
	}
	b.WriteString("[/server]\n")
	return []byte(b.String())
}

// ChannelEndpoint is the game endpoint one channel is reachable on.
//
// Separate ports are the only way the gateway can tell channels apart: the
// client picks a channel from this directory and dials the address listed for
// it, but the game connection itself never carries a channel number.
type ChannelEndpoint struct {
	ID   uint32
	Host string
	Port uint16
}

func (c Config) Directory(endpoints map[uint32]ChannelEndpoint) ([]byte, error) {
	for _, ch := range c.Channels {
		ep, ok := endpoints[ch.ID]
		ip := net.ParseIP(ep.Host)
		// The client dials this host directly, so a wildcard or a non-IPv4
		// value would hand it an address it cannot use.
		if !ok || ep.Port == 0 || ip == nil || ip.To4() == nil || ip.IsUnspecified() {
			return nil, fmt.Errorf("invalid advertised game endpoint for channel %d", ch.ID)
		}
	}
	appendFixed := func(b []byte, s string, n int) []byte { out := make([]byte, n); copy(out, s); return append(b, out...) }
	b := binary.LittleEndian.AppendUint32(nil, 1)
	b = appendFixed(b, c.ServerKey, 20)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(c.Channels)))
	for _, ch := range c.Channels {
		ep := endpoints[ch.ID]
		// Current1451fa5e0 extracts decimal digits from this field to obtain
		// the channel ID; the friendly label lives in the script's channel row.
		b = appendFixed(b, fmt.Sprintf("#%d", ch.ID), 20)
		b = binary.LittleEndian.AppendUint32(b, c.MaxUsers)
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = appendFixed(b, ep.Host, 16)
		b = binary.LittleEndian.AppendUint32(b, uint32(ep.Port))
	}
	return b, nil
}

func frame(id byte, body []byte) []byte {
	b := make([]byte, 11)
	b[0] = 1
	b[1] = id
	b[10] = 1
	binary.LittleEndian.PutUint32(b[2:], uint32(11+len(body)))
	return append(b, body...)
}
func encrypted(data, key []byte) ([]byte, error) {
	block, e := aes.NewCipher(key[:16])
	if e != nil {
		return nil, e
	}
	p := make([]byte, (len(data)+15)/16*16)
	copy(p, data)
	for i := 0; i < len(p); i += 16 {
		block.Encrypt(p[i:], p[i:])
	}
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, e = w.Write(p); e != nil {
		return nil, e
	}
	if e = w.Close(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}

func Serve(c Config, endpoints map[uint32]ChannelEndpoint, event func(map[string]any)) (net.Listener, error) {
	directory, e := c.Directory(endpoints)
	if e != nil {
		return nil, e
	}
	l, e := net.Listen("tcp4", c.Listen)
	if e != nil {
		return nil, e
	}
	go func() {
		for {
			conn, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				start := time.Now()
				e := handle(conn, c.Script(), directory)
				event(map[string]any{"kind": "channel_refresh_finished", "success": e == nil, "elapsed_ms": time.Since(start).Milliseconds(), "error": errorText(e)})
			}()
		}
	}()
	return l, nil
}
func errorText(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func handle(conn net.Conn, script, directory []byte) error {
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var seed [16]byte
	if _, e := rand.Read(seed[:]); e != nil {
		return e
	}
	key := []byte(hex.EncodeToString(seed[:]))
	authed := false
	for i := 0; i < 5; i++ {
		h := make([]byte, 11)
		if _, e := io.ReadFull(conn, h); e != nil {
			return e
		}
		n := binary.LittleEndian.Uint32(h[2:])
		if h[0] != 0 || h[10] != 1 || n < 11 || n > 64 {
			return fmt.Errorf("invalid channel request header")
		}
		body := make([]byte, int(n)-11)
		if _, e := io.ReadFull(conn, body); e != nil {
			return e
		}
		var reply []byte
		switch h[1] {
		case 11:
			if authed || len(body) != 32 {
				return fmt.Errorf("invalid channel preface")
			}
			authed = true
			reply = frame(12, append(make([]byte, 4), key...))
		case 9:
			if !authed || len(body) != 0 {
				return fmt.Errorf("invalid channel script request")
			}
			// Client first consumes the key response, then the script response.
			p, e := encrypted(script, key)
			if e != nil {
				return e
			}
			reply = frame(10, p)
		case 1:
			if !authed || len(body) != 0 {
				return fmt.Errorf("invalid directory request")
			}
			p, e := encrypted(directory, key)
			if e != nil {
				return e
			}
			reply = frame(3, p)
		default:
			return fmt.Errorf("unsupported channel request %d", h[1])
		}
		if _, e := io.Copy(conn, bytes.NewReader(reply)); e != nil {
			return e
		}
		if h[1] == 1 {
			return nil
		}
	}
	return fmt.Errorf("channel handshake exceeded bound")
}
