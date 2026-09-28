package inventory

import "testing"

// TestProtectionTicketTemplateSets 钉住两个模板清单的规模与各族锚点。
//
// 为什么值得钉：这份清单是**客户端强制语义**（PVF 物品脚本里的 [action type]）的投影。
// 少一个 id 的直接后果是「玩家手里那个版本的保护券不被识别、装备照样碎」，而且
// 日志里没有任何异常。2026-09-28 实测：交付方初始清单只有 15 / 21，按 [action type]
// 取证后实机是 38 / 25 —— 漏掉的 23 / 4 个版本全部有物品脚本段证据。
// 所以这里不仅断言数量，还把每一族的代表 id 逐个点名。
func TestProtectionTicketTemplateSets(t *testing.T) {
	if got, want := len(amplifyProtectionTemplates), 38; got != want {
		t.Fatalf("增幅保护券模板数 = %d，期望 %d（清单被动过就要重跑取证脚本并同步这行）", got, want)
	}
	if got, want := len(reinforceProtectionTemplates), 25; got != want {
		t.Fatalf("强化保护券模板数 = %d，期望 %d", got, want)
	}
	// 增幅：原始清单版 / 按 [action type] 补入的两套命名 / 115 版本
	for _, id := range []uint32{50022395, 590005429, 10014791} {
		if !IsAmplifyProtectionTicket(id) {
			t.Fatalf("增幅保护券 %d 应被识别", id)
		}
	}
	for _, id := range []uint32{50006291, 501600220, 590704273} {
		if !IsAmplifyProtectionTicket(id) {
			t.Fatalf("增幅保护券 %d 应被识别（按 [action type] 补入：%s）", id, "amp_protection / boost 命名族")
		}
	}
	// 强化：原始清单版 / 按 [action type] 补入 / 115 版本
	for _, id := range []uint32{50022396, 10093371} {
		if !IsReinforceProtectionTicket(id) {
			t.Fatalf("强化保护券 %d 应被识别", id)
		}
	}
	for _, id := range []uint32{50041931, 590703970} {
		if !IsReinforceProtectionTicket(id) {
			t.Fatalf("强化保护券 %d 应被识别（按 [action type] 补入：re_protect 命名族）", id)
		}
	}
}

// TestProtectionTemplatesAreDisjoint 两个表不能有交集。
//
// 交集意味着同一张券既算增幅保护券、又算强化保护券 —— 在增幅窗口里消耗掉一张
// 强化保护券（或反之），玩家会莫名其妙丢东西。PVF 里那两段的取值本来就互斥
// （`[protect equipment]` 不是 `[amplify protect equipment]` 的子串），所以必须为空。
func TestProtectionTemplatesAreDisjoint(t *testing.T) {
	for id := range amplifyProtectionTemplates {
		if reinforceProtectionTemplates[id] {
			t.Fatalf("模板 %d 同时出现在增幅与强化保护券清单里", id)
		}
	}
}

// TestProtectionTicketIsNotOrdinaryMaterial 普通材料与无效模板不能被误判成保护券。
func TestProtectionTicketIsNotOrdinaryMaterial(t *testing.T) {
	// 3242 矛盾结晶体 / 3037 无色小晶块 / 3171 炉岩核 都是强化增幅的材料，不是保护券。
	for _, id := range []uint32{3242, 3037, 3171, 0, 1} {
		if IsProtectionTicket(id) {
			t.Fatalf("模板 %d 不是保护券，却被识别为保护券", id)
		}
	}
}

// TestFindProtectionTicketByKind 按类型查找不能串型。
func TestFindProtectionTicketByKind(t *testing.T) {
	bag := Bag{Items: []BagItem{
		{Slot: 144, Template: 50022395, Amount: 4}, // 增幅保护券
		{Slot: 145, Template: 50022396, Amount: 4}, // 强化保护券
	}}
	if slot, ok := findProtectionTicket(bag, true); !ok || slot != 144 {
		t.Fatalf("找增幅保护券得到 (%d, %v)，期望 (144, true)", slot, ok)
	}
	if slot, ok := findProtectionTicket(bag, false); !ok || slot != 145 {
		t.Fatalf("找强化保护券得到 (%d, %v)，期望 (145, true)", slot, ok)
	}
	// 反例：背包里只有增幅券时，强化失败**不能**消耗它。
	onlyAmplify := Bag{Items: []BagItem{{Slot: 144, Template: 50022395, Amount: 1}}}
	if slot, ok := findProtectionTicket(onlyAmplify, false); ok {
		t.Fatalf("背包里只有增幅保护券，找强化保护券却命中了槽 %d", slot)
	}
	// protectionTicketAtSlot 也要给出正确类型。
	if isAmp, present := protectionTicketAtSlot(bag, 144); !present || !isAmp {
		t.Fatalf("槽 144 应判为增幅保护券，得到 (isAmplify=%v, present=%v)", isAmp, present)
	}
	if isAmp, present := protectionTicketAtSlot(bag, 145); !present || isAmp {
		t.Fatalf("槽 145 应判为强化保护券，得到 (isAmplify=%v, present=%v)", isAmp, present)
	}
	if _, present := protectionTicketAtSlot(bag, 7); present {
		t.Fatal("空槽不应报有保护券")
	}
}

// TestConsumeProtectionTicket 扣券语义：扣 1、扣空移除该行、非保护券/不存在的槽位报错，
// 且**不能改动入参背包**（调用方在失败路径上还会用原背包刷行）。
func TestConsumeProtectionTicket(t *testing.T) {
	// 数量 4 -> 3，行保留
	bag := Bag{Items: []BagItem{{Slot: 144, Template: 50022395, Amount: 4}}}
	got, err := consumeProtectionTicket(bag, 144)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].Amount != 3 {
		t.Fatalf("扣 1 张后应为 Amount=3 且保留该行，得到 %+v", got.Items)
	}
	if bag.Items[0].Amount != 4 {
		t.Fatalf("不应改动入参背包，但原背包已被改成 Amount=%d", bag.Items[0].Amount)
	}

	// 数量 1 -> 整行移除
	one := Bag{Items: []BagItem{{Slot: 144, Template: 50022395, Amount: 1}}}
	got2, err := consumeProtectionTicket(one, 144)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2.Items) != 0 {
		t.Fatalf("扣空后应移除该行，得到 %+v", got2.Items)
	}

	// 槽位上不是保护券 -> 报错
	bad := Bag{Items: []BagItem{{Slot: 7, Template: 3242, Amount: 5}}}
	if _, err := consumeProtectionTicket(bad, 7); err == nil {
		t.Fatal("槽位上不是保护券时应报错")
	}

	// 槽位不存在 -> 报错
	if _, err := consumeProtectionTicket(one, 999); err == nil {
		t.Fatal("槽位不存在时应报错")
	}
}
