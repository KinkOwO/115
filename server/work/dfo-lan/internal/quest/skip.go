package quest

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ForceClearedModel is the progress model a skipped row carries when this build
// has no model for the quest's objective type. It is a name of its own rather than
// a borrowed one: a row that says "single-clear-map" would claim the server can
// check a map clear it never parsed, and storage refuses an empty model outright.
const ForceClearedModel = "force-cleared-objective-v1"

// RewardableOnForceClear reports whether the source marks a quest as paying its
// reward when it is force-cleared instead of played. The marker is an empty
// section, so it must survive runtime projection even when it has no cells.
func RewardableOnForceClear(d catalog.QuestDefinition) bool {
	for _, t := range d.Script.Cells {
		if t.Type == 3 && t.Text == "[rewardable on force clear]" {
			return true
		}
	}
	return false
}

// sweepModel names the rule a sweep's audit row was written under. It is a
// literal rather than a source checksum for the same reason the operator records
// are: the row says what ran, not which catalog happened to be loaded.
const sweepModel = "gm-quest-sweep-v1"

// SweepOutcome is one quest's fate inside a sweep apart from settlement: the
// grade filter excluded it, or the sweep never reached it. Settlement itself is
// reported through SkipResult.Settled.
type SweepOutcome struct {
	Quest uint16 `json:"quest"`
	Grade string `json:"grade,omitempty"`
	// Excluded marks a quest the character holds or can take whose [grade] is
	// outside the requested set. The sweep did not touch it.
	Excluded bool `json:"excluded,omitempty"`
}

// BlockedQuest is one main-line quest the sweep could not take, with the gate
// that stopped it.
type BlockedQuest struct {
	Quest  uint16 `json:"quest"`
	Grade  string `json:"grade,omitempty"`
	Reason string `json:"reason"`
}

// sweepRecord is the audit row of one press. One row per press rather than one
// per quest: a press that settles 940 quests is one decision, and 940 rows of it
// bury the ledger the row exists to be read from.
type sweepRecord struct {
	Operator string   `json:"operator"`
	Reason   string   `json:"reason"`
	Settled  []uint32 `json:"settled"`
	// Paid names the quests that actually handed something over, which is the
	// short list the source marks rewardable on force clear. Everything else on a
	// skip pays nothing, and "nothing" is the answer that has to stay auditable.
	Paid []uint32 `json:"paid,omitempty"`
}

// sweepKey derives the audit row's key from the set a press settled, so a retried
// press writes the row once instead of once per attempt. The ids are hashed
// rather than listed because the key has a length limit and a first/last/count
// summary can collide between two genuinely different sets.
func sweepKey(order []uint32) string {
	h := sha256.New()
	var b [4]byte
	for _, id := range order {
		binary.LittleEndian.PutUint32(b[:], id)
		h.Write(b[:])
	}
	return "gm-quest-sweep:" + hex.EncodeToString(h.Sum(nil))
}

// SkipRow is one quest a sweep settles. Model is the progress model the row is
// written under and Receipt the reward record that answers "what did this pay" -
// empty on the way in, because only the callback running under the character's
// lock knows what the bag ended up holding.
type SkipRow struct {
	Quest   uint16
	Model   string
	Receipt json.RawMessage
}

// SkipCommit is one sweep's answer.
type SkipCommit struct {
	Character character.Character
	// Settled is the rows this call wrote, in the order the caller supplied them,
	// each carrying the receipt the callback produced for it.
	Settled []SkipRow
	// EventApplied reports whether the audit row was new. A retried press that
	// settles the same set writes nothing the second time.
	EventApplied bool
}

// SkipResult is what one press of the client's button settled, plus why it
// stopped. The stop reason is the part an operator needs: without it a button
// that settled nothing is indistinguishable from a button that was never
// answered.
//
// There is no per-quest failure list. A sweep is one transaction, so it either
// wrote everything it decided on or nothing at all, and a quest it could not
// settle never entered the decision: the closure applies the same gates the offer
// list does.
type SkipResult struct {
	Settled  []FinishResult
	Excluded []SweepOutcome
	// Stopped names why no further quest was taken: "exhausted" (nothing main-line
	// left that this character qualifies for) or "limit" (the operator cap).
	Stopped string
	// Blocked is the frontier the sweep stopped at: main-line quests whose
	// prerequisites are all completed but that this build cannot offer.
	Blocked []BlockedQuest
	// Role is the character after everything the sweep wrote, the key items
	// included. The packet plan has to describe this state, not the state after
	// the last quest settled.
	Role character.Character
	// KeyItems are the templates list/questitembyticket.cos says a skip has to
	// hand over, granted once per character. Any grant failure aborts the whole
	// transaction instead of persisting a partial success receipt.
	KeyItems []uint32 `json:"key_items,omitempty"`
	// UnlockedEquipment is the OR of the extended-slot bits this sweep opened
	// (the three [slot expansion] main quests). The reply re-publishes the actor
	// record when it is non-zero, the same way a hand-in does.
	UnlockedEquipment byte `json:"unlocked_equipment,omitempty"`
}

// offeredNotSettled is the fallback reason blockedFrontier reports when a quest
// passed every gate it checks. SkipAllNow relabels it under a limit stop, where
// "not reached yet" is the truth; anywhere else it means a filter drifted.
const offeredNotSettled = "offered but not settled"

// audienceAllowed mirrors the offer list's audience gates (available.go) and
// names the one that refused, so a blocked frontier says which rule stopped
// instead of collapsing several rules into one sentence. An absent section is no
// restriction rather than a refusal, which is the reading the accept path has
// used since the [target character] fix.
func audienceAllowed(en *Entry, job string, advancement, awakening byte) (bool, string) {
	if !en.TargetUsable {
		return false, "audience condition is not readable"
	}
	if !jobAllowed(en.Jobs, job) {
		return false, fmt.Sprintf("job %s is not in the quest's [job] list", job)
	}
	if !en.GrowUsable {
		return false, "grow type condition is not readable"
	}
	for _, g := range en.GrowTypes {
		if g >= 0 && g != int32(advancement) {
			return false, fmt.Sprintf("class branch %d is not in the quest's [grow type] list", advancement)
		}
	}
	if !targetCharacterAllowed(en.TargetCharacters, job, advancement, awakening) {
		return false, fmt.Sprintf("[target character] does not list %s on class branch %d at awakening %d", job, advancement, awakening)
	}
	return true, ""
}

// applySkip is the sweep's whole effect on the character, run once under the
// character's row lock. A skip pays no experience and no gold, so the level and
// the wallet are left exactly as they were - which is also what keeps a sweep
// from cascading forever: no level-ups means no new quests released by level.
// The only things that move are the items on the handful of quests the source
// marks rewardable on force clear, and the extended-slot unlock bits.
//
// This is not the Finish path reused. Finish pays exactly what a hand-in pays; a
// skip that paid full rewards would turn the button into an experience and gold
// farm over the 1031 main-line quests and hand out gear the character never
// earned.
func (s *Service) applySkip(current character.Character, pending []SkipRow, operator, reason string, rewards FinishRewards) (json.RawMessage, []json.RawMessage, json.RawMessage, error) {
	var state character.State
	if e := json.Unmarshal(current.State, &state); e != nil {
		return nil, nil, nil, e
	}
	x := s.Index()
	saved := current
	receipts := make([]json.RawMessage, len(pending))
	settled := make([]uint32, 0, len(pending))
	var paid []uint32
	for i, row := range pending {
		var items []inventory.AwardReceipt
		var unlock byte
		en := x.Entries[uint32(row.Quest)]
		if en != nil && en.PaysOnForceClear {
			d := s.Catalog.Quests[uint32(row.Quest)]
			awards, e := rewards.Items(d.RewardCells, saved.Profession, state.Advancement)
			if e != nil {
				return nil, nil, nil, fmt.Errorf("quest %d pays on force clear but its reward cannot be selected: %w", row.Quest, e)
			}
			if len(awards) > 0 && s.Inventory == nil {
				return nil, nil, nil, ErrRewardPending
			}
			for _, a := range awards {
				var item inventory.AwardReceipt
				saved.State, item, e = s.Inventory.Grant(saved.State, a.Template, a.Amount)
				if e != nil {
					return nil, nil, nil, fmt.Errorf("quest %d pays on force clear but item %d does not fit: %w", row.Quest, a.Template, e)
				}
				items = append(items, item)
			}
			paid = append(paid, uint32(row.Quest))
		}
		// A sweep pays no experience, no gold and no items, but it does open a
		// slot. The sweep marks the quest completed, so refusing the slot expansion
		// here would lose it permanently with no way to earn it back - and unlike
		// gold or gear it is not something a player can farm again.
		if d, ok := s.Catalog.Quests[uint32(row.Quest)]; ok {
			if slot, isSlot := slotExpansion(d); isSlot {
				mask, known := slotUnlockMask(slot)
				if !known {
					return nil, nil, nil, fmt.Errorf("quest %d slot expansion index %d is out of range", row.Quest, slot)
				}
				if next, unlockErr := inventory.UnlockEquipSlots(saved.State, mask); unlockErr != nil {
					return nil, nil, nil, fmt.Errorf("quest %d slot expansion: %w", row.Quest, unlockErr)
				} else {
					saved.State = next
				}
				unlock = mask
			}
		}
		receipt, e := json.Marshal(FinishReceipt{Quest: row.Quest, Source: s.Catalog.Source.SaveIdentity(), Model: s.Progression.Rules.Model, Items: items, UnlockedEquipment: unlock})
		if e != nil {
			return nil, nil, nil, e
		}
		receipts[i] = receipt
		settled = append(settled, uint32(row.Quest))
	}
	record, e := json.Marshal(sweepRecord{Operator: operator, Reason: reason, Settled: settled, Paid: paid})
	return saved.State, receipts, record, e
}

// SkipBatch is the complete state and receipt change for one locked press.
type SkipBatch struct {
	State         json.RawMessage
	Rows          []SkipRow
	EventKey      string
	Outcome       json.RawMessage
	TicketOutcome json.RawMessage
}
type SkipStore interface {
	CommitMainlineSkip(context.Context, int64, int64, string, string, func(character.Character, []QuestState, json.RawMessage, bool) (SkipBatch, error)) (SkipCommit, error)
}

// SkipAllNow settles every main quest one press of the client's button reaches.
// It decides the whole set first and writes it once: the cascade is a closure
// over the source's prerequisite graph, so it needs no round trip per quest, and
// the write is one transaction that either lands completely or not at all.
//
// The gates are the source's - level, audience, prerequisites and
// list/notclearquestbyticket.lst - and the grade filter keeps the sweep off the
// side, daily and dungeon-life quests, which is the boundary a bulk sweep once
// got wrong. A quest whose objective type this build does not model is settled
// anyway: the source does not forbid skipping it, and a skip never evaluates the
// objective.
//
// The key items list/questitembyticket.cos names are handed over in the same
// transaction, once per character. Legacy partial receipts retry only failures.
//
// limit caps one press; zero means no cap.
func (s *Service) SkipAllNow(ctx context.Context, role character.Character, operator, reason string, grades GradeSet, limit int, rewards FinishRewards) (SkipResult, error) {
	var out SkipResult
	if s == nil || s.Skip == nil || s.Progression == nil || operator == "" || reason == "" {
		return out, fmt.Errorf("mainline skipping requires native rules, progression and audit identity")
	}
	if s.Skip.Source.SaveIdentity() != s.Catalog.Source.SaveIdentity() {
		return out, fmt.Errorf("quest skip source mismatch")
	}
	store, ok := s.Store.(SkipStore)
	if !ok {
		return out, fmt.Errorf("quest store lacks atomic mainline settlement")
	}
	commit, err := store.CommitMainlineSkip(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), s.Progression.Rules.Model, func(current character.Character, states []QuestState, ticket json.RawMessage, priorSweep bool) (SkipBatch, error) {
		batch := SkipBatch{State: current.State}
		var state character.State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return batch, e
		}
		status := map[uint32]string{}
		for _, q := range states {
			status[uint32(q.ID)] = q.Status
			if q.Status == "accepted" && q.ConfigVersion != current.ConfigVersion {
				return batch, fmt.Errorf("accepted quest %d requires source migration", q.ID)
			}
		}
		job := s.Professions.Professions[current.Profession].Job
		order, excluded, stopped := s.skipClosure(job, state, status, grades, limit)
		out.Excluded, out.Stopped = excluded, stopped
		for _, id := range order {
			model := ForceClearedModel
			if en := s.Index().Entries[id]; en != nil && en.Implemented {
				model = en.Model
			}
			batch.Rows = append(batch.Rows, SkipRow{Quest: uint16(id), Model: model})
		}
		if len(batch.Rows) > 0 {
			next, receipts, audit, e := s.applySkip(current, batch.Rows, operator, reason, rewards)
			if e != nil {
				return batch, e
			}
			batch.State, batch.Outcome, batch.EventKey = next, audit, sweepKey(order)
			for i := range batch.Rows {
				batch.Rows[i].Receipt = receipts[i]
				status[uint32(batch.Rows[i].Quest)] = "completed"
			}
		}
		if len(batch.Rows) > 0 || priorSweep {
			updated, items, receipt, e := s.prepareTicketItems(batch.State, ticket, operator, reason)
			if e != nil {
				return batch, e
			}
			batch.State, batch.TicketOutcome, out.KeyItems = updated, receipt, items
		}
		out.Blocked = s.blockedFrontier(job, state, status, 12)
		return batch, nil
	})
	if err != nil {
		return SkipResult{}, err
	}
	out.Role = commit.Character
	for _, row := range commit.Settled {
		var receipt FinishReceipt
		if e := json.Unmarshal(row.Receipt, &receipt); e != nil {
			return out, e
		}
		out.Settled = append(out.Settled, FinishResult{Role: out.Role, Receipt: receipt, Applied: true})
		out.UnlockedEquipment |= receipt.UnlockedEquipment
	}
	return out, nil
}

// skipClosure computes the set one press settles, in memory. Completing a quest
// releases its dependants and nothing else moves - a skip pays no experience, so
// the level stays put and releases nothing by level - which makes the answer a
// closure over the catalog's prerequisite graph and the completed set, both of
// which are already in hand. Passes repeat until one adds nothing; a pass is a
// walk of the 2844-row index, so the depth of the chain costs microseconds.
//
// The gates mirror available() with the unmodelled-objective gate lifted, and a
// quest the character already holds is taken whatever its gates say, which is
// also what the offer list does: an accepted row is a commitment the character
// already made.
func (s *Service) skipClosure(job string, state character.State, status map[uint32]string, grades GradeSet, limit int) (order []uint32, excluded []SweepOutcome, stopped string) {
	x := s.Index()
	adv, awaken := state.Advancement, state.Awakening
	done := make(map[uint32]bool, len(status))
	for id, st := range status {
		if st == "completed" {
			done[id] = true
		}
	}
	taken := make(map[uint32]bool)
	reported := make(map[uint32]bool)
	// A quest this press has already decided on counts as complete for the ones
	// behind it, which is what makes the cascade a closure rather than a re-read.
	reach := make(map[uint32]string, len(status)+64)
	for id, st := range status {
		reach[id] = st
	}
	for {
		added := false
		for _, id := range x.Ordered {
			if id == 0 || id >= 65535 || done[id] || taken[id] {
				continue
			}
			if s.Skip.Blocked[id] {
				continue
			}
			en := x.Entries[id]
			if en == nil || collisionsBlocked(en.Collisions, reach) {
				continue
			}
			// Choosing an unaccepted mutually exclusive branch is a player choice,
			// not something the source's numeric quest order may decide for them.
			if status[id] != "accepted" && len(en.Collisions) > 0 {
				continue
			}
			if status[id] != "accepted" {
				// A skip mints no experience, no gold and no items unless the source
				// marks the quest rewardable on force clear, so an unmodellable reward
				// selection cannot matter for the other 2839. A marked quest still has
				// to be settleable, because that one really does pay.
				payable := en.RewardUsable || !en.PaysOnForceClear
				if !payable || !en.GrowUsable {
					continue
				}
				if uint32(state.Level) < en.MinimumLevel || uint32(state.Level) > en.MaximumLevel {
					continue
				}
				if ok, _ := audienceAllowed(en, job, adv, awaken); !ok {
					continue
				}
				if !prerequisitesMet(en.PrerequisiteGroups, reach) {
					continue
				}
			}
			if !grades.Allows(en.Grade) {
				if !reported[id] {
					reported[id] = true
					excluded = append(excluded, SweepOutcome{Quest: uint16(id), Grade: en.Grade, Excluded: true})
				}
				continue
			}
			if limit > 0 && len(order) >= limit {
				return order, excluded, "limit"
			}
			taken[id] = true
			reach[id] = "completed"
			order = append(order, id)
			added = true
		}
		if !added {
			return order, excluded, "exhausted"
		}
	}
}

// blockedFrontier names the wall a cascade stopped at. Without it a sweep that
// settles nothing is indistinguishable from a sweep that was never asked, and
// "the story stopped here" has no answer except reading the catalog by hand.
//
// Only the frontier is reported - a quest whose prerequisites are not met is not
// blocked, it is simply not reached yet - so the list stays short and every entry
// in it is something worth fixing. It reads the caller's status map rather than
// the database, so the sweep can ask for the frontier of the state it is about to
// leave behind without another round trip.
func (s *Service) blockedFrontier(job string, state character.State, status map[uint32]string, limit int) []BlockedQuest {
	adv, awaken := state.Advancement, state.Awakening
	x := s.Index()
	grades := MainlineGrades()
	// walls are the stops worth fixing; otherCharacter are quests the source
	// wrote for a different class. They are kept apart and the walls reported
	// first, because a level 111 mage's frontier is otherwise twelve lines of
	// "written for a thief" with the real stop pushed off the end of the list.
	var walls, otherCharacter []BlockedQuest
	for _, id := range x.Ordered {
		if id == 0 || id >= 65535 || status[id] != "" {
			continue
		}
		en := x.Entries[id]
		if en == nil || !grades.Allows(en.Grade) {
			continue
		}
		if !prerequisitesMet(en.PrerequisiteGroups, status) {
			continue
		}
		ok, why := audienceAllowed(en, job, adv, awaken)
		if !ok && en.TargetUsable && en.GrowUsable {
			otherCharacter = append(otherCharacter, BlockedQuest{Quest: uint16(id), Grade: en.Grade, Reason: "written for another character: " + why})
			continue
		}
		var reason string
		// The gates mirror the skip path's, so every entry here is a real stop and
		// not a quest the sweep simply was not allowed to look at. An unmodelled
		// objective is deliberately absent from the list: a skip settles those.
		payable := en.RewardUsable || !en.PaysOnForceClear
		switch {
		case collisionsBlocked(en.Collisions, status):
			reason = "mutually exclusive branch already selected"
		case len(en.Collisions) > 0:
			reason = "mutually exclusive branch requires the player to accept a quest"
		case !en.TargetUsable:
			reason = "audience condition is not readable"
		case !en.GrowUsable:
			reason = "grow type condition is not readable"
		case !ok:
			reason = why
		case s.Skip != nil && s.Skip.Blocked[id]:
			reason = "list/notclearquestbyticket.lst forbids skipping it"
		case !payable:
			reason = "pays on force clear but its reward selection is not settleable"
		case uint32(state.Level) < en.MinimumLevel:
			reason = fmt.Sprintf("level %d below the quest minimum %d", state.Level, en.MinimumLevel)
		case uint32(state.Level) > en.MaximumLevel:
			reason = fmt.Sprintf("level %d above the quest maximum %d", state.Level, en.MaximumLevel)
		}
		if reason == "" {
			// Offered after all, so the sweep should have taken it. Naming that
			// keeps a filter drift from hiding behind an empty report.
			reason = offeredNotSettled
		}
		walls = append(walls, BlockedQuest{Quest: uint16(id), Grade: en.Grade, Reason: reason})
	}
	out := append(walls, otherCharacter...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ticketItemsModel is the character-event model behind the once-per-character key
// item grant.
const ticketItemsModel = "quest-skip-ticket-items-v1"

// prepareTicketItems is pure: any failed grant aborts the whole settlement.
// Existing receipts are honored. A legacy partial receipt retries only its failed
// templates, preserving the successful grants and the original event model.
func (s *Service) prepareTicketItems(state json.RawMessage, prior json.RawMessage, operator, reason string) (json.RawMessage, []uint32, json.RawMessage, error) {
	type ticketRecord struct {
		Operator string            `json:"operator"`
		Reason   string            `json:"reason"`
		Granted  []uint32          `json:"granted"`
		Failed   map[uint32]string `json:"failed,omitempty"`
	}
	rec := ticketRecord{Operator: operator, Reason: reason}
	if len(prior) > 0 {
		if err := json.Unmarshal(prior, &rec); err != nil {
			return nil, nil, nil, err
		}
		if len(rec.Failed) == 0 {
			return state, nil, nil, nil
		}
	}
	if s.Inventory == nil && len(s.Skip.Items) > 0 {
		return nil, nil, nil, ErrRewardPending
	}
	var items []uint32
	pending := rec.Failed
	for _, id := range s.Skip.Items {
		if len(prior) > 0 {
			if _, needed := pending[id]; !needed {
				continue
			}
		}
		next, _, err := s.Inventory.Grant(state, id, 1)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("mainline key item %d: %w", id, err)
		}
		state = next
		items = append(items, id)
		rec.Granted = append(rec.Granted, id)
		delete(rec.Failed, id)
	}
	if len(rec.Failed) > 0 {
		return nil, nil, nil, fmt.Errorf("legacy key receipt contains unresolved templates")
	}
	rec.Operator, rec.Reason = operator, reason
	receipt, err := json.Marshal(rec)
	return state, items, receipt, err
}
