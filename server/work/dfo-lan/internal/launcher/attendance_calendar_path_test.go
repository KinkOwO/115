package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayCalendarReadableFromDefaultAndCandidateWorkingDirectories(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	calendar := filepath.Join(module, "configs", "attendance-calendar.json")
	content := `{"timezone":"Asia/Hong_Kong","reset_hour":6,"source":"release-calendar"}`
	if err := os.WriteFile(calendar, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, explicit := range []bool{false, true} {
		in := launcherGatewayInput(t, module)
		in.Env.Set("DFO_ATTENDANCE_CALENDAR", "stale-other-installation.json")
		if _, err := buildGatewayCommand(in); err != nil {
			t.Fatal(err)
		}
		path := in.Env.Get("DFO_ATTENDANCE_CALENDAR")
		if !filepath.IsAbs(path) || path != calendar {
			t.Fatalf("calendar must belong to the launched module: %q", path)
		}
		cwd := gatewayWorkingDirectory(module, InnerPVFStatus{Explicit: explicit})
		// Simulate os.ReadFile's cwd resolution without changing global test cwd.
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		body, err := os.ReadFile(path)
		if err != nil || string(body) != content {
			t.Fatalf("calendar unreadable with cwd=%s: %v", cwd, err)
		}
	}
}
