package protocol

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHoneyTime115NativeReaderLayout(t *testing.T) {
	// Distinct bytes expose skipped group 3, the group-1 leading flag, the
	// sixteen-byte versus u32 branch, and the uncounted four-word tail.
	s := HoneyTimeStatus115{SourceFlag: 0x3b, PeriodSeconds: 0x3f3e3d3c,
		LoopCounts: [4]uint32{0x43424140, 0x47464544, 0x4b4a4948, 0x4f4e4d4c}, FinalFlag: 0x50}
	s.Groups[0] = HoneyTimeGroup115{Seconds: 0x07060504, Step: 8}
	for i := range s.Groups[0].Values {
		s.Groups[0].Values[i] = byte(9 + i)
	}
	s.Groups[1] = HoneyTimeGroup115{Enabled: 0x19, Seconds: 0x1d1c1b1a, Step: 0x1e, Limit: 0x2f}
	for i := range s.Groups[1].Values {
		s.Groups[1].Values[i] = byte(0x1f + i)
	}
	s.Groups[2] = HoneyTimeGroup115{Enabled: 0x30, Seconds: 0x34333231, Step: 0x35, Mask: 0x39383736, Limit: 0x3a}
	p, err := s.Bytes()
	require.NoError(t, err)
	require.Len(t, p, 81)
	want := "390100000405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f50"
	require.Equal(t, want, hex.EncodeToString(p))
}

func TestHoneyTime115RejectsUnconsumedFieldsAndWrongCommands(t *testing.T) {
	for _, mutate := range []func(*HoneyTimeStatus115){
		func(s *HoneyTimeStatus115) { s.Groups[0].Enabled = 1 },
		func(s *HoneyTimeStatus115) { s.Groups[0].Limit = 1 },
		func(s *HoneyTimeStatus115) { s.Groups[1].Mask = 1 },
		func(s *HoneyTimeStatus115) { s.Groups[2].Values[0] = 1 },
	} {
		var s HoneyTimeStatus115
		mutate(&s)
		_, err := s.Bytes()
		require.Error(t, err)
	}
	p, err := hex.DecodeString("3901000000000000")
	require.NoError(t, err)
	require.NoError(t, DecodeHoneyTimeSync115(p))
	for _, bad := range [][]byte{p[:7], append(append([]byte{}, p...), 0), {0x39, 1, 0, 0, 1, 0, 0, 0}, {0x67, 1, 0, 0, 0, 0, 0, 0}} {
		require.Error(t, DecodeHoneyTimeSync115(bad))
	}
}

func TestHoneyTime115NativeClaimActionsAndOffsets(t *testing.T) {
	// Native callback 14412c8e7..936 writes three dwords; the group-3
	// branch explicitly skips action 4. Distinct offsets catch day/bit shifts.
	for _, tc := range []struct {
		wire string
		want HoneyTimeRequest115
	}{
		{"3901000000000000", HoneyTimeRequest115{Sync: true}},
		{"390100000100000000000000", HoneyTimeRequest115{Group: 0, Offset: 0}},
		{"390100000200000003000000", HoneyTimeRequest115{Group: 1, Offset: 3}},
		{"39010000030000000f000000", HoneyTimeRequest115{Group: 2, Offset: 15}},
		{"390100000500000002000000", HoneyTimeRequest115{Group: 3, Offset: 2}},
	} {
		p, err := hex.DecodeString(tc.wire)
		require.NoError(t, err)
		got, err := DecodeHoneyTimeRequest115(p)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
		for n := 0; n < len(p); n++ {
			_, err = DecodeHoneyTimeRequest115(p[:n])
			require.Error(t, err)
		}
		_, err = DecodeHoneyTimeRequest115(append(p, 0))
		require.Error(t, err)
	}
	for _, wire := range []string{
		"390100000000000000000000", // sync has no offset
		"390100000400000000000000", // skipped action
		"390100000600000000000000",
		"3901000001000000ffffffff", // signed native UI position
		"590300000100000000000000", // event857 needs a separate contract
	} {
		p, err := hex.DecodeString(wire)
		require.NoError(t, err)
		_, err = DecodeHoneyTimeRequest115(p)
		require.Error(t, err)
	}
}
