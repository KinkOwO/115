package catalog

import (
	"container/list"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sync"
)

// SizedCache bounds retained payloads; eviction never mutates values held by a
// caller. Oversized valid definitions can be used without entering the cache.
type SizedCache[K comparable, V any] struct {
	mu    sync.Mutex
	rows  map[K]*list.Element
	order list.List
	bytes int
}
type sizedValue[K comparable, V any] struct {
	key   K
	value V
	size  int
}

func (c *SizedCache[K, V]) Get(key K) (v V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.rows[key]
	if e == nil {
		return v, false
	}
	c.order.MoveToBack(e)
	return e.Value.(sizedValue[K, V]).value, true
}
func (c *SizedCache[K, V]) Put(key K, v V, size, maxBytes, maxEntries int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if size < 0 || maxBytes <= 0 || maxEntries <= 0 || size > maxBytes {
		return
	}
	if c.rows == nil {
		c.rows = map[K]*list.Element{}
	}
	if e := c.rows[key]; e != nil {
		c.bytes -= e.Value.(sizedValue[K, V]).size
		c.order.Remove(e)
		delete(c.rows, key)
	}
	for (c.bytes+size > maxBytes || c.order.Len() >= maxEntries) && c.order.Len() > 0 {
		e := c.order.Front()
		r := e.Value.(sizedValue[K, V])
		delete(c.rows, r.key)
		c.bytes -= r.size
		c.order.Remove(e)
	}
	c.rows[key] = c.order.PushBack(sizedValue[K, V]{key, v, size})
	c.bytes += size
}
func (c *SizedCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = nil
	c.order.Init()
	c.bytes = 0
}
func (c *SizedCache[K, V]) Usage() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes, c.order.Len()
}

func ScriptBytes(s ScriptRecord) int {
	n := len(s.Path) + len(s.SHA256) + 48*len(s.Cells)
	for _, t := range s.Cells {
		n += len(t.Text)
	}
	return n
}

// ScriptDetails owns an independent view of the original verified file. It
// keeps provenance only, never a parent Archive or all expanded script trees.
type ScriptDetails[K comparable, V any] struct {
	mu      sync.Mutex
	archive *pvf.Archive
	refs    map[K]detailReference
	project func(K, ScriptRecord) V
	size    func(V) int
	cache   SizedCache[K, V]
	closed  bool
}
type detailReference struct{ Path, SHA256 string }

func NewScriptDetails[K comparable, V any](a *pvf.Archive, refs map[K]ScriptRecord, project func(K, ScriptRecord) V, size func(V) int) (*ScriptDetails[K, V], error) {
	paths := make([]string, 0, len(refs))
	meta := make(map[K]detailReference, len(refs))
	for k, r := range refs {
		if r.Path == "" {
			return nil, fmt.Errorf("missing detail path")
		}
		paths = append(paths, r.Path)
		meta[k] = detailReference{Path: r.Path, SHA256: r.SHA256}
	}
	v, err := a.ReadOnlyView(paths)
	if err != nil {
		return nil, err
	}
	return &ScriptDetails[K, V]{archive: v, refs: meta, project: project, size: size}, nil
}
func (d *ScriptDetails[K, V]) Get(key K) (out V, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return out, fmt.Errorf("PVF details closed")
	}
	if err := d.archive.ReadError(); err != nil {
		return out, err
	}
	ref, ok := d.refs[key]
	if !ok {
		return out, fmt.Errorf("PVF detail absent from source index")
	}
	if v, ok := d.cache.Get(key); ok {
		return v, nil
	}
	s, err := ReadScript(d.archive, ref.Path)
	if err != nil {
		return out, err
	}
	if ref.SHA256 != "" && s.SHA256 != ref.SHA256 {
		return out, fmt.Errorf("PVF detail checksum mismatch: %s", ref.Path)
	}
	out = d.project(key, s)
	d.cache.Put(key, out, d.size(out), 16*1024*1024, 256)
	return out, nil
}
func (d *ScriptDetails[K, V]) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache.Clear()
	d.archive.ReleaseReadCaches()
}
func (d *ScriptDetails[K, V]) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.cache.Clear()
	return d.archive.Close()
}
func (d *ScriptDetails[K, V]) Usage() (int, int) { return d.cache.Usage() }
