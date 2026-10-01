package pvf

import "fmt"

func (a *Archive) readTextIndex(idx int) (string, error) {
	if err := a.poolError(); err != nil {
		return "", err
	}
	if a.closed.Load() {
		return "", fmt.Errorf("%w: archive is closed", ErrInvalidArchive)
	}
	if cached, ok := a.cachedText(idx); ok {
		return cached, nil
	}
	raw, err := a.readRawIndex(idx)
	if err != nil {
		return "", err
	}
	var text string
	switch a.itemAt(idx).dataType {
	case 1:
		text = a.decodeScript(raw)
	case 3:
		text = decodeUTF16LE(raw)
	default:
		text = ""
	}
	if err := a.poolError(); err != nil {
		return "", err
	}
	return a.cacheText(idx, text), nil
}

func (a *Archive) readRawIndex(idx int) ([]byte, error) {
	if err := a.poolError(); err != nil {
		return nil, err
	}
	count := len(a.items)
	if a.compactDirectory {
		count = a.FileCount()
	}
	if idx < 0 || idx >= count || a.closed.Load() {
		return nil, fmt.Errorf("%w: file index %d", ErrFileNotFound, idx)
	}
	item := a.itemAt(idx)
	chunk, err := a.chunk(item.chunkIndex)
	if err != nil {
		return nil, err
	}
	if item.dataOffset < 0 || item.dataSize < 0 || item.dataOffset+item.dataSize > len(chunk) {
		return nil, fmt.Errorf("%w: file %d data range is invalid", ErrInvalidArchive, idx)
	}
	out := make([]byte, item.dataSize)
	copy(out, chunk[item.dataOffset:item.dataOffset+item.dataSize])
	return out, nil
}

func (a *Archive) chunk(idx int) ([]byte, error) {
	if idx < 0 || idx >= len(a.groups) {
		return nil, fmt.Errorf("%w: chunk %d is out of range", ErrInvalidArchive, idx)
	}
	if a.closed.Load() {
		return nil, fmt.Errorf("%w: archive is closed", ErrInvalidArchive)
	}
	if cached, ok := a.cachedChunk(idx); ok {
		return cached, nil
	}
	prev := 0
	if idx > 0 {
		prev = a.groups[idx-1].compressedSize
	}
	curr := a.groups[idx].compressedSize
	if curr <= prev {
		return nil, fmt.Errorf("%w: chunk %d compressed range is invalid", ErrInvalidArchive, idx)
	}
	start := a.bodyOff + prev
	size := curr - prev
	if start < 0 || size <= 0 || int64(start)+int64(size) > a.sourceSize() || a.closed.Load() {
		return nil, fmt.Errorf("%w: chunk %d exceeds archive", ErrInvalidArchive, idx)
	}
	var encrypted []byte
	if a.backing != nil {
		encrypted = make([]byte, size)
		if _, err := a.backing.file.ReadAt(encrypted, int64(start)); err != nil {
			return nil, fmt.Errorf("read PVF chunk %d: %w", idx, err)
		}
	} else {
		encrypted = append([]byte(nil), a.data[start:start+size]...)
	}
	switch a.format {
	case FormatDFO20260901:
		decryptProtected("mAIn", encrypted)
	case FormatProtectedNKPI:
		decryptProtected("bODy", encrypted)
	default:
		decrypt("BodY", encrypted)
	}
	chunk, err := zlibBytes(encrypted)
	if err != nil {
		return nil, fmt.Errorf("%w: chunk %d decompress: %w", ErrInvalidArchive, idx, err)
	}
	if want := a.groups[idx].originalSize; want >= 0 && want != len(chunk) {
		return nil, fmt.Errorf("%w: chunk %d original size mismatch: want %d got %d", ErrInvalidArchive, idx, want, len(chunk))
	}
	return a.cacheChunk(idx, chunk), nil
}
