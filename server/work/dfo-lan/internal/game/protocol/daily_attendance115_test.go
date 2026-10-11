package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyAttendance115NativeReaderLayout(t *testing.T) {
	// Literal native read order, with distinct bytes for all four tail words.
	s := DailyAttendanceStatus115{CurrentDay: 1, Field281: 2,
		Counts:   []uint32{0x07060504, 0x0b0a0908, 0x0f0e0d0c},
		Field2A0: 0x13121110, Field2A4: 0x17161514, Claimed: 0x1b1a1918,
		Cumulative: 0x1f1e1d1c, Field2B0: 0x20}
	p, err := s.Bytes()
	require.NoError(t, err)
	require.Equal(t, "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", hex.EncodeToString(p))
	for _, n := range []int{0, 1, 32, 255} {
		s.Counts = make([]uint32, n)
		p, err = s.Bytes()
		require.NoError(t, err)
		require.Len(t, p, 20+4*n)
		require.Equal(t, byte(n), p[2])
	}
	s.Counts = make([]uint32, 256)
	_, err = s.Bytes()
	require.Error(t, err)
}

func TestDailyAttendance115NativeRequestsRejectAlternativeShapes(t *testing.T) {
	for _, tc := range []struct {
		wire string
		want DailyAttendanceRequest115
	}{
		{"6d0100000100000001000000", DailyAttendanceRequest115{Index: 1}},
		{"6d0100000100000020000000", DailyAttendanceRequest115{Index: 32}},
		{"6d0100000200000005000000", DailyAttendanceRequest115{Cumulative: true, Index: 5}},
	} {
		p, err := hex.DecodeString(tc.wire)
		require.NoError(t, err)
		got, err := DecodeDailyAttendanceRequest115(p)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
		for n := 0; n < len(p); n++ {
			_, err = DecodeDailyAttendanceRequest115(p[:n])
			require.Error(t, err)
		}
		_, err = DecodeDailyAttendanceRequest115(append(p, 0))
		require.Error(t, err)
	}
	for _, wire := range []string{
		"6d0100000000000001000000", "6d0100000300000001000000",
		"6d0100000100000000000000", "6d0100000100000021000000",
		"6d01000001000000ffffffff", "4b0100000100000001000000",
	} {
		p, err := hex.DecodeString(wire)
		require.NoError(t, err)
		_, err = DecodeDailyAttendanceRequest115(p)
		require.Error(t, err)
	}
}
