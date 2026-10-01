package inventory

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

type equipmentLocation struct {
	Offset int64
	Size   int
	SHA256 string
	Path   string `json:",omitempty"`
}
type FullEquipmentCatalog struct {
	bindings    []equipmentBinding
	Source      pvf.ArchiveSnapshot
	IndexSHA256 string
	Records     map[uint32]equipmentLocation
	Errors      map[uint32]string
	file        *os.File
	archive     *pvf.Archive
	mu          sync.Mutex
	cache       catalog.SizedCache[uint32, EquipmentDefinition]
	closed      bool
}
type equipmentBinding struct{ ID, File uint32 }

func (c *FullEquipmentCatalog) RecordCount() int {
	if c.archive != nil {
		return len(c.bindings)
	}
	return len(c.Records)
}
func (c *FullEquipmentCatalog) HasDefinition(id uint32) bool {
	if c.archive != nil {
		i := sort.Search(len(c.bindings), func(i int) bool { return c.bindings[i].ID >= id })
		return i < len(c.bindings) && c.bindings[i].ID == id
	}
	_, ok := c.Records[id]
	return ok
}

func OpenFullEquipmentCatalog(prefix, source string) (*FullEquipmentCatalog, error) {
	raw, e := os.ReadFile(prefix + ".index.json")
	if e != nil {
		return nil, e
	}
	c := &FullEquipmentCatalog{}
	if e = json.Unmarshal(raw, c); e != nil {
		return nil, e
	}
	if c.Source.Checksum != source || len(c.IndexSHA256) != 64 || len(c.Records) == 0 {
		return nil, fmt.Errorf("full equipment source mismatch")
	}
	c.file, e = os.Open(prefix + ".data")
	if e != nil {
		return nil, e
	}
	info, e := c.file.Stat()
	if e != nil {
		c.file.Close()
		return nil, e
	}
	for _, r := range c.Records {
		if r.Offset < 0 || r.Size <= 0 || r.Size > 16*1024*1024 || r.Offset > info.Size()-int64(r.Size) || len(r.SHA256) != 64 {
			c.file.Close()
			return nil, fmt.Errorf("invalid equipment record location")
		}
	}
	return c, nil
}
func (c *FullEquipmentCatalog) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.cache.Clear()
	if c.archive != nil {
		return c.archive.Close()
	}
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// OpenPVFEquipmentCatalog keeps a compact immutable view for lazy definitions.
// It does not alter the smaller drop/quest catalog or its allowed item pool.
func OpenPVFEquipmentCatalog(a *pvf.Archive, index catalog.ItemIndex) (*FullEquipmentCatalog, error) {
	if a == nil || index.Source.Checksum != a.Snapshot().Checksum {
		return nil, fmt.Errorf("PVF equipment source mismatch")
	}
	hash := index.IndexHashes["list/equipment.lst"]
	if len(hash) != 64 {
		return nil, fmt.Errorf("missing equipment source index hash")
	}
	c := &FullEquipmentCatalog{Source: a.Snapshot(), IndexSHA256: hash,
		Records: map[uint32]equipmentLocation{}, Errors: map[uint32]string{}}
	paths := make([]string, 0, len(index.Items))
	for id, entry := range index.Items {
		if entry.Kind != "equipment" && entry.Kind != "avatar" {
			continue
		}
		p := catalog.ResolveScriptPath(a, entry.Path)
		f, ok := a.FindFile(p)
		if !ok || f.DataType != 1 {
			return nil, fmt.Errorf("equipment %d source script missing: %s", id, p)
		}
		c.Records[id] = equipmentLocation{Path: p}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("empty PVF equipment index")
	}
	v, err := a.ReadOnlyView(paths)
	if err != nil {
		return nil, err
	}
	c.archive = v
	c.bindings = make([]equipmentBinding, 0, len(c.Records))
	for id, r := range c.Records {
		index := v.FindFileIndex(r.Path)
		if index < 0 {
			v.Close()
			return nil, fmt.Errorf("equipment view binding missing: %d", id)
		}
		c.bindings = append(c.bindings, equipmentBinding{id, uint32(index)})
	}
	sort.Slice(c.bindings, func(i, j int) bool { return c.bindings[i].ID < c.bindings[j].ID })
	c.Records = nil // Runtime keeps 8-byte native bindings, not JSON record locations.
	return c, nil
}

func (c *FullEquipmentCatalog) Definition(id uint32) (EquipmentDefinition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return EquipmentDefinition{}, fmt.Errorf("equipment source closed")
	}
	if c.archive != nil {
		if err := c.archive.ReadError(); err != nil {
			return EquipmentDefinition{}, err
		}
	}
	if d, ok := c.cache.Get(id); ok {
		return d, nil
	}
	if c.archive != nil {
		i := sort.Search(len(c.bindings), func(i int) bool { return c.bindings[i].ID >= id })
		if i >= len(c.bindings) || c.bindings[i].ID != id {
			return EquipmentDefinition{}, fmt.Errorf("equipment %d absent from source index", id)
		}
		file, err := c.archive.FileInfo(int(c.bindings[i].File))
		if err != nil {
			return EquipmentDefinition{}, err
		}
		s, err := catalog.ReadScript(c.archive, file.ArchivePath)
		if err != nil {
			return EquipmentDefinition{}, err
		}
		d := equipmentDefinitionFromScript(id, s)
		c.cacheDefinition(id, d)
		return d, nil
	}
	loc, ok := c.Records[id]
	if !ok {
		return EquipmentDefinition{}, fmt.Errorf("equipment%d definition missing: %s", id, c.Errors[id])
	}
	packed := make([]byte, loc.Size)
	if _, e := c.file.ReadAt(packed, loc.Offset); e != nil {
		return EquipmentDefinition{}, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(packed)) != loc.SHA256 {
		return EquipmentDefinition{}, fmt.Errorf("equipment record checksum mismatch")
	}
	z, e := zlib.NewReader(bytes.NewReader(packed))
	if e != nil {
		return EquipmentDefinition{}, e
	}
	defer z.Close()
	raw, e := io.ReadAll(io.LimitReader(z, 32*1024*1024+1))
	if e != nil {
		return EquipmentDefinition{}, e
	}
	if len(raw) > 32*1024*1024 {
		return EquipmentDefinition{}, fmt.Errorf("equipment record too large")
	}
	var s catalog.ScriptRecord
	if e = json.Unmarshal(raw, &s); e != nil {
		return EquipmentDefinition{}, e
	}
	if len(s.SHA256) != 64 || s.Path == "" {
		return EquipmentDefinition{}, fmt.Errorf("invalid equipment provenance")
	}
	d := equipmentDefinitionFromScript(id, s)
	c.cacheDefinition(id, d)
	return d, nil
}

func equipmentDefinitionFromScript(id uint32, s catalog.ScriptRecord) EquipmentDefinition {
	d := EquipmentDefinition{ID: id, Path: s.Path, SHA256: s.SHA256, Fields: map[string][]pvf.Token{}}
	d.fameFields, d.fameLevels = equipmentFameSections(s.Cells)
	tag := ""
	for _, t := range s.Cells {
		if t.Type == 3 {
			tag = t.Text
			continue
		}
		d.Fields[tag] = append(d.Fields[tag], t)
	}
	return d
}

func (c *FullEquipmentCatalog) cacheDefinition(id uint32, d EquipmentDefinition) {
	n := len(d.Path) + len(d.SHA256)
	for tag, ts := range d.Fields {
		n += len(tag) + 64 + 48*len(ts)
		for _, t := range ts {
			n += len(t.Text)
		}
	}
	for tag, ts := range d.fameFields {
		n += len(tag) + 64 + 48*len(ts)
		for _, t := range ts {
			n += len(t.Text)
		}
	}
	n += len(d.fameLevels) * 64
	c.cache.Put(id, d, n, 32*1024*1024, 2048)
}

func (c *EquipmentCatalog) Definition(id uint32) (EquipmentDefinition, error) {
	if c.Full != nil {
		return c.Full.Definition(id)
	}
	d, ok := c.index[id]
	if !ok {
		return d, fmt.Errorf("equipment definition missing")
	}
	return d, nil
}
