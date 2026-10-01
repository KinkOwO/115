package inventory

import "testing"

// 2026-10-01（next146）：直读模式下无色小晶块叠加目录的源身份必须能从编译期
// 常量切到当次内层 checksum。非 64 位十六进制一律忽略。
func TestSetClearCubeSourceValidates(t *testing.T) {
	original := clearCubeSourceChecksum
	t.Cleanup(func() { clearCubeSourceChecksum = original })

	good := "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
	SetClearCubeSource(good)
	if clearCubeSourceChecksum != good {
		t.Fatalf("SetClearCubeSource did not switch: %s", clearCubeSourceChecksum)
	}
	for _, bad := range []string{"", "abc", good[:63], "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		SetClearCubeSource(bad)
		if clearCubeSourceChecksum != good {
			t.Fatalf("garbage %q changed the token", bad)
		}
	}
}
