package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// Native CMD2015 from the blocked Elvenmere run, events.jsonl:14617.
const eliteElvenmereNativeBody = "0000000000000000000000000000000000ffffffffffffffff00000000000000"

func TestEliteElvenmereNativeTeleportDispatch(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, name := range []string{"prepared", "without-apcs", "owner", "channel", "settings", "empty", "missing", "unloaded", "wire-zero", "wire-sentinel", "ordinary-dungeon", "short", "checksum"} {
		t.Run(name, func(t *testing.T) {
			client, conn, events := newDispatchTestClient()
			w := probeWorld()
			w.selectingDungeon = false
			w.adventureEliteEntryProbeUsed = true
			w.activeDungeon = &dungeon.Session{RunID: "elvenmere-native", Loaded: true,
				Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 1}
			body, err := hex.DecodeString(eliteElvenmereNativeBody)
			require.NoError(t, err)
			verified := true
			switch name {
			case "without-apcs":
				w.adventureElitePrepared = nil
				w.adventureEliteEntryProbeUsed = false
			case "owner":
				w.adventureElitePrepared.Owner++
			case "channel":
				w.adventureElitePrepared.Channel++
			case "settings":
				w.adventureEliteSnapshot[0]++
			case "empty":
				w.adventureElitePrepared.Selected = [3]int64{}
			case "missing":
				w.adventureElitePrepared = nil
			case "unloaded":
				w.activeDungeon.Loaded = false
			case "wire-zero":
				w.role.WireID = 0
			case "wire-sentinel":
				w.role.WireID = 65535
			case "ordinary-dungeon":
				w.activeDungeon.Definition.ID = 7
			case "short":
				body = body[:10]
			case "checksum":
				verified = false
			}
			client.worldState = w
			old := w.activeDungeon
			var frozen adventureElitePreparation
			if w.adventureElitePrepared != nil {
				frozen = *w.adventureElitePrepared
			}
			require.Equal(t, dispatchHandled, client.dispatchDungeon(&clientRequest{
				frame: wire.Frame{Type: 1, ID: 2015}, plaintext: body, verified: verified}))
			require.Same(t, old, w.activeDungeon)
			if w.adventureElitePrepared != nil {
				require.Equal(t, frozen, *w.adventureElitePrepared)
			}
			var sent []uint16
			for _, event := range *events {
				if event["kind"] == "elvenmere_teleport_ack" || event["kind"] == "elvenmere_info_sent" {
					sent = append(sent, event["id"].(uint16))
				}
			}
			if name == "prepared" || name == "without-apcs" {
				require.Equal(t, uint16(2), old.Extra)
				require.Equal(t, []uint16{2015, 2193}, sent)
				require.NotZero(t, conn.Len())
			} else {
				require.Equal(t, uint16(1), old.Extra)
				require.Empty(t, sent)
				require.Zero(t, conn.Len())
			}
		})
	}
}
