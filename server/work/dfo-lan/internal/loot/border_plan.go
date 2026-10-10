package loot

import (
	"dfolan/internal/dungeon"
	"fmt"
)

// This plan owns the opened rewards announced before the Border symbol starts.
// Publication commits the omen ledger only after all drop validation succeeds.
type borderRewardPlan struct {
	run           string
	dungeon, maze uint32
	source        string
	awards        []Award
	skipped       []string
	next          uint32
	grade         uint32
	omen          *OmenOutcome
	oath          bool
}

func IsBorderDungeon(id uint32) bool {
	return id >= 100005066 && id <= 100005068
}

// PrepareBorderRewards is idempotent for one run, including loading retries.
// No reward objects or persistent omen progress are published here.
func (s *Session) PrepareBorderRewards(d *dungeon.Session, seed uint32) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d == nil || d.RunID != s.Run || !IsBorderDungeon(d.Definition.ID) || !s.Attunement.Enabled() || s.attunementRolled {
		return 0, fmt.Errorf("border reward preparation outside an unpaid owned run")
	}
	if p := s.borderPlan; p != nil {
		if !s.borderPlanMatches(d) {
			return 0, fmt.Errorf("border reward plan identity changed")
		}
		return p.grade, nil
	}
	if s.RewardBoxes == nil || s.Equipment == nil {
		return 0, fmt.Errorf("border reward source unavailable")
	}
	// 档位已预掷（进本时随 noti 2838 下发，见 cmd/wireprobe/oath_info.go）⇒ 按档位选池：
	// 冻结下来的这份奖单与客户端珠子/天平显示的那一档必然是同一个数字。没预掷
	// （非调律副本、或调用方没接线）则退回内部预掷，行为与旧版逐字节相同。
	var awards []Award
	var next uint32
	var err error
	if s.Tiers != (RunTiers{}) {
		awards, next, err = s.Attunement.RollPlanned(seed, d.Definition.ID, uint32(d.Maze.Index), s.Tiers)
	} else {
		awards, next, err = s.Attunement.Roll(seed, d.Definition.ID, uint32(d.Maze.Index))
	}
	if err != nil {
		return 0, err
	}
	p := &borderRewardPlan{run: s.Run, dungeon: d.Definition.ID, maze: uint32(d.Maze.Index), source: s.Catalog.Source.Checksum, grade: 40}
	if s.Omen != nil && !s.omenRolled {
		out, extra, err := s.Omen.preview(s.Character, d.Definition.ID, next)
		if err != nil {
			return 0, err
		}
		awards = append(awards, extra...)
		next = out.Seed
		p.omen = &out
	}
	if coffer := oathTierCoffer(s.OathTier); coffer != 0 && !s.oathTierRolled {
		awards = append(awards, Award{Template: coffer, Amount: 1})
		p.oath = true
	}
	p.awards, p.next, p.skipped = OpenRewardBoxes(next, s.RewardBoxes, awards)
	for _, award := range p.awards {
		if s.stackable(award.Template) {
			continue
		}
		def, err := s.Equipment.Definition(award.Template)
		if err != nil {
			return 0, fmt.Errorf("border equipment %d: %w", award.Template, err)
		}
		rarity := def.Fields["[rarity]"]
		if len(rarity) != 1 || rarity[0].Type != 0 {
			return 0, fmt.Errorf("border equipment %d has invalid rarity", award.Template)
		}
		grade, err := borderRarityGrade(rarity[0].Value)
		if err != nil {
			return 0, err
		}
		if grade > p.grade {
			p.grade = grade
		}
	}
	s.borderPlan = p
	return p.grade, nil
}

func (s *Session) borderPlanMatches(d *dungeon.Session) bool {
	p := s.borderPlan
	return p != nil && p.run == s.Run && d.RunID == p.run && p.dungeon == d.Definition.ID && p.maze == uint32(d.Maze.Index) && p.source == s.Catalog.Source.Checksum
}

// PVF equipment rarity values and the ACT's 40..45 reward grades differ.
func borderRarityGrade(rarity int32) (uint32, error) {
	switch rarity {
	case 0, 1:
		return 40, nil
	case 2:
		return 41, nil
	case 3, 5:
		return 42, nil
	case 6:
		return 43, nil
	case 4:
		return 44, nil
	case 8:
		return 45, nil
	default:
		return 0, fmt.Errorf("unsupported Border equipment rarity %d", rarity)
	}
}
