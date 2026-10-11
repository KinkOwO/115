package catalog

import (
	"fmt"
	"strings"

	"dfolan/internal/catalog/pvf"
)

// BakalRaidSource pins the Bakal raid rules script inside the inner PVF. Every
// raid-wide rule the wire layer enforces — waiting room, member limits, the
// anger engine, settlement timers, bidding weights — is declared by this one
// file, so the reconstruction never carries a second handwritten copy of the
// operator's tuning values.
const BakalRaidSource = "contents/2022/bakalraid/etc/bakal.etc"

// BakalRaidRules is the server-facing projection of bakal.etc. All field values
// are copied verbatim from the script; nothing here is a server-side decision.
type BakalRaidRules struct {
	Source string `json:"source"`
	SHA256 string `json:"sha256"`

	PhaseMax               int       `json:"phase_max"`
	RaidMemberMax          int       `json:"raid_member_max"`
	RaidStartMinimumMember int       `json:"raid_start_minimum_member"`
	WaitingRoomTown        int       `json:"waiting_room_town"`
	WaitingRoomArea        int       `json:"waiting_room_area"`
	WaitingRoom            *TownArea `json:"waiting_room_binding"`
	WaitingRoomNPC         []int     `json:"waiting_room_npc"`
	RaidPartyMax           int       `json:"raid_party_max"`
	DisconnMemberWaitSecs  int       `json:"disconn_member_wait_secs"`
	StartDelaySecs         int       `json:"start_delay_secs"`
	PhaseTimeOverSecs      int       `json:"phase_time_over_secs"`
	PhaseBreakSecs         int       `json:"phase_break_secs"`
	MovieEndTimes          []int     `json:"movie_end_times"`
	RetryLimit             int       `json:"retry_limit"`
	WeeklyClearLimit       int       `json:"weekly_clear_limit"`
	WeeklyRewardLimit      int       `json:"weekly_reward_limit"`
	RevivalTimes           []int     `json:"revival_times"`

	PartyCoinLimit       int `json:"party_coin_limit"`
	GuaranteePartyCoin   int `json:"guarantee_party_coin"`
	GuaranteePartyPotion int `json:"guarantee_party_potion"`

	RankDeadCounts           []BakalRankDeadCount `json:"rank_dead_counts"`
	MinimalDungeonClearCount int                  `json:"minimal_dungeon_clear_count"`

	Bidding     BakalBiddingRules `json:"bidding"`
	Bosses      []BakalBossReport `json:"bosses"`
	RewardRoles []string          `json:"reward_roles"`

	Dungeons               []BakalDungeonInfo             `json:"dungeons"`
	Locations              []BakalLocationInfo            `json:"locations"`
	Slots                  map[uint32]BakalRaidSlot       `json:"slots"`
	Monsters               map[string]BakalRaidMonster    `json:"monsters"`
	BuffDefinitions        map[string]BakalBuffDefinition `json:"buff_definitions"`
	RaidBuffCooldownMillis int                            `json:"raid_buff_cooldown_millis"`
	Stage                  BakalStageRule
	Symbols                map[string]uint32  `json:"symbols"`
	DungeonCatalog         *DungeonCatalog    `json:"-"`
	Rewards                []BakalRewardEntry `json:"rewards"`
	RaidID                 int                `json:"raid_id"`

	NormalPhase *BakalPhaseRules `json:"normal_phase"`
	HardPhase   *BakalPhaseRules `json:"hard_phase"`

	HardRaidPartyMax int `json:"hard_raid_party_max"`
	HardLimitFame    int `json:"hard_limit_fame"`

	ChannelSlotRewardItems []uint32 `json:"channel_slot_reward_items"`
	MonsterPieceRewards    []uint32 `json:"monster_piece_rewards"`
}

// BakalRewardEntry is one row of a frozen reward plan. The handover binary
// keeps this type in catalog (40 bytes, template at offset 0x18 — the claim
// fallback converts each row into one unit of the template) next to the
// per-category weighted reward projection (catalog/bakal_rewards.go, source
// lost, see next151 §6). Category survives here as the reconstruction's audit
// tag recording which bakal.etc row the entry was fed from.
type BakalRewardEntry struct {
	Category   string `json:"category"`
	Template   uint32 `json:"template"`
	Amount     uint32 `json:"amount"`
	Weight     uint32 `json:"weight,omitempty"`
	Difficulty int    `json:"difficulty,omitempty"`
}

// BakalRankDeadCount is one [DEAD COUNT] row of [CONDITION FOR RANK].
type BakalRankDeadCount struct {
	MinDead int `json:"min_dead"`
	MaxDead int `json:"max_dead"`
	Rank    int `json:"rank"`
}

// BakalBiddingRules collects the auction tuning declared by bakal.etc.
type BakalBiddingRules struct {
	StartDelaySecs   int                        `json:"start_delay_secs"`
	BreakSecs        int                        `json:"break_secs"`
	TimeSecs         int                        `json:"time_secs"`
	MailPrevDay      int                        `json:"mail_prev_day"`
	WaitSecs         int                        `json:"wait_secs"`
	WarningSecs      int                        `json:"warning_secs"`
	DecreaseBidCount int                        `json:"decrease_bid_count"`
	DecreasePerBid   float64                    `json:"decrease_per_bid"`
	LowestTimeSecs   int                        `json:"lowest_time_secs"`
	Hards            []BakalBiddingHard         `json:"hards"`
	Items            []BakalRewardEntry         `json:"items,omitempty"`
	WeeklyCounts     []BakalBiddingHard         `json:"weekly_counts,omitempty"`
	WeeklyRates      []BakalBiddingRate         `json:"weekly_rates,omitempty"`
	WeeklyGroups     map[int][]BakalBiddingItem `json:"weekly_groups,omitempty"`
}

// The five trailing source values stay explicit; they are not silently
// interpreted as equipment options without the native auction item contract.
type BakalBiddingItem struct {
	Template uint32
	Weight   int
	Values   [5]int
}

// BakalBiddingHard is one [HARD] block of [BIDDING REWARD COUNT].
type BakalBiddingHard struct {
	Hard  int                `json:"hard"`
	Rates []BakalBiddingRate `json:"rates"`
}

// BakalBiddingRate is one (grade, weight) pair of a [RATE LIST]; the weights of
// one list sum to 100000 (the client's fixed denominator).
type BakalBiddingRate struct {
	Grade  int `json:"grade"`
	Weight int `json:"weight"`
}

// BakalBossReport is one boss block of [RAID COMBAT REPORT INFO]: the wire layer
// uses these monster indexes and damage symbols to render battle reports.
type BakalBossReport struct {
	Color         string `json:"color"`
	Monster       uint32 `json:"monster"`
	Face          string `json:"face"`
	DamageSymbols []int  `json:"damage_symbols"`
}

// BakalDungeonInfo is one [DUNGEON INFO] block of the raid phase.
type BakalDungeonInfo struct {
	Index           uint32 `json:"index"`
	Type            string `json:"type"` // bakal / skasa / sparazzi / hisma; empty for field maps
	HasHPInfo       bool   `json:"has_hp_info"`
	InitialState    string `json:"initial_state"`
	PartyMax        int    `json:"party_max"`
	MemberRetrySecs *int   `json:"member_retry_secs"`
	RetryTimeGroup  *int   `json:"retry_time_group"`
}

// BakalLocationInfo is one [LOCATION INFO] block of [LOCATIONS].
type BakalLocationInfo struct {
	Index     int      `json:"index"`
	Dungeon   uint32   `json:"dungeon"`
	X         int      `json:"x"`
	Y         int      `json:"y"`
	SpecificX *int     `json:"specific_x,omitempty"`
	SpecificY *int     `json:"specific_y,omitempty"`
	Type      string   `json:"type"`
	Movable   []int    `json:"movable"`
	Creatable []string `json:"creatable"`
}

// BakalTimerRule is one [SET TIMER] declaration.
type BakalTimerRule struct {
	ID   int `json:"id"`
	Sub  int `json:"sub"`
	Secs int `json:"secs"`
}

// BakalMonsterSpawn is one CREATE/RESERVE monster row of the phase init script.
type BakalMonsterSpawn struct {
	Location int    `json:"location"`
	Name     string `json:"name"`
}

// BakalBuffGrant is one [ADD BAKAL RAID BUFF] row.
type BakalBuffGrant struct {
	Slot  int `json:"slot"`
	Count int `json:"count"`
}

// BakalHPFloor clamps the boss HP when the unlock grade drops (SET BAKAL HP).
type BakalHPFloor struct {
	Grade int `json:"grade"`
	Floor int `json:"floor"`
}

// BakalPeriodicAnger is one INCREASE BAKAL ANGER row. EverySecs is zero for
// one-shot increases; periodic rows repeat on their companion [SET TIMER].
type BakalPeriodicAnger struct {
	Location  int `json:"location"`
	Amount    int `json:"amount"`
	EverySecs int `json:"every_secs"`
}

// BakalTriggerRecord keeps the raw trigger/behavior pair for audits.
type BakalTriggerRecord struct {
	Trigger  []pvf.Token `json:"trigger"`
	Behavior []pvf.Token `json:"behavior"`
}

// BakalPhaseRules is the typed view of one raid phase script: [PHASE] holds the
// normal rules and [RAID PHASE OF HARDMODE] the hard-mode variant.
type BakalPhaseRules struct {
	Events          []BakalScriptEvent    `json:"events"`
	EnterAwakenings []BakalEnterAwakening `json:"enter_awakenings"`
	MonsterMaxHP    int                   `json:"monster_max_hp"`
	BossHP          map[string]int        `json:"boss_hp"`
	HPUnlockGrade   int                   `json:"hp_unlock_grade"`
	HPFloors        []BakalHPFloor        `json:"hp_floors"`

	InitMonsters    []BakalMonsterSpawn `json:"init_monsters"`
	ReserveMonsters []BakalMonsterSpawn `json:"reserve_monsters"` // legacy Location field is the reserve delay in seconds
	RaidBuffs       []BakalBuffGrant    `json:"raid_buffs"`
	InitTimers      []BakalTimerRule    `json:"init_timers"`

	AngerInit               int                  `json:"anger_init"`
	AngerInitPerSecond      int                  `json:"anger_init_per_second"`
	AngerEnterPerSecond     int                  `json:"anger_enter_per_second"`
	AngerWindowEndPerSecond int                  `json:"anger_window_end_per_second"`
	AngerWindow             BakalTimerRule       `json:"anger_window"`
	AngerFailThreshold      int                  `json:"anger_fail_threshold"`
	AwakeAnger              []BakalPeriodicAnger `json:"awake_anger"`
	EscapeAnger             []BakalPeriodicAnger `json:"escape_anger"`
	AwakeDragonDelays       []int                `json:"awake_dragon_delays"`

	EnterBakalDungeon     int            `json:"enter_bakal_dungeon"`
	EnterBakalTimer       BakalTimerRule `json:"enter_bakal_timer"`
	EnterBakalKickDungeon int            `json:"enter_bakal_kick_dungeon"`

	SettlementDungeon     int            `json:"settlement_dungeon"`
	SettlementTimer       BakalTimerRule `json:"settlement_timer"`
	SettlementKickDungeon int            `json:"settlement_kick_dungeon"`

	FinalClearDungeon     int `json:"final_clear_dungeon"`
	FinalClearKickDungeon int `json:"final_clear_kick_dungeon"`

	Triggers       []BakalTriggerRecord `json:"triggers"`
	InitialSymbols map[string]int32     `json:"initial_symbols"`
}

// A native ON ENTER/IF awake0/IF hp>0/SET awake1 rule, not an entry lock.
type BakalEnterAwakening struct {
	Dungeon               uint32
	AwakeSymbol, HPSymbol string
	From, To              int32
}

func ImportBakalRaid(a *pvf.Archive) (*BakalRaidRules, error) {
	if a == nil {
		return nil, fmt.Errorf("bakal raid import requires a PVF archive")
	}
	script, err := ResolveScript(a, BakalRaidSource)
	if err != nil {
		return nil, err
	}
	cells := script.Cells
	r := &BakalRaidRules{Source: script.Path, SHA256: script.SHA256}
	var waiting, npcs, movies, revival []int
	if r.PhaseMax, err = bakalRowInt(cells, "[phase max]"); err != nil {
		return nil, err
	}
	if r.RaidMemberMax, err = bakalRowInt(cells, "[raid member max]"); err != nil {
		return nil, err
	}
	if r.RaidStartMinimumMember, err = bakalRowInt(cells, "[raid start minimum member]"); err != nil {
		return nil, err
	}
	if waiting, err = bakalRowInts(cells, "[waiting room]"); err != nil {
		return nil, err
	}
	if len(waiting) != 2 {
		return nil, fmt.Errorf("bakal.etc: [waiting room] carries %d values, want town and area", len(waiting))
	}
	r.WaitingRoomTown, r.WaitingRoomArea = waiting[0], waiting[1]
	if npcs, err = bakalRowInts(cells, "[waiting room npc]"); err != nil {
		return nil, err
	}
	if len(npcs) == 0 {
		return nil, fmt.Errorf("bakal.etc: [waiting room npc] is empty")
	}
	r.WaitingRoomNPC = npcs
	if r.RaidPartyMax, err = bakalRowInt(cells, "[raid party max]"); err != nil {
		return nil, err
	}
	if r.DisconnMemberWaitSecs, err = bakalRowInt(cells, "[disconn member wait time]"); err != nil {
		return nil, err
	}
	if r.StartDelaySecs, err = bakalRowInt(cells, "[start delay time]"); err != nil {
		return nil, err
	}
	if r.PhaseTimeOverSecs, err = bakalRowInt(cells, "[phase time over]"); err != nil {
		return nil, err
	}
	if r.PhaseBreakSecs, err = bakalRowInt(cells, "[phase break time]"); err != nil {
		return nil, err
	}
	if movies, err = bakalRowInts(cells, "[movie end time]"); err != nil {
		return nil, err
	}
	r.MovieEndTimes = movies
	if r.RetryLimit, err = bakalRowInt(cells, "[retry]"); err != nil {
		return nil, err
	}
	if r.WeeklyClearLimit, err = bakalRowInt(cells, "[weekly clear count]"); err != nil {
		return nil, err
	}
	if r.WeeklyRewardLimit, err = bakalRowInt(cells, "[weekly reward count]"); err != nil {
		return nil, err
	}
	if revival, err = bakalRowInts(cells, "[revival time]"); err != nil {
		return nil, err
	}
	if len(revival) == 0 {
		return nil, fmt.Errorf("bakal.etc: [revival time] is empty")
	}
	r.RevivalTimes = revival
	if r.PartyCoinLimit, err = bakalRowInt(cells, "[party coin limit]"); err != nil {
		return nil, err
	}
	if r.GuaranteePartyCoin, err = bakalRowInt(cells, "[guarantee party coin]"); err != nil {
		return nil, err
	}
	if r.GuaranteePartyPotion, err = bakalRowInt(cells, "[guarantee party potion]"); err != nil {
		return nil, err
	}
	if r.HardRaidPartyMax, err = bakalRowInt(cells, "[hard raid party max]"); err != nil {
		return nil, err
	}
	if r.HardLimitFame, err = bakalRowInt(cells, "[hard limit fame]"); err != nil {
		return nil, err
	}

	report, err := bakalBlock(cells, "[raid combat report info]", "[/raid combat report info]")
	if err != nil {
		return nil, err
	}
	if r.Bosses, r.RewardRoles, err = parseBakalCombatReport(report); err != nil {
		return nil, err
	}

	bidding, err := bakalBlock(cells, "[bidding reward count]", "[/bidding reward count]")
	if err != nil {
		return nil, err
	}
	if r.Bidding, err = parseBakalBidding(cells, bidding); err != nil {
		return nil, err
	}

	normal, err := bakalBlock(cells, "[phase]", "[/phase]")
	if err != nil {
		return nil, err
	}
	if r.MinimalDungeonClearCount, err = bakalRowInt(normal, "[minimal dungeon clear count]"); err != nil {
		return nil, err
	}
	cond, err := bakalBlock(normal, "[condition for rank]", "[/condition for rank]")
	if err != nil {
		return nil, err
	}
	dead := bakalAllRowsInts(cond, "[dead count]")
	if len(dead)%3 != 0 {
		return nil, fmt.Errorf("bakal.etc: [dead count] rows carry %d values, want triples", len(dead))
	}
	for i := 0; i < len(dead); i += 3 {
		r.RankDeadCounts = append(r.RankDeadCounts, BakalRankDeadCount{MinDead: dead[i], MaxDead: dead[i+1], Rank: dead[i+2]})
	}

	dungeonBlocks, err := bakalBlocks(normal, "[dungeon info]", "[/dungeon info]")
	if err != nil {
		return nil, err
	}
	for _, block := range dungeonBlocks {
		dungeon, err := parseBakalDungeonInfo(block)
		if err != nil {
			return nil, err
		}
		r.Dungeons = append(r.Dungeons, dungeon)
	}

	if r.NormalPhase, err = parseBakalPhaseRules(normal); err != nil {
		return nil, err
	}

	hard, err := bakalBlock(cells, "[raid phase of hardmode]", "[/raid phase of hardmode]")
	if err != nil {
		return nil, err
	}
	if r.HardPhase, err = parseBakalPhaseRules(hard); err != nil {
		return nil, err
	}

	locationBlocks, err := bakalBlocks(cells, "[location info]", "[/location info]")
	if err != nil {
		return nil, err
	}
	for _, block := range locationBlocks {
		location, err := parseBakalLocationInfo(block)
		if err != nil {
			return nil, err
		}
		r.Locations = append(r.Locations, location)
	}

	slot, err := bakalRowInts(cells, "[channel slot expected reward item]")
	if err != nil {
		return nil, err
	}
	for _, v := range slot {
		r.ChannelSlotRewardItems = append(r.ChannelSlotRewardItems, uint32(v))
	}
	piece, err := bakalRowInts(cells, "[monster piece reward]")
	if err != nil {
		return nil, err
	}
	for _, v := range piece {
		r.MonsterPieceRewards = append(r.MonsterPieceRewards, uint32(v))
	}

	if err := verifyBakalRaidShape(r); err != nil {
		return nil, err
	}
	if err := loadBakalRaidTables(a, r); err != nil {
		return nil, err
	}
	if err := bindBakalWaitingRoom(a, r); err != nil {
		return nil, err
	}
	return r, nil
}

// parseBakalCombatReport reads the four boss blocks; the color order matches
// the report frame layout the wire layer renders.
func parseBakalCombatReport(block []pvf.Token) ([]BakalBossReport, []string, error) {
	var bosses []BakalBossReport
	for _, color := range []string{"red", "green", "yellow", "blue"} {
		monster, err := bakalRowInt(block, "["+color+" boss index]")
		if err != nil {
			return nil, nil, err
		}
		face := bakalRowStrings(block, "["+color+" boss face animation]")
		if len(face) != 1 {
			return nil, nil, fmt.Errorf("bakal.etc: [%s boss face animation] carries %d names, want one", color, len(face))
		}
		symbols, err := bakalRowInts(block, "["+color+" boss party damage symbol]")
		if err != nil {
			return nil, nil, err
		}
		if len(symbols) == 0 {
			return nil, nil, fmt.Errorf("bakal.etc: [%s boss party damage symbol] is empty", color)
		}
		bosses = append(bosses, BakalBossReport{Color: color, Monster: uint32(monster), Face: face[0], DamageSymbols: symbols})
	}
	roles := bakalRowStrings(block, "[reward list]")
	if len(roles) == 0 {
		return nil, nil, fmt.Errorf("bakal.etc: [reward list] is empty")
	}
	return bosses, roles, nil
}

// parseBakalBidding reads the auction scalars plus the per-hard [RATE LIST]
// weighted grades. The source keeps a historical double-i typo in
// [BIDDIING MAIL PREV DAY]; the label is preserved verbatim.
func parseBakalBidding(cells, block []pvf.Token) (BakalBiddingRules, error) {
	b := BakalBiddingRules{}
	var err error
	if b.StartDelaySecs, err = bakalRowInt(cells, "[bidding start delay time]"); err != nil {
		return b, err
	}
	if b.BreakSecs, err = bakalRowInt(cells, "[bidding break time]"); err != nil {
		return b, err
	}
	if b.TimeSecs, err = bakalRowInt(cells, "[bidding time]"); err != nil {
		return b, err
	}
	if b.MailPrevDay, err = bakalRowInt(cells, "[biddiing mail prev day]"); err != nil {
		return b, err
	}
	if b.WaitSecs, err = bakalRowInt(cells, "[bidding wait time]"); err != nil {
		return b, err
	}
	if b.WarningSecs, err = bakalRowInt(cells, "[bidding warning time]"); err != nil {
		return b, err
	}
	if b.DecreaseBidCount, err = bakalRowInt(cells, "[decrease bidding time bid count]"); err != nil {
		return b, err
	}
	if b.DecreasePerBid, err = bakalRowFloat(cells, "[decrease bidding time per bid]"); err != nil {
		return b, err
	}
	if b.LowestTimeSecs, err = bakalRowInt(cells, "[lowest bidding time]"); err != nil {
		return b, err
	}
	hards, err := bakalBlocks(block, "[hard]", "[/hard]")
	if err != nil {
		return b, err
	}
	if len(hards) == 0 {
		return b, fmt.Errorf("bakal.etc: [bidding reward count] has no [hard] blocks")
	}
	for _, hardBlock := range hards {
		rows := bakalRows(hardBlock)
		if len(rows) == 0 || len(bakalInts(rows[0].Args)) != 1 {
			return b, fmt.Errorf("bakal.etc: [hard] block does not open with its identifier")
		}
		hard := BakalBiddingHard{Hard: bakalInts(rows[0].Args)[0]}
		rateBlock, err := bakalBlock(hardBlock, "[rate list]", "[/rate list]")
		if err != nil {
			return b, err
		}
		values := bakalInts(rateBlock)
		if len(values)%2 != 0 {
			return b, fmt.Errorf("bakal.etc: [rate list] carries %d values, want grade/weight pairs", len(values))
		}
		for i := 0; i < len(values); i += 2 {
			hard.Rates = append(hard.Rates, BakalBiddingRate{Grade: values[i], Weight: values[i+1]})
		}
		if len(hard.Rates) == 0 {
			return b, fmt.Errorf("bakal.etc: [rate list] of hard %d is empty", hard.Hard)
		}
		b.Hards = append(b.Hards, hard)
	}
	return b, nil
}

func parseBakalDungeonInfo(block []pvf.Token) (BakalDungeonInfo, error) {
	var d BakalDungeonInfo
	index, err := bakalRowInt(block, "[index]")
	if err != nil {
		return d, err
	}
	d.Index = uint32(index)
	if row, ok := bakalRowByLabel(block, "[type]"); ok {
		names := bakalStrings(row.Args)
		if len(names) != 1 {
			return d, fmt.Errorf("bakal.etc: dungeon %d [type] carries %d names", index, len(names))
		}
		d.Type = strings.ToLower(names[0])
	}
	d.HasHPInfo = bakalHasLabel(block, "[hp info]")
	if row, ok := bakalRowByLabel(block, "[init state]"); ok {
		names := bakalStrings(row.Args)
		if len(names) != 1 {
			return d, fmt.Errorf("bakal.etc: dungeon %d [init state] carries %d names", index, len(names))
		}
		d.InitialState = strings.ToLower(names[0])
	}
	if d.PartyMax, err = bakalRowInt(block, "[party max]"); err != nil {
		return d, err
	}
	if row, ok := bakalRowByLabel(block, "[member retry time]"); ok {
		values := bakalInts(row.Args)
		if len(values) != 1 {
			return d, fmt.Errorf("bakal.etc: dungeon %d [member retry time] carries %d values", index, len(values))
		}
		d.MemberRetrySecs = &values[0]
	}
	if row, ok := bakalRowByLabel(block, "[retry time group]"); ok {
		values := bakalInts(row.Args)
		if len(values) != 1 {
			return d, fmt.Errorf("bakal.etc: dungeon %d [retry time group] carries %d values", index, len(values))
		}
		d.RetryTimeGroup = &values[0]
	}
	return d, nil
}

func parseBakalLocationInfo(block []pvf.Token) (BakalLocationInfo, error) {
	var l BakalLocationInfo
	index, err := bakalRowInt(block, "[location index]")
	if err != nil {
		return l, err
	}
	l.Index = index
	dungeon, err := bakalRowInt(block, "[dungeon index]")
	if err != nil {
		return l, err
	}
	l.Dungeon = uint32(dungeon)
	xy, err := bakalRowInts(block, "[location xy]")
	if err != nil {
		return l, err
	}
	if len(xy) != 2 {
		return l, fmt.Errorf("bakal.etc: location %d [location xy] carries %d values", index, len(xy))
	}
	l.X, l.Y = xy[0], xy[1]
	if row, ok := bakalRowByLabel(block, "[location specific xy]"); ok {
		values := bakalInts(row.Args)
		if len(values) != 2 {
			return l, fmt.Errorf("bakal.etc: location %d [location specific xy] carries %d values", index, len(values))
		}
		l.SpecificX, l.SpecificY = &values[0], &values[1]
	}
	if row, ok := bakalRowByLabel(block, "[location type]"); ok {
		names := bakalStrings(row.Args)
		if len(names) != 1 {
			return l, fmt.Errorf("bakal.etc: location %d [location type] carries %d names", index, len(names))
		}
		l.Type = names[0]
	}
	if row, ok := bakalRowByLabel(block, "[movable location]"); ok {
		l.Movable = bakalInts(row.Args)
	}
	if row, ok := bakalRowByLabel(block, "[creatable monster]"); ok {
		l.Creatable = bakalStrings(row.Args)
	}
	return l, nil
}

// parseBakalPhaseRules pairs every [TRIGGER] with its [BEHAVIOR] and routes the
// pairs the wire layer implements into typed fields; every pair is retained in
// Triggers for audits regardless of routing.
func parseBakalPhaseRules(cells []pvf.Token) (*BakalPhaseRules, error) {
	p := &BakalPhaseRules{BossHP: map[string]int{}}
	triggers, err := bakalBlocks(cells, "[trigger]", "[/trigger]")
	if err != nil {
		return nil, err
	}
	behaviors, err := bakalBlocks(cells, "[behavior]", "[/behavior]")
	if err != nil {
		return nil, err
	}
	if len(triggers) != len(behaviors) {
		return nil, fmt.Errorf("bakal.etc: %d triggers face %d behaviors", len(triggers), len(behaviors))
	}
	var timerKicks []bakalTimerKick
	for i := range triggers {
		p.Triggers = append(p.Triggers, BakalTriggerRecord{Trigger: triggers[i], Behavior: behaviors[i]})
		trigger, err := bakalInstructions(triggers[i])
		if err != nil {
			return nil, err
		}
		behavior, err := bakalInstructions(behaviors[i])
		if err != nil {
			return nil, err
		}
		p.Events = append(p.Events, BakalScriptEvent{Trigger: trigger, Behavior: behavior})
		if err := classifyBakalPair(p, triggers[i], behaviors[i], &timerKicks); err != nil {
			return nil, err
		}
	}
	// The anger window length lives in the init script; its identity comes from
	// the CHECK TIMER END pair that accelerates the anger clock.
	for _, timer := range p.InitTimers {
		if timer.ID == p.AngerWindow.ID && timer.Sub == p.AngerWindow.Sub && timer.Secs > 0 {
			p.AngerWindow.Secs = timer.Secs
		}
	}
	// Kick-out targets are bound by timer identity so the wiring follows the
	// script instead of hardcoding timer numbers.
	for _, kick := range timerKicks {
		if kick.id == p.EnterBakalTimer.ID && kick.sub == p.EnterBakalTimer.Sub {
			p.EnterBakalKickDungeon = kick.dungeon
		}
		if kick.id == p.SettlementTimer.ID && kick.sub == p.SettlementTimer.Sub {
			p.SettlementKickDungeon = kick.dungeon
		}
	}
	if err := verifyBakalPhaseRules(p); err != nil {
		return nil, err
	}
	return p, nil
}

type bakalTimerKick struct {
	id, sub, dungeon int
}

func classifyBakalPair(p *BakalPhaseRules, trig, beh []pvf.Token, kicks *[]bakalTimerKick) error {
	if err := parseBakalEnterAwakening(p, trig, beh); err != nil {
		return err
	}
	switch {
	case bakalHasLabel(trig, "[1phase init]"):
		return parseBakalInitBehavior(p, beh)
	case bakalHasLabel(beh, "[fail phase]"):
		threshold, err := bakalRowInt(trig, "[=>]")
		if err != nil {
			return err
		}
		p.AngerFailThreshold = threshold
	case bakalHasLabel(beh, "[bakal anger per second]"):
		value, err := bakalRowInt(beh, "[bakal anger per second]")
		if err != nil {
			return err
		}
		if bakalHasLabel(trig, "[check timer end]") {
			window, err := bakalRowInts(trig, "[check timer end]")
			if err != nil {
				return err
			}
			if len(window) != 2 {
				return fmt.Errorf("bakal.etc: [check timer end] carries %d values, want timer id and sub", len(window))
			}
			p.AngerWindow.ID, p.AngerWindow.Sub = window[0], window[1]
			p.AngerWindowEndPerSecond = value
		} else {
			p.AngerEnterPerSecond = value
		}
	case bakalHasLabel(beh, "[awake random dragon]"):
		if len(p.AwakeDragonDelays) == 0 {
			p.AwakeDragonDelays = bakalAllRowsInts(beh, "[awake random dragon]")
		}
	case bakalHasLabel(beh, "[increase bakal anger]"):
		increase, err := bakalRowInts(beh, "[increase bakal anger]")
		if err != nil {
			return err
		}
		if len(increase) != 2 {
			return fmt.Errorf("bakal.etc: [increase bakal anger] carries %d values, want location and amount", len(increase))
		}
		entry := BakalPeriodicAnger{Location: increase[0], Amount: increase[1]}
		if bakalHasLabel(beh, "[delete monster]") {
			// The escape chain ends with DELETE MONSTER: a one-shot penalty.
			p.EscapeAnger = append(p.EscapeAnger, entry)
		} else {
			timers := bakalSetTimers(beh)
			if len(timers) != 1 {
				return fmt.Errorf("bakal.etc: periodic anger row has %d companion timers", len(timers))
			}
			entry.EverySecs = timers[0].Secs
			p.AwakeAnger = append(p.AwakeAnger, entry)
		}
	case bakalHasLabel(trig, "[bakal hp unlock grade]") && bakalHasLabel(trig, "[<]"):
		grade, err := bakalRowInt(trig, "[bakal hp unlock grade]")
		if err != nil {
			return err
		}
		floor, err := bakalRowInt(trig, "[<]")
		if err != nil {
			return err
		}
		p.HPFloors = append(p.HPFloors, BakalHPFloor{Grade: grade, Floor: floor})
	case bakalHasLabel(trig, "[check dungeon state]"):
		dungeon, err := bakalRowInt(trig, "[on change dungeon state]")
		if err != nil {
			return err
		}
		if bakalHasLabel(beh, "[move last bakal dungeon]") {
			// Bakal clear: freeze HP, move everyone to the last dungeon and
			// arm the settlement timer.
			timers := bakalSetTimers(beh)
			if len(timers) != 1 {
				return fmt.Errorf("bakal.etc: settlement behavior arms %d timers", len(timers))
			}
			p.SettlementTimer = timers[0]
			p.SettlementDungeon = dungeon
		} else if bakalHasLabel(beh, "[clear phase]") {
			p.FinalClearDungeon = dungeon
			if bakalHasLabel(beh, "[kick out dungeon no penalty]") {
				kick, err := bakalRowInt(beh, "[kick out dungeon no penalty]")
				if err != nil {
					return err
				}
				p.FinalClearKickDungeon = kick
			}
		}
	case bakalHasLabel(beh, "[kick out dungeon no penalty]"):
		timer, err := bakalRowInts(trig, "[check timer end]")
		if err != nil {
			return err
		}
		if len(timer) != 2 {
			return fmt.Errorf("bakal.etc: [check timer end] carries %d values, want timer id and sub", len(timer))
		}
		dungeon, err := bakalRowInt(beh, "[kick out dungeon no penalty]")
		if err != nil {
			return err
		}
		*kicks = append(*kicks, bakalTimerKick{id: timer[0], sub: timer[1], dungeon: dungeon})
	case bakalHasLabel(trig, "[on enter dungeon]") && bakalHasLabel(beh, "[set timer]"):
		timers := bakalSetTimers(beh)
		if len(timers) != 1 {
			return fmt.Errorf("bakal.etc: enter behavior arms %d timers", len(timers))
		}
		dungeon, err := bakalRowInt(trig, "[on enter dungeon]")
		if err != nil {
			return err
		}
		p.EnterBakalTimer = timers[0]
		p.EnterBakalDungeon = dungeon
	}
	return nil
}

func parseBakalEnterAwakening(p *BakalPhaseRules, trig, beh []pvf.Token) error {
	if !bakalHasLabel(trig, "[on enter dungeon]") || !bakalHasLabel(trig, "[>]") {
		return nil
	}
	var awake, hp string
	for _, row := range bakalRows(beh) {
		label := strings.ToUpper(row.Label)
		if strings.HasSuffix(label, " AWAKEN]") && len(bakalInts(row.Args)) == 1 {
			awake = label
		}
	}
	if awake == "" {
		return nil
	}
	hp = strings.TrimSuffix(awake, " AWAKEN]") + " HP]"
	dungeon, err := bakalRowInt(trig, "[on enter dungeon]")
	if err != nil || dungeon <= 0 {
		return fmt.Errorf("invalid native awakening dungeon")
	}
	from, err := bakalRowInt(trig, awake)
	if err != nil {
		return err
	}
	to, err := bakalRowInt(beh, awake)
	if err != nil {
		return err
	}
	threshold, err := bakalRowInt(trig, "[>]")
	if err != nil || threshold != 0 || !bakalHasLabel(trig, hp) {
		return fmt.Errorf("unsupported native awakening HP condition")
	}
	p.EnterAwakenings = append(p.EnterAwakenings, BakalEnterAwakening{Dungeon: uint32(dungeon), AwakeSymbol: awake, HPSymbol: hp, From: int32(from), To: int32(to)})
	return nil
}

// parseBakalInitBehavior walks the 1PHASE INIT behavior: boss monsters, reserve
// monsters, initial timers, buffs and the HP board are all declared here.
func parseBakalInitBehavior(p *BakalPhaseRules, beh []pvf.Token) error {
	p.InitialSymbols = map[string]int32{}
	previous := ""
	for _, row := range bakalRows(beh) {
		args := bakalInts(row.Args)
		if bakalEq(previous, "[set]") && len(args) == 1 {
			p.InitialSymbols[strings.ToUpper(row.Label)] = int32(args[0])
		}
		previous = row.Label
		switch {
		case bakalEq(row.Label, "[create monster]"):
			name := bakalStrings(row.Args)
			if len(args) < 1 || len(name) != 1 {
				return fmt.Errorf("bakal.etc: [create monster] row is malformed")
			}
			p.InitMonsters = append(p.InitMonsters, BakalMonsterSpawn{Location: args[0], Name: name[0]})
		case bakalEq(row.Label, "[reserve create monster]"):
			name := bakalStrings(row.Args)
			if len(args) < 1 || len(name) != 1 {
				return fmt.Errorf("bakal.etc: [reserve create monster] row is malformed")
			}
			p.ReserveMonsters = append(p.ReserveMonsters, BakalMonsterSpawn{Location: args[0], Name: name[0]})
		case bakalEq(row.Label, "[add bakal raid buff]"):
			if len(args) != 2 {
				return fmt.Errorf("bakal.etc: [add bakal raid buff] carries %d values, want slot and count", len(args))
			}
			p.RaidBuffs = append(p.RaidBuffs, BakalBuffGrant{Slot: args[0], Count: args[1]})
		case bakalEq(row.Label, "[set timer]"):
			if len(args) != 3 {
				return fmt.Errorf("bakal.etc: [set timer] carries %d values, want id, sub and secs", len(args))
			}
			p.InitTimers = append(p.InitTimers, BakalTimerRule{ID: args[0], Sub: args[1], Secs: args[2]})
		case bakalEq(row.Label, "[monster max hp]"):
			if len(args) != 1 {
				return fmt.Errorf("bakal.etc: [monster max hp] carries %d values", len(args))
			}
			p.MonsterMaxHP = args[0]
		case bakalEq(row.Label, "[bakal hp unlock grade]"):
			if len(args) != 1 {
				return fmt.Errorf("bakal.etc: [bakal hp unlock grade] carries %d values", len(args))
			}
			p.HPUnlockGrade = args[0]
		case bakalEq(row.Label, "[bakal anger]"):
			if len(args) != 1 {
				return fmt.Errorf("bakal.etc: [bakal anger] carries %d values", len(args))
			}
			p.AngerInit = args[0]
		case bakalEq(row.Label, "[bakal anger per second]"):
			if len(args) != 1 {
				return fmt.Errorf("invalid initial Bakal anger rate")
			}
			p.AngerInitPerSecond = args[0]
		}
		if key, ok := bakalBossHPLabel(row.Label); ok {
			if len(args) != 1 {
				return fmt.Errorf("bakal.etc: %s carries %d values, want the HP", row.Label, len(args))
			}
			p.BossHP[key] = args[0]
		}
	}
	return nil
}

var bakalBossHPLabels = map[string]string{
	"[bakal hp]":    "bakal",
	"[sparazzi hp]": "sparazzi",
	"[skasa hp]":    "skasa",
	"[hisma hp]":    "hisma",
}

func bakalBossHPLabel(label string) (string, bool) {
	for name, key := range bakalBossHPLabels {
		if bakalEq(label, name) {
			return key, true
		}
	}
	return "", false
}

func bakalSetTimers(cells []pvf.Token) []BakalTimerRule {
	var timers []BakalTimerRule
	for _, row := range bakalRows(cells) {
		if !bakalEq(row.Label, "[set timer]") {
			continue
		}
		values := bakalInts(row.Args)
		if len(values) == 3 {
			timers = append(timers, BakalTimerRule{ID: values[0], Sub: values[1], Secs: values[2]})
		}
	}
	return timers
}

func verifyBakalRaidShape(r *BakalRaidRules) error {
	if len(r.SHA256) != 64 {
		return fmt.Errorf("bakal raid rules have no source checksum")
	}
	if r.WaitingRoomTown <= 0 || r.WaitingRoomArea <= 0 {
		return fmt.Errorf("bakal raid waiting room %d/%d is incomplete", r.WaitingRoomTown, r.WaitingRoomArea)
	}
	if r.RaidMemberMax <= 0 || r.RaidStartMinimumMember <= 0 || r.RaidPartyMax <= 0 {
		return fmt.Errorf("bakal raid member limits are incomplete")
	}
	if r.WeeklyClearLimit <= 0 || r.WeeklyRewardLimit <= 0 {
		return fmt.Errorf("bakal raid weekly limits are incomplete")
	}
	if len(r.Dungeons) < 2 {
		return fmt.Errorf("bakal raid declares %d dungeons, want a raid-sized list", len(r.Dungeons))
	}
	seenDungeons := map[uint32]bool{}
	for i, dungeon := range r.Dungeons {
		if int(dungeon.Index) != int(r.Dungeons[0].Index)+i {
			return fmt.Errorf("bakal raid dungeon indexes are not contiguous at position %d", i)
		}
		if seenDungeons[dungeon.Index] {
			return fmt.Errorf("bakal raid dungeon %d is declared twice", dungeon.Index)
		}
		seenDungeons[dungeon.Index] = true
	}
	bossTypes := map[string]bool{}
	for _, dungeon := range r.Dungeons {
		if dungeon.Type == "" {
			continue
		}
		switch dungeon.Type {
		case "", "bakal", "skasa", "sparazzi", "hisma":
		default:
			return fmt.Errorf("bakal raid dungeon %d declares unknown type %q", dungeon.Index, dungeon.Type)
		}
		if bossTypes[dungeon.Type] {
			return fmt.Errorf("bakal raid type %q is declared by more than one dungeon", dungeon.Type)
		}
		bossTypes[dungeon.Type] = true
	}
	if !bossTypes["bakal"] {
		return fmt.Errorf("bakal raid has no bakal-typed boss dungeon")
	}
	if len(r.Bosses) != 4 {
		return fmt.Errorf("bakal raid combat report has %d bosses, want four", len(r.Bosses))
	}
	for _, boss := range r.Bosses {
		if boss.Monster == 0 || len(boss.DamageSymbols) == 0 {
			return fmt.Errorf("bakal raid boss %q is incomplete", boss.Color)
		}
	}
	if len(r.RewardRoles) == 0 {
		return fmt.Errorf("bakal raid combat report has no reward roles")
	}
	if len(r.Locations) == 0 {
		return fmt.Errorf("bakal raid has no locations")
	}
	seenLocations := map[int]bool{}
	for _, location := range r.Locations {
		if seenLocations[location.Index] {
			return fmt.Errorf("bakal raid location %d is declared twice", location.Index)
		}
		seenLocations[location.Index] = true
		if !seenDungeons[location.Dungeon] {
			return fmt.Errorf("bakal raid location %d references unknown dungeon %d", location.Index, location.Dungeon)
		}
	}
	lastDungeon := r.Dungeons[len(r.Dungeons)-1].Index
	highest := r.Locations[0]
	for _, location := range r.Locations {
		if location.Index > highest.Index {
			highest = location
		}
	}
	if highest.Dungeon != lastDungeon {
		return fmt.Errorf("bakal raid highest location %d does not open the last dungeon %d", highest.Index, lastDungeon)
	}
	if len(r.Bidding.Hards) == 0 {
		return fmt.Errorf("bakal raid bidding has no hard modes")
	}
	for _, hard := range r.Bidding.Hards {
		total := 0
		for _, rate := range hard.Rates {
			total += rate.Weight
		}
		if total <= 0 {
			return fmt.Errorf("bakal raid bidding hard %d has zero total weight", hard.Hard)
		}
	}
	if err := verifyBakalPhaseRules(r.NormalPhase); err != nil {
		return err
	}
	return verifyBakalPhaseRules(r.HardPhase)
}

func verifyBakalPhaseRules(p *BakalPhaseRules) error {
	if p == nil {
		return fmt.Errorf("bakal raid phase rules are missing")
	}
	if p.MonsterMaxHP <= 0 {
		return fmt.Errorf("bakal raid phase has no monster HP")
	}
	if len(p.BossHP) != 4 {
		return fmt.Errorf("bakal raid phase declares %d boss HP values, want four", len(p.BossHP))
	}
	if p.HPUnlockGrade <= 0 || len(p.HPFloors) == 0 {
		return fmt.Errorf("bakal raid phase HP unlock board is incomplete")
	}
	if p.AngerWindow.Secs <= 0 || p.AngerEnterPerSecond < 0 || p.AngerWindowEndPerSecond <= 0 {
		return fmt.Errorf("bakal raid phase anger clock is incomplete")
	}
	if p.AngerFailThreshold <= 0 {
		return fmt.Errorf("bakal raid phase has no anger fail threshold")
	}
	if len(p.InitMonsters) == 0 || len(p.ReserveMonsters) == 0 {
		return fmt.Errorf("bakal raid phase monster plan is incomplete")
	}
	if len(p.RaidBuffs) == 0 {
		return fmt.Errorf("bakal raid phase grants no buffs")
	}
	if len(p.AwakeDragonDelays) == 0 {
		return fmt.Errorf("bakal raid phase has no awake dragon delays")
	}
	if len(p.AwakeAnger) == 0 || len(p.EscapeAnger) == 0 {
		return fmt.Errorf("bakal raid phase anger increases are incomplete")
	}
	if p.EnterBakalTimer.Secs <= 0 || p.EnterBakalDungeon <= 0 || p.EnterBakalKickDungeon <= 0 {
		return fmt.Errorf("bakal raid phase enter-kick wiring is incomplete")
	}
	if p.SettlementTimer.Secs <= 0 || p.SettlementDungeon <= 0 {
		return fmt.Errorf("bakal raid phase settlement wiring is incomplete")
	}
	if p.FinalClearDungeon <= 0 {
		return fmt.Errorf("bakal raid phase final clear dungeon is missing")
	}
	return nil
}

// bakalEq compares a section label case-insensitively; the PVF string table
// preserves the author's case and different eras of the content disagree.
func bakalEq(label, name string) bool {
	return strings.EqualFold(label, name)
}

func bakalIsLabel(t pvf.Token, name string) bool {
	return t.Type == 3 && bakalEq(t.Text, name)
}

func bakalHasLabel(cells []pvf.Token, name string) bool {
	for _, c := range cells {
		if bakalIsLabel(c, name) {
			return true
		}
	}
	return false
}

// bakalRow is one section label plus the value cells that follow it, before the
// next label. PVF sections do not need close tags; the next label ends a row.
type bakalRow struct {
	Label string
	Args  []pvf.Token
}

func bakalRows(cells []pvf.Token) []bakalRow {
	var rows []bakalRow
	if len(cells) > 0 && cells[0].Type != 3 {
		rows = append(rows, bakalRow{})
	}
	for _, c := range cells {
		if c.Type == 3 {
			rows = append(rows, bakalRow{Label: c.Text})
			continue
		}
		if len(rows) == 0 {
			continue
		}
		rows[len(rows)-1].Args = append(rows[len(rows)-1].Args, c)
	}
	return rows
}

func bakalRowByLabel(cells []pvf.Token, name string) (bakalRow, bool) {
	for _, row := range bakalRows(cells) {
		if bakalEq(row.Label, name) {
			return row, true
		}
	}
	return bakalRow{}, false
}

func bakalInts(cells []pvf.Token) []int {
	var out []int
	for _, c := range cells {
		switch c.Type {
		case 0:
			out = append(out, int(c.Value))
		case 2:
			out = append(out, int(c.Number))
		}
	}
	return out
}

func bakalStrings(cells []pvf.Token) []string {
	var out []string
	for _, c := range cells {
		switch c.Type {
		case 6:
			out = append(out, c.Text)
		case 8:
			out = append(out, c.Reference)
		}
	}
	return out
}

func bakalRowInt(cells []pvf.Token, name string) (int, error) {
	values, err := bakalRowInts(cells, name)
	if err != nil {
		return 0, err
	}
	if len(values) != 1 {
		return 0, fmt.Errorf("bakal.etc: %s carries %d values, want one", name, len(values))
	}
	return values[0], nil
}

func bakalRowInts(cells []pvf.Token, name string) ([]int, error) {
	row, ok := bakalRowByLabel(cells, name)
	if !ok {
		return nil, fmt.Errorf("bakal.etc: missing %s section", name)
	}
	return bakalInts(row.Args), nil
}

func bakalRowStrings(cells []pvf.Token, name string) []string {
	row, ok := bakalRowByLabel(cells, name)
	if !ok {
		return nil
	}
	return bakalStrings(row.Args)
}

func bakalRowFloat(cells []pvf.Token, name string) (float64, error) {
	row, ok := bakalRowByLabel(cells, name)
	if !ok {
		return 0, fmt.Errorf("bakal.etc: missing %s section", name)
	}
	for _, c := range row.Args {
		switch c.Type {
		case 0:
			return float64(c.Value), nil
		case 2:
			return float64(c.Number), nil
		}
	}
	return 0, fmt.Errorf("bakal.etc: %s carries no number", name)
}

func bakalAllRowsInts(cells []pvf.Token, name string) []int {
	var out []int
	for _, row := range bakalRows(cells) {
		if bakalEq(row.Label, name) {
			out = append(out, bakalInts(row.Args)...)
		}
	}
	return out
}

// bakalBlocks returns the cells inside every open..close label pair. Nested
// same-name blocks do not occur in bakal.etc, so one flat scan is sufficient.
func bakalBlocks(cells []pvf.Token, open, close string) ([][]pvf.Token, error) {
	var out [][]pvf.Token
	for i := 0; i < len(cells); i++ {
		if !bakalIsLabel(cells[i], open) {
			continue
		}
		end := -1
		for j := i + 1; j < len(cells); j++ {
			if bakalIsLabel(cells[j], close) {
				end = j
				break
			}
		}
		if end < 0 {
			return nil, fmt.Errorf("bakal.etc: %s section is not closed", open)
		}
		out = append(out, cells[i+1:end])
		i = end
	}
	return out, nil
}

func bakalBlock(cells []pvf.Token, open, close string) ([]pvf.Token, error) {
	blocks, err := bakalBlocks(cells, open, close)
	if err != nil {
		return nil, err
	}
	if len(blocks) != 1 {
		return nil, fmt.Errorf("bakal.etc: found %d %s sections, want one", len(blocks), open)
	}
	return blocks[0], nil
}
