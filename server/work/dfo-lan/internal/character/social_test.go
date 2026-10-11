package character

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSocialCharacterUsesSavedLookAndNativeGrowth(t *testing.T) {
	raw := json.RawMessage(`{"level":115,"advancement":3,"awakening":3,"future":{"keep":true},"inventory":{"version":"ordinary-bag-v1","weapon_skin":456,"worn":[{"slot":0,"template":100,"group":0},{"slot":0,"template":101,"group":1},{"slot":12,"template":123}]}}`)
	before := bytes.Clone(raw)
	s := &Service{ChannelContext: [2]byte{1, 10}}
	info, err := s.SocialCharacterInfo(Character{Name: "角色甲", Profession: 2, State: raw})
	if err != nil || info.CharacterName != "角色甲" || info.Job != 2 || info.GrowthType != 0x33 || info.Level != 115 || info.AvatarIndex[0] != 101 || info.WeaponIndex != 123 || info.WeaponSkinIndex != 456 || !bytes.Equal(raw, before) {
		t.Fatalf("saved profile %+v %v", info, err)
	}
}
