package catalog

import (
	"reflect"
	"strings"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// Every assertion in TestImportBakalRaidRules is a value the pinned inner PVF
// declares in contents/2022/bakalraid/etc/bakal.etc; they are the replay anchors
// of the legion reconstruction (analysis/tasks/next151 §3), not server choices.
func TestImportBakalRaidRules(t *testing.T) {
	r, err := ImportBakalRaid(OpenNativeArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != BakalRaidSource || len(r.SHA256) != 64 {
		t.Fatalf("source identity is wrong: %q %q", r.Source, r.SHA256)
	}
	defer r.DungeonCatalog.CloseMapSource()
	if len(r.DungeonCatalog.Dungeons) != 17 || r.DungeonCatalog.Dungeons[100003156].MinimumLevel != 100 {
		t.Fatal("source DGN header/maze scope was not preserved")
	}
	if r.Symbols["[BAKAL ANGER]"] != 200 || r.Symbols["[BAKAL HP]"] != 201 {
		t.Fatal("native symbol index shifted")
	}
	if r.Slots[24].Dungeon != 100003149 || r.Monsters["bakal"].SecondTemplate != 109014483 {
		t.Fatal("native slots/monster phases incomplete")
	}
	if r.RaidID != 8 || len(r.Rewards) != 10 {
		t.Fatalf("native reward pool incomplete: raid=%d rewards=%d", r.RaidID, len(r.Rewards))
	}

	if r.PhaseMax != 1 || r.RaidMemberMax != 12 || r.RaidStartMinimumMember != 1 || r.RaidPartyMax != 3 {
		t.Errorf("member limits mutated: %+v", r)
	}
	if r.WaitingRoomTown != 152 || r.WaitingRoomArea != 2 {
		t.Errorf("waiting room mutated: %d/%d", r.WaitingRoomTown, r.WaitingRoomArea)
	}
	if r.WaitingRoom == nil || r.WaitingRoom.AreaID != 2 || !strings.HasSuffix(r.WaitingRoom.MapPath, "/camp1.map") || !r.OwnsWaitingRoom(152, 1) || !r.OwnsWaitingRoom(152, 2) || r.OwnsWaitingRoom(152, 0) {
		t.Fatal("native town waiting-room binding not resolved")
	}
	if !reflect.DeepEqual(r.WaitingRoomNPC, []int{100001562, 100001563}) {
		t.Errorf("waiting room npcs mutated: %v", r.WaitingRoomNPC)
	}
	if r.DisconnMemberWaitSecs != 480 || r.StartDelaySecs != 3 || r.PhaseTimeOverSecs != 9999 || r.PhaseBreakSecs != 300 {
		t.Errorf("phase timing mutated: %+v", r)
	}
	if !reflect.DeepEqual(r.MovieEndTimes, []int{16, 21, 40, 24}) {
		t.Errorf("movie end times mutated: %v", r.MovieEndTimes)
	}
	if r.RetryLimit != 1 || r.WeeklyClearLimit != 1 || r.WeeklyRewardLimit != 1 {
		t.Errorf("weekly limits mutated: %+v", r)
	}
	if !reflect.DeepEqual(r.RevivalTimes, []int{15, 30, 45, 60}) {
		t.Errorf("revival times mutated: %v", r.RevivalTimes)
	}
	if r.PartyCoinLimit != 5 || r.GuaranteePartyCoin != 2 || r.GuaranteePartyPotion != 5 {
		t.Errorf("party coin rules mutated: %+v", r)
	}
	if r.HardRaidPartyMax != 3 || r.HardLimitFame != 21817 {
		t.Errorf("hard mode gate mutated: %+v", r)
	}

	wantBosses := []BakalBossReport{
		{Color: "red", Monster: 109014482, DamageSymbols: []int{217, 218, 219}},
		{Color: "green", Monster: 109014470, DamageSymbols: []int{220, 221, 222}},
		{Color: "yellow", Monster: 109014477, DamageSymbols: []int{226, 227, 228}},
		{Color: "blue", Monster: 109014476, DamageSymbols: []int{223, 224, 225}},
	}
	if len(r.Bosses) != len(wantBosses) {
		t.Fatalf("bosses mutated: %+v", r.Bosses)
	}
	for i, want := range wantBosses {
		got := r.Bosses[i]
		if got.Color != want.Color || got.Monster != want.Monster || !reflect.DeepEqual(got.DamageSymbols, want.DamageSymbols) {
			t.Errorf("boss %d mutated: %+v", i, got)
		}
		if !strings.HasPrefix(got.Face, "Contents/System/BattleReport/Bakal/") {
			t.Errorf("boss %d face animation mutated: %q", i, got.Face)
		}
	}
	if !reflect.DeepEqual(r.RewardRoles, []string{"SUB DEALER", "CUBE", "HEALER", "AVOID", "ONE SHOT DEALER"}) {
		t.Errorf("reward roles mutated: %v", r.RewardRoles)
	}

	b := r.Bidding
	if b.StartDelaySecs != 10 || b.BreakSecs != 3 || b.TimeSecs != 8 || b.MailPrevDay != 15 || b.WaitSecs != 22 || b.WarningSecs != 2 {
		t.Errorf("bidding schedule mutated: %+v", b)
	}
	if b.DecreaseBidCount != 3 || b.DecreasePerBid != 1.0 || b.LowestTimeSecs != 5 {
		t.Errorf("bidding decrease rules mutated: %+v", b)
	}
	wantRates := []BakalBiddingRate{{Grade: 1, Weight: 30000}, {Grade: 2, Weight: 70000}}
	if len(b.Hards) != 2 {
		t.Fatalf("bidding hard modes mutated: %+v", b.Hards)
	}
	for _, hard := range b.Hards {
		if !reflect.DeepEqual(hard.Rates, wantRates) {
			t.Errorf("bidding hard %d rates mutated: %+v", hard.Hard, hard.Rates)
		}
	}

	if r.MinimalDungeonClearCount != 3 {
		t.Errorf("minimal dungeon clear count mutated: %d", r.MinimalDungeonClearCount)
	}
	wantRanks := []BakalRankDeadCount{{0, 1, 0}, {2, 5, 1}, {6, 9, 2}, {10, 14, 3}}
	if !reflect.DeepEqual(r.RankDeadCounts, wantRanks) {
		t.Errorf("rank dead counts mutated: %+v", r.RankDeadCounts)
	}

	if len(r.Dungeons) != 17 {
		t.Fatalf("dungeon list mutated: %d entries", len(r.Dungeons))
	}
	first, last := r.Dungeons[0], r.Dungeons[len(r.Dungeons)-1]
	if first.Index != 100003149 || first.Type != "bakal" || first.InitialState != "open" || first.PartyMax != 1 {
		t.Errorf("bakal dungeon mutated: %+v", first)
	}
	if first.MemberRetrySecs == nil || *first.MemberRetrySecs != 480 || first.RetryTimeGroup == nil || *first.RetryTimeGroup != 1 {
		t.Errorf("bakal dungeon retry rules mutated: %+v", first)
	}
	if !last.HasHPInfo || last.Index != 100003165 {
		t.Errorf("last dungeon mutated: %+v", last)
	}
	wantTypes := map[uint32]string{100003149: "bakal", 100003150: "skasa", 100003151: "sparazzi", 100003152: "hisma"}
	for _, dungeon := range r.Dungeons {
		if want, ok := wantTypes[dungeon.Index]; !ok || dungeon.Type != want {
			if ok {
				t.Errorf("dungeon %d type mutated: %q", dungeon.Index, dungeon.Type)
			} else if dungeon.Type != "" {
				t.Errorf("field dungeon %d unexpectedly typed %q", dungeon.Index, dungeon.Type)
			}
		}
	}

	if len(r.Locations) != 52 {
		t.Fatalf("location list mutated: %d entries", len(r.Locations))
	}
	byIndex := map[int]BakalLocationInfo{}
	for _, location := range r.Locations {
		byIndex[location.Index] = location
	}
	if location := byIndex[24]; location.Dungeon != 100003149 || location.Type != "BAKAL" || !reflect.DeepEqual(location.Movable, []int{23}) {
		t.Errorf("bakal location mutated: %+v", location)
	}
	if location := byIndex[7]; location.Type != "HATCHERY" || !reflect.DeepEqual(location.Creatable, []string{"blona"}) {
		t.Errorf("hatchery location mutated: %+v", location)
	}
	if location := byIndex[56]; location.Dungeon != 100003165 {
		t.Errorf("last location mutated: %+v", location)
	}
	if !reflect.DeepEqual(r.ChannelSlotRewardItems, []uint32{490021192, 490021383, 10323374, 10314858}) {
		t.Errorf("channel slot rewards mutated: %v", r.ChannelSlotRewardItems)
	}
	if !reflect.DeepEqual(r.MonsterPieceRewards, []uint32{0, 10337627, 1, 10337627}) {
		t.Errorf("monster piece rewards mutated: %v", r.MonsterPieceRewards)
	}

	normal := r.NormalPhase
	if normal.MonsterMaxHP != 10000 || normal.HPUnlockGrade != 3 {
		t.Errorf("normal HP board mutated: %+v", normal)
	}
	wantHP := map[string]int{"bakal": 10000, "sparazzi": 10000, "skasa": 10000, "hisma": 10000}
	if !reflect.DeepEqual(normal.BossHP, wantHP) {
		t.Errorf("normal boss HP mutated: %+v", normal.BossHP)
	}
	wantFloors := []BakalHPFloor{{Grade: 3, Floor: 5500}, {Grade: 2, Floor: 3000}, {Grade: 1, Floor: 1000}}
	if !reflect.DeepEqual(normal.HPFloors, wantFloors) {
		t.Errorf("normal HP floors mutated: %+v", normal.HPFloors)
	}
	if !reflect.DeepEqual(normal.InitTimers, []BakalTimerRule{{18, 0, 5}, {19, 0, 60}, {20, 0, 60}, {27, 24, 120}}) {
		t.Errorf("normal init timers mutated: %+v", normal.InitTimers)
	}
	if normal.AngerInit != 0 || normal.AngerEnterPerSecond != 1 || normal.AngerWindowEndPerSecond != 10 {
		t.Errorf("normal anger clock mutated: %+v", normal)
	}
	if normal.AngerWindow != (BakalTimerRule{27, 24, 120}) {
		t.Errorf("normal anger window mutated: %+v", normal.AngerWindow)
	}
	if normal.AngerFailThreshold != 8000 {
		t.Errorf("normal anger threshold mutated: %d", normal.AngerFailThreshold)
	}
	wantBuffs := []BakalBuffGrant{{0, 2}, {1, 2}, {2, 2}, {3, 2}, {4, 2}}
	if !reflect.DeepEqual(normal.RaidBuffs, wantBuffs) {
		t.Errorf("normal raid buffs mutated: %+v", normal.RaidBuffs)
	}
	wantSpawns := []BakalMonsterSpawn{
		{24, "bakal"}, {12, "sparazzi"}, {26, "skasa"}, {40, "hisma"},
		{7, "blona"}, {47, "nympha"}, {23, "basilisk"}, {25, "gerda"},
	}
	if !reflect.DeepEqual(normal.InitMonsters, wantSpawns) {
		t.Errorf("normal init monsters mutated: %+v", normal.InitMonsters)
	}
	if !reflect.DeepEqual(normal.ReserveMonsters, []BakalMonsterSpawn{{10, "swan"}, {10, "steich"}, {10, "eclair"}}) {
		t.Errorf("normal reserve monsters mutated: %+v", normal.ReserveMonsters)
	}
	if !reflect.DeepEqual(normal.AwakeDragonDelays, []int{0, 900, 1800}) {
		t.Errorf("normal awake delays mutated: %v", normal.AwakeDragonDelays)
	}
	wantAwakeAnger := []BakalPeriodicAnger{{12, 50, 60}, {26, 50, 60}, {40, 50, 60}}
	if !reflect.DeepEqual(normal.AwakeAnger, wantAwakeAnger) {
		t.Errorf("normal awake anger mutated: %+v", normal.AwakeAnger)
	}
	if len(normal.EscapeAnger) < 2 {
		t.Fatalf("normal escape anger mutated: %+v", normal.EscapeAnger)
	}
	escapeLocations := map[int]bool{}
	for _, entry := range normal.EscapeAnger {
		if entry.Amount != 2000 || entry.EverySecs != 0 {
			t.Errorf("normal escape anger row mutated: %+v", entry)
		}
		escapeLocations[entry.Location] = true
	}
	if !escapeLocations[11] {
		t.Errorf("normal escape anger lost location 11: %+v", normal.EscapeAnger)
	}
	if normal.EnterBakalDungeon != 100003149 || normal.EnterBakalTimer != (BakalTimerRule{28, 24, 600}) || normal.EnterBakalKickDungeon != 100003149 {
		t.Errorf("normal enter-kick wiring mutated: %+v", normal)
	}
	if normal.SettlementDungeon != 100003149 || normal.SettlementTimer != (BakalTimerRule{28, 56, 180}) || normal.SettlementKickDungeon != 100003165 {
		t.Errorf("normal settlement wiring mutated: %+v", normal)
	}
	if normal.FinalClearDungeon != 100003165 || normal.FinalClearKickDungeon != 100003165 {
		t.Errorf("normal final clear wiring mutated: %+v", normal)
	}

	hard := r.HardPhase
	if hard.AngerWindow != (BakalTimerRule{27, 24, 420}) || hard.AngerFailThreshold != 7000 {
		t.Errorf("hard anger rules mutated: window %+v threshold %d", hard.AngerWindow, hard.AngerFailThreshold)
	}
	if !reflect.DeepEqual(hard.RaidBuffs, []BakalBuffGrant{{4, 2}}) {
		t.Errorf("hard raid buffs mutated: %+v", hard.RaidBuffs)
	}
	if !reflect.DeepEqual(hard.AwakeDragonDelays, []int{0, 0, 0}) {
		t.Errorf("hard awake delays mutated: %v", hard.AwakeDragonDelays)
	}
	if hard.SettlementTimer != (BakalTimerRule{28, 56, 180}) || hard.SettlementKickDungeon != 0 {
		t.Errorf("hard settlement wiring mutated: %+v", hard)
	}
	if hard.FinalClearDungeon != 100003165 || hard.FinalClearKickDungeon != 0 {
		t.Errorf("hard final clear wiring mutated: %+v", hard)
	}
	if hard.EnterBakalTimer != (BakalTimerRule{28, 24, 600}) || hard.EnterBakalKickDungeon != 100003149 {
		t.Errorf("hard enter-kick wiring mutated: %+v", hard)
	}
	if hard.MonsterMaxHP != 10000 || !reflect.DeepEqual(hard.HPFloors, wantFloors) {
		t.Errorf("hard HP board mutated: %+v", hard)
	}
	if len(hard.Triggers) == 0 || len(normal.Triggers) == 0 {
		t.Errorf("phase trigger records were dropped")
	}
}

// The row/block helpers run without a PVF: this synthetic script pins their
// exact semantics (leading values, implicit section ends, close-tag pairs).
func TestBakalScriptRowsAndBlocks(t *testing.T) {
	cells := []pvf.Token{
		{Type: 0, Value: 7}, // leading value before any label
		{Type: 3, Text: "[Waiting Room]"},
		{Type: 0, Value: 152},
		{Type: 0, Value: 2},
		{Type: 3, Text: "[Hatchery]"},
		{Type: 6, Text: "blona"},
		{Type: 3, Text: "[/Hatchery]"},
		{Type: 3, Text: "[Trigger]"},
		{Type: 3, Text: "[If]"},
		{Type: 3, Text: "[/Trigger]"},
	}
	cells = append(cells, pvf.Token{Type: 3, Text: "[Phase Max]"}, pvf.Token{Type: 0, Value: 1})
	if v, err := bakalRowInt(cells, "[phase max]"); err != nil || v != 1 {
		t.Errorf("case-insensitive single-value lookup failed: %d %v", v, err)
	}
	if _, err := bakalRowInt(cells, "[waiting room]"); err == nil {
		t.Errorf("multi-value rows must be refused by the single-value reader")
	}
	if v, err := bakalRowInts(cells, "[waiting room]"); err != nil || !reflect.DeepEqual(v, []int{152, 2}) {
		t.Errorf("multi-value row failed: %v %v", v, err)
	}
	if names := bakalRowStrings(cells, "[hatchery]"); !reflect.DeepEqual(names, []string{"blona"}) {
		t.Errorf("string row failed: %v", names)
	}
	if values := bakalAllRowsInts(cells, "[if]"); len(values) != 0 {
		t.Errorf("empty rows should collect nothing: %v", values)
	}
	rows := bakalRows(cells)
	if len(rows) != 8 || len(bakalInts(rows[0].Args)) != 1 || bakalInts(rows[0].Args)[0] != 7 {
		t.Fatalf("row split failed: %+v", rows)
	}
	blocks, err := bakalBlocks(cells, "[trigger]", "[/trigger]")
	if err != nil || len(blocks) != 1 || len(blocks[0]) != 1 {
		t.Fatalf("block scan failed: %+v %v", blocks, err)
	}
	if _, err := bakalBlock(cells, "[dungeon info]", "[/dungeon info]"); err == nil {
		t.Errorf("missing block must be reported")
	}
	if _, err := bakalBlock(cells, "[hatchery]", "[/hatchery]"); err != nil {
		t.Errorf("existing block must be found: %v", err)
	}
	if !bakalEq("[PHASE MAX]", "[phase max]") {
		t.Errorf("label comparison must be case-insensitive")
	}
	if _, err := ImportBakalRaid(nil); err == nil {
		t.Errorf("nil archive must be refused")
	}
}
