package gamedata

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHoneyNativeCurrentUSIndexedSource(t *testing.T) {
	archive := os.Getenv("US115_TEST_HONEY_NATIVE_PVF")
	if archive == "" {
		t.Skip("set US115_TEST_HONEY_NATIVE_PVF to the current US inner archive")
	}
	s, e := Open(Options{Mode: PVF, ArchivePath: archive, DerivedCacheDir: "-"})
	require.NoError(t, e)
	defer s.Close()
	c, e := s.HoneyNative()
	require.NoError(t, e)
	require.Equal(t, "live/event/kor/2023/1026_honeytime/honeytime.evt", c.Path)
	require.Equal(t, s.Snapshot().Checksum, c.SourceChecksum)
	require.Len(t, c.Groups, 2)
	require.Len(t, c.Groups["daily"].Blocks, 1)
	require.Len(t, c.Groups["daily"].Blocks[0].Rewards, 4)
	require.Len(t, c.Groups["daily"].Unlimit, 4)
	require.EqualValues(t, 7200, c.Groups["daily"].Blocks[0].CumulativeSeconds)
	require.EqualValues(t, 288000, c.Groups["period"].Blocks[0].CumulativeSeconds)
	t.Logf("source=%s path=%s groups=%d parser-only; no gateway enabled", c.SourceChecksum, c.Path, len(c.Groups))
}
