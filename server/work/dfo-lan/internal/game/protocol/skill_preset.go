package protocol

import "fmt"

const SkillPresetConfigBytes = 14

// SkillPreset is the persistent part of CMD2346 SAVE_SKILL_PRESET. The native
// sender writes a 27-byte semantic request, but its first 13 bytes are scratch
// storage left by the request builder. The stable configuration begins at 13:
// a delay WORD, four skill WORDs, and four native state bytes. NOTI2758 reads
// exactly two consecutive 14-byte configurations.
type SkillPreset struct {
	Config [SkillPresetConfigBytes]byte
}

func DecodeSkillPresetSave(p []byte) (SkillPreset, error) {
	var out SkillPreset
	const semantic = 13 + SkillPresetConfigBytes
	if len(p) < semantic {
		return out, fmt.Errorf("short skill preset save")
	}
	// requestSaveSkillPreset uses the ordinary fixed-block writer, not the
	// folded-MD5 send-flush path. The five remaining bytes are block padding.
	if err := padding(p[semantic:], 16); err != nil {
		return out, err
	}
	copy(out.Config[:], p[13:semantic])
	return out, nil
}

// SkillPresetInfo encodes NOTI2758. This server exposes the same learned skill
// layout in both native skill contexts, so the saved preset is restored to
// both. The current client reader consumes exactly 28 bytes.
func SkillPresetInfo(p SkillPreset) []byte {
	out := make([]byte, SkillPresetConfigBytes*2)
	copy(out, p.Config[:])
	copy(out[SkillPresetConfigBytes:], p.Config[:])
	return out
}

func SkillPresetSaveSuccess() []byte { return []byte{1} }
