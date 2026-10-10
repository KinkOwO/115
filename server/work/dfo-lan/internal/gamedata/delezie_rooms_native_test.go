package gamedata

import (
	"os"
	"testing"
)

func TestDelezieRecruitingChannelsNativePVF(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("native PVF archive required")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var c Catalogs
	if err := preparePVFChannels(&c, s); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []uint32{112, 120} {
		entry, bound := c.RaidEntrances[typ]
		town, published := c.ChannelTowns[typ]
		if !bound || !published || entry.Recruiting == nil || town.TownID != entry.Recruiting.TownID || town.AreaID != entry.Recruiting.AreaID || town.MapSHA256 != entry.Recruiting.MapSHA256 || town.Source.Checksum != entry.Recruiting.Source.Checksum || town.AreaID == entry.Waiting.AreaID {
			t.Fatalf("channel %d did not publish native preparation room: entry=%+v town=%+v", typ, entry, town)
		}
		t.Logf("channel %d preparation=%d/%d source=%s waiting=%d/%d", typ, town.TownID, town.AreaID, town.MapPath, entry.Waiting.TownID, entry.Waiting.AreaID)
	}
}
