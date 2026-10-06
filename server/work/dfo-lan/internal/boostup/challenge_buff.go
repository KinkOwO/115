package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const VariousBuffPath = "etc/variousbufflist.etc"

// These effects are owned by the native client. N108 event665 + N2722
// enrollment enable its scene listener (140BAED70). Do not add them again to
// character attributes, fame, or damage in the server. Keep selected-source
// definitions here for validation/diagnostics, not a second buff calculator.
type ChallengeBuff struct {
	Index        uint32
	Contents     []uint32
	ChannelTypes []uint32
	Effects      map[string][]pvf.Token
	// This exception lives INSIDE [show buff alarm]: it suppresses the popup,
	// not necessarily the effect. Never use it to disable gameplay bonuses.
	AlarmExcludedDungeons []uint32
}

func positiveSourceIDs(c []pvf.Token, tag string) ([]uint32, error) {
	var out []uint32
	seen := map[uint32]bool{}
	for _, v := range values(c, tag) {
		if v.Type != 0 || v.Value <= 0 || seen[uint32(v.Value)] {
			return nil, fmt.Errorf("invalid/duplicate %s", tag)
		}
		seen[uint32(v.Value)] = true
		out = append(out, uint32(v.Value))
	}
	return out, nil
}

func ParseChallengeBuffs(challenge, definitions []pvf.Token) ([]ChallengeBuff, error) {
	sectionsFound, err := sections(challenge, "[buff info]", "[/buff info]")
	if err != nil {
		return nil, err
	}
	if len(sectionsFound) == 0 {
		return nil, nil
	}
	if len(sectionsFound) != 1 {
		return nil, fmt.Errorf("duplicate challenge buff info")
	}
	entries, err := sections(sectionsFound[0], "[buff]", "[/buff]")
	if err != nil {
		return nil, err
	}
	defs, err := sections(definitions, "[index]", "[/index]")
	if err != nil {
		return nil, err
	}
	var out []ChallengeBuff
	seen := map[uint32]bool{}
	for _, entry := range entries {
		id, err := number(entry, "[various buff index]", ^uint32(0))
		if err != nil || id == 0 || seen[id] {
			return nil, fmt.Errorf("invalid/duplicate challenge buff index")
		}
		seen[id] = true
		contents, err := positiveSourceIDs(entry, "[contents]")
		if err != nil {
			return nil, err
		}
		if len(contents) == 0 {
			return nil, fmt.Errorf("challenge buff %d has no contents", id)
		}
		var found []pvf.Token
		for _, d := range defs {
			if len(d) == 0 || d[0].Type != 0 || uint32(d[0].Value) != id {
				continue
			}
			if found != nil {
				return nil, fmt.Errorf("duplicate various buff %d", id)
			}
			found = d[1:]
		}
		if found == nil {
			return nil, fmt.Errorf("challenge references missing various buff %d", id)
		}
		b := ChallengeBuff{Index: id, Contents: contents, Effects: map[string][]pvf.Token{}}
		b.ChannelTypes, err = positiveSourceIDs(found, "[apply channel type]")
		if err != nil {
			return nil, err
		}
		effects, err := sections(found, "[effect]", "[/effect]")
		if err != nil {
			return nil, err
		}
		for _, effect := range effects {
			name := label(effect, "[name]")
			params := values(effect, "[param]")
			if name == "" || len(params) == 0 || b.Effects[name] != nil {
				return nil, fmt.Errorf("invalid various buff %d effect", id)
			}
			b.Effects[name] = append([]pvf.Token(nil), params...)
		}
		alarms, err := sections(found, "[show buff alarm]", "[/show buff alarm]")
		if err != nil {
			return nil, err
		}
		for _, alarm := range alarms {
			ids, err := positiveSourceIDs(alarm, "[except dungeon index]")
			if err != nil {
				return nil, err
			}
			b.AlarmExcludedDungeons = append(b.AlarmExcludedDungeons, ids...)
		}
		out = append(out, b)
	}
	return out, nil
}
