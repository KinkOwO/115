package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 2026-10-01（next146）：直读模式下背包规则表的 `source` 必须能被当次内层 checksum 取代。
//
// 语义与 LoadWearRules 不同 —— 这里用**覆盖**而不是拒绝：`inventory.current37.json` 的
// `source` 是整批 configs 导出族的**批次标记**（loot.next25 / inventory.next29 /
// compat90 / equipment.current37 全写同一个历史值），`Bag.Disjoint` 拿它断言
// "规则表与掉落目录同批"。直读模式下目录由内层 PVF 现场推导，调用方的 checksum 权威，
// 历史批次值必须被取代，否则内层一重建启动就断。
//
// 覆盖是**必需的**：BagRules.Source 同时是运行时不变量（amplify/enchant/inherit/refine/
// reinforcement/vault_transfer/stack_request/pet_move 都拿它比 role.ConfigVersion，
// main.go 也拿它比掉落目录 checksum），不回填会让这些操作在运行时全被拒。
func TestLoadBagRulesDerivesEmptySource(t *testing.T) {
	derived := "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
	historical := "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	base := BagRules{Model: "reference90-bag-v1", MissingStackLimit: 1000, Slots: map[string][2]uint16{"[material]": {1, 10}}}

	write := func(t *testing.T, r BagRules) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "bag.json")
		b, _ := json.Marshal(r)
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("empty-source-backfills-caller-checksum", func(t *testing.T) {
		r := base
		r.Source = ""
		got, err := LoadBagRules(write(t, r), derived)
		if err != nil {
			t.Fatal(err)
		}
		if got.Source != derived {
			t.Fatalf("source not back-filled: %q", got.Source)
		}
	})

	t.Run("historical-batch-marker-overridden-by-caller", func(t *testing.T) {
		// current37.json 的真实形状：批次标记 = 历史值，直读调用方传当次哈希。
		r := base
		r.Source = historical
		got, err := LoadBagRules(write(t, r), derived)
		if err != nil {
			t.Fatalf("direct-read caller over a batch-marked file must be accepted: %v", err)
		}
		if got.Source != derived {
			t.Fatalf("caller checksum must win: got %q want %q", got.Source, derived)
		}
	})

	t.Run("matching-source-accepted", func(t *testing.T) {
		r := base
		r.Source = derived
		got, err := LoadBagRules(write(t, r), derived)
		if err != nil {
			t.Fatal(err)
		}
		if got.Source != derived {
			t.Fatalf("source not preserved: %q", got.Source)
		}
	})

	t.Run("malformed-caller-checksum-refused", func(t *testing.T) {
		r := base
		r.Source = ""
		if _, err := LoadBagRules(write(t, r), "not-a-hash"); err == nil {
			t.Fatal("malformed caller checksum accepted")
		}
	})

	t.Run("legacy-no-caller-requires-pinned-source", func(t *testing.T) {
		r := base
		r.Source = ""
		if _, err := LoadBagRules(write(t, r)); err == nil {
			t.Fatal("empty source accepted without a caller checksum")
		}
	})

	t.Run("legacy-pinned-source-still-works-without-caller", func(t *testing.T) {
		r := base
		r.Source = historical
		got, err := LoadBagRules(write(t, r))
		if err != nil {
			t.Fatalf("legacy pinned file broke for old callers: %v", err)
		}
		if got.Source != historical {
			t.Fatalf("legacy pinned source must be preserved: %q", got.Source)
		}
	})
}
