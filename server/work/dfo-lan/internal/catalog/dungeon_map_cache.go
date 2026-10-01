package catalog

import (
	"container/list"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sync"
	"unsafe"
)

// The budget estimates tokens and their strings, excluding allocator rounding
// and the separately bounded PVF body cache. Entry count is also capped at 128.
const dungeonMapCacheBytes int64 = 32 * 1024 * 1024

type cachedMapScript struct {
	key    string
	script ScriptRecord
	bytes  int64
}
type mapScriptCache struct {
	archive *pvf.Archive
	mu      sync.Mutex
	order   list.List
	entries map[string]*list.Element
	bytes   int64
	closed  bool
}

// RestoreRuntimeDungeons reconnects a verified base projection to a fresh
// independent native map view. Runtime overlays are applied afterwards.
func RestoreRuntimeDungeons(a *pvf.Archive, c DungeonCatalog) (DungeonCatalog, error) {
	if a == nil || c.Source.Checksum != a.Snapshot().Checksum {
		return c, fmt.Errorf("cached dungeon source mismatch")
	}
	for _, r := range c.Maps {
		if r.Path == "" || len(r.SHA256) != 64 || r.Cells != nil {
			return c, fmt.Errorf("invalid cached map metadata")
		}
	}
	var err error
	c, err = ValidateDungeons(c)
	if err != nil {
		return c, err
	}
	if err = c.attachMapScripts(a); err != nil {
		return c, err
	}
	return c, nil
}

func (c *DungeonCatalog) attachMapScripts(a *pvf.Archive) error {
	paths := make([]string, 0, len(c.Maps))
	for _, script := range c.Maps {
		paths = append(paths, script.Path)
	}
	view, err := a.ReadOnlyView(paths)
	if err != nil {
		return err
	}
	c.mapScripts = &mapScriptCache{archive: view, entries: map[string]*list.Element{}}
	return nil
}

// MapScript is the shared read path for native runtime maps and legacy eager
// catalogs. Returned cells are immutable source facts. Overlays retain their
// own validated scripts and never replace source metadata on a cache read.
func (c DungeonCatalog) MapScript(id uint32) (ScriptRecord, error) {
	script, ok := c.Maps[id]
	if !ok {
		return ScriptRecord{}, fmt.Errorf("map %d not imported", id)
	}
	if c.mapScripts == nil || script.Cells != nil {
		return script, nil
	}
	return c.mapScripts.read(script)
}

func scriptMemoryBytes(script ScriptRecord) int64 {
	n := int64(len(script.Path)+len(script.SHA256)) + int64(len(script.Cells))*int64(unsafe.Sizeof(pvf.Token{}))
	for _, cell := range script.Cells {
		n += int64(len(cell.Text) + len(cell.Reference))
	}
	return n
}

func (c *mapScriptCache) read(expected ScriptRecord) (ScriptRecord, error) {
	return c.readWith(expected, func(path string) (ScriptRecord, error) { return ReadScript(c.archive, path) })
}

func (c *mapScriptCache) readWith(expected ScriptRecord, load func(string) (ScriptRecord, error)) (ScriptRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ScriptRecord{}, fmt.Errorf("map source is closed")
	}
	key := expected.Path + "|" + expected.SHA256
	if entry, ok := c.entries[key]; ok {
		c.order.MoveToBack(entry)
		return entry.Value.(cachedMapScript).script, nil
	}
	script, err := load(expected.Path)
	if err != nil {
		return ScriptRecord{}, err
	}
	if script.SHA256 != expected.SHA256 {
		return ScriptRecord{}, fmt.Errorf("map source hash mismatch: %s", expected.Path)
	}
	n := scriptMemoryBytes(script)
	if n > dungeonMapCacheBytes {
		return script, nil
	}
	for (c.bytes+n > dungeonMapCacheBytes || c.order.Len() >= 128) && c.order.Len() > 0 {
		old := c.order.Front()
		value := old.Value.(cachedMapScript)
		delete(c.entries, value.key)
		c.bytes -= value.bytes
		c.order.Remove(old)
	}
	c.entries[key] = c.order.PushBack(cachedMapScript{key, script, n})
	c.bytes += n
	return script, nil
}

func (c DungeonCatalog) ReleaseMapReadCache() {
	if c.mapScripts == nil {
		return
	}
	c.mapScripts.mu.Lock()
	defer c.mapScripts.mu.Unlock()
	c.mapScripts.clearLocked()
	c.mapScripts.archive.ReleaseReadCaches()
}

func (c *mapScriptCache) clearLocked() {
	c.entries = map[string]*list.Element{}
	c.order.Init()
	c.bytes = 0
}

func (c DungeonCatalog) CloseMapSource() error {
	if c.mapScripts == nil {
		return nil
	}
	c.mapScripts.mu.Lock()
	defer c.mapScripts.mu.Unlock()
	c.mapScripts.closed = true
	c.mapScripts.clearLocked()
	return c.mapScripts.archive.Close()
}

// ExpandedMaps is for explicit offline audits only. Normal preparation and
// runtime do not materialize this full token tree.
func (c DungeonCatalog) ExpandedMaps() (DungeonCatalog, error) {
	out := c
	out.Maps = make(map[uint32]ScriptRecord, len(c.Maps))
	for id := range c.Maps {
		script, err := c.MapScript(id)
		if err != nil {
			return DungeonCatalog{}, err
		}
		out.Maps[id] = script
	}
	return out, nil
}
