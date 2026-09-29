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
	"sync"
)

type equipmentLocation struct {
	Offset int64
	Size   int
	SHA256 string
}
type FullEquipmentCatalog struct {
	Source      pvf.ArchiveSnapshot
	IndexSHA256 string
	Records     map[uint32]equipmentLocation
	Errors      map[uint32]string
	file        *os.File
	mu          sync.Mutex
	cache       map[uint32]EquipmentDefinition
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
	c.cache = map[uint32]EquipmentDefinition{}
	return c, nil
}
func (c *FullEquipmentCatalog) Close() error { return c.file.Close() }
func (c *FullEquipmentCatalog) Definition(id uint32) (EquipmentDefinition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d, ok := c.cache[id]; ok {
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
	// Bound cache growth; a catalog may contain hundreds of thousands of avatars.
	if len(c.cache) >= 2048 {
		c.cache = map[uint32]EquipmentDefinition{}
	}
	c.cache[id] = d
	return d, nil
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
