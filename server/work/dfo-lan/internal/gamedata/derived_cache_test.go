package gamedata

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestDerivedCacheConcurrentPublish(t *testing.T) {
	key := sha256.Sum256([]byte("concurrent"))
	p := derivedPath(t.TempDir(), key)
	encode := func(e *json.Encoder) error { return e.Encode([]int{1, 2, 3}) }
	if err := saveDerived(p, key, encode); err != nil {
		t.Fatal(err)
	}
	decode := func(d *json.Decoder) error {
		var v []int
		if err := d.Decode(&v); err != nil {
			return err
		}
		if !reflect.DeepEqual(v, []int{1, 2, 3}) {
			return fmt.Errorf("partial published catalog: %v", v)
		}
		return nil
	}
	var wg sync.WaitGroup
	failures := make(chan error, 48)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(writer bool) {
			defer wg.Done()
			for i := 0; i < 12; i++ {
				var err error
				if writer {
					err = saveDerived(p, key, encode)
				} else {
					_, err = loadDerived(p, key, decode)
				}
				// Windows can temporarily deny a rename/open while another
				// reader holds the file. This must remain a cache miss;
				// checksum, decoding and partial-data errors are not allowed.
				var pathErr *os.PathError
				var linkErr *os.LinkError
				transientRename := writer && errors.As(err, &linkErr) && errors.Is(err, os.ErrPermission)
				if err != nil && !errors.As(err, &pathErr) && !transientRename {
					failures <- err
				}
			}
		}(worker < 2)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if hit, err := loadDerived(p, key, decode); !hit || err != nil {
		t.Fatal("no valid final cache", hit, err)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(p), ".joint-items-*.tmp"))
	if len(files) != 0 {
		t.Fatal("concurrent writers leaked temporary files", files)
	}
}

func TestDerivedCacheIntegrityAndIdentity(t *testing.T) {
	key := sha256.Sum256([]byte("source-parser-policy-domains"))
	p := derivedPath(t.TempDir(), key)
	encode := func(e *json.Encoder) error { return e.Encode([]int{1, 2, 3}) }
	called := false
	decode := func(d *json.Decoder) error { called = true; var v []int; return d.Decode(&v) }
	if hit, err := loadDerived(p, key, decode); hit || err != nil {
		t.Fatal(hit, err)
	}
	if err := saveDerived(p, key, encode); err != nil {
		t.Fatal(err)
	}
	if hit, err := loadDerived(p, key, decode); !hit || err != nil || !called {
		t.Fatal(hit, err)
	}
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { b[0] ^= 1; return b },
		func(b []byte) []byte { b[8] ^= 1; return b },
		func(b []byte) []byte { b[len(b)-1] ^= 1; return b },
		func(b []byte) []byte { return b[:len(b)-1] },
		func(b []byte) []byte { return append(b, 0) },
		func(b []byte) []byte { binary.LittleEndian.PutUint64(b[80:88], maxDerivedRaw+1); return b },
	} {
		bad := mutate(append([]byte(nil), original...))
		if err := os.WriteFile(p, bad, 0600); err != nil {
			t.Fatal(err)
		}
		called = false
		if hit, err := loadDerived(p, key, decode); hit || err == nil || called {
			t.Fatal("invalid cache reached decoder", hit, err)
		}
	}
	// Replacing corrupt content publishes a complete, verifiable image.
	if err := saveDerived(p, key, encode); err != nil {
		t.Fatal(err)
	}
	if hit, err := loadDerived(p, key, decode); !hit || err != nil {
		t.Fatal(hit, err)
	}
	if err := saveDerived(p, key, func(e *json.Encoder) error { return e.Encode(make(chan int)) }); err == nil {
		t.Fatal("encode failure accepted")
	}
	if hit, err := loadDerived(p, key, decode); !hit || err != nil {
		t.Fatal("failed writer replaced valid cache", hit, err)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(p), ".joint-items-*.tmp"))
	if len(files) != 0 {
		t.Fatal("temporary files leaked", files)
	}
}

func TestItemCacheKeyBindsEveryDependency(t *testing.T) {
	o := catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: true, Skins: true, Boosters: true}
	base := itemDerivedKey("parser", "source", o, true, "policy-path", "policy-bytes", true)
	keys := [][32]byte{
		itemDerivedKey("other", "source", o, true, "policy-path", "policy-bytes", true),
		itemDerivedKey("parser", "other", o, true, "policy-path", "policy-bytes", true),
		itemDerivedKey("parser", "source", o, true, "other", "policy-bytes", true),
		itemDerivedKey("parser", "source", o, true, "policy-path", "other", true),
		itemDerivedKey("parser", "source", o, false, "policy-path", "policy-bytes", true),
		itemDerivedKey("parser", "source", o, true, "policy-path", "policy-bytes", false),
	}
	for i := 0; i < 5; i++ {
		d := o
		flags := []*bool{&d.Periods, &d.Prices, &d.Materials, &d.Skins, &d.Boosters}
		*flags[i] = false
		keys = append(keys, itemDerivedKey("parser", "source", d, true, "policy-path", "policy-bytes", true))
	}
	for _, k := range keys {
		if k == base {
			t.Fatal("cache dependency not bound")
		}
	}
}

func TestJointCacheRestoresMaterialsAndZeroPrice(t *testing.T) {
	zero := uint32(0)
	m, err := catalog.RestoreItemMaterials(strings.Repeat("1", 64), []catalog.ItemMaterialEntry{{Template: 3, Materials: []catalog.ItemMaterialCost{{Template: 2, Count: 7}}}})
	if err != nil {
		t.Fatal(err)
	}
	out := JointItemCatalogs{Basics: catalog.ItemBasics{Index: catalog.ItemIndex{Items: map[uint32]catalog.ItemIndexEntry{3: {ID: 3}}}, Prices: &catalog.ShopPrices{Items: map[uint32]catalog.ShopPrice{2: {Buy: nil, Sell: 4}, 3: {Buy: &zero, Sell: 5}}}, Materials: m}}
	var b bytes.Buffer
	if err := encodeJointItems(json.NewEncoder(&b), out); err != nil {
		t.Fatal(err)
	}
	var decoded JointItemCatalogs
	if err := decodeJointItems(json.NewDecoder(&b), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Basics.Prices.Items[2].Buy != nil || decoded.Basics.Prices.Items[3].Buy == nil || *decoded.Basics.Prices.Items[3].Buy != 0 {
		t.Fatal("absent and zero purchase prices collapsed")
	}
	a, _ := m.Materials(3)
	v, ok := decoded.Basics.Materials.Materials(3)
	if !ok || !reflect.DeepEqual(a, v) {
		t.Fatal("private material index not restored")
	}
	if _, ok := decoded.Basics.Prices.Items[99]; ok {
		t.Fatal("missing price invented")
	}
}

func TestDerivedCacheRejectsExpandedLengthAndTrailingRecords(t *testing.T) {
	p := derivedPath(t.TempDir(), [32]byte{})
	if err := saveDerived(p, [32]byte{}, func(e *json.Encoder) error {
		if err := e.Encode(1); err != nil {
			return err
		}
		return e.Encode(2)
	}); err != nil {
		t.Fatal(err)
	}
	decode := func(d *json.Decoder) error { var v int; return d.Decode(&v) }
	if hit, err := loadDerived(p, [32]byte{}, decode); hit || err == nil {
		t.Fatal("trailing record accepted")
	}
	if err := saveDerived(p, [32]byte{}, func(e *json.Encoder) error { return e.Encode(1) }); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	binary.LittleEndian.PutUint64(b[80:88], binary.LittleEndian.Uint64(b[80:88])+1)
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	if hit, err := loadDerived(p, [32]byte{}, decode); hit || err == nil {
		t.Fatal("expanded length mismatch accepted")
	}
}
