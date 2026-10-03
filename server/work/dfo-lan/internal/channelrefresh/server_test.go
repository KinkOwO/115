package channelrefresh

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// testChannelAttrs 是测试用的频道属性桩：**真源对照**在 internal/catalog 的
// channel_native_test.go（那里压到真实内层 PVF）。这里只提供最小形状 ——
// 普通频道 1/6/10 的 Type 与 Area 照源写（1 -> type 2 [elven_guard]、
// 6 -> type 3 [none]、10 -> type 0 [granfloris]），其余按"ID = type"约定。
func testChannelAttrs(id uint32) (ChannelAttributes, bool) {
	values := make([]int32, 11)
	switch id {
	case 1:
		return ChannelAttributes{Type: 2, Area: "[elven_guard]", SourceValues: values}, true
	case 6:
		return ChannelAttributes{Type: 3, Area: "[none]", SourceValues: values}, true
	case 10:
		// 源：`10 0 [granfloris] 5 0 …` —— 注意 type 是 0（本地覆盖成 22）。
		values[0] = 5
		return ChannelAttributes{Type: 0, Area: "[granfloris]", SourceValues: values}, true
	case 119, 106, 63, 73, 74, 67, 76, 108, 103, 102, 101, 117, 116:
		return ChannelAttributes{Type: id, Area: "[none]", SourceValues: values}, true
	}
	return ChannelAttributes{}, false
}

// fixedEndpoints advertises the same address for every channel, which is how
// the directory fixtures were captured.
func fixedEndpoints(c Config, host string, port uint16) map[uint32]ChannelEndpoint {
	out := map[uint32]ChannelEndpoint{}
	for _, ch := range c.Channels {
		out[ch.ID] = ChannelEndpoint{ID: ch.ID, Host: host, Port: port}
	}
	return out
}

func TestCurrentNativeDirectoryFixture(t *testing.T) {
	c, e := Load("../../configs/channel.local31.json")
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.Directory(fixedEndpoints(c, "127.0.0.1", 12345))
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.ReadFile("../game/protocol/testdata/native_channel_directory31.json")
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Payload string `json:"payload_hex"`
	}
	if e = json.Unmarshal(f, &r); e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(p) != r.Payload {
		t.Fatal("current reader layout mismatch")
	}
}

func TestDirectoryNumberMatchesSourceChannel(t *testing.T) {
	c, err := Load("../../configs/channel.local31.json")
	if err != nil {
		t.Fatal(err)
	}
	// Native Cain enum0 is mapped to script server1 by144d989a9.
	if c.ServerKey != "first" || c.ServerID != 1 {
		t.Fatal("Cain source-server mapping drifted")
	}
	b, err := os.ReadFile("../game/protocol/testdata/native_channel_numbers31.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string
		ID   uint32 `json:"channel_id"`
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{1, 23} {
		c.Channels[0].ID = id
		p, err := c.Directory(fixedEndpoints(c, "127.0.0.1", 12345))
		if err != nil {
			t.Fatal(err)
		}
		name := string(bytes.TrimRight(p[28:48], "\x00"))
		matched := false
		for _, native := range cases {
			if native.Name == name && native.ID == id {
				matched = true
			}
		}
		if !matched {
			t.Fatal("directory name no longer extracts source channel", name, id)
		}
	}
}

func TestCurrentClientHasEligibleAutoEntryChannel(t *testing.T) {
	c, err := Load("../../configs/channel.local34.json")
	if err != nil {
		t.Fatal(err)
	}
	// 配置文件只写 {ID, Name}；规则层由直读属性补全（真源对照见 internal/catalog）。
	if err := c.Resolve(testChannelAttrs); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../game/protocol/testdata/native_channel_eligibility34.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Kind     uint32
		Eligible bool
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	eligible := map[uint32]bool{}
	for _, r := range rows {
		eligible[r.Kind] = r.Eligible
	}
	count := 0
	for _, ch := range c.Channels {
		if eligible[ch.Type] {
			count++
			if ch.ID != 10 || ch.Area != "[granfloris]" || ch.SourceValues[0] != 5 {
				t.Fatal("source channel identity/rules changed")
			}
		}
	}
	if count != 1 {
		t.Fatal("missing unambiguous current auto-entry channel", count)
	}
	p, err := c.Directory(fixedEndpoints(c, "127.0.0.2", 12345))
	if err != nil || !bytes.Contains(p, []byte("127.0.0.2")) {
		t.Fatal("auto-entry endpoint missing", err)
	}
}
func TestNativeChannelHandshakeAndDeadline(t *testing.T) {
	c, e := Load("../../configs/channel.local28.json")
	if e != nil {
		t.Fatal(e)
	}
	d, _ := c.Directory(fixedEndpoints(c, "127.0.0.1", 12345))
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan error, 1)
	go func() { defer server.Close(); done <- handle(server, c.Script(), d) }()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	send := func(id byte, p []byte) {
		t.Helper()
		b := frame(id, p)
		b[0] = 0
		if _, e = client.Write(b); e != nil {
			t.Fatal(e)
		}
	}
	recv := func(id byte) []byte {
		t.Helper()
		h := make([]byte, 11)
		if _, e = io.ReadFull(client, h); e != nil {
			t.Fatal(e)
		}
		if h[1] != id || h[10] != 1 {
			t.Fatal("wrong channel stage")
		}
		n := binary.LittleEndian.Uint32(h[2:])
		if n < 11 || n > 131083 {
			t.Fatal("bad frame length")
		}
		b := make([]byte, int(n)-11)
		if _, e = io.ReadFull(client, b); e != nil {
			t.Fatal(e)
		}
		return b
	}
	send(11, bytes.Repeat([]byte{'0'}, 32))
	ack := recv(12)
	if len(ack) != 36 {
		t.Fatal("bad key reply")
	}
	decode := func(b []byte) []byte {
		t.Helper()
		z, e := zlib.NewReader(bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		p, e := io.ReadAll(z)
		z.Close()
		if e != nil {
			t.Fatal(e)
		}
		a, _ := aes.NewCipher(ack[4:20])
		for i := 0; i < len(p); i += 16 {
			a.Decrypt(p[i:], p[i:])
		}
		return bytes.TrimRight(p, "\x00")
	}
	send(9, nil)
	if !bytes.Equal(decode(recv(10)), c.Script()) {
		t.Fatal("script did not survive transport")
	}
	send(1, nil)
	if !bytes.Equal(decode(recv(3)), bytes.TrimRight(d, "\x00")) {
		t.Fatal("directory did not survive transport")
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
}

// Every channel has to advertise its own game port. A shared port is exactly
// what made players on different channels see each other.
func TestDirectorySeparatesChannelPorts(t *testing.T) {
	c, err := Load("../../configs/channel.local34.json")
	if err != nil {
		t.Fatal(err)
	}
	// 配置文件只写 {ID, Name}；规则层由直读属性补全（真源对照见 internal/catalog）。
	if err := c.Resolve(testChannelAttrs); err != nil {
		t.Fatal(err)
	}
	endpoints := map[uint32]ChannelEndpoint{}
	for i, ch := range c.Channels {
		endpoints[ch.ID] = ChannelEndpoint{ID: ch.ID, Host: "192.168.1.10", Port: uint16(7002 + i)}
	}
	p, err := c.Directory(endpoints)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	for i := range c.Channels {
		base := 28 + i*48
		if host := string(bytes.TrimRight(p[base+28:base+44], "\x00")); host != "192.168.1.10" {
			t.Fatalf("channel %d advertised host %q", c.Channels[i].ID, host)
		}
		port := binary.LittleEndian.Uint32(p[base+44:])
		if seen[port] {
			t.Fatalf("channel %d reuses game port %d", c.Channels[i].ID, port)
		}
		seen[port] = true
		if port != uint32(7002+i) {
			t.Fatalf("channel %d advertised the wrong port: %d", c.Channels[i].ID, port)
		}
	}
}

// The legion channel is the first directory row with no source counterpart:
// analysis/tasks/next98-apocalypse-entry-evidence.md §16 shows channel_info.etc
// tops out at Type 6, so a special content channel has to be supplied here.
// The row fields follow the official 2026-10-02 channel script capture
// (analysis/tasks/next79-legion-weekly-open.md): the source [none] area made
// the client's legion open-schedule lookup fail with "not open today", so the
// row now names the official [apocalypse] area and the script ships the
// official dungeon block.
func TestLocalDirectoryPublishesLegionChannel(t *testing.T) {
	c, err := Load("../../configs/channel.local34.json")
	if err != nil {
		t.Fatal(err)
	}
	// 配置文件只写 {ID, Name}；规则层由直读属性补全（真源对照见 internal/catalog）。
	if err := c.Resolve(testChannelAttrs); err != nil {
		t.Fatal(err)
	}
	var row *Channel
	for i := range c.Channels {
		if c.Channels[i].ID == 30 {
			row = &c.Channels[i]
		}
	}
	if row == nil {
		t.Fatal("no channel 30 in the local directory")
	}
	// The Type is the clientChannelInfo channelType, which is what maps the
	// channel onto isLegion=1 and town 239. It is outside 22/28/33 on purpose:
	// native144d998b0 grants AUTO-SELECT only to those three, and next31 showed
	// an ineligible type:2 channel still being listed, so being listed and being
	// auto-selected are separate gates.
	if row.Type != 119 || row.Area != "[apocalypse]" {
		t.Fatalf("legion row type=%d area=%q, want 119 / [apocalypse]", row.Type, row.Area)
	}
	script := string(c.Script())
	if !bytes.Contains([]byte(script), []byte("30 `Apocalypse` 119 `[apocalypse]`")) {
		t.Fatalf("script is missing the legion row:\n%s", script)
	}
	if !bytes.Contains([]byte(script), []byte("[dungeon]\n  `[apocalypse]` `Apocalypse`\n\n[/dungeon]")) {
		t.Fatalf("script is missing the official apocalypse dungeon block:\n%s", script)
	}
	if bytes.Contains([]byte(script), []byte("[dungeon]\n`[none]`")) {
		t.Fatal("the [none] area grew a town gate list it does not have")
	}
	// The directory has to advertise a game endpoint for it, and a missing one
	// must fail the whole directory rather than quietly drop the channel.
	endpoints := fixedEndpoints(c, "127.0.0.1", 7001)
	if _, err = c.Directory(endpoints); err != nil {
		t.Fatal(err)
	}
	delete(endpoints, 30)
	if _, err = c.Directory(endpoints); err == nil {
		t.Fatal("directory accepted a channel with no game endpoint")
	}
}

// The Ispins rows reproduce the official 2026-10-02 script capture verbatim
// (86/87 `Ispins` 81 `[ispins_legion]`, all-zero rule scalars, official
// dungeon block with no ids). The former local row (81/[none]) popped the
// client's "not open today" gate even though the official server answered the
// same client on the same day, which pins the schedule lookup to these row
// fields.
func TestLocalDirectoryPublishesIspinsChannel(t *testing.T) {
	c, err := Load("../../configs/channel.local34.json")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[uint32]Channel{}
	for _, ch := range c.Channels {
		if ch.Type == 81 {
			rows[ch.ID] = ch
		}
	}
	if len(rows) != 2 {
		t.Fatalf("want exactly two Ispins channel rows, got %d", len(rows))
	}
	for _, id := range []uint32{86, 87} {
		ch, ok := rows[id]
		if !ok {
			t.Fatalf("missing official Ispins channel row %d", id)
		}
		if ch.Area != "[ispins_legion]" || ch.Name != "Ispins" {
			t.Fatalf("Ispins row %d = %q/%q, want Ispins/[ispins_legion]", id, ch.Name, ch.Area)
		}
	}
	script := string(c.Script())
	if !bytes.Contains([]byte(script), []byte("86 `Ispins` 81 `[ispins_legion]`")) {
		t.Fatalf("script is missing the Ispins row:\n%s", script)
	}
	if !bytes.Contains([]byte(script), []byte("[dungeon]\n  `[ispins_legion]` `Ispins`\n\n[/dungeon]")) {
		t.Fatalf("script is missing the official Ispins dungeon block:\n%s", script)
	}
	// Two rows share the [ispins_legion] area: the block must ship exactly once.
	if got := bytes.Count([]byte(script), []byte("`[ispins_legion]` `Ispins`")); got != 1 {
		t.Fatalf("Ispins dungeon block shipped %d times, want 1", got)
	}
}
