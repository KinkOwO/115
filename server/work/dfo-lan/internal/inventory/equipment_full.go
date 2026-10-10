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
	"log"
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
// PVFEquipmentProjection contains original archive record indices, not handles.
// Its cache key must bind the exact archive, parser and effective ItemIndex.
type PVFEquipmentProjection struct {
	Source      pvf.ArchiveSnapshot
	IndexSHA256 string
	Bindings    []PVFEquipmentBinding
}
type PVFEquipmentBinding struct{ ID, File uint32 }

func ProjectPVFEquipment(a *pvf.Archive, index catalog.ItemIndex) (PVFEquipmentProjection, error) {
	var p PVFEquipmentProjection
	if a == nil || index.Source.Checksum != a.Snapshot().Checksum {
		return p, fmt.Errorf("PVF equipment source mismatch")
	}
	p.Source = a.Snapshot()
	p.IndexSHA256 = index.IndexHashes["list/equipment.lst"]
	if len(p.IndexSHA256) != 64 {
		return p, fmt.Errorf("missing equipment source index hash")
	}
	missing := 0
	var missingSamples []string
	for id, entry := range index.Items {
		if entry.Kind != "equipment" && entry.Kind != "avatar" {
			continue
		}
		path := catalog.ResolveScriptPath(a, entry.Path)
		f, ok := a.FindFile(path)
		if !ok || f.DataType != 1 {
			// devpack 基线差异：缺失源脚本的物品不进懒加载绑定；运行期对它的
			// 脚本读取按物品报缺失，与合并前“物品不存在”等价。
			missing++
			if len(missingSamples) < 8 {
				missingSamples = append(missingSamples, fmt.Sprintf("%d %s", id, path))
			}
			continue
		}
		p.Bindings = append(p.Bindings, PVFEquipmentBinding{id, uint32(f.Index)})
	}
	if missing > 0 {
		log.Printf("PVF full equipment projection: %d source scripts missing (devpack baseline gap); samples: %v", missing, missingSamples)
	}
	sort.Slice(p.Bindings, func(i, j int) bool { return p.Bindings[i].ID < p.Bindings[j].ID })
	return p, nil
}

func ValidatePVFEquipmentProjection(a *pvf.Archive, p PVFEquipmentProjection) error {
	if a == nil || p.Source.Checksum != a.Snapshot().Checksum || len(p.IndexSHA256) != 64 || len(p.Bindings) == 0 {
		return fmt.Errorf("equipment projection source mismatch")
	}
	for i, b := range p.Bindings {
		if b.ID == 0 || i > 0 && p.Bindings[i-1].ID >= b.ID {
			return fmt.Errorf("invalid equipment binding order")
		}
		f, err := a.FileInfo(int(b.File))
		if err != nil || f.DataType != 1 {
			return fmt.Errorf("invalid equipment native binding %d", b.ID)
		}
	}
	return nil
}

func RestorePVFEquipment(a *pvf.Archive, p PVFEquipmentProjection) (*FullEquipmentCatalog, error) {
	if err := ValidatePVFEquipmentProjection(a, p); err != nil {
		return nil, err
	}
	order := make([]int, len(p.Bindings))
	for i, b := range p.Bindings {
		order[i] = int(b.File)
	}
	v, err := a.ReadOnlyViewIndices(order)
	if err != nil {
		return nil, err
	}
	sort.Ints(order)
	unique := order[:0]
	for _, i := range order {
		if len(unique) == 0 || unique[len(unique)-1] != i {
			unique = append(unique, i)
		}
	}
	c := &FullEquipmentCatalog{Source: a.Snapshot(), IndexSHA256: p.IndexSHA256, Errors: map[uint32]string{}, archive: v, bindings: make([]equipmentBinding, len(p.Bindings))}
	for i, b := range p.Bindings {
		c.bindings[i] = equipmentBinding{b.ID, uint32(sort.SearchInts(unique, int(b.File)))}
	}
	return c, nil
}

func OpenPVFEquipmentCatalog(a *pvf.Archive, index catalog.ItemIndex) (*FullEquipmentCatalog, error) {
	p, err := ProjectPVFEquipment(a, index)
	if err != nil {
		return nil, err
	}
	return RestorePVFEquipment(a, p)
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
