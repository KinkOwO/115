package catalog

import "testing"

// ResolveShop 必须挑出「在商店表里」的那个候选。
//
// 2026-09-29 实机取证（42/42 样本）：CMD21 的 p[8]/p[12] 都是 npc-like id，
// 哪个是商店取决于客户端从哪个 NPC 打开界面。两个「候选都不是商店」的情况为 0。
func TestResolveShopPicksKnown(t *testing.T) {
	shops := &ItemShops{byShop: map[uint32]map[uint32]ItemShopOffer{
		100001774: {10345008: {Tab: 1, Index: 61, Template: 10345008}},
	}}
	// 实机第一类：p8 是圣诞树(100000694)，p12 才是商店。
	if id, ok := shops.ResolveShop(100000694, 100001774); !ok || id != 100001774 {
		t.Fatalf("第一类：得 (%d,%v)，期望 (100001774,true)", id, ok)
	}
	// 顺序不该影响结果。
	if id, ok := shops.ResolveShop(100001774, 100000694); !ok || id != 100001774 {
		t.Fatalf("交换顺序：得 (%d,%v)", id, ok)
	}
}

// 第二类：p8 才是商店（奥德赛商店），p12 不是。
func TestResolveShopPicksFirstWhenThatIsTheShop(t *testing.T) {
	shops := &ItemShops{byShop: map[uint32]map[uint32]ItemShopOffer{
		100001019: {10417800: {Tab: 1, Index: 1, Template: 10417800}},
	}}
	if id, ok := shops.ResolveShop(100001019, 100003035); !ok || id != 100001019 {
		t.Fatalf("第二类：得 (%d,%v)，期望 (100001019,true)", id, ok)
	}
}

// 都不是商店时必须明确返回 false，而不是瞎猜一个 —— 调用方据此退回 NpcID。
func TestResolveShopNoneKnown(t *testing.T) {
	shops := &ItemShops{byShop: map[uint32]map[uint32]ItemShopOffer{
		100001774: {},
	}}
	if id, ok := shops.ResolveShop(100000694, 100003035); ok {
		t.Fatalf("都不在表里却返回了 (%d,true)", id)
	}
}

// 0 是「未设置」的常见取值，不能当成商店 id 命中去查。
func TestResolveShopSkipsZero(t *testing.T) {
	shops := &ItemShops{byShop: map[uint32]map[uint32]ItemShopOffer{
		100001774: {1: {Template: 1}},
	}}
	if id, ok := shops.ResolveShop(0, 100001774); !ok || id != 100001774 {
		t.Fatalf("跳过 0：得 (%d,%v)", id, ok)
	}
	if _, ok := shops.ResolveShop(0, 0); ok {
		t.Fatal("全 0 却命中")
	}
}

// nil 接收者不能 panic（lootService.ItemShops 未配置时就是这个状态）。
func TestResolveShopNilSafe(t *testing.T) {
	var shops *ItemShops
	if _, ok := shops.ResolveShop(1, 2); ok {
		t.Fatal("nil 竟然命中")
	}
}
