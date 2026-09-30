// apocalypseimport decodes the compiled `.ctp` tables that drive the legion /
// apocalypse subsystem and writes the truth the server consumes:
//
//	configs/apocalypse.generated.json
//
// The input is read straight out of the frozen client PVF, so the generated
// file can always be reproduced and diffed against the source hash. Nothing is
// invented: every field is a column read, and fields whose meaning is not yet
// proven keep their positional form plus a note.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	source := flag.String("source", "", "source PVF, opened read-only")
	out := flag.String("output", "", "output JSON path")
	flag.Parse()
	if *source == "" {
		log.Fatal("missing -source")
	}
	if *out == "" {
		log.Fatal("missing -output")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	doc, err := catalog.ImportApocalypse(a)
	if err != nil {
		log.Fatal(err)
	}
	duties := doc.Duties
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, append(b, '\n'), 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("apocalypse.ctp sha256=%s bytes=%d records=%d poolTags=%d\n",
		doc.SHA256[:16], doc.Bytes, doc.RecordCount, len(doc.PoolTags))
	fmt.Printf("phase clock: %s\n", formatClock(doc.PhaseClock))
	for _, op := range doc.Operations {
		fmt.Printf("operation row=%-3d index=%s type=%s allowCoin=%v gateSchedule=%v reward=%s\n",
			op.Row, opt(op.Index), opt(op.Type), op.AllowCoin, op.GateSchedule, rewardLabel(op.Reward))
	}
	fmt.Printf("duties: %s sha256=%s rows=%d\n", catalog.ApocalypseDutySource, duties.SHA256[:16], len(duties.Records))
	fmt.Printf("wrote %s\n", *out)
}

func formatClock(clock []catalog.ApocalypsePhase) string {
	parts := make([]string, 0, len(clock))
	for _, e := range clock {
		parts = append(parts, fmt.Sprintf("phase%d=%gs", e.Phase, e.Seconds))
	}
	return strings.Join(parts, " ")
}

func opt(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func rewardLabel(r *catalog.ApocalypseReward) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%s%v", r.Label, r.Values)
}
