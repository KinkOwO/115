package launcher

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/adventureelite"
	"github.com/stretchr/testify/require"
)

func TestAdventureEliteLaunchPolicy(t *testing.T) {
	for _, value := range []string{"", "0", "true", "01", "1 "} {
		t.Run("off-"+value, func(t *testing.T) {
			env := newChildEnv([]string{adventureelite.EnvKey + "=" + value, probeFallbackEnvKey + "=1"})
			dll, err := adventureEliteDLL(t.TempDir(), env)
			require.NoError(t, err)
			require.Empty(t, dll, "disabled must not require/load a DLL")
		})
	}
	env := newChildEnv([]string{adventureelite.EnvKey + "=1", probeFallbackEnvKey + "=1"})
	_, err := adventureEliteDLL(t.TempDir(), env)
	require.ErrorContains(t, err, "不能同时")
	env = newChildEnv([]string{adventureelite.EnvKey + "=1"})
	_, err = adventureEliteDLL(t.TempDir(), env)
	require.ErrorContains(t, err, "DLL 预检失败")
}

func TestAdventureEliteCallerSwitchWinsForBothChildren(t *testing.T) {
	for _, value := range []string{"", "0", "1"} {
		env := BuildServerEnv([]string{"dfo_adventure_elite=" + value}, map[string]string{adventureelite.EnvKey: "1"}, false)
		require.Equal(t, value, newChildEnv(env).Get(adventureelite.EnvKey))
	}
	env := BuildServerEnv(nil, map[string]string{adventureelite.EnvKey: "1"}, false)
	require.Empty(t, newChildEnv(env).Get(adventureelite.EnvKey), "a profile cannot enable the script switch")
}

func TestAdventureEliteSelectsCandidateEvenWithExplicitProfile(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	profile := filepath.Join(module, "configs", "elite-profile.json")
	require.NoError(t, os.WriteFile(profile, []byte(`{"binary":"bin/wireprobe-pvf.exe","environment":{"DFO_SHOP_OPEN_ALL":"1"}}`), 0600))
	report, err := LaunchPlan(root, LaunchOptions{Check: true, RepairProfile: profile})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(module, "bin", "wireprobe-handoff-source.exe"), report.Binary)
	require.Equal(t, "1", newChildEnv(BuildServerEnv(CurrentEnv(), report.ProfileEnv, false)).Get(adventureelite.EnvKey))
}
