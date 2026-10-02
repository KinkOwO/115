package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These vectors were captured from the original flag declarations and getenv
// helpers, before replacing them. They include every flag, every environment
// alias, and the launcher's existing native profile, without opening the PVF.
func TestWireprobeConfigLegacyContract(t *testing.T) {
	data, err := os.ReadFile("testdata/config_legacy.json")
	require.NoError(t, err)
	var fixture struct {
		Defaults map[string]json.RawMessage
		Cases    []struct {
			Name      string
			Args      []string
			Env       map[string]string
			Overrides map[string]json.RawMessage
		}
	}
	require.NoError(t, json.Unmarshal(data, &fixture))
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			values := make(map[string]json.RawMessage, len(fixture.Defaults))
			for key, value := range fixture.Defaults {
				values[key] = value
			}
			for key, value := range tc.Overrides {
				values[key] = value
			}
			encoded, err := json.Marshal(values)
			require.NoError(t, err)
			var expected Config
			require.NoError(t, json.Unmarshal(encoded, &expected))
			actual, err := loadConfig(tc.Args, func(key string) string { return tc.Env[key] }, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestWireprobeConfigHelpAndCLIRejection(t *testing.T) {
	getenv := func(string) string { return "" }
	t.Run("help retains original flag names, descriptions and defaults", func(t *testing.T) {
		data, err := os.ReadFile("testdata/config_help.json")
		require.NoError(t, err)
		var want string
		require.NoError(t, json.Unmarshal(data, &want))
		var output bytes.Buffer
		_, err = loadConfig([]string{"-h"}, getenv, &output)
		require.ErrorIs(t, err, flag.ErrHelp)
		assert.Equal(t, want, output.String())
	})
	for _, args := range [][]string{
		{"-unknown-config"},
		{"-game-listen"},
		{"-fatigue-free=invalid"},
		{"-skill-lock-offset=invalid"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var output bytes.Buffer
			_, err := loadConfig(args, getenv, &output)
			require.Error(t, err)
			assert.NotEmpty(t, output.String())
		})
	}
}

func TestWireprobeConfigEnvironmentByteBoundsAndCLIOverride(t *testing.T) {
	for _, tc := range []struct {
		Env  string
		Want int
	}{
		{"-1", 1}, {"256", 1}, {"not-a-number", 1}, {"", 1}, {"0", 0}, {"255", 255},
	} {
		t.Run(tc.Env, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "DFO_EQUIPMENT_CRAFT_WINDOW" {
					return tc.Env
				}
				return ""
			}
			cfg, err := loadConfig(nil, getenv, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.Want, cfg.EquipmentCraftWindow)
			cfg, err = loadConfig([]string{"-equipment-craft-window=300"}, getenv, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, 300, cfg.EquipmentCraftWindow, "legacy CLI integers are not clamped by environment rules")
		})
	}
}

func TestWireprobeConfigValidationBeforeRuntimeSetup(t *testing.T) {
	for _, tc := range []struct {
		Args []string
		Want string
	}{
		{[]string{"-pvf-check-heap-profile=heap.pprof"}, "pvf-check-heap-profile requires pvf-check-catalogs"},
		{[]string{"-pvf-check-catalogs", "-pvf-catalogs= "}, "pvf-check-catalogs requires explicit pvf-catalogs"},
		{[]string{"-pvf-check-catalogs", "-pvf-catalogs=items", "-pvf-check-heap-profile=heap.pprof"}, ""},
	} {
		t.Run(tc.Args[0], func(t *testing.T) {
			cfg, err := loadConfig(tc.Args, func(string) string { return "" }, io.Discard)
			require.NoError(t, err)
			if tc.Want == "" {
				require.NoError(t, cfg.validate())
			} else {
				require.EqualError(t, cfg.validate(), tc.Want)
			}
		})
	}
}
