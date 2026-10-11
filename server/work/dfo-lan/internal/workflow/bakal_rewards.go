// Bakal reward ledger (巴卡尔攻坚战奖励账). Reconstruction of the handover
// binary's internal/workflow/bakal_rewards.go (221 lines, symbols
// BakalRewardService{Claim,Freeze,Recover} / BakalRaidAdmission /
// readBakalProgress / saveBakalProgress / ErrBakalWeeklyClearLimit — all
// recovered from the wireprobe-pvf.exe disassembly, see next151 §0.3).
//
// State contract (A-layer, byte-verified against the binary's string blob):
//   - the ledger lives in the character state under the top-level key
//     `bakal_raid_rewards` (go:string.*+47661, 18 bytes);
//   - the week is the Ispins raid week (database.IspinsWeekStart) formatted
//     RFC3339 — the same week key the ispins weekly quota uses;
//   - a plan row freezes what one clear owes: {run, source, week, content,
//     items, products, eligible}; `source` is the character's save identity
//     (ConfigVersion) at freeze time and `content` is the catalog-side
//     identity string — Claim revalidates both plus the server's current
//     savecontract.Identity() (raid_bakal_rewards.go:174, four compares);
//   - every Freeze records the run: an *eligible* clear reserves a weekly
//     reward slot (rewards++ and the plan is stored), an *ineligible* clear
//     only advances the weekly clear counter (clears++). Admission then
//     rejects further entries once clears reach [WEEKLY CLEAR LIMIT] —
//     普通巴卡尔本周通关次数已用完 (ErrBakalWeeklyClearLimit, the binary's
//     only workflow-level error variable, .data at 0x1415bb1b0).
//
// Event keys are `bakal-clear:<run>` (Freeze) and `bakal-grant:<run>` (Claim)
// under the model `bakal-source-reward-v1`, replay-safe through
// commitEquipmentEvent: a replayed event restores the persisted receipt and
// never re-runs the rules.
//
// Payout shape (A-layer, Claim.func1 :179-:198): the plan's Products are what
// a claim grants — wrappers were already taken apart at freeze time — and only
// a plan frozen without Products falls back to its Items, one unit per row
// (:186, MOVL $0x1). Freeze draws its randomness from crypto/rand (:118/:144);
// replay safety comes from the event ledger, not from a derived seed.
//
// Normal rewards select one source-weighted row per party_card/squad_item
// category from raidsystem/raidreward.etc. Preview items and monster-piece
// rank references are not award pools. Bidding remains a separate pathway.
package workflow

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
)

// bakalProgressKey is the character-state top-level key of the ledger.
const bakalProgressKey = "bakal_raid_rewards"

// bakalRewardModel is the character-event model of both reward events.
const bakalRewardModel = "bakal-source-reward-v1"

// bakalRewardCategories is the A-layer freeze order of the plan item
// categories (raid_bakal_rewards.go:103, two 10-byte string constants).
var bakalRewardCategories = [2]string{"party_card", "squad_item"}

// ErrBakalWeeklyClearLimit rejects starting another run this week. The text is
// the binary's own (errorString at .data 0x1415bb1c0, 42 bytes UTF-8).
var ErrBakalWeeklyClearLimit = errors.New("普通巴卡尔本周通关次数已用完")

// BakalRewardEntry rows live in catalog (catalog.BakalRewardEntry); the plan
// below stores them as Items.

// BakalRewardPlan is what Freeze stored for one run (A-layer shape: Run,
// Source, Week, Content, Items, Products, Eligible — go:dict evidence
// commitEquipmentEvent[...BakalRewardPlan], 7 fields, 112 bytes).
type BakalRewardPlan struct {
	Hard     bool                       `json:"hard,omitempty"`
	Run      string                     `json:"run"`
	Source   string                     `json:"source"`
	Week     string                     `json:"week"`
	Content  string                     `json:"content"`
	Items    []catalog.BakalRewardEntry `json:"items"`
	Products []loot.Award               `json:"products"`
	Eligible bool                       `json:"eligible"`
}

// bakalProgress is the persisted weekly ledger: the week stamp, the two
// counters (clears advanced by ineligible freezes, rewards reserved by
// eligible ones) and the frozen plans still waiting to be claimed.
type bakalProgress struct {
	Week    string                     `json:"week"`
	Clears  uint32                     `json:"clears"`
	Rewards uint32                     `json:"rewards"`
	Plans   map[string]BakalRewardPlan `json:"plans"`
}

// bakalWeekOf returns the raid week key of now.
func bakalWeekOf(now time.Time) string {
	return database.IspinsWeekStart(now).Format(time.RFC3339)
}

// readBakalProgress loads the ledger out of a raw character state. It returns
// the parsed top-level map (saveBakalProgress writes back into it) and the
// progress. A state that is not a JSON object, or a ledger row that fails to
// decode, is `invalid character state for raid rewards` (go:string.*+248412).
func readBakalProgress(state json.RawMessage) (map[string]json.RawMessage, bakalProgress, error) {
	var p bakalProgress
	var top map[string]json.RawMessage
	if len(state) > 0 {
		if err := json.Unmarshal(state, &top); err != nil || top == nil {
			return nil, p, errors.New("invalid character state for raid rewards")
		}
	}
	if top == nil {
		top = make(map[string]json.RawMessage)
	}
	if raw, ok := top[bakalProgressKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, p, errors.New("invalid character state for raid rewards")
		}
	}
	if p.Plans == nil {
		p.Plans = make(map[string]BakalRewardPlan)
	}
	return top, p, nil
}

// saveBakalProgress writes the ledger back into the top-level state map and
// returns the re-encoded state blob.
func saveBakalProgress(top map[string]json.RawMessage, p bakalProgress) (json.RawMessage, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	top[bakalProgressKey] = raw
	return json.Marshal(top)
}

// BakalRaidAdmission reports whether one more run may start this week. The
// rules carry [WEEKLY CLEAR LIMIT]; a ledger stamped with a different week
// never blocks (the counters reset with the next Freeze).
func BakalRaidAdmission(rules *catalog.BakalRaidRules, role database.Character, now time.Time) error {
	if rules == nil {
		// go:string.*+102372, 25 bytes
		return errors.New("native raid rules missing")
	}
	if rules.WeeklyClearLimit <= 0 {
		return nil
	}
	usage, err := BakalWeeklyUsage(role, now)
	if err != nil {
		return err
	}
	if usage.Clears >= uint32(rules.WeeklyClearLimit) {
		return ErrBakalWeeklyClearLimit
	}
	return nil
}

// BakalWeeklyCounters retains the two distinct existing ledger counters.
type BakalWeeklyCounters struct{ Clears, Rewards uint32 }

// BakalWeeklyUsage projects the existing ledger for both admission and UI.
// These are used counters, not remaining amounts. A new week is projected
// without rewriting the character or its pending reward plans.
func BakalWeeklyUsage(role database.Character, now time.Time) (BakalWeeklyCounters, error) {
	var usage BakalWeeklyCounters
	_, p, err := readBakalProgress(role.State)
	if err != nil {
		return usage, err
	}
	if p.Week == bakalWeekOf(now) {
		usage.Clears, usage.Rewards = p.Clears, p.Rewards
	}
	return usage, nil
}

// BakalRewardService freezes and pays out the raid rewards.
type BakalRewardService struct {
	// Store commits the two ledger events. *database.Store satisfies the narrow
	// interface, so the production wiring is unchanged; the unit tests inject
	// the in-memory fake.
	Store equipmentEventStore
	// Loot supplies the awarder (catalog + bag rules + equipment) and the
	// reward-box resolver; wrappers a plan carries are opened at freeze time.
	Loot *loot.Service
	// Rules is the bakal.etc projection.
	Rules *catalog.BakalRaidRules
	// Content is the catalog-side identity string stamped into every plan
	// (A-layer: a string read off the rules object at Claim time).
	Content string
}

// Freeze records one clear of run and freezes what it owes. cleared is the
// run's confirmed dungeon-clear count: a clear under [MINIMAL DUNGEON CLEAR
// COUNT] earns nothing, and once the week's [WEEKLY REWARD LIMIT] rewards are
// reserved, further clears only advance the clear counter.
//
// Event key: bakal-clear:<run> (model bakal-source-reward-v1).
func (s *BakalRewardService) Freeze(ctx context.Context, role database.Character, run string, cleared int, now time.Time) (BakalRewardPlan, database.Character, error) {
	var plan BakalRewardPlan
	if s == nil || s.Rules == nil || s.Store == nil || run == "" || role.ID == 0 {
		// go:string.*+174460, 32 bytes
		return plan, role, errors.New("raid reward source/owner missing")
	}
	// go:string.*+21789 (bakal-clear:) + run
	saved, plan, err := commitEquipmentEvent(ctx, s.Store, role, "bakal-clear:"+run, bakalRewardModel,
		func(current database.Character) (json.RawMessage, BakalRewardPlan, error) {
			return s.freeze(current, run, cleared, now)
		})
	return plan, saved, err
}

// freeze is the Freeze body, factored out for readability.
func (s *BakalRewardService) freeze(current database.Character, run string, cleared int, now time.Time) (json.RawMessage, BakalRewardPlan, error) {
	top, p, err := readBakalProgress(current.State)
	if err != nil {
		return nil, BakalRewardPlan{}, err
	}
	week := bakalWeekOf(now)
	if p.Week != week {
		// Week rollover: both counters reset; frozen plans survive (the
		// A-layer passes the plans map through, and Claim never re-checks
		// the week — a Sunday plan is claimable on Monday).
		p.Week, p.Clears, p.Rewards = week, 0, 0
	}
	plan := BakalRewardPlan{
		Run:     run,
		Source:  current.ConfigVersion, // save identity at freeze time
		Week:    week,
		Content: s.Content,
	}
	rules := s.Rules
	plan.Eligible = rules.MinimalDungeonClearCount <= cleared &&
		(rules.WeeklyRewardLimit <= 0 || p.Rewards < uint32(rules.WeeklyRewardLimit))
	if !plan.Eligible {
		// The clear still counts toward the weekly clear limit.
		p.Clears++
		blob, err := saveBakalProgress(top, p)
		return blob, plan, err
	}
	p.Rewards++ // reserve this week's reward slot (A-layer INCL at :156)
	plan.Items, err = s.freezeItems()
	if err != nil {
		return nil, BakalRewardPlan{}, err
	}
	plan.Products, err = s.openProducts(plan.Items)
	if err != nil {
		return nil, BakalRewardPlan{}, err
	}
	p.Plans[run] = plan
	blob, err := saveBakalProgress(top, p)
	return blob, plan, err
}

// freezeItems walks the two reward rows bakal.etc declares, in the A-layer
// category order (party_card then squad_item).
func (s *BakalRewardService) freezeItems() ([]catalog.BakalRewardEntry, error) {
	items := make([]catalog.BakalRewardEntry, 0, len(bakalRewardCategories))
	for _, category := range bakalRewardCategories {
		var total uint64
		for _, row := range s.Rules.Rewards {
			if row.Category == category {
				total += uint64(row.Weight)
			}
		}
		if total == 0 {
			return nil, fmt.Errorf("native raid reward category %s is empty", category)
		}
		draw, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetUint64(total))
		if err != nil {
			return nil, err
		}
		roll := draw.Uint64()
		for _, row := range s.Rules.Rewards {
			if row.Category != category {
				continue
			}
			if roll < uint64(row.Weight) {
				items = append(items, row)
				break
			}
			roll -= uint64(row.Weight)
		}
	}
	return items, nil
}

// openProducts expands wrapper templates into what opening them pays. The
// A-layer draws the box-open seed fresh from crypto/rand (raid_bakal_rewards.
// go:144 calls crypto/rand.Int right before loot.OpenRewardBoxes at :148);
// replay safety comes from the event ledger, never from a derived seed.
func (s *BakalRewardService) openProducts(items []catalog.BakalRewardEntry) ([]loot.Award, error) {
	awards := make([]loot.Award, 0, len(items))
	for _, item := range items {
		awards = append(awards, loot.Award{Template: item.Template, Amount: item.Amount})
	}
	var boxes loot.RewardBoxSource
	if s.Loot != nil {
		boxes = s.Loot.RewardBoxes
	}
	var seed uint32
	var entropy [4]byte
	if _, err := cryptorand.Read(entropy[:]); err != nil {
		return nil, err
	}
	seed = binary.LittleEndian.Uint32(entropy[:])
	if boxes == nil {
		return nil, fmt.Errorf("native raid reward wrapper resolver missing")
	}
	products, _, skipped := loot.OpenRewardBoxes(seed, boxes, awards)
	if len(skipped) > 0 {
		return nil, fmt.Errorf("native raid reward wrapper unresolved: %v", skipped)
	}
	return products, nil
}

// Claim pays out the plan frozen for run. The plan must still belong to this
// character: same run id, the save identity it was frozen with, the same
// catalog content, and the character must be on the server's current save
// contract (four compares, raid_bakal_rewards.go:174). The claim consumes the
// plan; the weekly reward slot was already reserved at Freeze.
//
// Event key: bakal-grant:<run> (model bakal-source-reward-v1).
func (s *BakalRewardService) Claim(ctx context.Context, role database.Character, run string, now time.Time) ([]inventory.AwardReceipt, database.Character, error) {
	if s == nil || s.Store == nil || s.Loot == nil {
		// go:string.*+122255, 27 bytes
		return nil, role, errors.New("raid reward service missing")
	}
	// go:string.*+21801 (bakal-grant:) + run
	saved, receipts, err := commitEquipmentEvent(ctx, s.Store, role, "bakal-grant:"+run, bakalRewardModel,
		func(current database.Character) (json.RawMessage, []inventory.AwardReceipt, error) {
			return s.claim(current, run)
		})
	return receipts, saved, err
}

// claim is the Claim body. now is unused here: the week is never re-checked at
// claim time (A-layer evidence), but the parameter keeps the public surface
// aligned with Freeze for the wireprobe wiring.
func (s *BakalRewardService) claim(current database.Character, run string) (json.RawMessage, []inventory.AwardReceipt, error) {
	if current.ConfigVersion != savecontract.Identity() {
		return nil, nil, errors.New("no owned source raid reward plan")
	}
	_, p, err := readBakalProgress(current.State)
	if err != nil {
		return nil, nil, err
	}
	plan, ok := p.Plans[run]
	if !ok || plan.Run != run || plan.Source != current.ConfigVersion || plan.Content != s.Content {
		// go:string.*+174492, 32 bytes
		return nil, nil, errors.New("no owned source raid reward plan")
	}
	// A-layer payout shape (Claim.func1 :179-:198): the plan's Products are
	// what gets granted; only a plan frozen without them falls back to its
	// Items, and the fallback converts each row into one unit of its template
	// (MOVL $0x1 at :186) — never Items plus Products.
	awards := plan.Products
	if awards == nil {
		awards = make([]loot.Award, 0, len(plan.Items))
		for _, item := range plan.Items {
			if item.Template == 0 {
				// go:string.*+194881, 34 bytes
				return nil, nil, fmt.Errorf("invalid persisted raid reward item: %s", run)
			}
			awards = append(awards, loot.Award{Template: item.Template, Amount: 1})
		}
	}
	awarder := &inventory.Awarder{Catalog: s.Loot.Catalog, Rules: s.Loot.BagRules, Equipment: s.Loot.Equipment}
	next := current
	receipts := make([]inventory.AwardReceipt, 0, len(awards))
	for _, award := range awards {
		if award.Template == 0 || award.Amount == 0 {
			// go:string.*+194881, 34 bytes
			return nil, nil, fmt.Errorf("invalid persisted raid reward item: %s", run)
		}
		state, receipt, err := awarder.Grant(next.State, award.Template, award.Amount)
		if err != nil {
			return nil, nil, err
		}
		next.State = state
		receipts = append(receipts, receipt)
	}
	// Consume the plan out of the granted state (A-layer: re-read, mapdelete,
	// save — the reward counter is not touched here).
	granted, p2, err := readBakalProgress(next.State)
	if err != nil {
		return nil, nil, err
	}
	delete(p2.Plans, run)
	blob, err := saveBakalProgress(granted, p2)
	if err != nil {
		return nil, nil, err
	}
	next.State = blob
	return next.State, receipts, nil
}

// Recover claims every plan still pending for the role, oldest run first
// (sorted: A-layer sort.Strings at :219). A failed claim keeps the role on its
// pre-claim state and does not stop the remaining claims; all failures are
// joined into the returned error.
func (s *BakalRewardService) Recover(ctx context.Context, role database.Character, now time.Time) (database.Character, error) {
	if s == nil || s.Store == nil {
		return role, errors.New("raid reward service missing")
	}
	_, p, err := readBakalProgress(role.State)
	if err != nil {
		return role, err
	}
	runs := make([]string, 0, len(p.Plans))
	for run := range p.Plans {
		runs = append(runs, run)
	}
	sort.Strings(runs)
	var errs []error
	for _, run := range runs {
		_, saved, err := s.Claim(ctx, role, run, now)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		role = saved
	}
	return role, errors.Join(errs...)
}
