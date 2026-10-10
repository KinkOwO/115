package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These vectors were captured from the original flag declarations and getenv
// helpers, before replacing them. The empty selection/content policy defaults
// and current native profile were updated when their redundant files retired.
// They include every flag, every environment
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

func TestSoleQualityNativeConfigContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		env  string
		want bool
	}{
		{name: "default"},
		{name: "cli", args: []string{"-sole-quality-native"}, want: true},
		{name: "environment", env: "1", want: true},
		{name: "cli overrides environment", args: []string{"-sole-quality-native=false"}, env: "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(tc.args, func(key string) string {
				if key == "DFO_SOLE_QUALITY_NATIVE" {
					return tc.env
				}
				return ""
			}, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.SoleQualityNative)
		})
	}
}

// TestAccountConfigContract 钉住 -account / DFO_ACCOUNT 的取值口径：默认 probe，环境
// 变量能改名，显式参数赢过环境变量。这是「启动器选账号、服务端按该名字自动建号」的地基。
func TestAccountConfigContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		env  string
		want string
	}{
		{name: "default", want: "probe"},
		{name: "cli", args: []string{"-account=tomeu"}, want: "tomeu"},
		{name: "environment", env: "tomeu2", want: "tomeu2"},
		{name: "cli overrides environment", args: []string{"-account=tomeu"}, env: "tomeu2", want: "tomeu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(tc.args, func(key string) string {
				if key == "DFO_ACCOUNT" {
					return tc.env
				}
				return ""
			}, io.Discard)
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Account)
			require.NoError(t, cfg.validate())
		})
	}
}

// 账号名同时进 SQL 和客户端连接 payload（以 ? 分段），所以不合法的写法必须在任何运行期
// 副作用之前就报错，而不是等 prepareRuntime 撞一条看不懂的唯一键冲突。
func TestAccountConfigValidationRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", "probe?2", "a b", "玩家", "x=y", "pro	be", strings.Repeat("a", 33)} {
		cfg, err := loadConfig([]string{"-account=" + name}, func(string) string { return "" }, io.Discard)
		require.NoError(t, err)
		require.Error(t, cfg.validate(), name)
	}
	for _, name := range []string{"probe", "Tomeu-2", "tomeu_99", strings.Repeat("a", 32)} {
		cfg, err := loadConfig([]string{"-account=" + name}, func(string) string { return "" }, io.Discard)
		require.NoError(t, err)
		require.NoError(t, cfg.validate(), name)
	}
}
