package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func eliteNativeDeath(t *testing.T, killer uint16) []byte {
	t.Helper()
	text, err := os.ReadFile("testdata/adventure-elite-native-death-20261008.hex")
	require.NoError(t, err)
	body, err := hex.DecodeString(strings.TrimSpace(string(text)))
	require.NoError(t, err)
	r, err := protocol.DecodeMonsterDeath(body)
	require.NoError(t, err)
	require.Equal(t, uint32(4096), r.Entity)
	require.Equal(t, uint16(65535), r.Killer)
	binary.LittleEndian.PutUint16(body[4:6], killer)
	return body
}
func eliteCombatWorld() *worldSession {
	w := probeWorld()
	w.adventureEliteEntryProbeUsed = true
	w.activeDungeon = &dungeon.Session{Loaded: true, Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: 1}}, Dead: map[uint16]bool{}}
	w.deathSent = map[uint16]bool{}
	return w
}
func TestEliteNativeOwnedDeathReusesConfirmationAndDeduplicates(t *testing.T) {
	for _, killer := range []uint16{2, 65535, 7} {
		t.Run(fmt.Sprintf("killer_%04x", killer), func(t *testing.T) {
			w := eliteCombatWorld()
			body := eliteNativeDeath(t, killer)
			before := w.eliteCombatState(39, body)
			plan, err := w.monsterDeath(body, nil)
			if killer == 7 {
				require.Error(t, err)
				require.False(t, w.activeDungeon.Dead[4096])
				return
			}
			require.NoError(t, err)
			require.True(t, w.activeDungeon.Dead[4096])
			require.Equal(t, killer == 65535, w.activeDungeon.Unowned[4096])
			require.False(t, w.deathSent[4096]) // Dispatch marks this only after sending NOTI38.
			require.Len(t, plan, 2)
			require.Equal(t, uint16(39), plan[0].ID)
			require.Equal(t, uint16(38), plan[1].ID)
			var logged map[string]any
			w.noteEliteCombatRequest(39, body, before, nil, plan, nil, func(row map[string]any) { logged = row })
			require.Equal(t, false, before["dead"])
			require.Equal(t, true, logged["after"].(map[string]any)["dead"])
			require.Equal(t, "pending", logged["client_acceptance"])
			w.deathSent[4096] = true // Model the successful dispatch before the native retransmission.
			repeat, err := w.monsterDeath(body, nil)
			require.NoError(t, err)
			require.Len(t, repeat, 1)
			require.Equal(t, uint16(39), repeat[0].ID)
		})
	}
}
func TestEliteDiagnosticRecordsPendingRoomAndFailureWithoutMutatingState(t *testing.T) {
	w := eliteCombatWorld()
	before := w.eliteCombatState(45, nil)
	var logged map[string]any
	pending := &dungeon.Session{}
	pending.Room.Map = 999
	w.noteEliteCombatRequest(45, nil, before, pending, nil, nil, func(row map[string]any) { logged = row })
	require.Equal(t, uint32(999), logged["pending_map"])
	require.Equal(t, 0, len(w.activeDungeon.Dead))
	require.Equal(t, 0, len(w.deathSent))
	w.adventureElitePrepared = nil
	require.Nil(t, w.eliteCombatState(39, nil))
}

func TestEliteCommittedSnapshotSeparatesReturnedRunAndRetainsPreparation(t *testing.T) {
	w := eliteCombatWorld()
	w.adventureEliteEntrySerial = 2
	w.activeDungeon.RunID = "second-run"
	before := w.eliteCombatState(72, nil)
	frozen := *w.adventureElitePrepared
	w.activeDungeon = nil // Original dispatch transition after successful return ACK.
	w.deathSent = nil
	w.resetCards()
	var row map[string]any
	w.noteEliteCommittedState(72, nil, before, func(v map[string]any) { row = v })
	require.Equal(t, "second-run", row["origin_run_id"])
	require.Equal(t, uint64(2), row["entry_serial"])
	after := row["after"].(map[string]any)
	require.Equal(t, false, after["active"])
	require.Equal(t, 0, after["death_sent_count"])
	require.Equal(t, false, after["cards_active"])
	require.Equal(t, frozen, *w.adventureElitePrepared)
}
