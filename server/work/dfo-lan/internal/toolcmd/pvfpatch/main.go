// Prepare one reviewed UI script replacement in a separate inner PVF.
package pvfpatch

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
)

func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "original inner archive, read-only")
	entry := flag.String("entry", "", "exact existing source entry")
	expect := flag.String("expected-sha256", "", "required original entry checksum")
	replacement := flag.String("replacement", "", "reviewed replacement raw script")
	out := flag.String("output", "", "NEW output path, never overwritten")
	flag.Parse()
	if len(*expect) != 64 || *out == "" {
		log.Fatal("entry checksum and new output required")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	raw, e := a.ReadRaw(*entry)
	if e != nil {
		log.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != *expect {
		log.Fatal("source entry checksum mismatch")
	}
	p, e := os.ReadFile(*replacement)
	if e != nil {
		log.Fatal(e)
	}
	if e = a.WriteEntryCopy(*out, *entry, p); e != nil {
		log.Fatal(e)
	}
	fmt.Printf("PVF_ENTRY_COPY_WRITTEN entry=%s source_unchanged=true output=%s\n", *entry, *out)
}
