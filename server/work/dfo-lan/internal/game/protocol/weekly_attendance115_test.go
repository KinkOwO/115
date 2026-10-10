package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeeklyNativeRequestShapes115(t *testing.T) {
	for _, test := range []struct {
		hex                              string
		timeCumulative, sync, cumulative bool
		index                            uint32
	}{
		{"8301000001000000", true, true, false, 0},
		{"830100000200000005000000", true, false, false, 5},
		// Captured from the current native curRecv button at 2026-10-08 17:41:20.
		{"83010000020000000100000000000000", false, false, false, 1},
		{"83010000020000001f00000000000000", true, false, false, 31},
		{"8301000002000000cc000000cc000000", true, false, true, 4},
		{"83010000020000006800000068000000", false, false, true, 4},
	} {
		p, e := hex.DecodeString(test.hex)
		require.NoError(t, e)
		r, e := DecodeWeeklyAttendanceRequest115(p, test.timeCumulative)
		require.NoError(t, e)
		require.Equal(t, test.sync, r.Sync)
		require.Equal(t, test.cumulative, r.Cumulative)
		require.Equal(t, test.index, r.Index)
	}
	for _, bad := range []string{
		"830100000100000000000000",         // sync with an invented extra word
		"8301000002000000cc000000",         // truncated cumulative request
		"8301000002000000cc000000cb000000", // mismatched duplicated ID
		"83010000020000000500000005000000", // period with an invented extra word
		"83010000020000000000000000000000", // zero period with transport padding
		"83010000020000002000000000000000", // period outside the native mask
		"83010000020000006500000000000000", // cumulative needs duplicated identity
		"830100000200000000000000",         // index zero is not a period
	} {
		p, e := hex.DecodeString(bad)
		require.NoError(t, e)
		_, e = DecodeWeeklyAttendanceRequest115(p, true)
		require.Error(t, e)
	}
}

func TestWeeklyStatusConsumerOffsets115(t *testing.T) {
	s := WeeklyAttendanceStatus115{EligiblePeriods: 0x22, CurrentPeriod: 5, OnlineSeconds: 3600, ClaimedPeriods: 2, ClaimedCumulative: 0x12, ClaimedTimeCumulative: 2}
	p, e := s.Bytes()
	require.NoError(t, e)
	// Offsets come from 142e133b0, not the padded object layout at +178.
	require.Equal(t, "2200000005100e0000020000001200000002000000", hex.EncodeToString(p))
	s.EligiblePeriods = 1
	_, e = s.Bytes()
	require.Error(t, e)
}
