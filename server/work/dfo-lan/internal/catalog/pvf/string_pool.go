package pvf

import (
	"bytes"
	"compress/zlib"
	"container/list"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

const stringPoolBlockSize = 64 * 1024
const stringPoolCacheBytes = 16 * 1024 * 1024

type compressedPool struct {
	size   int
	blocks [][]byte
}
type stringPoolState struct {
	a, w                     []byte
	compressedA, compressedW *compressedPool
	compacted                bool
}
type poolBlockKey struct {
	wide  bool
	index int
}
type poolBlock struct {
	key  poolBlockKey
	data []byte
}
type poolFailure struct{ err error }

// Views share this owner, never the parent Archive. State publication is atomic:
// readers already holding raw bytes can finish while new reads use compressed blocks.
type runtimeStringPools struct {
	state      atomic.Pointer[stringPoolState]
	failure    atomic.Pointer[poolFailure]
	compactMu  sync.Mutex
	cacheMu    sync.Mutex
	cache      map[poolBlockKey]*list.Element
	order      list.List
	cacheBytes int
}

func newRuntimeStringPools(a, w []byte) *runtimeStringPools {
	p := &runtimeStringPools{cache: make(map[poolBlockKey]*list.Element)}
	p.state.Store(&stringPoolState{a: a, w: w})
	return p
}

func compressPool(data []byte) ([]byte, *compressedPool, error) {
	if len(data) < stringPoolBlockSize {
		return data, nil, nil
	}
	pool := &compressedPool{size: len(data)}
	var buffer bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buffer, zlib.BestSpeed)
	if err != nil {
		return nil, nil, err
	}
	for offset := 0; offset < len(data); offset += stringPoolBlockSize {
		buffer.Reset()
		writer.Reset(&buffer)
		if _, err = writer.Write(data[offset:min(offset+stringPoolBlockSize, len(data))]); err != nil {
			return nil, nil, err
		}
		if err = writer.Close(); err != nil {
			return nil, nil, err
		}
		pool.blocks = append(pool.blocks, append([]byte(nil), buffer.Bytes()...))
	}
	return nil, pool, nil
}

func (p *runtimeStringPools) compact() error {
	p.compactMu.Lock()
	defer p.compactMu.Unlock()
	if err := p.err(); err != nil {
		return err
	}
	old := p.state.Load()
	if old.compacted {
		return nil
	}
	next := &stringPoolState{compacted: true}
	var err error
	if next.a, next.compressedA, err = compressPool(old.a); err != nil {
		return err
	}
	if next.w, next.compressedW, err = compressPool(old.w); err != nil {
		return err
	}
	p.state.Store(next)
	return nil
}

func (p *runtimeStringPools) err() error {
	if p == nil {
		return nil
	}
	if failure := p.failure.Load(); failure != nil {
		return failure.err
	}
	return nil
}

func (a *Archive) poolError() error {
	if a == nil {
		return nil
	}
	return a.stringPools.err()
}

// CompactRuntimeStrings compresses the immutable pools after catalog preparation.
// Memory archives/editing tools keep their original buffers and behavior.
func (a *Archive) CompactRuntimeStrings() error {
	if a == nil || a.closed.Load() {
		return fmt.Errorf("PVF source is closed")
	}
	if a.stringPools == nil {
		return nil
	}
	return a.stringPools.compact()
}

func (p *runtimeStringPools) block(pool *compressedPool, key poolBlockKey) (data []byte, err error) {
	defer func() {
		if err != nil {
			p.failure.CompareAndSwap(nil, &poolFailure{err})
		}
	}()
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	if err = p.err(); err != nil {
		return nil, err
	}
	if key.index < 0 || key.index >= len(pool.blocks) {
		return nil, fmt.Errorf("%w: string pool block index", ErrInvalidArchive)
	}
	if entry := p.cache[key]; entry != nil {
		p.order.MoveToBack(entry)
		return entry.Value.(poolBlock).data, nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(pool.blocks[key.index]))
	if err != nil {
		return nil, fmt.Errorf("%w: string pool block %v: %w", ErrInvalidArchive, key, err)
	}
	want := min(stringPoolBlockSize, pool.size-key.index*stringPoolBlockSize)
	defer reader.Close()
	data = make([]byte, want)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, fmt.Errorf("%w: string pool block %v: %w", ErrInvalidArchive, key, err)
	}
	var extra [1]byte
	n, endErr := reader.Read(extra[:])
	if n != 0 || endErr != io.EOF {
		return nil, fmt.Errorf("%w: string pool block %v size", ErrInvalidArchive, key)
	}
	for p.cacheBytes+len(data) > stringPoolCacheBytes && p.order.Len() > 0 {
		old := p.order.Front()
		value := old.Value.(poolBlock)
		delete(p.cache, value.key)
		p.cacheBytes -= len(value.data)
		p.order.Remove(old)
	}
	p.cache[key] = p.order.PushBack(poolBlock{key, data})
	p.cacheBytes += len(data)
	return data, nil
}

// bytesAt preserves the old pool reader's behavior: ANSI needs a terminator;
// UTF-16 may end at EOF, and a dangling final byte is ignored.
func (p *runtimeStringPools) bytesAt(pool *compressedPool, wide bool, start int) ([]byte, error) {
	if start < 0 || start >= pool.size {
		return nil, nil
	}
	var joined []byte
	for offset := start; offset < pool.size; {
		data, err := p.block(pool, poolBlockKey{wide, offset / stringPoolBlockSize})
		if err != nil {
			return nil, err
		}
		data = data[offset%stringPoolBlockSize:]
		end := -1
		if wide {
			for i := 0; i+1 < len(data); i += 2 {
				if data[i] == 0 && data[i+1] == 0 {
					end = i
					break
				}
			}
		} else {
			end = bytes.IndexByte(data, 0)
		}
		if end >= 0 {
			if joined == nil {
				return data[:end], nil
			}
			return append(joined, data[:end]...), nil
		}
		offset += len(data)
		if wide {
			data = data[:len(data)/2*2]
		}
		joined = append(joined, data...)
	}
	if !wide {
		return nil, nil
	}
	return joined, nil
}

func (p *runtimeStringPools) resolve(offset int) string {
	if offset < 0 || p.err() != nil {
		return ""
	}
	state := p.state.Load()
	wide := offset&1 != 0
	start := offset >> 1
	pool := state.compressedA
	if wide {
		start *= 2
		pool = state.compressedW
		if pool == nil {
			return readUTF16String(state.w, start)
		}
	} else if pool == nil {
		return readUTF8String(state.a, start)
	}
	data, err := p.bytesAt(pool, wide, start)
	if err != nil {
		p.failure.CompareAndSwap(nil, &poolFailure{err})
		return ""
	}
	if wide {
		return decodeUTF16LE(data)
	}
	// Reuse the original UTF-8/GB18030 fallback without changing its semantics.
	return readUTF8String(append(append([]byte(nil), data...), 0), 0)
}

func (p *runtimeStringPools) expanded() ([]byte, []byte, error) {
	if err := p.err(); err != nil {
		return nil, nil, err
	}
	state := p.state.Load()
	expand := func(raw []byte, pool *compressedPool, wide bool) ([]byte, error) {
		if pool == nil {
			return append([]byte(nil), raw...), nil
		}
		out := make([]byte, 0, pool.size)
		for i := range pool.blocks {
			data, err := p.block(pool, poolBlockKey{wide, i})
			if err != nil {
				return nil, err
			}
			out = append(out, data...)
		}
		return out, nil
	}
	a, err := expand(state.a, state.compressedA, false)
	if err != nil {
		return nil, nil, err
	}
	w, err := expand(state.w, state.compressedW, true)
	return a, w, err
}
