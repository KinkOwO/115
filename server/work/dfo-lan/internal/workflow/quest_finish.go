package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/quest"
	"dfolan/internal/reward"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// QuestService coordinates persistence around quest domain state transitions.
type QuestService struct {
	Store *database.Store
	Quest *quest.Service
	// Rewards is the optional event-triggered reward notifier. It is only
	// called after a committed settlement; nil disables the feature.
	Rewards reward.Notifier
}

func (s *QuestService) Finish(ctx context.Context, role database.Character, r protocol.QuestSubmitRequest) (quest.FinishResult, error) {
	var out quest.FinishResult
	plan, e := s.Quest.PlanFinish(r)
	if e != nil {
		return out, e
	}
	d, model := plan.Definition, plan.Model
	// PrepareFinish runs under the character transaction and must not request
	// another pool connection for an independent contract read.
	growth, _ := s.Store.HasActivePremium(ctx, role.AccountID, database.PremiumGrowth, time.Now())
	commit, e := s.Store.CommitQuestReward(ctx, role.AccountID, role.ID, r.ID, s.Quest.Catalog.Source.SaveIdentity(), model, s.Quest.Progression.Rules.Model, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return s.Quest.PrepareFinish(current, plan, func() bool {
			return growth
		}, quest.FinishRewards{
			Items: questItemRewards,
			Experience: func(d catalog.QuestDefinition, level byte) (uint32, error) {
				base, err := character.GrowthQuestExperience(s.Quest.Progression.Catalog, d, level)
				if err != nil {
					return 0, err
				}
				return growthTopUpExperience(s.Quest.Catalog, s.Quest.Progression.Catalog.Thresholds, d, level, base)
			},
			Gold: func(d catalog.QuestDefinition, level byte) (uint32, error) {
				return character.GrowthQuestGold(s.Quest.Progression.Catalog, d, level)
			},
		})
	})
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(commit.Receipt, &out.Receipt); e != nil {
		return out, e
	}
	if out.Receipt.Quest != r.ID || out.Receipt.Source != s.Quest.Catalog.Source.SaveIdentity() {
		return out, fmt.Errorf("quest reward receipt mismatch")
	}
	out.Role, out.Applied = commit.Character, commit.Applied
	// Event-triggered rewards are best-effort and only fire on a fresh
	// settlement, never on an idempotent replay.
	if out.Applied && s.Rewards != nil {
		recipient := rewardRecipient(out.Role)
		var before character.State
		_ = json.Unmarshal(role.State, &before)
		if recipient.Level > before.Level {
			s.Rewards.LevelUp(ctx, recipient)
		}
		s.Rewards.QuestComplete(ctx, recipient, r.ID)
	}
	// Self-heal a pre-fix state: a character who once accepted several
	// [collision quest] branches still carries the unchosen factions' quests.
	// Once one branch completes, accepted siblings leave the journal (their
	// maps would otherwise be cleared again); completed siblings keep their
	// rewards. Best-effort: a sibling that vanished meanwhile is not an error.
	if out.Applied && len(d.Collisions) > 0 {
		if states, e := s.Store.Quests(ctx, role.AccountID, role.ID); e == nil {
			for _, q := range states {
				if q.Status != "accepted" {
					continue
				}
				for _, c := range d.Collisions {
					if uint32(q.ID) == c {
						_ = s.Store.AbandonQuest(ctx, role.AccountID, role.ID, q.ID)
						break
					}
				}
			}
		}
	}
	return out, nil
}

func questItemRewards(cells []pvf.Token, profession, advancement byte) ([]quest.RewardItem, error) {
	items, err := character.GrowthItemRewards(cells, profession, advancement)
	if err != nil {
		return nil, err
	}
	var out []quest.RewardItem
	for _, item := range items {
		out = append(out, quest.RewardItem{Template: item.Template, Amount: item.Amount})
	}
	return out, nil
}

// rewardRecipient builds the notifier recipient from a committed character.
// A malformed state leaves the level at 0, which suppresses level-up notices.
func rewardRecipient(c character.Character) reward.Recipient {
	var state character.State
	_ = json.Unmarshal(c.State, &state)
	return reward.Recipient{AccountID: c.AccountID, CharacterID: c.ID, Name: c.Name, Level: state.Level, ConfigVersion: c.ConfigVersion}
}

// ---------------------------------------------------------------------------
// Mainline quest experience top-up
//
// Story mainline quests are authored with [pre required quest] chains whose
// minimum levels climb faster than the quest reward experience (e.g. 3145 is
// a level-1 quest whose follower 4873 needs level 5; the 1200 base reward
// only reaches level 2). Players who only run mainline therefore stall
// between mainline quests. We raise the completion experience of a mainline
// quest so the character lands exactly on the minimum level of its next
// mainline follower. Side quests and already sufficient levels are untouched.
// ---------------------------------------------------------------------------

var (
	mainNextMu    sync.RWMutex
	mainNextBySrc = map[string]map[uint32]uint32{}
)

func isMainlineQuestPath(path string) bool {
	p := strings.ToLower(path)
	for _, seg := range []string{"new_scenario_renewal", "epic_quest", "episode_quest", "110levelscenario", "/mission/"} {
		if strings.Contains(p, seg) {
			return true
		}
	}
	return false
}

// mainlineNextLevels maps each mainline quest id to the smallest minimum
// level among its mainline followers (reverse [pre required quest]).
func mainlineNextLevels(cat catalog.QuestCatalog) map[uint32]uint32 {
	key := cat.Source.SaveIdentity()
	mainNextMu.RLock()
	m, ok := mainNextBySrc[key]
	mainNextMu.RUnlock()
	if ok {
		return m
	}
	m = map[uint32]uint32{}
	for _, q := range cat.Quests {
		if !isMainlineQuestPath(q.Script.Path) {
			continue
		}
		for _, p := range q.Prerequisites {
			if cur, ok := m[p]; !ok || q.MinimumLevel < cur {
				m[p] = q.MinimumLevel
			}
		}
	}
	mainNextMu.Lock()
	mainNextBySrc[key] = m
	mainNextMu.Unlock()
	return m
}

// growthTopUpExperience returns the top-up amount needed for a just-finished
// mainline quest so the character can immediately pick up the next mainline
// quest. Cumulative thresholds follow level L -> Thresholds[L-2] (see
// growth_rules.go ApplyGain).
func growthTopUpExperience(cat catalog.QuestCatalog, thresholds []uint64, d catalog.QuestDefinition, level byte, base uint32) (uint32, error) {
	if !isMainlineQuestPath(d.Script.Path) || len(thresholds) == 0 {
		return base, nil
	}
	next, ok := mainlineNextLevels(cat)[d.ID]
	if !ok || next <= uint32(level) {
		return base, nil
	}
	need := uint64(0)
	if next >= 2 && int(next)-2 < len(thresholds) {
		need = thresholds[next-2]
	}
	if level >= 2 && int(level)-2 < len(thresholds) {
		had := thresholds[level-2]
		if need > had {
			need -= had
		} else {
			need = 0
		}
	}
	if need == 0 || uint64(base) >= need {
		return base, nil
	}
	if need > math.MaxUint32 {
		need = math.MaxUint32
	}
	return uint32(need), nil
}
