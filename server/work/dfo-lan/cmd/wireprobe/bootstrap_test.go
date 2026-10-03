package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/wire"
	"dfolan/internal/quest"
	"dfolan/internal/world"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bootstrap tests use an empty temporary working directory and no storage or
// native archive. They cannot discover the launcher's real config or player DB.
func bootstrapTestConfig(t *testing.T) Config {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, key := range []string{"DFO_SKILL_CATALOG", "DFO_SKILL_RELEASE", "DFO_DUNGEON_CATALOG", "DFO_ODYSSEY_DUNGEON_CATALOG", "DFO_ODYSSEY_REWARDS_PILOT", "DFO_ODYSSEY_REWARDS_RELEASE", "DFO_ODYSSEY_CHAPTER_DROP", "DFO_CLEAR_CUBE_SOURCE"} {
		t.Setenv(key, "")
	}
	craftBytes := [6]byte{equipmentCraftWindow, equipmentCraftVariant, equipmentCraftConfirmWindow, equipmentCraftConfirmVariant, equipmentCraftGenerateWindow, equipmentCraftGenerateVariant}
	execute, executeOn, transform := equipmentCraftExecute, equipmentCraftExecuteOn, equipmentTransformApply
	t.Cleanup(func() {
		equipmentCraftWindow, equipmentCraftVariant = craftBytes[0], craftBytes[1]
		equipmentCraftConfirmWindow, equipmentCraftConfirmVariant = craftBytes[2], craftBytes[3]
		equipmentCraftGenerateWindow, equipmentCraftGenerateVariant = craftBytes[4], craftBytes[5]
		equipmentCraftExecute, equipmentCraftExecuteOn, equipmentTransformApply = execute, executeOn, transform
	})
	cfg, err := loadConfig(nil, func(string) string { return "" }, io.Discard)
	require.NoError(t, err)
	cfg.FatigueRules = ""
	return cfg
}

func TestPrepareRuntimeRetainsResolvedConfigAndFixtures(t *testing.T) {
	cfg := bootstrapTestConfig(t)
	require.NoError(t, os.Mkdir("configs", 0700))
	require.NoError(t, os.WriteFile("configs/randomoption.current37.json", []byte(`{}`), 0600))
	frame, err := wire.ServerFrame(0, 33, nil)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("fixture.bin", frame, 0600))
	require.NoError(t, os.WriteFile("responses.json", []byte(`{"33":"fixture.bin"}`), 0600))
	cfg.Fixture, cfg.Responses = "fixture.bin", "responses.json"
	runtime, cleanup, err := prepareRuntime(cfg)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	require.NotNil(t, cleanup)
	t.Cleanup(cleanup)
	assert.Equal(t, "configs/randomoption.current37.json", runtime.config.RandomOptionCatalog)
	assert.Empty(t, cfg.RandomOptionCatalog, "caller configuration must remain a value")
	assert.Equal(t, "127.0.0.1", runtime.gameHost)
	assert.Equal(t, frame, runtime.raw)
	assert.Equal(t, frame, runtime.responses[33])
	assert.NotNil(t, runtime.hub)
	assert.Nil(t, runtime.gameStore)
	_, err = os.Stat(cfg.Output)
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(cfg.Output, "ready.json"))
	assert.ErrorIs(t, err, os.ErrNotExist, "preparation must not publish readiness")
	cleanup()
	cleanup()
}

func TestPrepareRuntimeRejectsRetiredContentBeforeStorage(t *testing.T) {
	for _, row := range []struct {
		name string
		set  func(*Config)
	}{
		{"boosters", func(c *Config) { c.BoosterCatalog = "old.json" }},
		{"selection-boxes", func(c *Config) { c.SelectionBoxes = "old.json" }},
		{"prices", func(c *Config) { c.ShopPrices = "old.json" }},
		{"item index", func(c *Config) { c.ItemIndex = "old.json" }},
		{"full equipment", func(c *Config) { c.EquipmentFullCatalog = "old" }},
		{"world", func(c *Config) { c.WorldCatalog = "old.json" }},
		{"quests", func(c *Config) { c.QuestCatalog = "old.json" }},
		{"progression", func(c *Config) { c.ProgressionCatalog = "old.json" }},
		{"skills", func(c *Config) { c.SkillCatalog = "old.json" }},
		{"loot", func(c *Config) { c.LootCatalog = "old.json" }},
	} {
		t.Run(row.name, func(t *testing.T) {
			cfg := bootstrapTestConfig(t)
			// If storage is reached this path fails with a different error.
			cfg.CharacterStorage = "missing-storage.json"
			row.set(&cfg)
			runtime, cleanup, err := prepareRuntime(cfg)
			require.ErrorContains(t, err, "native PVF")
			require.ErrorContains(t, err, row.name)
			assert.Nil(t, runtime)
			assert.Nil(t, cleanup)
		})
	}
}

func TestPrepareRuntimeReturnsStartupFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"PVF check requires selection", func(c *Config) { c.PVFCheckCatalogs = true }, "requires explicit pvf-catalogs"},
		{"invalid game host", func(c *Config) { c.GameListen = "localhost:7001" }, "numeric IP host"},
		{"invalid omen", func(c *Config) { c.OmenInfo = "invalid" }, "bad -omen-info"},
		{"missing template", func(c *Config) { c.UnifiedCharacTemplate = "missing.bin" }, "missing.bin"},
		{"missing fixture", func(c *Config) { c.Fixture = "missing.bin" }, "read fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := bootstrapTestConfig(t)
			tc.change(&cfg)
			runtime, cleanup, err := prepareRuntime(cfg)
			require.ErrorContains(t, err, tc.want)
			assert.Nil(t, runtime)
			assert.Nil(t, cleanup, "failed preparation must clean up internally")
			if tc.name == "missing template" {
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestPrepareRuntimeRejectsShadowWorldBeforeStorage(t *testing.T) {
	cfg := bootstrapTestConfig(t)
	cfg.CharacterStorage = "missing-storage.json"
	t.Setenv("DFO_NPC_PRESENCE_WORLD", "old-world.json")
	runtime, cleanup, err := prepareRuntime(cfg)
	require.ErrorContains(t, err, "active native PVF world")
	assert.Nil(t, runtime)
	assert.Nil(t, cleanup)
}

func TestPrepareRuntimeRejectsRetiredEnvironmentBeforeStorage(t *testing.T) {
	for _, key := range []string{"DFO_SKILL_CATALOG", "DFO_DUNGEON_CATALOG", "DFO_ODYSSEY_DUNGEON_CATALOG"} {
		t.Run(key, func(t *testing.T) {
			cfg := bootstrapTestConfig(t)
			cfg.CharacterStorage = "missing-storage.json"
			t.Setenv(key, "old.json")
			runtime, cleanup, err := prepareRuntime(cfg)
			require.ErrorContains(t, err, "native PVF")
			assert.Nil(t, runtime)
			assert.Nil(t, cleanup)
		})
	}
}

func TestRunGatewayReleasesListenerOnStartupError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"advertised address", func(c *Config) { c.AdvertiseHost = "invalid" }, "advertise-host"},
		{"channel config", func(c *Config) { c.ChannelRefreshConfig = "missing-channel.json" }, "missing-channel.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := bootstrapTestConfig(t)
			probe, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			cfg.GameListen = probe.Addr().String()
			require.NoError(t, probe.Close())
			tc.change(&cfg)
			require.ErrorContains(t, runGateway(cfg), tc.want)
			listener, err := net.Listen("tcp4", cfg.GameListen)
			require.NoError(t, err, "startup failure must release the bound game port")
			require.NoError(t, listener.Close())
		})
	}
}

func TestTownArrivalScenesValidatedBeforeSessions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		world  *world.Service
		quests *quest.Service
		scenes map[uint32]catalog.TownArrivalScene
		fails  bool
	}{
		{"no services", nil, nil, nil, false},
		{"world only", &world.Service{}, nil, nil, false},
		{"quests only", nil, &quest.Service{}, nil, false},
		{"missing whitelist", &world.Service{}, &quest.Service{}, nil, true},
		{"valid empty whitelist", &world.Service{}, &quest.Service{}, map[uint32]catalog.TownArrivalScene{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTownArrivalScenes(tc.world, tc.quests, tc.scenes)
			if tc.fails {
				require.ErrorContains(t, err, "whitelist was not passed")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRuntimeCleanupReverseOrderAndOnce(t *testing.T) {
	resources := new(runtimeCleanup)
	var order []string
	for _, name := range []string{"catalogs", "database", "admin guard"} {
		resources.add(func() { order = append(order, name) })
	}
	var callers sync.WaitGroup
	for range 8 {
		callers.Go(resources.close)
	}
	callers.Wait()
	assert.Equal(t, []string{"admin guard", "database", "catalogs"}, order)
}
