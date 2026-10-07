package catalog

import "testing"

// 源里的 [create cost] 段（结构完全照 equipmentsetjournal.cos 的样子写：
// `[group]` **没有头**，序号在同级的 `[index] N` 里；`[costs]`/`[cost]`/`[item index]` 都是带尾标记的容器）。
const createCostSample = `
[create cost]
 [group]
  [index] 1
  [item index]
   100051317
   100101200
  [/item index]
  [costs]
   [cost] 1
    10361513 1
    0 30000
   [/cost]
   [cost] 2
    10361513 1
    10401346 6
   [/cost]
  [/costs]
 [/group]
 [group]
  [index] 2
  [item index]
   100051318
  [/item index]
  [costs]
   [cost] 1
    0 35000
   [/cost]
  [/costs]
 [/group]
[/create cost]
`

func TestParseEquipmentCreateCostSample(t *testing.T) {
	groups, e := ParseEquipmentCreateCost(createCostSample)
	if e != nil {
		t.Fatal(e)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	// ⚠️ 序号必须来自 [index] 子节点；按 [group] 的头去读会全部得到 0（踩过一次）。
	if groups[0].Index != 1 || groups[1].Index != 2 {
		t.Fatalf("indexes = %d/%d, want 1/2", groups[0].Index, groups[1].Index)
	}
	if len(groups[0].Items) != 2 || groups[0].Items[0] != 100051317 || groups[0].Items[1] != 100101200 {
		t.Fatalf("group 1 items = %v", groups[0].Items)
	}
	if len(groups[0].Costs) != 2 {
		t.Fatalf("group 1 costs = %d, want 2", len(groups[0].Costs))
	}
	c1 := groups[0].Costs[0]
	if c1.Number != 1 || len(c1.Pairs) != 2 {
		t.Fatalf("cost 1 = %+v", c1)
	}
	if c1.Pairs[0].Template != 10361513 || c1.Pairs[0].Amount != 1 {
		t.Fatalf("cost 1 pair 0 = %+v", c1.Pairs[0])
	}
	// 模板 0 = 金币，必须被 Gold() 认出来。
	if !c1.Pairs[1].Gold() || c1.Pairs[1].Amount != 30000 {
		t.Fatalf("cost 1 pair 1 = %+v, want gold 30000", c1.Pairs[1])
	}
	if c1.Pairs[0].Gold() {
		t.Fatalf("material pair must not report Gold()")
	}

	g, ok := EquipmentCreateCost{Groups: groups}.GroupFor(100101200)
	if !ok || g.Index != 1 {
		t.Fatalf("GroupFor(100101200) = %+v ok=%v, want group 1", g, ok)
	}
	if g, ok := (EquipmentCreateCost{Groups: groups}).GroupFor(100101200); !ok || g.Index != 1 {
		t.Fatalf("GroupFor(100101200) = %+v ok=%v, want group 1", g, ok)
	}
	if _, ok := (EquipmentCreateCost{Groups: groups}).GroupFor(42); ok {
		t.Fatalf("unknown template must not resolve")
	}
	if _, ok := (EquipmentCreateCost{Groups: groups}).GroupFor(0); ok {
		t.Fatalf("template 0 (gold sentinel) must not resolve")
	}
}

func TestParseEquipmentCreateCostRejectsBroken(t *testing.T) {
	cases := map[string]string{
		"缺整段":   "[other]\n [/other]\n",
		"组内无物品": "[create cost]\n [group]\n  [index] 1\n  [item index]\n  [/item index]\n  [costs]\n   [cost] 1\n    0 100\n   [/cost]\n  [/costs]\n [/group]\n[/create cost]\n",
		"组内无成本": "[create cost]\n [group]\n  [index] 1\n  [item index]\n   100\n  [/item index]\n [/group]\n[/create cost]\n",
	}
	for name, text := range cases {
		if _, e := ParseEquipmentCreateCost(text); e == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestEquipmentCreateCostGeneratedCatalog(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	c, e := LoadEquipmentCreateCost("../../configs/equipment-create-cost.generated.json", source)
	if e != nil {
		t.Fatal(e)
	}
	if c.Path != EquipmentJournalPath || len(c.SHA256) != 64 {
		t.Fatalf("provenance %q %q", c.Path, c.SHA256)
	}
	if c.Source.Checksum != source || c.Bytes != 122980 {
		t.Fatalf("source/bytes = %s/%d", c.Source.Checksum, c.Bytes)
	}
	// 代次收紧：source 对不上必须拒绝。
	if _, e := LoadEquipmentCreateCost("../../configs/equipment-create-cost.generated.json", "deadbeef"); e == nil {
		t.Fatalf("generation mismatch must be rejected")
	}
	if len(c.Groups) != 9 {
		t.Fatalf("groups = %d, want 9", len(c.Groups))
	}
	// 源里 9 组一共 162 行物品、且不重复（11+11+16+15+15+11+11+36+36）。
	rows, seen := 0, map[uint32]bool{}
	for _, g := range c.Groups {
		if g.Index <= 0 {
			t.Fatalf("group %d has no [index]", g.Index)
		}
		if len(g.Items) == 0 || len(g.Costs) == 0 {
			t.Fatalf("group %d: items=%d costs=%d", g.Index, len(g.Items), len(g.Costs))
		}
		for _, tpl := range g.Items {
			if tpl == 0 || seen[tpl] {
				t.Fatalf("template %d duplicated or zero", tpl)
			}
			seen[tpl] = true
			rows++
		}
		for _, opt := range g.Costs {
			if len(opt.Pairs) == 0 {
				t.Fatalf("group %d cost %d has no pairs", g.Index, opt.Number)
			}
			golds := 0
			for _, p := range opt.Pairs {
				if p.Amount == 0 {
					t.Fatalf("group %d cost %d has a zero-amount row", g.Index, opt.Number)
				}
				if p.Gold() {
					golds++
				}
			}
			if golds > 1 {
				t.Fatalf("group %d cost %d has %d gold rows", g.Index, opt.Number, golds)
			}
		}
	}
	if rows != 162 || len(seen) != 162 {
		t.Fatalf("item rows = %d, distinct templates = %d, want 162/162", rows, len(seen))
	}
	if len(c.Templates()) != 162 {
		t.Fatalf("Templates() = %d, want 162", len(c.Templates()))
	}

	// 玩家实际点过的三件都必须能查到档位（实机样本：头肩/腰带/耳环 → r6 档 = 组 2）。
	for _, tpl := range []uint32{100151142, 100201114, 100391056, 100101201} {
		g, ok := c.GroupFor(tpl)
		if !ok || g.Index != 2 {
			t.Fatalf("GroupFor(%d) = group %d ok=%v, want group 2", tpl, g.Index, ok)
		}
	}
	// r3 档（组 1）必须正好是"5 防具 + 6 首饰"，且与 [create cost] 的源顺序一致。
	g1, _ := c.GroupFor(100051317)
	if g1.Index != 1 || len(g1.Items) != 11 {
		t.Fatalf("group 1 = %+v", g1)
	}
	want := []uint32{100051317, 100101200, 100151141, 100201113, 100251153, 100301867, 100313570, 100323460, 100346002, 100354177, 100391055}
	for i := range want {
		if g1.Items[i] != want[i] {
			t.Fatalf("group 1 item %d = %d, want %d", i, g1.Items[i], want[i])
		}
	}
	// 组 1 的第一支成本 = 登记证 10361513×1 + 金币 30000（实机弹窗里那一行也是这个量级）。
	if len(g1.Costs[0].Pairs) != 2 || g1.Costs[0].Pairs[0].Template != 10361513 ||
		!g1.Costs[0].Pairs[1].Gold() || g1.Costs[0].Pairs[1].Amount != 30000 {
		t.Fatalf("group 1 cost 1 = %+v", g1.Costs[0])
	}
}
