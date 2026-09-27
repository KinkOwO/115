package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testSourceChecksum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

func writeCatalog(t *testing.T, data SkinStorageCatalog) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "skin-storage-items.json")
	b, e := json.Marshal(data)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, b, 0600); e != nil {
		t.Fatal(e)
	}
	return file
}

// The `[action type]` parameter, not `[damage font info] [index]`, is the cargo key:
// 10305398 registers skin 12 (`Skin/DamageFont/18_Miku.skn`) while its damage font
// index 11 selects a `.dfk` in a different namespace.
func TestSkinKeyUsesActionParameter(t *testing.T) {
	entry := SkinStorageEntry{
		Template: 10305398, SkinID: 12, SkinPath: "skin/damagefont/18_miku.skn",
		SkinType: "damage font", DamageFontIndex: 11, HasDamageFontInfo: true,
	}
	if got := entry.SkinKey(); got != 12 {
		t.Fatalf("SkinKey() = %d, want the [add skin storage] parameter 12", got)
	}
	if !entry.IsDamageFont() {
		t.Fatal("a .skn declaring [type] `damage font` must classify as a damage font")
	}
	// 10358669 registers damage font 59 without declaring [damage font info].
	if !(SkinStorageEntry{SkinType: "Damage Font"}).IsDamageFont() {
		t.Fatal("skin family must be decided case-insensitively by the .skn type")
	}
	if (SkinStorageEntry{SkinType: "instant emoticon"}).IsDamageFont() {
		t.Fatal("an emoticon skin is not a damage font")
	}
}

func TestLoadSkinStorage(t *testing.T) {
	source := pvf.ArchiveSnapshot{Checksum: testSourceChecksum}
	t.Run("accepts v2", func(t *testing.T) {
		file := writeCatalog(t, SkinStorageCatalog{
			Schema: SkinStorageSchema, Source: source,
			Entries: []SkinStorageEntry{{
				Template: 10358669, SkinID: 59, SkinPath: "skin/damagefont/x.skn", SkinType: "damage font",
			}},
			MissingSkins: []MissingSkin{{Template: 10158026, SkinID: 30002}},
		})
		entries, e := LoadSkinStorage(file, testSourceChecksum)
		if e != nil {
			t.Fatal(e)
		}
		if len(entries) != 1 || entries[10358669].SkinKey() != 59 {
			t.Fatalf("entries = %v", entries)
		}
	})
	t.Run("refuses the v1 schema", func(t *testing.T) {
		file := writeCatalog(t, SkinStorageCatalog{
			Schema: "skin-storage-items-v1", Source: source,
			Entries: []SkinStorageEntry{{Template: 1, SkinID: 1, SkinPath: "p", SkinType: "damage font"}},
		})
		if _, e := LoadSkinStorage(file, testSourceChecksum); e == nil {
			t.Fatal("v1 catalogs key skins by the disproven damage font index and must be refused")
		}
	})
	t.Run("refuses an entry with no resolved skin", func(t *testing.T) {
		file := writeCatalog(t, SkinStorageCatalog{
			Schema: SkinStorageSchema, Source: source,
			Entries: []SkinStorageEntry{{Template: 1, SkinID: 30002}},
		})
		if _, e := LoadSkinStorage(file, testSourceChecksum); e == nil {
			t.Fatal("a template whose skin is missing from list/skin.lst must not load")
		}
	})
}
