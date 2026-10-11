package launcher

import "path/filepath"

// Prepared candidates use the module's config defaults as well as the absolute
// paths built by the launcher. Normal launches retain their historical cwd.
func gatewayWorkingDirectory(module string, archive InnerPVFStatus) string {
	if archive.Explicit {
		return module
	}
	return filepath.Dir(filepath.Dir(module))
}
