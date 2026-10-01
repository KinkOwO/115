package pvf

import (
	"fmt"
	"sort"
)

// ReadOnlyView retains only the requested file metadata. It shares immutable,
// checksum-verified source bytes and string pools; releasing the original
// Archive therefore cannot change or invalidate a later lazy read. File.Index
// is local to the view. Editing APIs reject views because indices were remapped.
// Expanded body caches are bounded to 64 MiB and text caches to 2048 entries.
func (a *Archive) ReadOnlyView(paths []string) (*Archive, error) {
	if a == nil {
		return nil, fmt.Errorf("nil PVF archive")
	}
	indices := make(map[int]bool, len(paths))
	for _, p := range paths {
		i, ok := a.pathIdx[pathKey(p)]
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
		files: make([]File, 0, len(order)), items: make([]fileItem, 0, len(order)),
		pathIdx: make(map[string]int, len(order)), readOnlyView: true,
		maxChunkBytes: 64 * 1024 * 1024, maxTexts: 2048}
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
		a.chunks.Clear()
		a.cachedChunkBytes = 0
	}
	a.chunks.Store(idx, chunk)
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
	count := 0
	a.texts.Range(func(_, _ any) bool { count++; return true })
	if count >= a.maxTexts {
		a.texts.Clear()
	}
	a.texts.Store(idx, text)
	return text
}
