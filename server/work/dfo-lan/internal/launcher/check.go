package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CheckOptions selects which profile's dependencies to verify. They mirror the
// launch_local.py flags that choose a server binary and a data mode.
type CheckOptions struct {
	ServerOnly    bool
	ClientOnly    bool
	SourceBuild   bool
	JSONMode      bool
	RepairProfile string
}

// CheckReport is what a read-only dependency check found. It starts nothing.
type CheckReport struct {
	ClientDir    string
	Binary       string
	DataMode     string
	Storage      string
	ProfilePath  string
	ProfileEnv   map[string]string
	Dependencies []Dependency
}

// Dependency is one required path and whether it was present.
type Dependency struct {
	Label string
	Path  string
	Found bool
}

// Missing lists the required paths that are absent, in report order.
func (r CheckReport) Missing() []Dependency {
	var missing []Dependency
	for _, dependency := range r.Dependencies {
		if !dependency.Found {
			missing = append(missing, dependency)
		}
	}
	return missing
}

// localSettings is the part of launcher.local.json the launcher reads.
type localSettings struct {
	ClientDir    string `json:"client_dir"`
	ServerBinary string `json:"server_binary"`
}

// Check verifies the dependencies the selected mode needs, without starting anything.
//
// The profile decides the binary when one applies, exactly as gateway_configuration
// did: an explicit --repair-profile always wins, otherwise the PVF default profile
// applies to a normal (non JSON-mode, non client-only) run.
func Check(root string, opts CheckOptions) (CheckReport, error) {
	module := filepath.Join(root, "server", "work", "dfo-lan")
	probe := filepath.Join(root, "server", "work", "dfo_probe_tools")
	// launcher.local.json sits one level above the module (server/), which is also the
	// base its relative paths are written against - verified against the Python.
	settingsRoot := filepath.Join(root, "server")

	var report CheckReport
	settings, err := loadLocalSettings(filepath.Join(settingsRoot, "launcher.local.json"))
	if err != nil {
		return report, err
	}
	if settings.ClientDir == "" {
		return report, fmt.Errorf("launcher.local.json must set client_dir")
	}
	// client_dir is relative to the module root, exactly as the Python resolved it.
	clientDir := settings.ClientDir
	if !filepath.IsAbs(clientDir) {
		clientDir = filepath.Join(settingsRoot, clientDir)
	}
	report.ClientDir = filepath.Clean(clientDir)

	storage, err := LoadStorageConfig(root)
	if err != nil {
		return report, err
	}
	report.Storage = fmt.Sprintf("SQLite %s", storage.SQLitePath)

	report.Binary = selectBinary(module, settingsRoot, settings, opts)
	profilePath := opts.RepairProfile
	if profilePath == "" && !opts.JSONMode && !opts.ClientOnly && !opts.ServerOnly {
		profilePath = filepath.Join(module, "configs", "pvf-default.json")
	}
	var profileRequired []string
	if profilePath != "" {
		if !filepath.IsAbs(profilePath) {
			profilePath = filepath.Join(module, profilePath)
		}
		loaded, err := LoadProfile(profilePath, module)
		if err != nil {
			return report, err
		}
		report.ProfilePath = loaded.Path
		report.ProfileEnv = loaded.Env
		profileRequired = loaded.Required
		switch {
		case opts.SourceBuild && opts.RepairProfile == "":
			// The source build keeps its own binary; the profile's binary entry in the
			// required list is replaced by the one actually launched.
			profileRequired = replacePath(profileRequired, loaded.Binary, report.Binary)
		default:
			report.Binary = loaded.Binary
		}
	}
	switch {
	case opts.JSONMode:
		report.DataMode = "JSON / explicit profile"
	default:
		report.DataMode = "PVF direct"
	}

	// 2026-10-05 业主决策："最小预装环境、没有 python、不留回退" —— 依赖清单里不再有 .py 文件
	// （channel_probe.py / catalog_startup.py 是 Python 编排的必需品，Go 编排不用它们）。
	// probe.exe 保留：原生工具、不是环境依赖；Go 宿主装 WFP 过滤器失败时的回退路径要用它。
	required := []Dependency{}
	if !opts.ServerOnly {
		required = append(required,
			Dependency{Label: "probe runtime", Path: filepath.Join(probe, "probe.exe")},
			Dependency{Label: "client", Path: filepath.Join(clientDir, "DFO.exe")},
			Dependency{Label: "client script archive", Path: filepath.Join(clientDir, "Script.pvf")},
			Dependency{Label: "client key data", Path: filepath.Join(clientDir, "sk.dat")},
		)
	}
	if !opts.ClientOnly {
		required = append(required, Dependency{Label: "server binary", Path: report.Binary})
	}
	// The profile's own files (the PVF archive and the policy files it names) are as
	// required as the binary: without them the server refuses to start.
	for _, path := range profileRequired {
		if path == report.Binary {
			continue
		}
		required = append(required, Dependency{Label: "profile: " + filepath.Base(path), Path: path})
	}

	for i := range required {
		info, statErr := os.Stat(required[i].Path)
		required[i].Found = statErr == nil && !info.IsDir()
	}
	report.Dependencies = required
	return report, nil
}

// selectBinary mirrors the Python's explicit modes. The profile-driven case is the one
// still owned by launch_local.py, so it falls back to the PVF default binary.
func selectBinary(module, settingsRoot string, settings localSettings, opts CheckOptions) string {
	switch {
	case opts.SourceBuild:
		return filepath.Join(module, "bin", "wireprobe-handoff-source.exe")
	case opts.JSONMode && settings.ServerBinary != "":
		binary := settings.ServerBinary
		if !filepath.IsAbs(binary) {
			binary = filepath.Join(settingsRoot, binary)
		}
		return filepath.Clean(binary)
	default:
		return filepath.Join(module, "bin", "wireprobe-pvf.exe")
	}
}

func loadLocalSettings(path string) (localSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return localSettings{}, fmt.Errorf(
				"copy launcher.example.json to launcher.local.json and set client_dir")
		}
		return localSettings{}, err
	}
	var settings localSettings
	if err := json.Unmarshal(stripBOM(data), &settings); err != nil {
		return localSettings{}, fmt.Errorf("launcher.local.json: %w", err)
	}
	return settings, nil
}

// replacePath swaps one required path for another, keeping the position so the report
// stays readable.
func replacePath(paths []string, from, to string) []string {
	swapped := make([]string, len(paths))
	copy(swapped, paths)
	for i, path := range swapped {
		if path == from {
			swapped[i] = to
		}
	}
	return swapped
}
