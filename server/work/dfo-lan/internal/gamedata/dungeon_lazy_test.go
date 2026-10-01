package gamedata

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// Explicit local parity proof against all native maps; no player storage.
func TestRuntimeDungeonsLocalArchiveParity(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete lazy map parity")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w, err := catalog.ImportWorldRuntime(s.archive)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Training []uint32 `json:"training_dungeons"`
		Disabled []uint32 `json:"disabled_full_dungeons"`
	}
	b, err := os.ReadFile("../../configs/pvf-scene-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &policy); err != nil {
		t.Fatal(err)
	}
	excluded := append(policy.Training, policy.Disabled...)
	eager, err := s.FullDungeons(w, excluded)
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := s.RuntimeFullDungeons(w, excluded)
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.CloseMapSource()
	if !reflect.DeepEqual(eager.Dungeons, lazy.Dungeons) || !reflect.DeepEqual(eager.Skipped, lazy.Skipped) || len(eager.Maps) != len(lazy.Maps) {
		t.Fatal("changed dungeon admission, layouts or skipped order")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	ids := make([]uint32, 0, len(lazy.Maps))
	for id, metadata := range lazy.Maps {
		if metadata.Cells != nil || metadata.Path != eager.Maps[id].Path || metadata.SHA256 != eager.Maps[id].SHA256 {
			t.Fatalf("map %d retained tokens or changed identity", id)
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	begin := time.Now()
	for _, id := range ids {
		got, err := lazy.MapScript(id)
		if err != nil || !reflect.DeepEqual(got, eager.Maps[id]) {
			t.Fatalf("map %d differs after parent source close: %v", id, err)
		}
		if lazy.Maps[id].Cells != nil {
			t.Fatal("read populated public metadata")
		}
	}
	t.Logf("dungeons=%d maps=%d skipped=%d complete map read/compare=%s", len(lazy.Dungeons), len(ids), len(lazy.Skipped), time.Since(begin))
	var wg sync.WaitGroup
	for _, id := range ids[:min(8, len(ids))] {
		wg.Go(func() {
			for range 3 {
				got, err := lazy.MapScript(id)
				if err != nil || !reflect.DeepEqual(got, eager.Maps[id]) {
					t.Errorf("concurrent read %d: %v", id, err)
				}
				lazy.ReleaseMapReadCache()
			}
		})
	}
	wg.Wait()
	if len(ids) == 0 {
		t.Fatal("no imported maps")
	}
	id := ids[0]
	expected := lazy.Maps[id]
	broken := expected
	broken.SHA256 = strings.Repeat("0", 64)
	lazy.Maps[id] = broken
	if _, err = lazy.MapScript(id); err == nil {
		t.Fatal("accepted corrupted expected map hash")
	}
	lazy.Maps[id] = expected
	if err = lazy.CloseMapSource(); err != nil {
		t.Fatal(err)
	}
	if _, err = lazy.MapScript(id); err == nil {
		t.Fatal("accepted map read from closed source")
	}
}
