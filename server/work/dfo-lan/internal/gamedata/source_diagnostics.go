package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// Files lists the verified archive owned by this source for diagnostic scans.
func (s *Source) Files() []pvf.File {
	if s == nil || s.archive == nil {
		return nil
	}
	return s.archive.Files()
}

// Script reads a script without opening another archive or changing provenance.
func (s *Source) Script(path string) (catalog.ScriptRecord, error) {
	if s == nil || s.archive == nil {
		return catalog.ScriptRecord{}, fmt.Errorf("script diagnostics require a native PVF source")
	}
	return catalog.ReadScript(s.archive, path)
}

// ResolveScript applies the catalog's native linked-script resolution.
func (s *Source) ResolveScript(path string) (catalog.ScriptRecord, error) {
	if s == nil || s.archive == nil {
		return catalog.ScriptRecord{}, fmt.Errorf("script diagnostics require a native PVF source")
	}
	return catalog.ResolveScript(s.archive, path)
}
