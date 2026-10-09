package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type BakalWeightedBuff struct {
	Kind   string
	Weight int
}
type BakalBuffDefinition struct {
	Kind                            string
	Duration, Cooltime, ServerValue int
	Raid                            bool
	Appendage                       []int
}

func loadBakalMonsterBuffs(ts []pvf.Token, m *BakalRaidMonster) error {
	var err error
	if m.BuffCount, err = bakalRowInt(ts, "[buff count]"); err != nil {
		return err
	}
	check := bakalRowStrings(ts, "[buff check]")
	if len(check) != 1 {
		return fmt.Errorf("missing native buff check")
	}
	m.BuffCheck = check[0]
	if m.HardBuffCount, err = bakalRowInt(ts, "[hard buff count]"); err != nil {
		return err
	}
	for _, entry := range []struct {
		label string
		out   *[]BakalWeightedBuff
	}{{"[buff list]", &m.BuffPool}, {"[additional buff list]", &m.AdditionalBuffPool}, {"[hard buff list]", &m.HardBuffPool}, {"[hard additional buff list]", &m.HardAdditionalBuffPool}} {
		blocks, e := bakalBlocks(ts, entry.label, "[/"+entry.label[1:])
		if e != nil {
			return e
		}
		if len(blocks) == 0 {
			continue
		}
		if len(blocks) != 1 {
			return fmt.Errorf("duplicate buff pool")
		}
		b := blocks[0]
		if len(b)%2 != 0 {
			return fmt.Errorf("invalid buff weighted pool")
		}
		for i := 0; i < len(b); i += 2 {
			if b[i].Type != 6 || b[i].Text == "" || b[i+1].Type != 0 || b[i+1].Value <= 0 {
				return fmt.Errorf("invalid source buff weight")
			}
			*entry.out = append(*entry.out, BakalWeightedBuff{b[i].Text, int(b[i+1].Value)})
		}
	}
	return nil
}
func loadBakalBuffDefinitions(a *pvf.Archive, r *BakalRaidRules) error {
	ts, err := readBakalCOSTokens(a, "contents/2022/bakalraid/etc/bakalbuff.cos")
	if err != nil {
		return err
	}
	if r.RaidBuffCooldownMillis, err = bakalRowInt(ts, "[raid buff cooltime]"); err != nil || r.RaidBuffCooldownMillis < 0 {
		return fmt.Errorf("invalid source shared buff cooldown")
	}
	r.BuffDefinitions = map[string]BakalBuffDefinition{}
	for _, kind := range []string{"[party buff]", "[raid buff]"} {
		blocks, e := bakalBlocks(ts, kind, "[/"+kind[1:])
		if e != nil {
			return e
		}
		for _, b := range blocks {
			names := bakalRowStrings(b, "[type]")
			if len(names) != 1 {
				return fmt.Errorf("invalid source buff name")
			}
			d := BakalBuffDefinition{Kind: names[0], Raid: kind == "[raid buff]"}
			if d.Duration, e = bakalRowInt(b, "[duration]"); e != nil {
				return e
			}
			if d.Cooltime, e = bakalRowInt(b, "[cooltime]"); e != nil {
				return e
			}
			if d.ServerValue, e = bakalRowInt(b, "[server value]"); e != nil {
				return e
			}
			if d.Duration < 0 || d.Cooltime < 0 || d.ServerValue < 0 {
				return fmt.Errorf("negative native buff value")
			}
			if row, ok := bakalRowByLabel(b, "[appendage value]"); ok {
				d.Appendage = bakalInts(row.Args)
			}
			if _, dup := r.BuffDefinitions[d.Kind]; dup {
				return fmt.Errorf("duplicate source buff")
			}
			r.BuffDefinitions[d.Kind] = d
		}
	}
	return nil
}
