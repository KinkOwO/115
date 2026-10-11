package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// prepareSessionStorage is called only by actual Start after its private session
// directory exists. The gateway still requires absolute database paths; its cwd
// never decides which save is opened. The active configuration is never rewritten.
func prepareSessionStorage(module, sessionOut string) (string, error) {
	module, err := filepath.Abs(module)
	if err != nil {
		return "", err
	}
	sessionOut, err = filepath.Abs(sessionOut)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(filepath.Join(module, "runtime"), sessionOut)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resolved storage configuration requires a private runtime session directory")
	}
	path := filepath.Join(module, "runtime", "storage", "local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var storage storageFile
	if err := json.Unmarshal(stripBOM(data), &storage); err != nil {
		return "", fmt.Errorf("storage config %s: %w", path, err)
	}
	if err := validateStorage(storage); err != nil {
		return "", err
	}
	dbPath := strings.TrimSpace(*storage.SQLitePath)
	if !filepath.IsAbs(dbPath) {
		if filepath.VolumeName(dbPath) != "" {
			return "", fmt.Errorf("sqlite_path cannot be drive-relative: %q", dbPath)
		}
		dbPath = filepath.Join(module, dbPath)
	}
	dbPath, err = filepath.Abs(dbPath)
	if err != nil {
		return "", err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(stripBOM(data), &raw); err != nil {
		return "", err
	}
	// Match the current database.Config fields. Unknown/legacy fields stay in
	// the active file but are not copied into retained session diagnostics; in
	// particular, obsolete PostgreSQL credentials must not leak into this file.
	resolved := map[string]json.RawMessage{}
	for _, key := range []string{"durable_writes", "max_connections", "sqlite_busy_timeout_ms", "busy_timeout_ms", "max_read_connections"} {
		if value, ok := raw[key]; ok {
			resolved[key] = value
		}
	}
	resolved["driver"] = json.RawMessage(`"sqlite"`)
	resolved["sqlite_path"], err = json.Marshal(filepath.Clean(dbPath))
	if err != nil {
		return "", err
	}
	body, err := json.MarshalIndent(resolved, "", "  ")
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(sessionOut, "storage-resolved-*.json")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if _, err = file.Write(body); err != nil {
		file.Close()
		os.Remove(name)
		return "", err
	}
	if err = file.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
