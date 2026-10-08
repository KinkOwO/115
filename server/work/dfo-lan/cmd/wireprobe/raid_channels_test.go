package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/channelrefresh"
	"os"
	"testing"
)

func TestPublishRaidChannelsFromNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native raid publication")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	source, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	c := channelrefresh.Config{Channels: []channelrefresh.Channel{{ID: 86, Name: "Ispins", Type: 81, Area: "[ispins_legion]", SourceValues: make([]int32, 11)}}}
	if err := publishSourceRaidChannels(&c, &source); err != nil {
		t.Fatal(err)
	}
	seenIDs, seenTypes := map[uint32]bool{}, map[uint32]bool{}
	for _, ch := range c.Channels {
		if seenIDs[ch.ID] {
			t.Fatalf("duplicate directory ID %d", ch.ID)
		}
		seenIDs[ch.ID], seenTypes[ch.Type] = true, true
		t.Logf("directory ID=%d type=%d name=%s", ch.ID, ch.Type, ch.Name)
	}
	count := 0
	for _, typ := range source.Types() {
		attr, _ := source.Attributes(typ)
		if !attr.IsRaid {
			continue
		}
		count++
		if !seenTypes[typ] {
			t.Errorf("source raid type %d absent", typ)
		}
	}
	if count == 0 || !seenTypes[81] || c.Channels[0].ID != 86 {
		t.Fatal("source raids or Ispins lost")
	}
}
