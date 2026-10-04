package loot

import (
	"encoding/binary"
	"testing"

	"dfolan/internal/dungeon"
)

// [AZURE-CLEAR-REWARD] 蔚蓝号翻牌的随机装备段必须与月湖同口径（对照已跑通的沉月湖）：
// 件数 1..4、权重 {1,3,3,2}、等级取副本 MinimumLevel、boss 档 3。
func TestAzureMainEquipmentPolicyMatchesMoonCaliber(t *testing.T) {
	p := AzureMainEquipmentPolicy(115)
	if p.Min != 1 || p.Max != 4 || p.Level != 115 || p.Rank != 3 {
		t.Fatalf("policy = %+v, want min1 max4 level115 rank3", p)
	}
	if len(p.Weights) != int(p.Max-p.Min+1) {
		t.Fatalf("weights %v 必须覆盖 %d..%d 每一档", p.Weights, p.Min, p.Max)
	}

	seen := map[uint32]int{}
	for i := 0; i < 4096; i++ {
		var seed [8]byte
		binary.LittleEndian.PutUint64(seed[:], uint64(i)*0x9E3779B97F4A7C15)
		n, err := moonEquipmentCount(p, seed[:])
		if err != nil {
			t.Fatal(err)
		}
		if n < p.Min || n > p.Max {
			t.Fatalf("件数 %d 落在 %d..%d 之外", n, p.Min, p.Max)
		}
		seen[n]++
	}
	// {1,3,3,2} 里"1 件"占 1/9 ≈ 455/4096；等概率会是 1/4 = 1024。
	// 取 1/6（≈682）做分界：加权通过，等概率必然失败。
	if seen[1] > 4096/6 {
		t.Fatalf("只出一件占 %d/4096，看起来退化成等概率了（权重没生效）", seen[1])
	}
	// 1..4 每一档都要出现过，否则区间形同虚设。
	for n := uint32(1); n <= 4; n++ {
		if seen[n] == 0 {
			t.Fatalf("件数 %d 从未被抽到（权重 %v）", n, p.Weights)
		}
	}
}

// [AZURE-CLEAR-REWARD] 未通关 / 无会话一律不出奖单 —— 不能凭一个不完整的状态发东西。
func TestPlanAzureMainCardsRefusesIncompleteRun(t *testing.T) {
	svc := &Service{}
	if _, err := svc.PlanAzureMainCards(Role{}, &dungeon.Session{}, AzureMainEquipmentPolicy(115)); err == nil {
		t.Fatal("未通关的副本不该出奖单")
	}
	if _, err := svc.PlanAzureMainCards(Role{}, nil, AzureMainEquipmentPolicy(115)); err == nil {
		t.Fatal("nil 会话应当报错")
	}
}
