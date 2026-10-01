package pvf

import (
	"container/list"
	"fmt"
	"sort"
)

// ReadOnlyView retains only the requested file metadata. It shares immutable,
// checksum-verified source bytes and string pools; releasing the original
// Archive therefore cannot change or invalidate a later lazy read. File.Index
// is local to the view. Editing APIs reject views because indices were remapped.
// Expanded body caches are bounded to 64 MiB and text caches to 2048 entries.
func (a *Archive) ReadOnlyView(paths []string) (*Archive, error) {
	if a == nil || a.closed.Load() {
		return nil, fmt.Errorf("nil PVF archive")
	}
	indices := make(map[int]bool, len(paths))
	for _, p := range paths {
		i, ok := a.lookupPath(p)
		if !ok {
			return nil, fmt.Errorf("%w: view entry %s", ErrFileNotFound, p)
		}
		indices[i] = true
	}
	order := make([]int, 0, len(indices))
	for i := range indices {
		order = append(order, i)
	}
	sort.Ints(order)
	v := &Archive{snapshot: a.snapshot, format: a.format, header: a.header,
		data: a.data, groups: a.groups, bodyOff: a.bodyOff, strA: a.strA, strW: a.strW,
		readOnlyView:  true,
		maxChunkBytes: 64 * 1024 * 1024, maxTexts: 2048}
	if a.compactDirectory {
		v.compactDirectory = true
		v.compactTable = make([]byte, 0, len(order)*fileItemSize)
		v.files, v.items, v.pathIdx = nil, nil, nil
		v.backing = a.backing
		if v.backing != nil && !v.backing.retain() {
			return nil, fmt.Errorf("PVF source is closed")
		}
		for _, old := range order {
			v.compactTable = append(v.compactTable, a.compactTable[old*fileItemSize:(old+1)*fileItemSize]...)
		}
		v.buildCompactIndex()
		v.attachCleanup()
		return v, nil
	}
	v.files = make([]File, 0, len(order))
	v.items = make([]fileItem, 0, len(order))
	v.pathIdx = make(map[string]int, len(order))
	for _, old := range order {
		f := a.files[old]
		f.Index = len(v.files)
		v.pathIdx[pathKey(f.ArchivePath)] = f.Index
		v.files = append(v.files, f)
		v.items = append(v.items, a.items[old])
	}
	return v, nil
}

func (a *Archive) cacheChunk(idx int, chunk []byte) []byte {
	if a.maxChunkBytes == 0 {
		stored, _ := a.chunks.LoadOrStore(idx, chunk)
		return stored.([]byte)
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if stored, ok := a.chunks.Load(idx); ok {
		return stored.([]byte)
	}
	n := int64(len(chunk))
	if n > a.maxChunkBytes {
		return chunk
	}
	if a.cachedChunkBytes+n > a.maxChunkBytes {
		for a.cachedChunkBytes+n > a.maxChunkBytes && a.chunkOrder.Len() > 0 {
			e := a.chunkOrder.Front()
			old := e.Value.(int)
			if value, ok := a.chunks.LoadAndDelete(old); ok {
				a.cachedChunkBytes -= int64(len(value.([]byte)))
			}
			delete(a.chunkPositions, old)
			a.chunkOrder.Remove(e)
		}
	}
	if a.chunkPositions == nil {
		a.chunkPositions = make(map[int]*list.Element)
	}
	a.chunks.Store(idx, chunk)
	a.chunkPositions[idx] = a.chunkOrder.PushBack(idx)
	a.cachedChunkBytes += n
	return chunk
}

func (a *Archive) cacheText(idx int, text string) string {
	if a.maxTexts == 0 {
		stored, _ := a.texts.LoadOrStore(idx, text)
		return stored.(string)
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if stored, ok := a.texts.Load(idx); ok {
		return stored.(string)
	}
	if a.textOrder.Len() >= a.maxTexts {
		e := a.textOrder.Front()
		old := e.Value.(int)
		a.texts.Delete(old)
		delete(a.textPositions, old)
		a.textOrder.Remove(e)
	}
	if a.textPositions == nil {
		a.textPositions = make(map[int]*list.Element)
	}
	a.texts.Store(idx, text)
	a.textPositions[idx] = a.textOrder.PushBack(idx)
	return text
}

func (a *Archive) cachedChunk(idx int) ([]byte, bool) {
	if a.maxChunkBytes == 0 {
		v, ok := a.chunks.Load(idx)
		if !ok {
			return nil, false
		}
		return v.([]byte), true
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	v, ok := a.chunks.Load(idx)
	if !ok {
		return nil, false
	}
	if e := a.chunkPositions[idx]; e != nil {
		a.chunkOrder.MoveToBack(e)
	}
	return v.([]byte), true
}

func (a *Archive) cachedText(idx int) (string, bool) {
	if a.maxTexts == 0 {
		v, ok := a.texts.Load(idx)
		if !ok {
			return "", false
		}
		return v.(string), true
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	v, ok := a.texts.Load(idx)
	if !ok {
		return "", false
	}
	if e := a.textPositions[idx]; e != nil {
		a.textOrder.MoveToBack(e)
	}
	return v.(string), true
}
