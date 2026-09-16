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

func TestCurrentNativeDirectoryFixture(t *testing.T) {
	c, e := Load("../../configs/channel.local31.json")
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.Directory("127.0.0.1:12345")
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
		p, err := c.Directory("127.0.0.1:12345")
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
	p, err := c.Directory("127.0.0.2:12345")
	if err != nil || !bytes.Contains(p, []byte("127.0.0.2")) {
		t.Fatal("auto-entry endpoint missing", err)
	}
}
func TestNativeChannelHandshakeAndDeadline(t *testing.T) {
	c, e := Load("../../configs/channel.local28.json")
	if e != nil {
		t.Fatal(e)
	}
	d, _ := c.Directory("127.0.0.1:12345")
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
