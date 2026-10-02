package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
	"sort"
)

func (s *ProgressionService) chapterRewards(ch catalog.OdysseyChapter) []catalog.ChapterReward {
	if s.CompletionRewards != nil {
		return s.CompletionRewards.At(ch.Number)
	}
	return ch.Rewards // Historical catalog mode only; PVF startup requires the mapping.
}

// ApplyOdysseyChapterReward rechecks server-owned completion under the lock.
// Native inventory metadata handles selection boxes as well as booster boxes.
func (s *ProgressionService) ApplyOdysseyChapterReward(role Character, chapter uint8, line int, r catalog.ChapterReward) (json.RawMessage, json.RawMessage, error) {
	if s.Chapters == nil || s.Odyssey == nil || !CreatedAsOdyssey(role) || role.ConfigVersion != savecontract.Identity() {
		return nil, nil, fmt.Errorf("invalid Odyssey chapter reward")
	}
	ch, ok := s.Chapters.At(chapter)
	rewards := s.chapterRewards(ch)
	if !ok || line < 0 || line >= len(rewards) || rewards[line] != r {
		return nil, nil, fmt.Errorf("invalid Odyssey chapter reward line")
	}
	completed, e := s.odysseyCompleted(role)
	if e != nil {
		return nil, nil, e
	}
	cleared := false
	for _, id := range completed {
		if id == ch.Final {
			cleared = true
			break
		}
	}
	if !cleared {
		return nil, nil, fmt.Errorf("Odyssey chapter %d is not completed", chapter)
	}
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		r.Template: {ID: r.Template, Kind: "stackable", StackableType: "[booster]"},
	}}
	c.Source.Checksum = catalog.OdysseySource
	a := inventory.Awarder{Catalog: c, Rules: inventory.BagRules{Source: catalog.OdysseySource, Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1000}}
	if s.CompletionRewards != nil {
		if s.CompletionAwarder == nil {
			return nil, nil, fmt.Errorf("Odyssey completion inventory source missing")
		}
		a = *s.CompletionAwarder
	}
	raw, receipt, e := a.Grant(role.State, r.Template, r.Count)
	if e != nil {
		return nil, nil, e
	}
	proof, e := json.Marshal(map[string]any{"chapter": chapter, "line": line, "receipt": receipt})
	return raw, proof, e
}

// OdysseyChapterRewards pays every chapter whose final lord this character has
// really cleared, using only server-owned evidence: the completed-dungeon list
// and the persisted best times (see odysseyCompleted). Client claims are never
// consulted.
//
// Each line has its own receipt (odyssey-chapter-reward:<chapter>:<line>:<template>),
// so a full bag only leaves that one line owed — the next login or dungeon
// clear retries it, and nothing already paid is paid twice.
func (s *ProgressionService) OdysseyChapterRewards(ctx context.Context, role Character) (Character, bool, []error) {
	if s.Chapters == nil || s.Odyssey == nil || !CreatedAsOdyssey(role) || role.ConfigVersion != savecontract.Identity() {
		return role, false, nil
	}
	completed, e := s.odysseyCompleted(role)
	if e != nil {
		return role, false, []error{e}
	}
	cleared := make(map[uint32]bool, len(completed))
	for _, id := range completed {
		cleared[id] = true
	}
	var chapters []int
	for number := range s.Chapters.Chapters {
		ch := s.Chapters.Chapters[number]
		if cleared[ch.Final] {
			chapters = append(chapters, int(ch.Number))
		}
	}
	sort.Ints(chapters)
	changed := false
	var pending []error
	for _, n := range chapters {
		ch, _ := s.Chapters.At(uint8(n))
		for line, r := range s.chapterRewards(ch) {
			key := fmt.Sprintf("odyssey-chapter-reward:%d:%d:%d", ch.Number, line, r.Template)
			reward, idx := r, line
			next, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, savecontract.Identity(), key, "odyssey-source-chapter-reward-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
				return s.ApplyOdysseyChapterReward(current, ch.Number, idx, reward)
			})
			if e != nil {
				pending = append(pending, fmt.Errorf("chapter %d reward %d remains owed: %w", ch.Number, r.Template, e))
				continue
			}
			next.WireID = role.WireID
			role = next
			changed = changed || applied
		}
	}
	return role, changed, pending
}
