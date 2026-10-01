package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"

	"encoding/json"
	"testing"
)

// AppearanceProbe 是换装后的外观刷新，它重发的是一份 mode0 userinfo。
// userinfo 里的 advancement 字节是 wire 编码：低 4 位是转职分支、bit4..6 是觉醒阶段
// （internal/game/protocol/advancement.go 的注释与 DecodeChangeGrowType 都这么读）。
//
// 实机 2026-09-23（test-jh）：开箱换上奥德赛装备后，客户端每次都弹一次「2次觉醒」
// 对话框（"你的角色属性已提升 / 你已学会以下技能"）。根因就是这里只写了
// state.Advancement —— 每次换装都把觉醒阶段抹成 0，客户端于是以为"刚从无觉醒变成
// 二觉"。列表（service.go:167）与进城（service.go:242）都用了 WireAdvancement，
// 只有这个换装刷新漏了。
func TestAppearanceProbeKeepsAwakeningStage(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn:    []inventory.BagEquipment{{Slot: 12, Template: 101010438}},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":90,"advancement":1,"awakening":2}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(Character{
		WireID: 1, Name: "LanTest01", Profession: 0, State: state,
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	// profession / advancement / level / pad / pad 紧随角色名之后。
	want := []byte{0x00, 0x21, 0x5a, 0x00, 0x00} // 0x21 = advancement 1 | awakening 2<<4
	if !bytes.Contains(got, want) {
		t.Fatalf("mode0 userinfo 丢了觉醒阶段：payload 里找不到 % x", want)
	}
	if bytes.Contains(got, []byte{0x00, 0x01, 0x5a, 0x00, 0x00}) {
		t.Fatal("mode0 userinfo 只带了 advancement，觉醒阶段被抹成 0（换装必弹觉醒提示）")
	}
}

func TestAppearanceProbePreservesCharacterMode(t *testing.T) {
	state := json.RawMessage(`{"level":90,"advancement":1}`)
	role := Character{WireID: 1, Name: "LanTest01", Profession: 0, State: state}
	s := &Service{}
	for _, tc := range []struct {
		name, setting string
		want          byte
	}{
		{name: "odyssey", setting: "1", want: 5},
		{name: "story", setting: "0", want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DFO_ODYSSEY_MODE", tc.setting)
			got, err := s.AppearanceProbe(role, [2]byte{})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) < 13 {
				t.Fatalf("appearance payload too short: %d", len(got))
			}
			if got[len(got)-13] != tc.want {
				t.Fatalf("appearance mode byte = %d, want %d", got[len(got)-13], tc.want)
			}
			entry, err := s.EntryBasicProbe(role, [2]byte{})
			if err != nil {
				t.Fatal(err)
			}
			if got[len(got)-13] != entry[len(entry)-13] {
				t.Fatal("appearance mode differs from entry mode")
			}
		})
	}
}
