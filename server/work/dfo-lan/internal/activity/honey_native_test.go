package activity

import (
	"dfolan/internal/catalog/pvf"
	"testing"

	"github.com/stretchr/testify/require"
)

func honeyNativeFixture() []pvf.Token {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	s := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	n := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	return []pvf.Token{
		tag("[control panel]"), s("fixture"), s("2026-10-08 06:00:00"), s("2026-11-08 06:00:00"), tag("[/control panel]"),
		tag("[ui file]"), s("ui/native.xui"), tag("[explicit weekly start time]"), s("2026-10-08 06:00:01"),
		tag("[pc room event]"), s("false"), tag("[reward postal type]"), n(1),
		tag("[max reward offset count]"), s("daily"), n(2), tag("[/max reward offset count]"),
		tag("[reward info]"), tag("[reward type]"), s("daily"), tag("[loop]"), n(1),
		tag("[reward data]"), n(0), n(10001), n(2), n(7), n(1999), n(1), n(10002), n(3), n(0), n(1001), tag("[/reward data]"),
		tag("[reward data]"), n(0), n(10003), n(4), n(0), n(999), tag("[/reward data]"),
		tag("[apply unlimit flag]"), n(0), tag("[unlimit reward data]"), n(0), n(10004), n(5), n(0), n(1234), tag("[/unlimit reward data]"), tag("[/reward info]"),
	}
}

func TestHoneyNativePreservesBlocksOffsetsAndAuxiliaryRewards(t *testing.T) {
	c, e := ParseHoneyNative(honeyNativeFixture())
	require.NoError(t, e)
	g := c.Groups["daily"]
	require.Len(t, g.Blocks, 2)
	require.Len(t, g.Blocks[0].Rewards, 2)
	require.EqualValues(t, 7, g.Blocks[0].Rewards[0].Option)
	require.EqualValues(t, 3, g.Blocks[0].CumulativeSeconds)
	// Native divides each block's sum, then adds the preceding block's seconds.
	// A flattened list of intervals or a final rounded sum produces a different result.
	require.EqualValues(t, 3, g.Blocks[1].CumulativeSeconds)
	require.EqualValues(t, 0, g.Blocks[1].Rewards[0].Offset)
	require.Len(t, g.Unlimit, 1)
	require.EqualValues(t, 10004, g.Unlimit[0].Item)
	require.EqualValues(t, 1234, g.Unlimit[0].Milliseconds)
	tokens := honeyNativeFixture()
	for i := range tokens {
		if tokens[i].Value == 1999 {
			tokens[i].Value = 3999
		}
	}
	changed, e := ParseHoneyNative(tokens)
	require.NoError(t, e)
	require.EqualValues(t, 5, changed.Groups["daily"].Blocks[1].CumulativeSeconds)
}

func TestHoneyNativeRejectsCountersAndUnboundCalendarOverrides(t *testing.T) {
	for _, tc := range []struct{ from, to int32 }{{1999, -1}, {1999, 0x7fffffff}, {10001, 1}} {
		tokens := honeyNativeFixture()
		for i := range tokens {
			if tokens[i].Type == 0 && tokens[i].Value == tc.from {
				tokens[i].Value = tc.to
			}
		}
		_, e := ParseHoneyNative(tokens)
		require.Error(t, e)
	}
	tokens := honeyNativeFixture()
	tokens = append(tokens, pvf.Token{Type: 3, Text: "[day of week reward info]"})
	_, e := ParseHoneyNative(tokens)
	require.Error(t, e)
	tokens = honeyNativeFixture()
	tokens = append(tokens, tokens[22:]...)
	_, e = ParseHoneyNative(tokens)
	require.Error(t, e)
}

func TestHoneyNativeUsesSelectedEventIndex(t *testing.T) {
	index := []pvf.Token{{Type: 0, Value: 365}, {Type: 6, Text: "other.evt"}, {Type: 0, Value: 313}, {Type: 6, Text: "Live/Event/Changed/HoneyTime.evt"}}
	p, e := HoneyNativeEventPath(index)
	require.NoError(t, e)
	require.Equal(t, "live/event/changed/honeytime.evt", p)
	_, e = HoneyNativeEventPath(index[:2])
	require.Error(t, e)
	_, e = HoneyNativeEventPath(append(append([]pvf.Token{}, index...), index[2:]...))
	require.Error(t, e)
	index[3].Text = "../HoneyTime.evt"
	_, e = HoneyNativeEventPath(index)
	require.Error(t, e)
}
