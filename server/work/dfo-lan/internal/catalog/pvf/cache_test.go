package pvf

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestReleaseReadCachesPreservesArchiveAndRereads(t *testing.T) {
	raw := []byte{'O', 0, 'K', 0}
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := append([]byte(nil), compressed.Bytes()...)
	decryptProtected("mAIn", encoded)
	expected := append([]byte(nil), encoded...)
	a := &Archive{
		format: FormatDFO20260901, data: encoded,
		items:   []fileItem{{chunkIndex: 0, dataOffset: 0, dataSize: len(raw), dataType: 3}},
		groups:  []groupItem{{compressedSize: len(encoded), originalSize: len(raw)}},
		pathIdx: map[string]int{"entry.txt": 0},
	}
	first, err := a.ReadText("entry.txt")
	if err != nil || first != "OK" {
		t.Fatalf("initial read: %q %v", first, err)
	}
	if s := a.Snapshot(); s.CachedChunks != 1 || s.CachedTexts != 1 {
		t.Fatal(s)
	}
	a.ReleaseReadCaches()
	if s := a.Snapshot(); s.CachedChunks != 0 || s.CachedTexts != 0 {
		t.Fatal(s)
	}
	second, err := a.ReadText("entry.txt")
	if err != nil || second != first || !bytes.Equal(a.data, expected) {
		t.Fatalf("reread: %q %v", second, err)
	}
}
