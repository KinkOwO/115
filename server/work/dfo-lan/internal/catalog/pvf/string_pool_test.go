package pvf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"unicode/utf16"
)

func writePoolWide(dst []byte, offset int, text string, terminated bool) {
	for _, unit := range utf16.Encode([]rune(text)) {
		binary.LittleEndian.PutUint16(dst[offset:], unit)
		offset += 2
	}
	if terminated {
		binary.LittleEndian.PutUint16(dst[offset:], 0)
	}
}

func TestCompressedPoolsPreserveTextBoundariesAndFallback(t *testing.T) {
	a := make([]byte, 4*stringPoolBlockSize+7)
	w := make([]byte, len(a))
	copy(a[17:], []byte{0xd6, 0xd0, 0xce, 0xc4, 0}) // GB18030 Chinese.
	copy(a[stringPoolBlockSize-3:], []byte("boundary汉字\x00"))
	copy(a[len(a)-9:], bytes.Repeat([]byte{'x'}, 9)) // ANSI without NUL stays empty.
	writePoolWide(w, stringPoolBlockSize-4, "A😀汉字", true)
	writePoolWide(w, 2*stringPoolBlockSize+8, string(bytes.Repeat([]byte{'y'}, 35000)), true)
	writePoolWide(w, len(w)-9, "尾尾尾尾", false) // Unterminated UTF-16 and dangling byte.
	offsets := []int{-1, 0, 34, (stringPoolBlockSize - 3) * 2, (len(a) - 9) * 2,
		stringPoolBlockSize - 3, stringPoolBlockSize - 1, 2*stringPoolBlockSize + 9, len(w) - 8,
		len(a) * 2, len(w) + 1}
	want := map[int]string{}
	for _, offset := range offsets {
		old := &Archive{strA: a, strW: w}
		want[offset] = old.resolveString(offset)
	}
	p := newRuntimeStringPools(a, w)
	if err := p.compact(); err != nil {
		t.Fatal(err)
	}
	if p.state.Load().a != nil || p.state.Load().w != nil {
		t.Fatal("raw pools retained after publication")
	}
	for _, offset := range offsets {
		if got := p.resolve(offset); got != want[offset] {
			t.Fatalf("offset %d: %q versus %q", offset, got, want[offset])
		}
	}
	if got := p.resolve(stringPoolBlockSize - 3); got != "A😀汉字" {
		t.Fatal("split surrogate lost", got)
	}
	gotA, gotW, err := p.expanded()
	if err != nil || !bytes.Equal(gotA, a) || !bytes.Equal(gotW, w) {
		t.Fatal("pool audit bytes differ", err)
	}
}

func TestCompressedPoolLRUIsBoundedAndDoesNotMutateHeldBlocks(t *testing.T) {
	w := make([]byte, stringPoolCacheBytes+2*stringPoolBlockSize)
	for offset := 0; offset < len(w); offset += stringPoolBlockSize {
		writePoolWide(w, offset, string(rune('a'+offset/stringPoolBlockSize%26)), true)
	}
	p := newRuntimeStringPools(nil, w)
	if err := p.compact(); err != nil {
		t.Fatal(err)
	}
	pool := p.state.Load().compressedW
	held, err := p.block(pool, poolBlockKey{true, 1})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < stringPoolCacheBytes/stringPoolBlockSize; i++ {
		p.resolve(i*stringPoolBlockSize + 1)
	}
	p.resolve(1) // Keep the active first block hot.
	p.resolve(stringPoolCacheBytes + 1)
	if p.cache[poolBlockKey{true, 0}] == nil || p.cache[poolBlockKey{true, 1}] != nil || p.cacheBytes > stringPoolCacheBytes {
		t.Fatal("did not evict only oldest block", p.cacheBytes)
	}
	if !bytes.Equal(held, w[stringPoolBlockSize:2*stringPoolBlockSize]) {
		t.Fatal("LRU eviction modified held bytes")
	}
}

func TestCompressedPoolsPublishSafelyDuringConcurrentReads(t *testing.T) {
	w := make([]byte, 3*stringPoolBlockSize)
	writePoolWide(w, stringPoolBlockSize-4, "A😀文本", true)
	p := newRuntimeStringPools(nil, w)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 1000 {
				if got := p.resolve(stringPoolBlockSize - 3); got != "A😀文本" {
					t.Errorf("concurrent text changed: %q", got)
					return
				}
			}
		})
	}
	wg.Go(func() {
		for range 3 {
			if err := p.compact(); err != nil {
				t.Error(err)
			}
		}
	})
	wg.Wait()
	if p.err() != nil || p.resolve(stringPoolBlockSize-3) != "A😀文本" {
		t.Fatal("published state unusable", p.err())
	}
}

func TestCompressedPoolFailureRefusesPartialTokensAndCachedReads(t *testing.T) {
	p := newRuntimeStringPools(nil, make([]byte, stringPoolBlockSize))
	if err := p.compact(); err != nil {
		t.Fatal(err)
	}
	p.state.Load().compressedW.blocks[0] = []byte{1, 2, 3}
	a := &Archive{stringPools: p}
	raw := []byte{3, 1, 0, 0, 0}
	if cells, err := a.TokensFromRaw(raw); !errors.Is(err, ErrInvalidArchive) || cells != nil {
		t.Fatal("published partial tokens", cells, err)
	}
	if _, err := a.ReadText("anything"); !errors.Is(err, ErrInvalidArchive) {
		t.Fatal("text ignored pool failure", err)
	}
	if _, err := a.FileRaw(0); !errors.Is(err, ErrInvalidArchive) {
		t.Fatal("raw ignored pool failure", err)
	}
	if _, err := a.TokensFromRaw([]byte{0, 0, 0, 0, 0}); !errors.Is(err, ErrInvalidArchive) {
		t.Fatal("later numeric script ignored failure", err)
	}
	if err := a.IterateFiles(func(File) error { t.Fatal("visited bad pool"); return nil }); !errors.Is(err, ErrInvalidArchive) {
		t.Fatal(err)
	}
	if _, _, err := p.expanded(); !errors.Is(err, ErrInvalidArchive) {
		t.Fatal("expanded partial pool", err)
	}
}

func TestCompressedPoolRejectsDamagedChecksumAndWrongDecodedLength(t *testing.T) {
	for _, kind := range []string{"checksum", "truncated", "short", "oversized", "missing"} {
		t.Run(kind, func(t *testing.T) {
			p := newRuntimeStringPools(nil, make([]byte, stringPoolBlockSize))
			if err := p.compact(); err != nil {
				t.Fatal(err)
			}
			pool := p.state.Load().compressedW
			switch kind {
			case "checksum":
				block := pool.blocks[0]
				block[len(block)-1] ^= 1
			case "truncated":
				pool.blocks[0] = pool.blocks[0][:len(pool.blocks[0])-3]
			case "short":
				_, shorter, err := compressPool(make([]byte, stringPoolBlockSize+1))
				if err != nil {
					t.Fatal(err)
				}
				pool.blocks[0] = shorter.blocks[1] // Only one decoded byte.
			case "oversized":
				pool.size--
			case "missing":
				pool.blocks = nil
			}
			key := poolBlockKey{wide: true}
			data, err := p.block(pool, key)
			if data != nil || !errors.Is(err, ErrInvalidArchive) || p.err() == nil || p.cacheBytes != 0 {
				t.Fatal("damaged block was accepted or cached", len(data), err, p.cacheBytes)
			}
		})
	}
}
