package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/database"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdventureEliteDiagnosticPreservesOwnerAndPacketEvidence(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := &worldSession{role: database.Character{ID: 900, WireID: 7}, channelType: 22, eliteChannelID: 10, odyssey: true,
		adventureElitePrepared: &adventureElitePreparation{Owner: 900, Selected: [3]int64{901, 902}}}
	body := []byte{1, 7, 0, 2}
	packets := []outboundPacket{{"info", 0, 1382, body}, {"done", 0, 1879, []byte{2, 0}}}
	event := adventureEliteDiagnostic(w, 1811, packets, nil)
	require.Equal(t, w.role.WireID, event["expected_owner_wire_id"], "wire identity must not be replaced by persistent ID")
	require.Equal(t, int64(900), event["character_id"])
	require.Equal(t, true, event["odyssey"])
	require.Equal(t, true, event["accepted"])
	out := event["outputs_planned"].([]map[string]any)
	require.Equal(t, hex.EncodeToString(body), out[0]["payload_hex"])
	require.Equal(t, []byte{1, 7, 0, 2}, body, "diagnostics must not change the actual payload")
	require.Equal(t, [3]int64{901, 902}, w.adventureElitePrepared.Selected)
}

func TestAdventureEliteDiagnosticRejectAndSizeBoundary(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "0")
	packets := []outboundPacket{{"oversize", 0, 1382, make([]byte, 256*1024+1)}, {"other", 0, 99, []byte{9}}}
	event := adventureEliteDiagnostic(nil, 1811, packets, fmt.Errorf("owner changed"))
	require.Equal(t, false, event["enabled"])
	require.Equal(t, false, event["accepted"])
	require.Equal(t, "owner changed", event["reason"])
	out := event["outputs_planned"].([]map[string]any)
	require.Len(t, out, 1, "another system's body must not enter elite diagnostics")
	require.NotContains(t, out[0], "payload_hex")
	require.Equal(t, "diagnostic-size-limit", out[0]["body_omitted"])
	require.Len(t, out[0]["sha256"], 64)
}
