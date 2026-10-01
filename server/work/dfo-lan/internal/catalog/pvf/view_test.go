package pvf

import (
	"bytes"
	"compress/zlib"
	"path/filepath"
	"sync"
	"testing"
)

func TestReadOnlyViewRemapsEntriesAndPreservesReads(t *testing.T) {
	raw := []byte{'A', 0, 'B', 0}
	var packed bytes.Buffer
	z := zlib.NewWriter(&packed)
	z.Write(raw)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	data := packed.Bytes()
	decryptProtected("mAIn", data)
	a := &Archive{format: FormatDFO20260901, data: data,
		files:   []File{{Index: 0, ArchivePath: "first.txt", DataType: 3}, {Index: 1, ArchivePath: "second.txt", DataType: 3}},
		items:   []fileItem{{chunkIndex: 0, dataSize: 2, dataType: 3}, {chunkIndex: 0, dataOffset: 2, dataSize: 2, dataType: 3}},
		groups:  []groupItem{{compressedSize: len(data), originalSize: len(raw)}},
		pathIdx: map[string]int{"first.txt": 0, "second.txt": 1}}
	v, err := a.ReadOnlyView([]string{"SECOND.txt", "second.txt"})
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := a.ReadOnlyViewIndices([]int{1, 1})
	if err != nil || indexed.FileCount() != 1 || indexed.FindFileIndex("second.txt") != 0 {
		t.Fatal("indexed view remap", err)
	}
	for _, indices := range [][]int{{-1}, {2}} {
		if _, err = a.ReadOnlyViewIndices(indices); err == nil {
			t.Fatal("invalid native index accepted")
		}
	}
	if v.FileCount() != 1 || v.FindFileIndex("second.txt") != 0 || v.FindFileIndex("first.txt") != -1 {
		t.Fatal("view metadata was not compacted")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				text, e := v.ReadText("second.txt")
				if e != nil || text != "B" {
					t.Errorf("read=%q error=%v", text, e)
				}
				v.ReleaseReadCaches()
			}
		}()
	}
	wg.Wait()
	output := filepath.Join(t.TempDir(), "forbidden.pvf")
	if err := v.WriteEntryCopy(output, "second.txt", make([]byte, 5)); err == nil {
		t.Fatal("view allowed rewriting")
	}
	if _, err := v.Localize(output, func(s string) (string, bool) { return s, true }); err == nil {
		t.Fatal("view allowed localization")
	}
	if _, err := a.ReadOnlyView([]string{"missing.txt"}); err == nil {
		t.Fatal("missing view entry accepted")
	}
}

func TestViewCacheBoundAndReread(t *testing.T) {
	a := &Archive{maxChunkBytes: 4, maxTexts: 1}
	a.cacheChunk(0, []byte{1, 2, 3})
	a.cacheChunk(1, []byte{4, 5, 6})
	if _, ok := a.chunks.Load(0); ok {
		t.Fatal("old chunks exceeded cache limit")
	}
	if a.cachedChunkBytes != 3 {
		t.Fatal(a.cachedChunkBytes)
	}
	a.cacheChunk(2, make([]byte, 5))
	if _, ok := a.chunks.Load(2); ok {
		t.Fatal("oversized group retained")
	}
	a.cacheText(0, "first")
	a.cacheText(1, "second")
	if _, ok := a.texts.Load(0); ok {
		t.Fatal("text cache limit ignored")
	}
	a.ReleaseReadCaches()
	if a.cachedChunkBytes != 0 {
		t.Fatal("cache byte accounting not reset")
	}
}
