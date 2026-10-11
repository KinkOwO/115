package featurebridge

import (
	"bytes"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func TestExtensionProtocolRejectsWrongGenerationAndBounds(t *testing.T) {
	p, e := (Frame{Kind: Snapshot, ID: 1}).Bytes()
	require.NoError(t, e)
	require.Equal(t, "553131350100010001000000000000000000000000000000", fmtHex(p))
	f, e := Read(bytes.NewReader(p))
	require.NoError(t, e)
	require.EqualValues(t, 1, f.ID)
	for _, offset := range []int{0, 4, 6, 8, 16, 20} {
		q := append([]byte(nil), p...)
		if offset == 8 {
			q[8] = 0
		} else {
			q[offset] = 0xff
		}
		_, e = Read(bytes.NewReader(q))
		require.Error(t, e)
	}
	_, e = (Frame{Kind: Snapshot, ID: 1, Body: make([]byte, MaxBody+1)}).Bytes()
	require.Error(t, e)
	_, e = Read(bytes.NewReader(p[:23]))
	require.Error(t, e)
}
func fmtHex(p []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(p)*2)
	for i, b := range p {
		out[i*2] = h[b>>4]
		out[i*2+1] = h[b&15]
	}
	return string(out)
}
func TestExtensionClaimRejectsMalformedIdentity(t *testing.T) {
	p := make([]byte, 104)
	p[0] = Weekly
	p[1] = Period
	binary.LittleEndian.PutUint32(p[4:], 1)
	copy(p[8:], strings.Repeat("a", 64))
	copy(p[72:], strings.Repeat("b", 32))
	c, e := DecodeClaim(p)
	require.NoError(t, e)
	require.EqualValues(t, 1, c.Index)
	for _, offset := range []int{0, 1, 2, 4, 8, 72} {
		q := append([]byte(nil), p...)
		if offset == 4 {
			q[4] = 0
		} else {
			q[offset] = 255
		}
		_, e = DecodeClaim(q)
		require.Error(t, e)
	}
	_, e = DecodeClaim(append(p, 0))
	require.Error(t, e)
}
func TestExtensionViewUTF8AndBounds(t *testing.T) {
	v := View{Role: 7, Campaigns: []CampaignView{{Family: Weekly, Title: "周常", Source: strings.Repeat("a", 64), Campaign: strings.Repeat("b", 32), Rows: []Row{{Group: Period, State: Claimable, Index: 1, Item: 10001, Count: 3, Goal: 60, Progress: 60, Label: "第1期"}}}}}
	p, e := v.Bytes()
	require.NoError(t, e)
	require.Contains(t, string(p), "周常")
	require.EqualValues(t, 7, binary.LittleEndian.Uint64(p))
	if path := os.Getenv("US115_FEATURE_VECTOR"); path != "" {
		require.NoError(t, os.WriteFile(path, p, 0600))
	}
	v.Campaigns[0].Rows[0].Label = string([]byte{0xff})
	_, e = v.Bytes()
	require.Error(t, e)
	v.Campaigns[0].Rows[0].Label = strings.Repeat("a", 4097)
	_, e = v.Bytes()
	require.Error(t, e)
}
