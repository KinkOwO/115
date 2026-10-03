package gamedata

import (
	"dfolan/internal/catalog"
	"fmt"
	"runtime"
	"time"
)

type AuditResult struct {
	Domain                   string                    `json:"domain"`
	Baseline                 string                    `json:"baseline"`
	SourceChecksum           string                    `json:"source_checksum,omitempty"`
	Equivalent               bool                      `json:"equivalent"`
	SourceMatches            bool                      `json:"source_matches"`
	FieldsEquivalent         bool                      `json:"fields_equivalent"`
	MigrationCompatible      bool                      `json:"migration_compatible"`
	MigrationComparison      *Comparison               `json:"migration_comparison,omitempty"`
	WorldProjectionAdditions []WorldProjectionAddition `json:"world_projection_additions,omitempty"`
	Error                    string                    `json:"error,omitempty"`
	DurationMS               int64                     `json:"duration_ms"`
	HeapAfterBytes           uint64                    `json:"heap_after_bytes"`
	Comparison
}

// AuditCatalog compares against the effective JSON loader, including its
// compatibility projections and side catalogs. It writes no runtime state.
func AuditCatalog(domain, path string, direct *Source, limit int) (result AuditResult) {
	result.Domain, result.Baseline = domain, path
	started := time.Now()
	defer func() {
		result.DurationMS = time.Since(started).Milliseconds()
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		result.HeapAfterBytes = memory.HeapAlloc
	}()
	var left, right any
	var err error
	switch domain {
	case "characters":
		var value catalog.Characters
		value, err = catalog.LoadCharacters(path)
		result.SourceChecksum, left = value.Source.Checksum, value
		if err == nil {
			right, err = direct.Characters(path)
		}
	case "world":
		var value catalog.WorldCatalog
		value, err = catalog.LoadWorld(path)
		result.SourceChecksum, left = value.Source.Checksum, value
		if err == nil {
			right, err = direct.World(path)
		}
	case "quests":
		var value catalog.QuestCatalog
		value, err = catalog.LoadQuests(path)
		result.SourceChecksum, left = value.Source.Checksum, value
		if err == nil {
			right, err = direct.Quests(path)
		}
	case "progression":
		var value catalog.Progression
		value, err = catalog.LoadProgression(path)
		result.SourceChecksum, left = value.Source.Checksum, value
		if err == nil {
			right, err = direct.Progression(path)
		}
	default:
		err = fmt.Errorf("unknown audit domain %q", domain)
	}
	if err == nil && direct.mode != PVF {
		err = fmt.Errorf("audit requires a PVF source")
	}
	if err != nil {
		result.Error = err.Error()
		return
	}
	result.Comparison = Compare(left, right, limit)
	result.SourceMatches = result.SourceChecksum == direct.Snapshot().Checksum
	result.FieldsEquivalent = result.Count == 0
	result.Equivalent = result.SourceMatches && result.FieldsEquivalent
	result.MigrationCompatible = result.Equivalent
	if domain == "world" {
		comparison, additions, e := CompareWorldMigration(left.(catalog.WorldCatalog), right.(catalog.WorldCatalog), limit)
		result.MigrationComparison = &comparison
		result.WorldProjectionAdditions = additions
		result.MigrationCompatible = result.SourceMatches && e == nil && comparison.Count == 0
		if e != nil {
			result.Error = e.Error()
		}
	}
	if !result.SourceMatches {
		result.Error = fmt.Sprintf("baseline source %s differs from inner PVF source %s; field comparison is diagnostic only", result.SourceChecksum, direct.Snapshot().Checksum)
	}
	return
}
