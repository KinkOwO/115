package raid

import (
	"dfolan/internal/catalog"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *BakalOpening) grantWeightedBuffs(kind string, pool []catalog.BakalWeightedBuff, count uint32, now time.Time) error {
	pool = append([]catalog.BakalWeightedBuff(nil), pool...)
	check := s.rules.MonsterDefinitions[kind].BuffCheck
	for count > 0 && len(pool) > 0 {
		var total uint32
		for _, b := range pool {
			if b.Kind != s.grantedBuffs[check] {
				total += b.Weight
			}
		}
		if total == 0 {
			return nil
		}
		roll := uint32(s.randomIndex(int(total)))
		index := -1
		for i, b := range pool {
			if b.Kind == s.grantedBuffs[check] {
				continue
			}
			if roll < b.Weight {
				index = i
				break
			}
			roll -= b.Weight
		}
		if index < 0 {
			return fmt.Errorf("native buff weighted selection failed")
		}
		buff := pool[index]
		if err := s.grantBuff(buff.Kind, now); err != nil {
			return err
		}
		s.grantedBuffs[kind] = buff.Kind
		pool = append(pool[:index], pool[index+1:]...)
		count--
	}
	return nil
}

func (s *BakalOpening) grantBuff(kind string, now time.Time) error {
	b, exists := s.rules.BuffDefinitions[kind]
	if !exists {
		return fmt.Errorf("unknown source boss buff %s", kind)
	}
	if b.Raid {
		if b.Index < 20 || b.Index >= 25 {
			return fmt.Errorf("native raid buff index")
		}
		s.raidBuffs[b.Index-20]++
		s.effects = append(s.effects, BakalEffect{Op: "raid-buff", ID: b.Index, Value: int32(s.raidBuffs[b.Index-20])})
		return nil
	}
	if b.Kind == "AddRandomRaidBuff" {
		var options []int
		for _, r := range s.rules.BuffDefinitions {
			if r.Raid {
				options = append(options, int(r.Index))
			}
		}
		sort.Ints(options)
		if len(options) == 0 {
			return fmt.Errorf("source random raid buff pool empty")
		}
		index := uint32(options[s.randomIndex(len(options))])
		s.raidBuffs[index-20] += b.ServerValue
		s.effects = append(s.effects, BakalEffect{Op: "raid-buff", ID: index, Value: int32(s.raidBuffs[index-20])})
		return nil
	}
	duration := b.Duration
	if duration == 0 {
		duration = b.Cooltime
	}
	s.partyBuffs[kind] = now.Add(time.Duration(duration) * time.Second)
	s.effects = append(s.effects, BakalEffect{Op: "party-buff", Kind: kind, ID: b.Index, Value: int32(duration)})
	return nil
}

func (s *BakalOpening) RaidBuffCounts() [5]byte {
	var counts [5]byte
	for i, v := range s.raidBuffs {
		if v > 255 {
			v = 255
		}
		counts[i] = byte(v)
	}
	return counts
}
func (s *BakalOpening) PartyBuffs(now time.Time) map[uint32]uint32 {
	out := map[uint32]uint32{}
	for kind, expires := range s.partyBuffs {
		if expires.After(now) {
			out[s.rules.BuffDefinitions[kind].Index] = uint32(expires.Unix())
		}
	}
	return out
}

func (s *BakalOpening) UseRaidBuff(index uint32, now time.Time) error {
	if s == nil || !s.active || s.ended != "" {
		return fmt.Errorf("buff use outside active phase")
	}
	var b catalog.BakalBuffDefinition
	found := false
	for _, def := range s.rules.BuffDefinitions {
		if def.Index == index && def.Raid {
			b = def
			found = true
			break
		}
	}
	if !found || index < 20 || index >= 25 || s.raidBuffs[index-20] == 0 || s.buffCooldowns[b.Kind].After(now) || s.buffCooldowns["*"].After(now) {
		return fmt.Errorf("native raid buff unavailable or cooling down")
	}
	c := s.cloneScript()
	c.raidBuffs[index-20]--
	c.buffCooldowns[b.Kind] = now.Add(time.Duration(b.Cooltime) * time.Second)
	c.buffCooldowns["*"] = now.Add(time.Duration(c.rules.RaidBuffCooldownMillis) * time.Millisecond)
	if b.Duration > 0 {
		c.partyBuffs[b.Kind] = now.Add(time.Duration(b.Duration) * time.Second)
		c.effects = append(c.effects, BakalEffect{Op: "party-buff", ID: index, Kind: b.Kind, Value: int32(b.Duration)})
	}
	c.effects = append(c.effects, BakalEffect{Op: "raid-buff-used", ID: index, Value: int32(c.raidBuffs[index-20])})
	if b.Kind == "RaidBuffCampTeleport" {
		c.effects = append(c.effects, BakalEffect{Op: "kick", ID: c.currentDungeon})
		if c.currentDungeon != 0 {
			if err := c.GiveupDungeon(c.currentDungeon, now); err != nil {
				return err
			}
		}
	}
	*s = *c
	return nil
}

func (s *BakalOpening) ReportHealth(slot uint32, health int32, now time.Time) error {
	m, exists := s.monsters[slot]
	loc := s.rules.Locations[slot]
	if !s.active || s.ended != "" || !exists || loc.Dungeon != s.currentDungeon || health < 1 || health > s.variables["[MONSTER MAX HP]"] {
		return fmt.Errorf("health report outside owned native boss")
	}
	prefix := strings.ToUpper(m.Kind)
	hpName := "[" + prefix + " HP]"
	damageName := fmt.Sprintf("[%s PARTY%d DAMAGE]", prefix, s.roster[0].Position)
	if _, shared := s.rules.Symbols[hpName]; shared {
		current := s.variables[hpName]
		if health > current {
			return fmt.Errorf("client cannot heal shared native boss")
		}
		c := s.cloneScript()
		if err := c.ReportSymbol(c.rules.Symbols[damageName], c.variables[damageName]+current-health, now); err != nil {
			return err
		}
		// Locked HP is damage the boss did not take. Do not accumulate it as
		// hidden damage and spend it again after another dragon unlocks a floor.
		if accepted := c.variables["[MONSTER MAX HP]"] - c.variables[hpName]; c.variables[damageName] > accepted {
			var queue []BakalSignal
			if err := c.setScriptSymbol(damageName, accepted, &queue); err != nil {
				return err
			}
			for _, sig := range queue {
				if err := c.dispatchScript(sig, now); err != nil {
					return err
				}
			}
		}
		if m.SecondID != 0 && c.rules.Stage.HealthPercent > 0 && int64(c.variables[hpName])*100 <= int64(c.variables["[MONSTER MAX HP]"])*int64(c.rules.Stage.HealthPercent) && c.variables["[BAKAL HP UNLOCK GRADE]"] < c.rules.Stage.UnlockGradeBelow && c.variables["[BAKAL PHASE]"] == 0 {
			var queue []BakalSignal
			if err := c.setScriptSymbol("[BAKAL PHASE]", 1, &queue); err != nil {
				return err
			}
			for _, sig := range queue {
				if err := c.dispatchScript(sig, now); err != nil {
					return err
				}
			}
		}
		*s = *c
		return nil
	}
	c := s.cloneScript()
	if previous, ok := c.monsterHealth[slot]; ok && health > previous {
		return fmt.Errorf("client cannot heal native boss")
	}
	c.monsterHealth[slot] = health
	c.effects = append(c.effects, BakalEffect{Op: "health", Location: slot, Value: health})
	*s = *c
	return nil
}
