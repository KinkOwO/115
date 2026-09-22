package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// 实机 2026-09-23：从奥德赛商店（NPC id 100001019，银币的 [open shop] 值）买
// 10417798，客户端要求 100 个银币，服务端却只扣了 1 金币。价格来自
// itemshop/100001019_aradodyssey.shp 的 [need material] 段；这张表必须被加载。
func TestOdysseyShopPricesInCoins(t *testing.T) {
	s, err := LoadItemShops("../../configs/itemshop-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != ItemShopModel || len(s.Source.Checksum) != 64 {
		t.Fatalf("model=%q source=%+v", s.Model, s.Source)
	}
	if len(s.Shops) < 100 {
		t.Fatalf("可疑的商店数量: %d", len(s.Shops))
	}

	// 请求里的 NpcID 是商店 id（文件名前缀），不是表内 [NPC] 的 actor id。
	mats, listed, paid := s.Materials(100001019, 10417798)
	if !listed || !paid {
		t.Fatalf("奥德赛商店没把 10417798 标成材料支付: listed=%v paid=%v", listed, paid)
	}
	if len(mats) != 1 || mats[0].Template != 10418036 || mats[0].Count != 100 {
		t.Fatalf("银币价格不对: %+v", mats)
	}
	// 同店的另一条：金币 10418035 × 60
	if mats, _, paid := s.Materials(100001019, 10417799); !paid || mats[0].Template != 10418035 || mats[0].Count != 60 {
		t.Fatalf("金币商品价格不对: %+v", mats)
	}
	if !s.Listed(100001019, 10417798) {
		t.Fatal("Listed 没认出已上架的模板")
	}
	if _, listed, _ := s.Materials(100001019, 999999); listed {
		t.Fatal("未上架的模板被当成已上架")
	}
	if _, _, paid := s.Materials(424242, 10417798); paid {
		t.Fatal("未知商店被当成材料支付")
	}
}

func TestLoadItemShopsRefusesBrokenArtifacts(t *testing.T) {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dir := t.TempDir()
	body := func(model string) string {
		return `{"model":"` + model + `","source":{"checksum":"` + hash + `"},"shops":{"1":{"path":"itemshop/1_x.shp","sha256":"` + hash + `","offers":[{"template":2,"materials":[{"template":3,"count":1}]}]}}}`
	}
	if _, err := LoadItemShops(writeTemp(t, dir, "model.json", body("other"))); err == nil {
		t.Fatal("外来 model 被接受")
	}
	if _, err := LoadItemShops(writeTemp(t, dir, "empty.json",
		`{"model":"`+ItemShopModel+`","source":{"checksum":"`+hash+`"},"shops":{}}`)); err == nil {
		t.Fatal("空目录被接受")
	}
	if _, err := LoadItemShops(writeTemp(t, dir, "badmat.json",
		`{"model":"`+ItemShopModel+`","source":{"checksum":"`+hash+`"},"shops":{"1":{"path":"p","sha256":"`+hash+`","offers":[{"template":2,"materials":[{"template":0,"count":1}]}]}}}`)); err == nil {
		t.Fatal("非法材料被接受")
	}
	if _, err := LoadItemShops(writeTemp(t, dir, "badhash.json",
		`{"model":"`+ItemShopModel+`","source":{"checksum":"`+hash+`"},"shops":{"1":{"path":"p","sha256":"nope","offers":[{"template":2}]}}}`)); err == nil {
		t.Fatal("坏脚本哈希被接受")
	}
}
