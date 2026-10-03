package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

// 2026-10-01（next145 续）：wear 规则表的 `source` 允许留空 = 自动派生。
//
// 背景：`preparePVFShields` 对 LoadWearRules 的调用**不受 checksBaselines 保护**
// （cmd/wireprobe/pvf_catalogs.go:318），是 PVF 直读启动的必由之路。若把这份手写
// 槽位表钉在某个历史内层哈希上，内层一重建就越过启动。本测试锁定三件事：
//   - 留空 ⇒ 接受，并**回填**为调用方给的当次哈希（运行时不变量不破）；
//   - 与调用方一致 ⇒ 接受；
//   - 与调用方不一致 ⇒ 仍拒（显式钉住的校验能力没丢）。
func TestLoadWearRulesDerivesEmptySource(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "wear.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	const src = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	slots := `"[weapon]":12,"[coat]":14,"[pants]":16`

	t.Run("empty source derives and backfills", func(t *testing.T) {
		p := write(`{"source":"","special":true,"slots":{` + slots + `}}`)
		r, err := LoadWearRules(p, src)
		if err != nil {
			t.Fatalf("empty source rejected: %v", err)
		}
		if r.Source != src {
			t.Fatalf("source not backfilled: got %q want %q", r.Source, src)
		}
		if r.Slots["[weapon]"] != 12 {
			t.Fatalf("slots lost: %+v", r.Slots)
		}
	})

	t.Run("matching source accepted", func(t *testing.T) {
		p := write(`{"source":"` + src + `","special":true,"slots":{` + slots + `}}`)
		if _, err := LoadWearRules(p, src); err != nil {
			t.Fatalf("matching source rejected: %v", err)
		}
	})

	t.Run("explicit mismatch still refused", func(t *testing.T) {
		p := write(`{"source":"0000000000000000000000000000000000000000000000000000000000000000","special":true,"slots":{` + slots + `}}`)
		if _, err := LoadWearRules(p, src); err == nil {
			t.Fatal("explicit wrong pin must be refused")
		}
	})

	t.Run("empty slots still refused", func(t *testing.T) {
		p := write(`{"source":"","special":true,"slots":{}}`)
		if _, err := LoadWearRules(p, src); err == nil {
			t.Fatal("empty slot table must be refused")
		}
	})

	// 调用方给的来源必须是完整 64 位 hex：留空 + 垃圾来源不能悄悄通过。
	t.Run("empty source with bad caller checksum refused", func(t *testing.T) {
		p := write(`{"source":"","special":true,"slots":{` + slots + `}}`)
		if _, err := LoadWearRules(p, "short"); err == nil {
			t.Fatal("empty source with non-64 caller checksum must be refused")
		}
	})
}
