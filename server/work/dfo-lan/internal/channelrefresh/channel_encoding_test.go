package channelrefresh

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestChannelWireScriptNativeCP936(t *testing.T) {
	c := Config{ServerID: 1, Channels: []Channel{
		{ID: 10, Name: "普通频道", Type: 22, Area: "[granfloris]"},
		{ID: 86, Name: "伊斯大陆", Type: 81, Area: "[ispins_legion]"},
	}, DungeonTitles: map[string]string{"[ispins_legion]": "伊斯大陆"}}
	wire, err := c.WireScript()
	if err != nil {
		t.Fatal(err)
	}
	// Independently captured CP936 bytes, not UTF-8 JSON or a second Script call.
	for _, h := range []string{"c6d5cda8c6b5b5c0", "d2c1cbb9b4f3c2bd"} {
		b, err := hex.DecodeString(h)
		if err != nil || !bytes.Contains(wire, b) {
			t.Fatalf("missing native text bytes %s", h)
		}
	}
	if bytes.Contains(wire, []byte("普通频道")) {
		t.Fatal("UTF-8 leaked into native script")
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(wire)
	if err != nil || !bytes.Equal(decoded, c.Script()) {
		t.Fatal("native decoding changed text, IDs, areas or scalars")
	}
	if !strings.Contains(c.Channels[0].Name, "普通") {
		t.Fatal("configuration was mutated")
	}
}

func TestChannelWireScriptASCIIAndUnsupportedText(t *testing.T) {
	c := Config{ServerID: 1, Channels: []Channel{{ID: 10, Name: "LAN Normal", Type: 22, Area: "[granfloris]"}}}
	wire, err := c.WireScript()
	if err != nil || !bytes.Equal(wire, c.Script()) {
		t.Fatal("ASCII directory changed")
	}
	c.Channels[0].Name = "频道😀"
	if _, err := c.WireScript(); err == nil {
		t.Fatal("unrepresentable name silently replaced")
	}
}

func TestCurrentChineseChannelConfigNativeRoundTrip(t *testing.T) {
	c, err := Load("../../configs/channel.local35.json")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := c.WireScript()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(wire)
	if err != nil || !bytes.Equal(decoded, c.Script()) {
		t.Fatal("current channel text did not survive native decoding")
	}
}
