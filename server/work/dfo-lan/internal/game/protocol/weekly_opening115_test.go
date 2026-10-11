package protocol

import (
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWeeklyOpeningUsesNativePresenceAndRejectsBrokenSnapshots(t *testing.T) {
	p, err := AppendWeeklyOpening115([]byte{0, 0, 0}, "W", 1, 2)
	require.NoError(t, err)
	require.Equal(t, "0100830101000001000000570000000000000000010000000200000000000000000000000000", hex.EncodeToString(p))
	_, err = AppendWeeklyOpening115(p, "W", 1, 2)
	require.ErrorContains(t, err, "duplicate")
	for i := 0; i < len(p); i++ {
		_, err = AppendWeeklyOpening115(p[:i], "W", 1, 2)
		require.Error(t, err, "prefix=%d", i)
	}
	// An absent row is a legal input. It remains untouched by the append.
	absent := []byte{1, 0, 99, 0, 0, 0}
	merged, err := AppendWeeklyOpening115(absent, "W", 1, 2)
	require.NoError(t, err)
	require.Equal(t, absent[2:5], merged[2:5])
	for _, bad := range [][]byte{{0, 0, 1, 0}, {1, 0, 2, 0, 1, 0, 0, 255, 255, 255, 255}} {
		_, err = AppendWeeklyOpening115(bad, "W", 1, 2)
		require.Error(t, err)
	}
	require.Equal(t, "00170083010000", hex.EncodeToString(WeeklyAttendanceRefusal115(23)))
}
