package protocol

import "testing"

// mode0（UserInfoBasicProbe）与 mode1（UserInfoAdditionProbe）都必须携带
// "技能类型"选择字节，缺一个客户端就看不到第二技能页。这条断言把 mode0 的
// 位置与三种取值（0xff 未解锁 / 0x00 类型1 / 0x01 类型2）钉死：
// 86JP UserInfoSubtype0Builder 在 MoodValue(u16) 之后紧跟写 SkillTreeIndex。
func TestMode0CarriesSkillTreeSelector(t *testing.T) {
	build := func(selection byte) []byte {
		p, err := UserInfoBasicProbe(EntryBasicProbe{
			ActorServerID: 3,
			Context:       [2]byte{1, 0},
			Character:     CharacterRow{Name: "LanTest01", Level: 1},
			SkillTreeType: selection,
		})
		if err != nil {
			t.Fatalf("build mode0 for selection %d: %v", selection, err)
		}
		return p
	}
	locked, first, second := build(0), build(1), build(2)
	if len(locked) != len(first) || len(locked) != len(second) {
		t.Fatalf("selector changed the packet length: %d/%d/%d", len(locked), len(first), len(second))
	}
	offset := -1
	for i := range locked {
		if locked[i] != first[i] {
			if offset >= 0 {
				t.Fatalf("selector moved more than one byte (offsets %d and %d)", offset, i)
			}
			offset = i
		}
	}
	if offset < 0 {
		t.Fatalf("unlocked packet is byte-identical to the locked one")
	}
	if locked[offset] != 0xff || first[offset] != 0x00 || second[offset] != 0x01 {
		t.Fatalf("selector bytes = %#x / %#x / %#x, want 0xff / 0x00 / 0x01", locked[offset], first[offset], second[offset])
	}
	t.Logf("mode0 skill-tree selector at offset %d", offset)
}
