package main

import (
	"bytes"
	"os"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// 蔚蓝号的源驱动翻牌装配：与沉月湖同一套，但**结算层必须是频道 [guide dungeon index]**
// （实机 100004131），固定产物数量取 L0（官服抓包 F16-s2c.txt #682 的 N35）。
//
// 这条测试的意义：把「蔚蓝号有没有被接上」钉死。装配错了会有两种典型故障 ——
// 一种是结算层指错副本（发奖对着月湖的单子），一种是族并集漏了成员（品级池只剩
// rarity 2，正是 next190 §R2 记的那个坑）。
func TestAzureFlipPolicyFromSource(t *testing.T) {
	path := conquestTestArchive(t)
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	ids := []uint32{100004136, 100004137, 100004131, 100004134}
	dungeons, err := catalog.ImportDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := dungeons.Dungeons[100004131]; !ok {
		t.Fatal("100004131 不在直读副本目录里")
	}
	items, err := catalog.ImportLoot(a, 150)
	if err != nil {
		t.Fatal(err)
	}
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := items.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	full, err := inventory.OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = full.Close() }()
	bag, err := inventory.LoadBagRules("../../configs/inventory.current37.json", dungeons.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loot.Parse(items)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := loot.LoadRules("../../configs/drop.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	svc := &loot.Service{Catalog: items, BagRules: bag, Equipment: &inventory.EquipmentCatalog{Full: full}, Tables: tables, Rules: rules}

	cfg, err := azureFlipPolicy(svc, &dungeons, 100004131, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("蔚蓝号翻牌装配：%s", cfg.Note)
	p := cfg.Policy
	if p.SettlementDungeon != 100004131 {
		t.Fatalf("结算层 = %d，期望 100004131（频道的 [guide dungeon index]）", p.SettlementDungeon)
	}
	if p.Level != 115 || p.MaxRows != 126 {
		t.Fatalf("Level=%d MaxRows=%d，期望 115 / 126", p.Level, p.MaxRows)
	}
	// 固定产物数量 = L0（官服 N35 头两条行：10362432 x160、10362429 x100）。
	want := map[uint32]uint32{10362432: 160, 10362429: 100}
	got := map[uint32]uint32{}
	for _, c := range p.Fixed {
		got[c.Template] = c.Count
	}
	for tpl, n := range want {
		if got[tpl] != n {
			t.Fatalf("固定产物 %d = %d，期望 %d（全部：%v）", tpl, got[tpl], n, got)
		}
	}
	// 族并集：品级组只由 100004136 声明 ⇒ 少了它就只剩 rarity 2。
	if len(p.RarityPool) != 4 {
		t.Fatalf("品级池档数 = %d，期望 4（rarity 2/3/4/6）", len(p.RarityPool))
	}
	if len(p.OathPool) != 100 {
		t.Fatalf("誓约/结晶装备池 = %d，期望 100", len(p.OathPool))
	}
	if len(p.SetEquipmentPool) != 396 {
		t.Fatalf("12 套装装备池 = %d，期望 396", len(p.SetEquipmentPool))
	}
	// 老上界 16 对蔚蓝号**结构性**不够：固定产物 + 装备倍数(10) + 誓约倍数(4) 已经越过 16
	//（还没算基础产物那 5 条 Special）—— 这正是 2026-10-09「结算面板根本不出现」的成因：
	// 奖单写完了却读不回来（DecodeMoonReward 判 foreign/corrupt）。见 MoonRewardMaxRows。
	if rows := len(p.Fixed) + 10 + 4; rows <= 16 {
		t.Fatalf("固定+效率只算出 %d 行，与「16 上界不够」的前提不符", rows)
	}
	if err := svc.ValidateMoonSourcePolicy(p); err != nil {
		t.Fatal("蔚蓝号源驱动策略未通过入场自检:", err)
	}
}

// 蔚蓝号 N35 的行形态必须与官服抓包一致：29 字节 = template u32 + value u32 + 21B metadata。
//
// 官服 `F16-s2c.txt` 第 682 帧（id=35, body=616）解出来的前两条行就是
// `10362432 160` 与 `10362429 100` —— 这条测试钉住同一种行字节。
func TestAzureClearRewardRowShape(t *testing.T) {
	w := &worldSession{azure: azureMainState{flip: azureFlipState{plan: &loot.MoonRewardPlan{
		Run: "0123456789abcdef0123456789abcdef",
		Grants: []loot.MoonRewardGrant{
			{Template: 10362432, Count: 160},
			{Template: 10362429, Count: 100},
		},
	}}}}
	body, err := w.azureClearRewardBody()
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("空载荷")
	}
	for _, row := range [][]byte{
		append([]byte{0x40, 0x1e, 0x9e, 0x00, 0xa0, 0x00, 0x00, 0x00}, make([]byte, 21)...),
		append([]byte{0x3d, 0x1e, 0x9e, 0x00, 0x64, 0x00, 0x00, 0x00}, make([]byte, 21)...),
	} {
		if !bytes.Contains(body, row) {
			t.Fatalf("载荷里找不到 29B 行 % x", row[:8])
		}
	}
	// 空奖单必须报错，不能发一帧空奖励出去。
	empty := &worldSession{azure: azureMainState{flip: azureFlipState{plan: &loot.MoonRewardPlan{Run: "0123456789abcdef0123456789abcdef"}}}}
	if _, err := empty.azureClearRewardBody(); err == nil {
		t.Fatal("空奖单应当报错")
	}
}

// 蔚蓝号领奖的 CMD71 应答必须带**被翻的那张牌的下标**。
//
// 2026-10-09 21:1x 实机教训：「自动翻牌没了」的成因就是把旧帧 `01 00 ff 00 00 …`
// 误读成 `CardSelected(-1)`（所有行的标记位都是 0xff = 一张牌都没翻）并照抄进实现 ——
// 服务端把奖发进背包了，客户端面板上却没有任何一张牌被翻开，玩家只能自己再点一下。
// 正确的旧帧是 `CardSelected(0)`（自动翻牌选第 0 张，第一行的标记位为 0x00）。
func TestAzureClaimAckCarriesFlippedIndex(t *testing.T) {
	const run = "0123456789abcdef0123456789abcdef"
	w := &worldSession{
		activeDungeon: &dungeon.Session{RunID: run},
		azure: azureMainState{flip: azureFlipState{
			plan:    &loot.MoonRewardPlan{Run: run, Source: "0123456789abcdef0123456789abcdef"},
			claimed: true, // 已领过 ⇒ 走「只回应答」的幂等分支，不碰 store
		}},
	}
	for _, idx := range []byte{0, 1, 2, 3} {
		packets, e := w.azureClaim(idx)
		if e != nil {
			t.Fatal(e)
		}
		if len(packets) != 1 || packets[0].Name != "azure_card_selection_ack" || packets[0].ID != 71 {
			t.Fatalf("下标 %d 的重复领奖应答形状不对：%+v", idx, packets)
		}
		p := packets[0].Payload
		if len(p) != 33 || p[0] != 1 {
			t.Fatalf("下标 %d 的应答长度/首字节不对：% x", idx, p)
		}
		for i := 0; i < 8; i++ {
			want := byte(0xff)
			if byte(i) == idx {
				want = 0
			}
			if p[1+4*i] != want {
				t.Fatalf("下标 %d：第 %d 行的标记位 = %#x，期望 %#x（整帧 % x）", idx, i, p[1+4*i], want, p)
			}
		}
	}
}
