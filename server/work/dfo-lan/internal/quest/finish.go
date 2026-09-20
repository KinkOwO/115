package quest

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

type FinishReceipt struct {
	Quest      uint16                   `json:"quest"`
	Experience uint32                   `json:"experience"`
	Gold       uint32                   `json:"gold,omitempty"`
	Source     string                   `json:"source"`
	Model      string                   `json:"model"`
	Items      []inventory.AwardReceipt `json:"items,omitempty"`
}
type FinishResult struct {
	Role    storage.Character
	Receipt FinishReceipt
	Applied bool
}

func cells(c []pvf.Token, name string) []pvf.Token {
	var a []pvf.Token
	on := false
	for _, v := range c {
		if v.Type == 3 {
			on = v.Text == name
			continue
		}
		if on {
			a = append(a, v)
		}
	}
	return a
}

func (s *Service) Finish(ctx context.Context, role storage.Character, r protocol.QuestSubmitRequest) (FinishResult, error) {
	var out FinishResult
	if s.Progression == nil {
		return out, ErrRewardPending
	}
	if r.RewardSelection != 65535 || r.Option != 1 {
		return out, fmt.Errorf("quest reward selection requires inventory settlement")
	}
	d, ok := s.Catalog.Quests[uint32(r.ID)]
	if !ok {
		return out, fmt.Errorf("quest source missing")
	}
	_, model, e := InitialProgress(d)
	if e != nil {
		return out, e
	}
	// Current3145 omits reward type but supplies typed [job] item tuples.
	// Matching item tuples use the shared inventory awarder. An absent type
	// never implies a profession change or a fabricated reward. [none] carries
	// no items and settles on experience alone. The same test gates the
	// available list, so a quest that cannot settle is never offered.
	if !rewardUsable(d) {
		return out, ErrRewardPending
	}
	commit, e := s.Store.CommitQuestReward(ctx, role.AccountID, role.ID, r.ID, s.Catalog.Source.Checksum, model, s.Progression.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state character.State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		awards, e := progression.ItemRewards(d.RewardCells, current.Profession, state.Advancement)
		if e != nil {
			return nil, nil, e
		}
		if len(awards) > 0 && s.Inventory == nil {
			return nil, nil, ErrRewardPending
		}
		// EXP, gold, inventory and completion are prepared within one
		// transaction.
		gain, e := progression.QuestExperience(s.Progression.Catalog, d, state.Level)
		if e != nil {
			return nil, nil, e
		}
		if s.Store != nil {
			if hasGrowth, _ := s.Store.HasActivePremium(ctx, role.AccountID, storage.PremiumGrowth, time.Now()); hasGrowth {
				gain = gain + gain*20/100
			}
		}
		gold, e := progression.QuestGold(s.Progression.Catalog, d, state.Level)
		if e != nil {
			return nil, nil, e
		}
		saved, _, e := s.Progression.ApplyGain(current, uint64(gain))
		if e != nil {
			return nil, nil, e
		}
		// Completion gold comes from the [gold reward table], not from the
		// quest's own cells; it is credited to the wallet (award id 0) in the
		// same transaction. Item awards need the inventory service; gold does
		// not, so a quest that pays only gold still settles when items are
		// unconfigured, as long as the wallet can take it.
		if gold > 0 {
			if s.Inventory == nil {
				return nil, nil, ErrRewardPending
			}
			saved.State, _, e = s.Inventory.Grant(saved.State, 0, gold)
			if e != nil {
				return nil, nil, e
			}
		}
		var items []inventory.AwardReceipt
		for _, a := range awards {
			var item inventory.AwardReceipt
			saved.State, item, e = s.Inventory.Grant(saved.State, a.Template, a.Amount)
			if e != nil {
				return nil, nil, e
			}
			items = append(items, item)
		}
		receipt, e := json.Marshal(FinishReceipt{Quest: r.ID, Experience: gain, Gold: gold, Source: s.Catalog.Source.Checksum, Model: s.Progression.Rules.Model, Items: items})
		return saved.State, receipt, e
	})
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(commit.Receipt, &out.Receipt); e != nil {
		return out, e
	}
	if out.Receipt.Quest != r.ID || out.Receipt.Source != s.Catalog.Source.Checksum {
		return out, fmt.Errorf("quest reward receipt mismatch")
	}
	out.Role, out.Applied = commit.Character, commit.Applied
	return out, nil
}

func (s *Service) Completed(ctx context.Context, role storage.Character) ([]uint32, error) {
	states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
	if e != nil {
		return nil, e
	}
	var ids []uint32
	for _, q := range states {
		if q.Status == "completed" {
			if q.ConfigVersion != s.Catalog.Source.Checksum {
				return nil, fmt.Errorf("completed quest source mismatch")
			}
			ids = append(ids, uint32(q.ID))
		}
	}
	return ids, nil
}
