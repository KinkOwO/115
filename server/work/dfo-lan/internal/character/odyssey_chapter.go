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

// ApplyOdysseyChapterReward grants one line of one chapter's [reward] block.
//
// The catalog carries only this one template, so a stale or tampered journal
// cannot make the server pay anything the journal did not name. We deliberately
// do not invent a stack limit: every chapter reward template has no explicit
// [stackable limit] in the source, so a "x2" line lands in one row.
func (s *ProgressionService) ApplyOdysseyChapterReward(role Character, chapter uint8, line int, r catalog.ChapterReward) (json.RawMessage, json.RawMessage, error) {
	if s.Chapters == nil || !OdysseyRole(role) || role.ConfigVersion != savecontract.Identity() {
		return nil, nil, fmt.Errorf("invalid Odyssey chapter reward")
	}
	ch, ok := s.Chapters.At(chapter)
	if !ok || line < 0 || line >= len(ch.Rewards) || ch.Rewards[line] != r {
		return nil, nil, fmt.Errorf("invalid Odyssey chapter reward line")
	}
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		r.Template: {ID: r.Template, Kind: "stackable", StackableType: "[booster]"},
	}}
	c.Source.Checksum = catalog.OdysseySource
	a := inventory.Awarder{Catalog: c, Rules: inventory.BagRules{Source: catalog.OdysseySource, Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1000}}
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
	if s.Chapters == nil || s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != savecontract.Identity() {
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
		for line, r := range ch.Rewards {
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
