package loot

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const blackCardScript = "etc/dungeonspecialreward.etc"
const blackGroupScript = "etc/itemdictionary/customroutingwaygroup.cos"
const blackRoutingScript = "etc/itemdictionary/customroutingway.etc"

type BlackPurgatoryPolicy struct {
	Denominator uint32            `json:"denominator"`
	Rates       map[string]uint32 `json:"rates"`
}

// ReadBlackPurgatoryRewards keeps the source reward scope independent of the
// operator's probabilities. ClientSource records the archive actually read;
// the old export's outer-archive provenance is checked separately by auditing.
func ReadBlackPurgatoryRewards(a *pvf.Archive, index catalog.ItemIndex, policy BlackPurgatoryPolicy) (*BlackPurgatoryRewards, error) {
	if a == nil || a.Snapshot().Checksum != catalog.OdysseySource || index.Source.Checksum != catalog.OdysseySource || policy.Denominator != 1000000 || len(policy.Rates) != 3 {
		return nil, fmt.Errorf("invalid Black Purgatory source/policy")
	}
	c := BlackPurgatoryRewards{Model: blackPurgatoryCardModel, Source: a.Snapshot().Checksum, ClientSource: a.Snapshot().Checksum, Script: blackCardScript, Dungeon: BlackPurgatorySquadDungeon}
	script, err := catalog.ReadScript(a, blackCardScript)
	if err != nil {
		return nil, err
	}
	c.ScriptHash = script.SHA256
	c.Cards, c.VIPSourceOnly, err = blackNativeCards(script.Cells)
	if err != nil {
		return nil, err
	}
	c.Boss = blackPurgatoryBossRules{Model: blackPurgatoryBossModel, Denominator: policy.Denominator, Rates: map[string]uint32{}, Groups: map[string]blackPurgatoryBossGroup{}, GroupScript: blackGroupScript, RoutingScript: blackRoutingScript}
	for _, tier := range blackPurgatoryBossTiers {
		rate, ok := policy.Rates[tier]
		if !ok || rate > policy.Denominator {
			return nil, fmt.Errorf("invalid Black Purgatory rate %s", tier)
		}
		c.Boss.Rates[tier] = rate
	}
	raw, err := a.ReadRaw(blackGroupScript)
	if err != nil {
		return nil, err
	}
	c.Boss.GroupHash = fmt.Sprintf("%x", sha256.Sum256(raw))
	raw, err = a.ReadRaw(blackRoutingScript)
	if err != nil {
		return nil, err
	}
	c.Boss.RoutingHash = fmt.Sprintf("%x", sha256.Sum256(raw))
	text, err := a.ReadText(blackGroupScript)
	if err != nil {
		return nil, err
	}
	groups := regexp.MustCompile(`(?s)\[group info\](.*?)\[/group info\]`).FindAllStringSubmatch(text, -1)
	idRE := regexp.MustCompile(`\[id\]\s*(\d+)`)
	targetRE := regexp.MustCompile(`(?s)\[target\](.*?)\[/target\]`)
	for _, section := range groups {
		m := idRE.FindStringSubmatch(section[1])
		if m == nil {
			continue
		}
		groupID, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			return nil, err
		}
		var tier string
		var rarity int32
		switch groupID {
		case 1010001:
			tier, rarity = "epic", 4
		case 1010500:
			tier, rarity = "mythic", 7
		case 1010007:
			tier, rarity = "corrupt_product", 4
		default:
			continue
		}
		if _, duplicate := c.Boss.Groups[tier]; duplicate {
			return nil, fmt.Errorf("duplicate Black Purgatory source group %s", tier)
		}
		target := targetRE.FindStringSubmatch(section[1])
		if target == nil {
			return nil, fmt.Errorf("missing Black Purgatory group target")
		}
		group := blackPurgatoryBossGroup{SourceGroup: uint32(groupID)}
		seen := map[uint32]bool{}
		for _, word := range strings.Fields(target[1]) {
			n, err := strconv.ParseUint(word, 10, 32)
			if err != nil || n == 0 || seen[uint32(n)] {
				return nil, fmt.Errorf("invalid Black Purgatory target %q", word)
			}
			id := uint32(n)
			seen[id] = true
			entry, ok := index.Items[id]
			if !ok || entry.Kind != "equipment" {
				return nil, fmt.Errorf("Black Purgatory equipment %d missing source index", id)
			}
			definition, err := catalog.ReadScript(a, entry.Path)
			if err != nil {
				return nil, err
			}
			level, ok := blackNativeInt(definition.Cells, "[minimum level]")
			if !ok || level <= 0 {
				return nil, fmt.Errorf("invalid Black Purgatory equipment level %d", id)
			}
			r, ok := blackNativeInt(definition.Cells, "[rarity]")
			if !ok || r != rarity {
				return nil, fmt.Errorf("invalid Black Purgatory equipment rarity %d", id)
			}
			row := struct {
				Template     uint32 `json:"template"`
				MinimumLevel int32  `json:"minimum_level"`
				Rarity       int32  `json:"rarity"`
				Path         string `json:"path"`
				SHA256       string `json:"sha256"`
			}{id, level, rarity, definition.Path, definition.SHA256}
			group.Candidates = append(group.Candidates, row)
		}
		if len(group.Candidates) == 0 {
			return nil, fmt.Errorf("empty Black Purgatory source group")
		}
		c.Boss.Groups[tier] = group
	}
	if len(c.Boss.Groups) != 3 {
		return nil, fmt.Errorf("missing Black Purgatory source groups")
	}
	return &c, nil
}

func blackNativeInt(cells []pvf.Token, tag string) (int32, bool) {
	for i, c := range cells {
		if c.Type == 3 && c.Text == tag && i+1 < len(cells) && cells[i+1].Type == 0 {
			if i+2 < len(cells) && cells[i+2].Type != 3 {
				return 0, false
			}
			return cells[i+1].Value, true
		}
	}
	return 0, false
}

// The eight-column interpretation and five identical condition branches are
// shared with the existing 115 source exporter and native reward reader.
func blackNativeCards(cells []pvf.Token) ([]RewardBoxCandidate, []RewardBoxCandidate, error) {
	var selected []pvf.Token
	for i, c := range cells {
		if c.Type != 3 || c.Text != "[dungeon]" || i+4 >= len(cells) {
			continue
		}
		header := cells[i+1 : i+5]
		want := [4]int32{int32(BlackPurgatorySquadDungeon), 0, 1, 1}
		match := true
		for n, v := range header {
			if v.Type != 0 || v.Value != want[n] {
				match = false
			}
		}
		if !match {
			continue
		}
		if selected != nil {
			return nil, nil, fmt.Errorf("ambiguous Black Purgatory card section")
		}
		end := i + 5
		for end < len(cells) && !(cells[end].Type == 3 && cells[end].Text == "[/dungeon]") {
			end++
		}
		if end == len(cells) {
			return nil, nil, fmt.Errorf("unterminated Black Purgatory card section")
		}
		selected = cells[i+5 : end]
	}
	if len(selected) == 0 || len(selected)%8 != 0 {
		return nil, nil, fmt.Errorf("invalid Black Purgatory eight-column table")
	}
	var ordinary, vip [5][]RewardBoxCandidate
	for i := 0; i < len(selected); i += 8 {
		var row [8]int32
		for n, v := range selected[i : i+8] {
			if v.Type != 0 {
				return nil, nil, fmt.Errorf("non-number Black Purgatory reward row")
			}
			row[n] = v.Value
		}
		id, kind, reserved, conditionType, condition, weight, style, count := row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7]
		if id <= 0 || (kind != 0 && kind != 1) || reserved != -1 || conditionType != 2 || condition < 0 || condition > 4 || weight <= 0 || weight > 10000 || count != 1 || style < 0 || style > 3 {
			return nil, nil, fmt.Errorf("unverified Black Purgatory reward row")
		}
		candidate := RewardBoxCandidate{Template: uint32(id), Weight: uint32(weight), Count: uint32(count)}
		if kind == 0 {
			ordinary[condition] = append(ordinary[condition], candidate)
		} else {
			vip[condition] = append(vip[condition], candidate)
		}
	}
	for _, table := range [][5][]RewardBoxCandidate{ordinary, vip} {
		for n, rows := range table {
			if !reflect.DeepEqual(rows, table[0]) {
				return nil, nil, fmt.Errorf("Black Purgatory condition %d differs", n)
			}
			var total uint32
			for _, r := range rows {
				total += r.Weight
			}
			if total != 10000 {
				return nil, nil, fmt.Errorf("invalid Black Purgatory card weights")
			}
		}
	}
	if len(ordinary[0]) != 5 || len(vip[0]) != 1 {
		return nil, nil, fmt.Errorf("Black Purgatory card scope changed")
	}
	return ordinary[0], vip[0], nil
}
