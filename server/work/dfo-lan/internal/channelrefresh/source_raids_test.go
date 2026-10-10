package channelrefresh

import (
	"bytes"
	"reflect"
	"testing"
)

func raidAttrs(channelType uint32) (ChannelAttributes, bool) {
	return ChannelAttributes{Type: channelType, Area: "[none]", SourceValues: make([]int32, 11)}, true
}

func TestSourceRaidsPreserveIspinsCollisionAndDirectory(t *testing.T) {
	c := Config{ServerID: 1, Channels: []Channel{{ID: 86, Name: "Ispins", Type: 81, Area: "[ispins_legion]", SourceValues: make([]int32, 11)}}}
	before := c.Channels[0]
	if err := c.PublishSourceRaids([]uint32{98, 86, 82, 85, 93, 107, 111, 82}, raidAttrs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Channels[0], before) || len(c.Channels) != 8 {
		t.Fatalf("existing channel changed or source types duplicated: %+v", c.Channels)
	}
	var colliding Channel
	for _, ch := range c.Channels {
		if ch.Type == 86 {
			colliding = ch
		}
	}
	if colliding.ID == 86 || colliding.Type != 86 {
		t.Fatalf("raid reused Ispins ID: %+v", colliding)
	}
	if !bytes.Contains(c.Script(), []byte("255 `Raid 86` 86 `[none]`")) {
		t.Fatal("script lost distinct raid directory ID/type")
	}
	if _, err := c.Directory(fixedEndpoints(c, "127.0.0.2", 12345)); err != nil {
		t.Fatal(err)
	}
	snapshot := append([]Channel(nil), c.Channels...)
	if err := c.PublishSourceRaids([]uint32{82, 85, 86, 93, 98, 107, 111}, raidAttrs); err != nil || !reflect.DeepEqual(c.Channels, snapshot) {
		t.Fatalf("repeated source preparation changed channels: %v", err)
	}
}

func TestSourceRaidPublicationFailsWithoutPartialMutation(t *testing.T) {
	c := Config{Channels: []Channel{{ID: 1, Name: "Local", Type: 2}}}
	before := append([]Channel(nil), c.Channels...)
	err := c.PublishSourceRaids([]uint32{82, 93}, func(typ uint32) (ChannelAttributes, bool) {
		if typ == 93 {
			return ChannelAttributes{}, false
		}
		return raidAttrs(typ)
	})
	if err == nil || !reflect.DeepEqual(c.Channels, before) {
		t.Fatalf("invalid source caused partial publication: %v %+v", err, c.Channels)
	}
}
