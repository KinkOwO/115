package derivedcache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetentionProtectsActiveRecentAndForeignFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	family := Family{Prefix: "owned-", Magic: "TESTRET1"}
	format := Format{Magic: family.Magic, PackedLimit: 1024, RawLimit: 1024}
	var files []string
	for i := 0; i < 6; i++ {
		key := sha256.Sum256([]byte(fmt.Sprint(i)))
		p := filepath.Join(dir, fmt.Sprintf("owned-%x.pvfc", key))
		files = append(files, p)
		if e := Save(p, key, format, func(w io.Writer) error { _, e := w.Write([]byte("payload")); return e }); e != nil {
			t.Fatal(e)
		}
		stamp := now.Add(-time.Duration(6-i) * 48 * time.Hour)
		if i == 5 {
			stamp = now
		}
		os.Chtimes(p, stamp, stamp)
	}
	foreign := filepath.Join(dir, "owned-"+fmt.Sprintf("%064x", 7)+".pvfc")
	os.WriteFile(foreign, make([]byte, 88), 0600)
	nested := filepath.Join(dir, "nested")
	os.Mkdir(nested, 0700)
	os.WriteFile(filepath.Join(nested, "save"), []byte("player"), 0600)
	result, e := Prune(dir, []Family{family}, map[string]bool{files[0]: true}, Retention{MaxBytes: 1 << 20, KeepPerFamily: 3, Grace: 24 * time.Hour}, now)
	if e != nil || result.Removed != 2 {
		t.Fatal(result, e)
	}
	for _, p := range []string{files[0], files[3], files[4], files[5], foreign, filepath.Join(nested, "save")} {
		if _, e = os.Stat(p); e != nil {
			t.Fatal("removed protected/foreign file", p, e)
		}
	}
	result, e = Prune(dir, []Family{family}, map[string]bool{files[0]: true}, Retention{MaxBytes: 1, KeepPerFamily: 3, Grace: 24 * time.Hour}, now)
	if e != nil || result.Removed != 2 {
		t.Fatal("budget pruning failed", result, e)
	}
	for _, p := range []string{files[0], files[5]} {
		if _, e = os.Stat(p); e != nil {
			t.Fatal("active/recent removed")
		}
	}
	if result, e = Prune("-", []Family{family}, nil, Retention{}, now); e != nil || result.Removed != 0 {
		t.Fatal("disabled retention")
	}
}
