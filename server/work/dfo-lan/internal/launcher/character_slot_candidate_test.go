package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCharacterSlotCandidatePreservesDefaultArchive(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server/work/dfo-lan")
	original := filepath.Join(root, "server/work/client-build/Script.inner.pvf")
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(module, "configs/slot.inner.pvf")
	if err := os.WriteFile(candidate, []byte("explicit-candidate"), 0644); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(module, "configs/slot.json")
	calendar := filepath.Join(module, "configs/attendance-calendar.json")
	if err := os.WriteFile(calendar, []byte("candidate-calendar"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte(`{"binary":"bin/wireprobe-pvf.exe","environment":{"DFO_PVF_CATALOGS":"items","DFO_PVF_ARCHIVE":"configs/slot.inner.pvf"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, dryRun := range []bool{false, true} {
		report, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: dryRun, RepairProfile: profile})
		if err != nil {
			t.Fatal(err)
		}
		if report.InnerPVF.Path != candidate || report.InnerPVF.Checked || report.InnerPVF.NeedsBuild || report.ProfileEnv["DFO_PVF_ARCHIVE"] != candidate {
			t.Fatalf("wrong candidate: %+v", report)
		}
		if dryRun && !strings.Contains(report.Plan[2].Detail, candidate) {
			t.Fatalf("gateway plan lost candidate: %+v", report.Plan)
		}
		body, err := os.ReadFile(filepath.Join(gatewayWorkingDirectory(module, report.InnerPVF), "configs/attendance-calendar.json"))
		if err != nil || string(body) != "candidate-calendar" {
			t.Fatal("candidate runtime cannot read its relative calendar", err)
		}
		if _, err := os.Stat(original); !os.IsNotExist(err) {
			t.Fatal("default archive was rebuilt", err)
		}
	}
}
