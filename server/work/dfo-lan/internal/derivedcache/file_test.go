package derivedcache

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryEnvelopeBoundsAndTrailingStreams(t *testing.T) {
	f := Format{Magic: "TESTBIN1", PackedLimit: 4096, RawLimit: 4096}
	p := filepath.Join(t.TempDir(), "binary.pvfc")
	key := sha256.Sum256([]byte("identity"))
	encode := func(w io.Writer) error { _, err := io.WriteString(w, "native metadata"); return err }
	decode := func(r io.Reader) error {
		b := make([]byte, len("native metadata"))
		_, err := io.ReadFull(r, b)
		return err
	}
	if err := Save(p, key, f, encode); err != nil {
		t.Fatal(err)
	}
	if hit, err := Load(p, key, f, decode); !hit || err != nil {
		t.Fatal(hit, err)
	}
	for _, bad := range []Format{{Magic: "short", PackedLimit: 4096, RawLimit: 4096}, {Magic: f.Magic, PackedLimit: 4096, RawLimit: -1}, {Magic: f.Magic, PackedLimit: 1, RawLimit: 4096}, {Magic: f.Magic, PackedLimit: 4096, RawLimit: 1}} {
		called := false
		if hit, err := Load(p, key, bad, func(r io.Reader) error { called = true; return decode(r) }); hit || err == nil || called {
			t.Fatal("budget/format reached decoder", hit, err, called)
		}
	}
	if err := Save(p, key, Format{Magic: f.Magic, PackedLimit: 4096, RawLimit: 1}, encode); err == nil {
		t.Fatal("raw write budget ignored")
	}
	if err := Save(p, key, f, func(w io.Writer) error { return nil }); err == nil {
		t.Fatal("empty publication accepted")
	}
	if hit, err := Load(p, key, f, decode); !hit || err != nil {
		t.Fatal("failed writer replaced valid image", hit, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, []byte("trailing compressed junk")...)
	sum := sha256.Sum256(b[HeaderSize:])
	copy(b[40:72], sum[:])
	binary.LittleEndian.PutUint64(b[72:80], uint64(len(b)-HeaderSize))
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if hit, err := Load(p, key, f, decode); hit || err == nil || !strings.Contains(err.Error(), "trailing compressed") {
		t.Fatal("extra compressed bytes accepted", hit, err)
	}
}
