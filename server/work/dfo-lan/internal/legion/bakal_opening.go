// BakalOpening — the run-state machine of the Bakal raid assault
// (巴卡尔攻坚战). Mirrors the handover binary's internal/raid.BakalOpening
// (28 methods, see next151 §0.3): it owns the N574 phase lifecycle, the
// open/health/settlement bursts, dungeon authorization, the script warp /
// camp-return gates and the buff bookkeeping, all fed by the catalog
// projection of bakal.etc (internal/catalog BakalRaidRules) — no rule value
// is re-typed here.
//
// Lifecycle (B-layer clear run, 241-event timeline):
//
//	CMD656 create (idle) → CMD2089 start → preparing (vote burst, N2343/
//	N2344/N578/N574=0100/N581) → +[START DELAY TIME] 3s → active (open burst
//	N574=0200/N2285 camp/N2286 roster/N2288/N572/N584) → +1s (roster repeat +
//	four boss health rows) → 24 map loads (per-dungeon: N2281+N2285 on entry,
//	2073 loading ack, 2070 warp acks, N2286 defeat rows on confirmed native
//	entity deaths, N572 single-dungeon state on boss clears) →
//	final boss defeat in [SettlementDungeon] (N2286 defeat row → N27 final
//	selection → N2281 final move → N2285 settlement room) → +[SET TIMER]
//	28 56 180 → settlement burst (N2285 return-to-camp → reward inventory →
//	N588 clear result → N574=0000 phase ended).
//
// Refusal reasons are byte-identical to the handover event log
// (testdata/bakal_refusals.json): scripted warps outside loaded owned combat
// and invalid native camp returns keep the captured texts.
//
// N2194 uses the ordinary registered dynamic entity/template spawn contract.
// Member counters in N2285 remain wire presentation data for this solo run.
package legion

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"dfolan/internal/catalog"
)

// BakalCampLocation is the raid-map location the open burst, the legal
// retreats and the settlement return land on (0x34=52 on every captured
// N2285 camp frame).
const BakalCampLocation uint32 = 52

// BakalOpeningStage is the N574-visible lifecycle stage of one run.
type BakalOpeningStage int

const (
	BakalOpeningIdle      BakalOpeningStage = iota // raid created (CMD656), not started
	BakalOpeningPreparing                          // CMD2089 accepted, vote window
	BakalOpeningActive                             // after [START DELAY TIME]
	BakalOpeningFinal                              // final boss defeated, settlement window
	BakalOpeningEnded                              // settlement burst sent
	BakalOpeningFailed                             // source anger/time failure
)

// BakalOpening is one raid run. Construction: PrepareBakalOpening. A zero
// value is not usable.
type BakalOpening struct {
	runtime  *bakalRuntime
	eventErr error
	rules    *catalog.BakalRaidRules
	script   *catalog.BakalPhaseRules // NormalPhase or HardPhase per hard
	hard     bool

	stage BakalOpeningStage
	name  string // the owned waiting-room member that started the run

	startedAt time.Time
	readyAt   time.Time // startedAt + [START DELAY TIME]
	settleAt  time.Time // final defeat + [SET TIMER] 28 56 180

	openPublished        bool
	healthPublished      bool
	remainingPublishedAt time.Time
	settled              bool

	buffs        [5]byte
	symbolValues map[string]int32

	current           uint32 // dungeon the member is inside (0 = camp)
	location          uint32 // current raid-map location
	cleared           map[uint32]bool
	defeatedLocations map[uint32]bool // field leaders persist across source room changes
	placements        map[uint32]string
	reservedPublished map[int]bool
	loaded            map[uint32]bool
	bossTypes         map[uint32]string // boss dungeon id → bakal.etc type

	anger              int // advanceAnger bookkeeping (A-layer structural, no frames)
	angerLast          time.Time
	angerRate          int
	angerWindowApplied bool
	dungeonEnteredAt   time.Time
	settlementReady    bool
	firstPhaseDefeated bool
	secondPhase        bool
	campReturnPending  bool
	recoveryUntil      time.Time
	penalizedReturns   int

	// rewardInventory is spliced into the settlement burst between the
	// return-to-camp and the clear result (timeline 237→238→239→240). The
	// 10141B N13 body is the reward service's (impl: workflow BakalRewardService).
	rewardInventory []byte

	// GrantHook returns the buff slots a boss-dungeon clear grants. The
	// captured grants are weighted-random (binary bakalWeightedNumbers) and
	// the weights are not in the catalog projection yet, so the policy stays
	// injectable: nil = no N2288 grant frames.
	GrantHook func(dungeon uint32) []int
}

// PrepareBakalOpening binds a run to its catalog rules. The rules carry the
// waiting-room anchors (town 152 area 2) — without them the waiting room is
// not bound, so the captured refusal reason applies.
func PrepareBakalOpening(rules *catalog.BakalRaidRules, hard bool) (*BakalOpening, error) {
	if rules == nil {
		return nil, ErrBakalWaitingRoomNotBound
	}
	script := rules.NormalPhase
	if hard {
		script = rules.HardPhase
	}
	if script == nil {
		return nil, fmt.Errorf("bakal raid rules carry no %s phase script", map[bool]string{false: "normal", true: "hard"}[hard])
	}
	bossTypes := make(map[uint32]string)
	for _, dungeon := range rules.Dungeons {
		if dungeon.Type != "" {
			bossTypes[dungeon.Index] = dungeon.Type
		}
	}
	if len(bossTypes) == 0 {
		return nil, fmt.Errorf("bakal raid rules carry no boss dungeons")
	}
	values := make(map[string]int32, len(script.InitialSymbols))
	for key, value := range script.InitialSymbols {
		values[key] = value
	}
	placements := make(map[uint32]string, len(script.InitMonsters))
	for _, spawn := range script.InitMonsters {
		placements[uint32(spawn.Location)] = spawn.Name
	}
	return &BakalOpening{
		placements:        placements,
		reservedPublished: make(map[int]bool),
		symbolValues:      values,
		rules:             rules,
		script:            script,
		hard:              hard,
		cleared:           make(map[uint32]bool),
		defeatedLocations: make(map[uint32]bool),
		loaded:            make(map[uint32]bool),
		bossTypes:         bossTypes,
	}, nil
}

// Stage reports the lifecycle stage.
func (o *BakalOpening) Stage() BakalOpeningStage { return o.stage }

func (o *BakalOpening) ActiveSince() time.Time { return o.readyAt }

// Anger reports the run's accumulated anger (advanceAnger bookkeeping: the
// [INCREASE BAKAL ANGER] per-second channel drives N570 on the handover
// server; the frames are not in the exported timeline, so anger is tracked
// structurally only).
func (o *BakalOpening) Anger() int { return o.anger }

// Re-publish owned inventory after scene initialization without activating a
// commander effect. Native142542f70 reads five u8 counts, used kind25, and
// an unset target (-1). Reading a snapshot never consumes or grants stock.
func (o *BakalOpening) BuffInventorySnapshot() BakalFrame {
	return BakalFrame{"bakal_commander_inventory_restored", NotiBakalBuffInventory, BakalBuffInventoryFrame(o.buffs, BakalBuffTailOpen)}
}

// Start accepts CMD2089 and publishes the vote burst (timeline 000–005; the
// 2089 ACK stays with the dispatch layer). The vote window closes after
// [START DELAY TIME] seconds; Tick publishes the open burst then.
func (o *BakalOpening) Start(name string, now time.Time) ([]BakalFrame, error) {
	if o.stage != BakalOpeningIdle {
		return nil, fmt.Errorf("bakal start from stage %d", o.stage)
	}
	if name == "" {
		return nil, ErrBakalStartRequiresMember
	}
	o.stage = BakalOpeningPreparing
	o.name = name
	o.startedAt = now
	o.readyAt = now.Add(time.Duration(o.rules.StartDelaySecs) * time.Second)
	return []BakalFrame{
		{"bakal_vote_start", NotiBakalVoteStart, BakalVoteStartFrame()},
		{"bakal_vote_state", NotiBakalVoteState, BakalVoteStateFrame()},
		{"bakal_real_member_assigned", NotiBakalMemberAssigned, BakalMemberAssignedFrame(name)},
		{"bakal_preparing", NotiBakalRaidState, []byte(BakalRaidStatePreparing)},
		{"bakal_countdown", NotiBakalCountdown, nil},
	}, nil
}

// Tick drives the timed transitions: the open burst at readyAt, the +1s boss
// health publish, per-second anger advance, and the settlement burst at
// settleAt. Safe to call every server tick; each transition fires once.
func (o *BakalOpening) Tick(now time.Time) []BakalFrame {
	var frames []BakalFrame
	if o.stage == BakalOpeningPreparing && !now.Before(o.readyAt) {
		frames = append(frames, o.activate()...)
	}
	if o.stage == BakalOpeningActive && !o.healthPublished && now.Sub(o.readyAt) >= time.Second {
		o.healthPublished = true
		roster := BakalOpeningRoster(o.script)
		if o.runtime != nil {
			roster = o.eventRoster()
		}
		frames = append(frames,
			BakalFrame{"bakal_script_monsters", NotiBakalMonsters, BakalMonstersFrame(roster)},
			BakalFrame{"bakal_script_actor_health", NotiBakalMonsters, BakalMonstersFrame(BakalBossHealthRows(o.script))},
		)
	}
	if o.stage == BakalOpeningActive {
		if o.runtime != nil {
			next, err := o.tickEvents(now)
			o.eventErr = err
			if err != nil {
				return nil
			}
			frames = append(frames, next...)
		} else {
			frames = append(frames, o.publishInitialReservations(now)...)
		}
		// Official N584 synchronizes the source countdown about every10s.
		// This is wire publication cadence, not a second gameplay duration.
		if now.Sub(o.remainingPublishedAt) >= 10*time.Second {
			elapsed := int64(now.Sub(o.readyAt) / time.Second)
			remaining := int64(o.rules.PhaseTimeOverSecs) - elapsed
			if remaining < 0 {
				remaining = 0
			}
			frames = append(frames, BakalFrame{"bakal_remaining", NotiBakalRemaining, BakalRemainingFrame(uint32(remaining))})
			o.remainingPublishedAt = now
		}
		before := o.anger
		if o.runtime == nil {
			o.advanceAnger(now)
		}
		if before != o.anger {
			frames = append(frames, o.SymbolFrame("[BAKAL ANGER]", int32(o.anger))...)
		}
		if o.stage == BakalOpeningActive && (o.script.AngerFailThreshold > 0 && o.anger >= o.script.AngerFailThreshold || o.rules.PhaseTimeOverSecs > 0 && !now.Before(o.readyAt.Add(time.Duration(o.rules.PhaseTimeOverSecs)*time.Second))) {
			o.stage = BakalOpeningFailed
			frames = append(frames, BakalFrame{"bakal_script_phase_ended", NotiBakalRaidState, []byte(BakalRaidStateEnded)})
		}
		if o.stage == BakalOpeningActive && o.current == uint32(o.script.EnterBakalDungeon) && o.script.EnterBakalTimer.Secs > 0 && !now.Before(o.dungeonEnteredAt.Add(time.Duration(o.script.EnterBakalTimer.Secs)*time.Second)) {
			o.current = 0
			o.location = BakalCampLocation
			o.campReturnPending = true
		}
		if o.campReturnPending {
			o.guaranteeCampBudget()
			frames = append(frames, BakalFrame{"bakal_retreat_to_camp", NotiBakalPartyLocation, o.PartyFrame(BakalCampLocation, now)})
		}
	}
	if o.stage == BakalOpeningFinal && o.SettleDue(now) && o.settlementReady {
		frames = append(frames, o.settle()...)
	}
	return frames
}

// SettleDue reports whether the settlement window of the final stage has
// matured. The wiring layer calls it before Tick so the reward freeze and the
// full-bag splice land in the same tick the settlement burst goes out.
func (o *BakalOpening) SettleDue(now time.Time) bool {
	return o.stage == BakalOpeningFinal && !o.settled && !now.Before(o.settleAt)
}

// The source deadline is reported for manual finale verification; it is not
// replaced by a guessed duration from the four distinct movie-time fields.
func (o *BakalOpening) FinalDeadline() time.Time { return o.settleAt }

// activate publishes the open burst (timeline 012–017) and moves to active.
func (o *BakalOpening) activate() []BakalFrame {
	o.stage = BakalOpeningActive
	o.location = BakalCampLocation
	o.buffs = BakalOpenBuffCounts(o.script)
	o.anger = o.script.AngerInit
	o.angerLast = o.readyAt
	o.angerRate = o.script.AngerInitPerSecond
	o.openPublished = true
	o.remainingPublishedAt = o.readyAt
	if len(o.script.Events) > 0 {
		if err := o.startRuntime(o.readyAt); err != nil {
			o.eventErr = err
			return nil
		}
	}
	roster := BakalOpeningRoster(o.script)
	if o.runtime != nil {
		roster = o.eventRoster()
	}
	frames := []BakalFrame{
		{"bakal_active", NotiBakalRaidState, []byte(BakalRaidStateActive)},
		{"bakal_party", NotiBakalPartyLocation, o.PartyFrame(BakalCampLocation, o.readyAt)},
		{"bakal_monsters", NotiBakalMonsters, BakalMonstersFrame(roster)},
		{"bakal_buffs", NotiBakalBuffInventory, BakalBuffInventoryFrame(o.buffs, BakalBuffTailOpen)},
		{"bakal_open_dungeons", NotiBakalDungeonStates, BakalOpenDungeonsFrame(o.OpenDungeons())},
		{"bakal_remaining", NotiBakalRemaining, BakalRemainingFrame(o.Remaining())},
	}
	if o.runtime != nil {
		// N2286 is an incremental location table. N574 end/start does not
		// erase vacant locations from the previous run, so new random leaders
		// otherwise leave ghost portraits at their old locations.
		var reset []BakalMonster
		for _, loc := range o.rules.Locations {
			if loc.Dungeon != uint32(o.script.FinalClearDungeon) {
				reset = append(reset, BakalDefeatRow(uint32(loc.Index)))
			}
		}
		if len(reset) > 0 {
			frames = append([]BakalFrame{{"bakal_map_locations_reset", NotiBakalMonsters, BakalMonstersFrame(reset)}}, frames...)
		}
		return append(frames, o.eventFrames()...)
	}
	return append(frames, o.presenceFrames()...)
}

// advanceAnger accrues [INCREASE BAKAL ANGER ENTER] per full active second.
func (o *BakalOpening) advanceAnger(now time.Time) {
	if o.angerLast.IsZero() {
		o.angerLast = now
		return
	}
	secs := int(now.Sub(o.angerLast) / time.Second)
	if secs <= 0 {
		return
	}
	end := o.angerLast.Add(time.Duration(secs) * time.Second)
	window := o.readyAt.Add(time.Duration(o.script.AngerWindow.Secs) * time.Second)
	if !o.angerWindowApplied && o.script.AngerWindow.Secs > 0 && !end.Before(window) {
		if o.angerLast.Before(window) {
			o.anger += int(window.Sub(o.angerLast)/time.Second) * o.angerRate
			o.angerLast = window
		}
		o.angerRate = o.script.AngerWindowEndPerSecond
		o.angerWindowApplied = true
	}
	o.anger += int(end.Sub(o.angerLast)/time.Second) * o.angerRate
	o.angerLast = end
}

// EnterDungeon authorizes one of the 17 opened maps and answers with the
// portal-ready + room-location pair (timeline 023–024). Boss dungeons are
// resolved through the catalog type table. The caller supplies the raid-map
// location the move lands on (decoded from the client's move/position
// traffic upstream).
func (o *BakalOpening) EnterDungeon(dungeon, location uint32, now time.Time) ([]BakalFrame, error) {
	if o.Recovering(now) {
		return nil, fmt.Errorf("Bakal recovery until %s", o.recoveryUntil.Format(time.RFC3339))
	}
	if o.campReturnPending {
		return nil, fmt.Errorf("Bakal camp return still pending")
	}
	if o.stage != BakalOpeningActive && !(o.stage == BakalOpeningFinal && dungeon == uint32(o.script.FinalClearDungeon)) {
		return nil, fmt.Errorf("bakal enter dungeon from stage %d", o.stage)
	}
	if !o.HasDungeon(dungeon) {
		return nil, fmt.Errorf("bakal enter dungeon %d outside the opened list", dungeon)
	}
	if o.cleared[dungeon] {
		return nil, fmt.Errorf("bakal dungeon %d already cleared", dungeon)
	}
	if o.runtime != nil {
		return o.eventEnter(dungeon, location, now)
	}
	o.current = dungeon
	o.location = location
	o.dungeonEnteredAt = now
	for _, rule := range o.script.EnterAwakenings {
		if rule.Dungeon == dungeon && o.symbolValues[rule.AwakeSymbol] == rule.From && o.symbolValues[rule.HPSymbol] > 0 {
			o.symbolValues[rule.AwakeSymbol] = rule.To
		}
	}
	if dungeon == uint32(o.script.EnterBakalDungeon) {
		o.advanceAnger(now)
		o.angerRate = o.script.AngerEnterPerSecond
	}
	delete(o.loaded, dungeon)
	return []BakalFrame{
		{"bakal_portal_ready", NotiBakalPortalReady, BakalPortalReadyFrame()},
		{"bakal_room_party_location", NotiBakalPartyLocation, o.PartyFrame(location, now)},
	}, nil
}

// LoadingDone records the CMD2073 load completion of the current dungeon —
// the gate scripted warps check (the captured refusal "Bakal scripted warp
// outside loaded owned combat").
func (o *BakalOpening) LoadingDone(dungeon uint32) error {
	if o.current == 0 || dungeon != o.current {
		return ErrBakalWarpOutsideCombat
	}
	o.loaded[dungeon] = true
	return nil
}

// StageWarp applies a CMD2070 scripted warp inside the loaded owned combat
// and publishes the new room location (timeline 044–045; the 2070 ACK stays
// with the dispatch layer).
func (o *BakalOpening) StageWarp(location uint32) ([]BakalFrame, error) {
	if o.current == 0 || !o.loaded[o.current] {
		return nil, ErrBakalWarpOutsideCombat
	}
	o.location = location
	return []BakalFrame{
		{"bakal_room_party_location", NotiBakalPartyLocation, o.PartyFrame(location, time.Now())},
	}, nil
}

// DefeatMonster confirms one battle report (CMD2069 → bakalConfirmDefeats):
// the N2286 defeat row, then — for a boss dungeon — the N572 single-dungeon
// state plus the GrantHook's N2288 charge grant; for the [FINAL CLEAR
// DUNGEON] the final-selection burst instead (timeline 229–232) and the
// settlement window opens at +[SET TIMER] 28 56 180.
func (o *BakalOpening) DefeatMonster(dungeon, location uint32, now time.Time) ([]BakalFrame, error) {
	if o.stage != BakalOpeningActive {
		return nil, fmt.Errorf("bakal defeat report from stage %d", o.stage)
	}
	if o.current == 0 || dungeon != o.current || !o.loaded[dungeon] {
		return nil, fmt.Errorf("bakal defeat report outside the loaded dungeon %d", dungeon)
	}
	if o.defeatedLocations[location] {
		return nil, nil
	}
	if o.runtime != nil {
		return o.eventDefeat(dungeon, location, now)
	}
	o.defeatedLocations[location] = true
	frames := []BakalFrame{
		{"bakal_script_monsters", NotiBakalMonsters, BakalMonstersFrame([]BakalMonster{BakalDefeatRow(location)})},
	}
	frames = append(frames, o.presenceFrames(o.placements[location])...)
	if dungeon == uint32(o.script.SettlementDungeon) {
		o.cleared[dungeon] = true
		o.stage = BakalOpeningFinal
		o.settleAt = now.Add(time.Duration(o.script.SettlementTimer.Secs) * time.Second)
		return append(frames,
			BakalFrame{"bakal_final_selection", NotiBakalFinalSelection, BakalFinalSelectionFrame()},
			BakalFrame{"bakal_final_move", NotiBakalPortalReady, BakalPortalReadyFrame()},
			BakalFrame{"bakal_room_party_location", NotiBakalPartyLocation, BakalPartyFrame(uint32(o.script.SettlementTimer.Sub), nil)},
		), nil
	}
	if _, boss := o.bossTypes[dungeon]; boss && !o.cleared[dungeon] {
		o.cleared[dungeon] = true
		name := strings.ToUpper(o.bossTypes[dungeon])
		// Native clear behavior resets HP and awake. Keep this state so
		// later snapshots cannot reawaken a defeated dragon.
		for _, suffix := range []string{" HP]", " AWAKEN]"} {
			key := "[" + name + suffix
			if _, present := o.symbolValues[key]; present {
				o.symbolValues[key] = 0
			}
		}
		frames = append(frames, BakalFrame{
			"bakal_script_dungeon_states", NotiBakalDungeonStates, BakalDungeonStateFrame(dungeon),
		})
		frames = append(frames, o.grantBuffs(dungeon)...)
	}
	return frames, nil
}

// grantBuffs applies the GrantHook policy and publishes one N2288 charge
// update when it grants anything (timeline 101/155/184: boss clears bump
// weighted-random slots).
func (o *BakalOpening) grantBuffs(dungeon uint32) []BakalFrame {
	if o.GrantHook == nil {
		return nil
	}
	slots := o.GrantHook(dungeon)
	granted := false
	for _, slot := range slots {
		if slot >= 0 && slot < len(o.buffs) {
			o.buffs[slot]++
			granted = true
		}
	}
	if !granted {
		return nil
	}
	return []BakalFrame{{
		"bakal_script_buff_inventory", NotiBakalBuffInventory,
		BakalBuffInventoryFrame(o.buffs, BakalBuffTailScript),
	}}
}

// GrantBuffs publishes an explicit N2288 charge update (the dispatch-side
// grant path for policies outside the boss-clear hook).
func (o *BakalOpening) GrantBuffs(slots ...int) []BakalFrame {
	granted := false
	for _, slot := range slots {
		if slot >= 0 && slot < len(o.buffs) {
			o.buffs[slot]++
			granted = true
		}
	}
	if !granted {
		return nil
	}
	return []BakalFrame{{
		"bakal_script_buff_inventory", NotiBakalBuffInventory,
		BakalBuffInventoryFrame(o.buffs, BakalBuffTailScript),
	}}
}

// UseRaidBuff spends one charge of a buff slot (A-layer UseRaidBuff /
// protocol.DecodeBakalBuffUse115). No c2s instance survives in the B-layer
// captures, so the coupling is A-layer-derived: the N2288 re-publish shape
// is the captured script variant.
func (o *BakalOpening) UseRaidBuff(slot int) ([]BakalFrame, error) {
	if o.runtime != nil {
		return o.UseRaidBuffAt(slot, time.Now())
	}
	if o.stage != BakalOpeningActive && o.stage != BakalOpeningFinal {
		return nil, fmt.Errorf("bakal buff use from stage %d", o.stage)
	}
	if slot < 0 || slot >= len(o.buffs) {
		return nil, fmt.Errorf("bakal buff slot %d out of range", slot)
	}
	if o.buffs[slot] == 0 {
		return nil, fmt.Errorf("bakal buff slot %d has no charges", slot)
	}
	o.buffs[slot]--
	return []BakalFrame{{
		"bakal_script_buff_inventory", NotiBakalBuffInventory,
		BakalBuffInventoryFrame(o.buffs, BakalBuffTailScript),
	}}, nil
}

// ReportHealth publishes the boss HP progress pair as two N2286 frames
// (timeline 224–225: a row-state-1 20B frame followed by a row-state-2 20B
// frame carrying the same current HP).
func (o *BakalOpening) ReportHealth(slot, location, hp uint32) []BakalFrame {
	if o.stage != BakalOpeningActive && o.stage != BakalOpeningFinal {
		return nil
	}
	rows := BakalBossProgressRows(slot, location, hp)
	return []BakalFrame{
		{"bakal_script_monsters", NotiBakalMonsters, BakalMonstersFrame(rows[:1])},
		{"bakal_script_actor_health", NotiBakalMonsters, BakalMonstersFrame(rows[1:])},
	}
}

// CampReturn applies the CMD2074 native retreat (B' failure runs: legal
// inside an owned loaded dungeon ×3, "invalid native Bakal camp return"
// otherwise) and re-lands the member on the camp location.
func (o *BakalOpening) CampReturn() ([]BakalFrame, error) {
	return o.CampReturnAt(time.Now(), BakalReturnVoluntary)
}

func (o *BakalOpening) CampReturnAt(now time.Time, cause BakalReturnCause) ([]BakalFrame, error) {
	if !o.CanRetreat() {
		return nil, ErrBakalInvalidCampReturn
	}
	if o.runtime != nil {
		c := o.eventClone()
		old := c.current
		c.current, c.location = 0, BakalCampLocation
		c.guaranteeCampBudget()
		if err := c.applyRecovery(now, cause); err != nil {
			return nil, err
		}
		if err := c.dispatchEvent(bakalSignal{op: "[ON GIVEUP DUNGEON]", id: old}, now); err != nil {
			return nil, err
		}
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_retreat_to_camp", 2285, c.PartyFrame(BakalCampLocation, now)})
		*o = *c
		return o.eventFrames(), nil
	}
	if err := o.applyRecovery(now, cause); err != nil {
		return nil, err
	}
	o.current = 0
	o.location = BakalCampLocation
	return []BakalFrame{
		{"bakal_retreat_to_camp", NotiBakalPartyLocation, BakalPartyFrame(BakalCampLocation, nil)},
	}, nil
}

func (o *BakalOpening) CanRetreat() bool {
	return o.stage == BakalOpeningActive && o.current != 0 && o.loaded[o.current]
}

func (o *BakalOpening) NeedsCampReturn() bool  { return o.campReturnPending }
func (o *BakalOpening) AcknowledgeCampReturn() { o.campReturnPending = false }

// FinalConfirm validates the CMD1134 post-settlement confirm: the final map
// must be the captured BakalFinalMap.
func (o *BakalOpening) FinalConfirm(finalMap uint32) error {
	if o.stage != BakalOpeningFinal && o.stage != BakalOpeningEnded {
		return fmt.Errorf("bakal final confirm from stage %d", o.stage)
	}
	if finalMap != BakalFinalMap {
		return fmt.Errorf("bakal final confirm map %d is not the final dungeon map", finalMap)
	}
	return nil
}

// Remaining is the phase countdown the N584 channel carries
// ([PHASE TIME OVER]); Tick republishes the decreasing value every10s.
func (o *BakalOpening) Remaining() uint32 {
	return uint32(o.rules.PhaseTimeOverSecs)
}

// SetRewardInventory splices the N13 reward inventory body into the settlement
// burst (between the return-to-camp and the clear result, timeline 238).
func (o *BakalOpening) SetRewardInventory(body []byte) {
	o.rewardInventory = body
	o.settlementReady = body != nil
}

func (o *BakalOpening) HasDungeon(id uint32) bool {
	for _, d := range o.rules.Dungeons {
		if d.Index == id {
			return true
		}
	}
	return false
}
func (o *BakalOpening) OpenDungeons() []uint32 {
	ids := make([]uint32, 0, len(o.rules.Dungeons))
	for _, d := range o.rules.Dungeons {
		ids = append(ids, d.Index)
	}
	return ids
}
func (o *BakalOpening) IsCleared(id uint32) bool { return o.cleared[id] }

func (o *BakalOpening) IsLocationDefeated(location uint32) bool { return o.defeatedLocations[location] }
func (o *BakalOpening) SymbolFrame(name string, value int32) []BakalFrame {
	if id, ok := o.rules.Symbols[name]; ok {
		return []BakalFrame{{"bakal_source_symbol", NotiBakalSourceSymbol, BakalSourceSymbolFrame(id, value)}}
	}
	return nil
}

// Source CLEAR PHASE can also be triggered by defeating the final actor.
func (o *BakalOpening) ClearFinalDungeon(now time.Time) error {
	if o.stage != BakalOpeningFinal || o.current != uint32(o.script.FinalClearDungeon) || !o.loaded[o.current] {
		return fmt.Errorf("Bakal final clear outside owned loaded dungeon")
	}
	o.cleared[o.current] = true
	o.settleAt = now
	return nil
}

func (o *BakalOpening) SecondPhase() bool { return o.secondPhase }
func (o *BakalOpening) MarkFirstPhaseDefeated() {
	o.firstPhaseDefeated = true
	o.defeatedLocations[o.location] = true
}
func (o *BakalOpening) BeginSecondPhase() error {
	if o.stage != BakalOpeningActive || !o.firstPhaseDefeated || o.secondPhase {
		return fmt.Errorf("phase cinematic has no owned first-stage actor")
	}
	// HP unlock grade is reduced by clearing the three source dragon types.
	for _, d := range o.rules.Dungeons {
		if d.Type != "" && d.Type != "bakal" && !o.cleared[d.Index] {
			return fmt.Errorf("phase cinematic before source HP unlock")
		}
	}
	o.secondPhase = true
	if o.runtime != nil {
		o.symbolValues["[BAKAL PHASE]"] = 1
	}
	// The source cinematic authorizes a new template at the same location.
	delete(o.defeatedLocations, o.location)
	return nil
}

// Source symbol index and initial SET rows drive the client script getters.
func (o *BakalOpening) SymbolsSnapshot() []BakalFrame {
	values := map[uint32]int32{}
	for name, value := range o.symbolValues {
		if id, ok := o.rules.Symbols[name]; ok {
			values[id] = value
		}
	}
	for name, value := range o.presenceValues() {
		values[o.rules.Symbols[name]] = value
	}
	grade := o.script.HPUnlockGrade
	for _, d := range o.rules.Dungeons {
		if d.Type != "" && d.Type != "bakal" && o.cleared[d.Index] {
			grade--
		}
		if d.Type != "" && o.cleared[d.Index] {
			if id, ok := o.rules.Symbols["["+strings.ToUpper(d.Type)+" HP]"]; ok {
				values[id] = 0
			}
		}
	}
	if id, ok := o.rules.Symbols["[BAKAL ANGER]"]; ok {
		values[id] = int32(o.anger)
	}
	if id, ok := o.rules.Symbols["[BAKAL HP UNLOCK GRADE]"]; ok {
		values[id] = int32(grade)
	}
	if o.secondPhase {
		if id, ok := o.rules.Symbols["[BAKAL PHASE]"]; ok {
			if values[id] < 1 {
				values[id] = 1
			}
		}
	}
	ids := make([]uint32, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var frames []BakalFrame
	for _, id := range ids {
		frames = append(frames, BakalFrame{"bakal_source_symbol", NotiBakalSourceSymbol, BakalSourceSymbolFrame(id, values[id])})
	}
	return frames
}

// BakalSettlement is the settlement identity of a finished run (A-layer
// SettlementIdentity) — the event record the settlement burst carries.
type BakalSettlement struct {
	Name         string    `json:"name"`
	Hard         bool      `json:"hard"`
	ClearedCount int       `json:"cleared_count"`
	FinalDungeon uint32    `json:"final_dungeon"`
	CampLocation uint32    `json:"camp_location"`
	SettledAt    time.Time `json:"settled_at"`
}

// SettlementIdentity reports the settlement record once the final dungeon
// has cleared (stage final or later).
func (o *BakalOpening) SettlementIdentity(at time.Time) (BakalSettlement, bool) {
	if o.stage != BakalOpeningFinal && o.stage != BakalOpeningEnded {
		return BakalSettlement{}, false
	}
	cleared := 0
	for _, ok := range o.cleared {
		if ok {
			cleared++
		}
	}
	return BakalSettlement{
		Name:         o.name,
		Hard:         o.hard,
		ClearedCount: cleared,
		FinalDungeon: uint32(o.script.FinalClearDungeon),
		CampLocation: BakalCampLocation,
		SettledAt:    at,
	}, true
}

// settle publishes the settlement burst (timeline 237–240): return-to-camp,
// the reward inventory when provided, the clear result and phase ended.
func (o *BakalOpening) settle() []BakalFrame {
	o.settled = true
	o.stage = BakalOpeningEnded
	o.current = 0
	o.location = BakalCampLocation
	frames := []BakalFrame{
		{"bakal_script_return_to_camp", NotiBakalPartyLocation, BakalPartyFrame(BakalCampLocation, nil)},
	}
	if o.rewardInventory != nil {
		frames = append(frames, BakalFrame{"bakal_source_reward_inventory", 13, o.rewardInventory})
	}
	return append(frames,
		BakalFrame{"bakal_clear_result", NotiBakalClearResult, BakalClearResultFrame()},
		BakalFrame{"bakal_script_phase_ended", NotiBakalRaidState, []byte(BakalRaidStateEnded)},
	)
}
