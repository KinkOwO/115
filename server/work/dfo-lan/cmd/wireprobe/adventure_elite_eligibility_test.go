package main

import (
	"bytes"
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A temporary SQLite database exercises the real save/load handlers without
// depending on runtime/storage or skipping the low-level regression.
func eliteEligibilityWorld(t *testing.T, level, awakening byte, odyssey bool) (*worldSession, database.Character, int32) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "elite.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateAdventure} {
		require.NoError(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "elite-eligibility")
	require.NoError(t, err)
	const source = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	professions := catalog.Characters{Professions: map[byte]catalog.Profession{0: {ID: 0, RawSHA256: source}}}
	create := func(name string, lv, aw byte, od bool) database.Character {
		creation := make([]byte, 12)
		creation[6] = 255
		if od {
			creation[10] = 2
		}
		state, e := json.Marshal(character.State{Level: lv, Advancement: 1, Awakening: aw, CreationMode: 0,
			CreationOptions: creation, SourceSHA256: source, Attributes: map[string]float32{"[hp max]": 123, "[mp max]": 45},
			LearnedSkills: [2]map[uint16]byte{{19: 1}, {}}, SkillSlots: [2]map[uint16]uint16{{19: 0}, {}}})
		require.NoError(t, e)
		state, e = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1"})
		require.NoError(t, e)
		// Native create body: profession, length-prefixed name, 12 options.
		request := []byte{0}
		request = binary.LittleEndian.AppendUint32(request, uint32(len(name)))
		request = append(request, []byte(name)...)
		request = append(request, creation...)
		role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0,
			ConfigVersion: source, Request: request, State: state}, 24)
		require.NoError(t, e)
		return role
	}
	human := create("EliteHuman", 115, 3, false)
	apc := create("EliteCompanion", level, awakening, odyssey)
	w := &worldSession{store: store, account: account, role: human, channelType: 73,
		characters: &character.Service{Catalog: professions}, fatigue: &character.FatigueService{Location: time.UTC}}
	// Initialize the account projection before capturing the original state.
	// CharactersWithAdventure projects account season fields onto every role.
	_, err = w.prepareAdventure(ctx)
	require.NoError(t, err)
	roles, err := store.Characters(ctx, account)
	require.NoError(t, err)
	var slot int32 = -1
	for i, role := range roles {
		if role.ID == apc.ID {
			slot, apc = int32(i), role
		}
		if role.ID == human.ID {
			human = role
		}
	}
	require.NotEqual(t, int32(-1), slot)
	w.role = human
	require.Equal(t, odyssey, character.CreatedAsOdyssey(apc))
	return w, apc, slot
}

func eliteEligibilityRequest(t *testing.T, slot int32, skill int32) []byte {
	t.Helper()
	row := protocol.AdventureEliteSelection{Mode: 2, Slots: [3]int32{slot, -1, -1}}
	row.SkillUsage[0][0] = skill
	encoded, err := protocol.AdventureEliteSelections([]protocol.AdventureEliteSelection{row})
	require.NoError(t, err)
	return encoded[1:]
}

func TestAdventureEliteLowLevelSaveAndLoadPreserveRealSnapshot(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, tc := range []struct {
		level, awakening byte
		odyssey          bool
	}{{1, 0, false}, {50, 1, false}, {99, 2, false}, {100, 0, false}, {1, 0, true}} {
		t.Run(fmt.Sprintf("level%d-awakening%d-odyssey%t", tc.level, tc.awakening, tc.odyssey), func(t *testing.T) {
			w, apc, slot := eliteEligibilityWorld(t, tc.level, tc.awakening, tc.odyssey)
			ctx := context.Background()
			request := eliteEligibilityRequest(t, slot, 19)
			saved, err := w.setAdventureElite(ctx, request, request, "eligibility-test")
			require.NoError(t, err)
			require.Len(t, saved, 2)
			require.Equal(t, uint16(1754), saved[0].ID)
			require.Equal(t, uint16(1719), saved[1].ID)
			loaded, err := w.loadAdventureElite(ctx, []byte{2, 0})
			require.NoError(t, err)
			require.Len(t, loaded, 2)
			require.Equal(t, uint16(1382), loaded[0].ID)
			require.Equal(t, uint16(1879), loaded[1].ID)
			require.Equal(t, []byte{2, 0}, loaded[1].Payload)
			// Compare the actual packet to an independent read of the original
			// persisted role, including level/advancement/stats and learned skills.
			snapshot, err := w.characters.TagCharacterSnapshot(apc)
			require.NoError(t, err)
			require.Equal(t, tc.level, snapshot.Level)
			require.Equal(t, uint32(1230), snapshot.Stats.HP)
			require.Equal(t, uint16(19), snapshot.Skills[0].ID)
			want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, []byte{byte(slot)}, []protocol.TagCharacter{snapshot})
			require.NoError(t, err)
			require.True(t, bytes.Equal(want, loaded[0].Payload))
			roles, err := w.store.Characters(ctx, w.account)
			require.NoError(t, err)
			for _, role := range roles {
				if role.ID == apc.ID {
					require.JSONEq(t, string(apc.State), string(role.State), "APC preparation mutated the source character")
				}
			}
			profile, err := w.prepareAdventure(ctx)
			require.NoError(t, err)
			require.Equal(t, [3]int64{apc.ID, 0, 0}, profile.Data.EliteSelections[2])
		})
	}
}

func TestAdventureEliteLowLevelStillRejectsUnlearnedSkillsAndUnsupportedChannel(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, _, slot := eliteEligibilityWorld(t, 1, 0, false)
	request := eliteEligibilityRequest(t, slot, 65534)
	_, err := w.setAdventureElite(context.Background(), request, request, "bad-skill")
	require.ErrorContains(t, err, "未掌握")
	request = eliteEligibilityRequest(t, slot, 19)
	_, err = w.setAdventureElite(context.Background(), request, request, "known-skill")
	require.NoError(t, err)
	w.channelType = 0
	_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
	require.ErrorContains(t, err, "当前频道未启用")
	// Canceling eligibility is not authorization to route ordinary N1382 into
	// the client's unrelated type-0 container before the adapter exists.
	w.channelType = 73
	load := make([]byte, 2)
	binary.LittleEndian.PutUint16(load, 1)
	_, err = w.loadAdventureElite(context.Background(), load)
	require.ErrorContains(t, err, "未启用此精锐模式")
}

func TestAdventureEliteEnvironmentRestoresNativeEligibility(t *testing.T) {
	for _, value := range []string{"", "0", "true", "01", "1 "} {
		t.Run(fmt.Sprintf("off-%q", value), func(t *testing.T) {
			t.Setenv(adventureelite.EnvKey, value)
			w, _, slot := eliteEligibilityWorld(t, 1, 0, false)
			request := eliteEligibilityRequest(t, slot, 19)
			_, err := w.setAdventureElite(context.Background(), request, request, "native-reject")
			require.ErrorContains(t, err, "100级")
			// A low-level list previously saved while enabled remains intact, but
			// loading after disabling must reapply the native qualification.
			t.Setenv(adventureelite.EnvKey, "1")
			_, err = w.setAdventureElite(context.Background(), request, request, "save-before-off")
			require.NoError(t, err)
			t.Setenv(adventureelite.EnvKey, value)
			_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
			require.ErrorContains(t, err, "100级")
		})
	}
	t.Run("native-qualified", func(t *testing.T) {
		t.Setenv(adventureelite.EnvKey, "0")
		w, _, slot := eliteEligibilityWorld(t, 100, 2, false)
		request := eliteEligibilityRequest(t, slot, 19)
		_, err := w.setAdventureElite(context.Background(), request, request, "native-qualified")
		require.NoError(t, err)
		_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
		require.NoError(t, err)
	})
}
