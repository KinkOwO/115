//go:build !windows

package launcher

import "fmt"

func injectAdventureElite(_ int, _, _ string, _ uintptr) error {
	return fmt.Errorf("精锐资格 DLL 注入仅支持 Windows x64")
}
