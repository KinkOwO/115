package adventure

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const seasonSourcePath = "contents/system/seasonlevel/main.cos"

// ImportSeasonRules reads the COS chart, exact cost key and every source-listed
// experience capsule. Item paths and rewards are never inferred from filenames.
func ImportSeasonRules(a *pvf.Archive, index catalog.ItemIndex) (*SeasonRules, error) {
	if a == nil || index.Source.Checksum == "" || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("season PVF/index source mismatch")
	}
	raw, err := a.ReadRaw(seasonSourcePath)
	if err != nil {
		return nil, err
	}
	text, err := a.ReadText(seasonSourcePath)
	if err != nil {
		return nil, err
	}
	r, err := parseSeasonRules(text)
	if err != nil {
		return nil, err
	}
	r.SourcePath = seasonSourcePath
	r.SourceChecksum = a.Snapshot().Checksum
	r.SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	costs, err := a.CTP("etc/costs.ctp")
	if err != nil {
		return nil, err
	}
	r.OathCost, err = seasonCostByKey(costs, r.OathCostKey)
	if err != nil {
		return nil, err
	}
	r.CostSHA256 = costs.SHA256
	r.Items = map[uint32]Item{}
	for _, reward := range r.Rewards {
		item, err := readRuleItem(a, index, reward.Template)
		if err != nil {
			return nil, err
		}
		r.Items[reward.Template] = item
	}
	special, err := strconv.ParseUint(r.SpecialReward[2], 10, 32)
	if err != nil {
		return nil, err
	}
	item, err := readRuleItem(a, index, uint32(special))
	if err != nil {
		return nil, err
	}
	r.Items[uint32(special)] = item
	r.Capsules, err = readSeasonCapsules(a, index)
	if err != nil {
		return nil, err
	}
	return NewSeasonRules(r)
}

func seasonBlocks(text, name string) []string {
	re := regexp.MustCompile(`(?s)\[` + regexp.QuoteMeta(name) + `\](.*?)\[/` + regexp.QuoteMeta(name) + `\]`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

func seasonBlock(text, name string) (string, error) {
	blocks := seasonBlocks(text, name)
	if len(blocks) == 0 {
		return "", fmt.Errorf("missing season block %s", name)
	}
	return blocks[0], nil
}

func seasonNumber(text, name string) (uint32, error) {
	re := regexp.MustCompile(`\[` + regexp.QuoteMeta(name) + `\]\s*(-?\d+)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("missing season number %s", name)
	}
	v, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid season number %s: %w", name, err)
	}
	return uint32(v), nil
}

var seasonIntegers = regexp.MustCompile(`-?\d+`)

func seasonNumbers(text string) ([]int64, error) {
	out := []int64{}
	for _, cell := range seasonIntegers.FindAllString(text, -1) {
		v, err := strconv.ParseInt(cell, 10, 64)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func seasonUnsigned(v int64) (uint32, error) {
	if v < 0 || v > math.MaxUint32 {
		return 0, fmt.Errorf("season number outside uint32: %d", v)
	}
	return uint32(v), nil
}

func parseSeasonRules(text string) (SeasonRules, error) {
	var r SeasonRules
	var err error
	r.Season, err = seasonNumber(text, "season id")
	if err != nil {
		return r, err
	}
	level, err := seasonNumber(text, "require minimum level")
	if err != nil || level > math.MaxUint8 {
		return r, fmt.Errorf("invalid season minimum level")
	}
	r.MinimumLevel = byte(level)
	r.OathCostKey, err = seasonNumber(text, "oath cost key")
	if err != nil {
		return r, err
	}
	r.MaxAcquisitions, err = seasonNumber(text, "max oath acquisition count")
	if err != nil {
		return r, err
	}
	chart, err := seasonBlock(text, "exp level chart")
	if err != nil {
		return r, err
	}
	for _, row := range seasonBlocks(chart, "level") {
		fields := strings.Fields(row)
		if len(fields) == 0 {
			return r, fmt.Errorf("empty season level")
		}
		n, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return r, err
		}
		upper, err := seasonNumber(row, "acc exp")
		if err != nil {
			return r, err
		}
		fame, err := seasonNumber(row, "add fame value")
		if err != nil {
			return r, err
		}
		r.Levels = append(r.Levels, SeasonLevel{Level: uint32(n), Upper: upper, Fame: fame})
		if strings.Contains(row, "[display max level]") {
			r.DisplayMaxLevel = uint32(n)
		}
	}
	penalties, err := seasonBlock(text, "penalty rule set")
	if err != nil {
		return r, err
	}
	for _, group := range seasonBlocks(penalties, "level") {
		fields := strings.Fields(group)
		if len(fields) < 2 {
			return r, fmt.Errorf("invalid season penalty levels")
		}
		low, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return r, err
		}
		high, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil || high < low {
			return r, fmt.Errorf("invalid season penalty range")
		}
		for _, row := range seasonBlocks(group, "rule") {
			p := SeasonPenalty{MinimumLevel: uint32(low), MaximumLevel: uint32(high), Ranges: [][3]uint32{}}
			p.Rule, err = seasonNumber(row, "no")
			if err != nil {
				return r, err
			}
			p.DefaultRatio, err = seasonNumber(row, "default exp ratio")
			if err != nil {
				return r, err
			}
			body, err := seasonBlock(row, "exp ratio")
			if err != nil {
				return r, err
			}
			cells, err := seasonNumbers(body)
			if err != nil {
				return r, err
			}
			if len(cells)%3 != 0 {
				return r, fmt.Errorf("incomplete season penalty triple")
			}
			for i := 0; i < len(cells); i += 3 {
				var triple [3]uint32
				for j := range triple {
					triple[j], err = seasonUnsigned(cells[i+j])
					if err != nil {
						return r, err
					}
				}
				if triple[1] < triple[0] {
					return r, fmt.Errorf("inverted season penalty bounds")
				}
				p.Ranges = append(p.Ranges, triple)
			}
			r.Penalties = append(r.Penalties, p)
		}
	}
	types := map[string]uint32{"normal dungeon": 1, "higher dungeon": 2, "legion": 3, "raid": 4, "etc": 5, "oath": 6}
	typePattern := regexp.MustCompile("\\[type\\]\\s*`([^`]+)`")
	idsPattern := regexp.MustCompile(`(?m)^\s*(\d+)\s+`)
	for _, row := range seasonBlocks(text, "dungeon category") {
		m := typePattern.FindStringSubmatch(row)
		if m == nil || types[m[1]] == 0 {
			return r, fmt.Errorf("unsupported season category")
		}
		category := types[m[1]]
		body, err := seasonBlock(row, "default exp")
		if err != nil {
			return r, err
		}
		exp, err := seasonNumbers(body)
		if err != nil {
			return r, err
		}
		if len(exp) != 1 && (len(exp) == 0 || exp[0] != -1 || len(exp)%2 != 1) {
			return r, fmt.Errorf("invalid season default experience")
		}
		if exp[0] < -1 {
			return r, fmt.Errorf("invalid season experience sentinel")
		}
		difficulties := map[uint32]uint32{}
		for i := 1; i < len(exp); i += 2 {
			d, err := seasonUnsigned(exp[i])
			if err != nil {
				return r, err
			}
			v, err := seasonUnsigned(exp[i+1])
			if err != nil {
				return r, err
			}
			if _, ok := difficulties[d]; ok {
				return r, fmt.Errorf("duplicate season difficulty")
			}
			difficulties[d] = v
		}
		ids := seasonBlocks(row, "dungeon index")
		if len(ids) == 0 {
			ids = seasonBlocks(row, "raid area")
		}
		if len(ids) == 0 {
			return r, fmt.Errorf("missing season content references")
		}
		rule, err := seasonNumber(row, "enable penalty rule set")
		if err != nil {
			return r, err
		}
		for _, m := range idsPattern.FindAllStringSubmatch(ids[0], -1) {
			id, err := strconv.ParseUint(m[1], 10, 32)
			if err != nil {
				return r, err
			}
			r.Contents = append(r.Contents, SeasonContent{ID: uint32(id), Category: category, DefaultExp: exp[0], Difficulties: difficulties, Rule: rule, CSOnly: strings.Contains(row, "[cs only]")})
		}
	}
	rewards, err := seasonBlock(text, "mist oath level reward")
	if err != nil {
		return r, err
	}
	rewardPattern := regexp.MustCompile(`\[reward\]\s*(\d+)\s+(\d+)`)
	for _, row := range seasonBlocks(rewards, "level") {
		fields := strings.Fields(row)
		if len(fields) == 0 {
			return r, fmt.Errorf("missing season reward level")
		}
		n, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return r, err
		}
		mask, err := seasonNumber(row, "bit mask")
		if err != nil {
			return r, err
		}
		m := rewardPattern.FindStringSubmatch(row)
		if m == nil {
			return r, fmt.Errorf("missing season reward")
		}
		id, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			return r, err
		}
		count, err := strconv.ParseUint(m[2], 10, 32)
		if err != nil {
			return r, err
		}
		r.Rewards = append(r.Rewards, SeasonReward{Level: uint32(n), Mask: mask, Template: uint32(id), Count: uint32(count)})
	}
	body, err := seasonBlock(text, "oath equipment list")
	if err != nil {
		return r, err
	}
	equipment, err := seasonNumbers(body)
	if err != nil {
		return r, err
	}
	for _, v := range equipment {
		id, err := seasonUnsigned(v)
		if err != nil {
			return r, err
		}
		r.OathEquipment = append(r.OathEquipment, id)
	}
	specialPattern := regexp.MustCompile("\\[30lv special reward\\]\\s*`([^`]+)`\\s*`([^`]+)`\\s*(\\d+)")
	special := specialPattern.FindStringSubmatch(text)
	if special == nil {
		return r, fmt.Errorf("missing season special reward")
	}
	r.SpecialReward = special[1:]
	return r, nil
}

func readSeasonCapsules(a *pvf.Archive, index catalog.ItemIndex) (map[uint32]SeasonCapsule, error) {
	type entry struct {
		item catalog.ItemIndexEntry
		file int
	}
	entries := []entry{}
	missing := 0
	for _, item := range index.Items {
		if item.Kind != "stackable" {
			continue
		}
		f, ok := a.FindFile(item.Path)
		if !ok {
			// devpack 基线差异：缺失源脚本的堆叠物品不可能是赛季胶囊，跳过。
			missing++
			continue
		}
		entries = append(entries, entry{item, f.Index})
	}
	if missing > 0 {
		log.Printf("PVF season capsules: %d stackable scripts missing (devpack baseline gap)", missing)
	}
	slices.SortFunc(entries, func(a, b entry) int { return a.file - b.file })
	out := map[uint32]SeasonCapsule{}
	for i, row := range entries {
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
		cells, err := a.Tokens(row.item.Path)
		if err != nil {
			return nil, err
		}
		action := ruleSection(cells, "[action type]")
		if len(action) == 0 || action[0].Type != 6 || action[0].Text != "[season level system exp]" {
			continue
		}
		if len(action) != 4 {
			return nil, fmt.Errorf("invalid season capsule %d", row.item.ID)
		}
		for _, t := range action[1:] {
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid season capsule action")
			}
		}
		minimum := int64(1)
		v, err := ruleNumbers(cells, "[minimum level]")
		if err != nil {
			return nil, err
		}
		if len(v) > 0 {
			minimum = v[0]
		}
		script, err := catalog.ReadScript(a, row.item.Path)
		if err != nil {
			return nil, err
		}
		out[row.item.ID] = SeasonCapsule{Category: uint32(action[1].Value), ID: uint32(action[2].Value), Difficulty: uint32(action[3].Value), MinimumLevel: int(minimum), Path: script.Path, SHA256: script.SHA256}
	}
	return out, nil
}

func seasonCostChild(table *pvf.CTPTable, parent pvf.CTPRecord, name string) (pvf.CTPRecord, error) {
	var refs []uint64
	for _, ref := range parent.Refs {
		if ref.Name == name {
			refs = append(refs, ref.Values...)
		}
	}
	if len(refs) != 1 || refs[0] >= uint64(len(table.Records)) {
		return pvf.CTPRecord{}, fmt.Errorf("invalid season cost reference %s", name)
	}
	row := table.Records[refs[0]]
	if row.Name != name || row.Parent != parent.Index {
		return row, fmt.Errorf("season cost owner mismatch %s", name)
	}
	return row, nil
}

func seasonCostNumbers(row pvf.CTPRecord) ([]int64, error) {
	if len(row.Refs) != 0 {
		return nil, fmt.Errorf("unsupported nested season cost value")
	}
	values := []int64{}
	for _, cell := range row.Cells {
		if cell.Kind != "float" || math.IsNaN(cell.Float) || math.IsInf(cell.Float, 0) || math.Trunc(cell.Float) != cell.Float || cell.Float < math.MinInt32 || cell.Float > math.MaxUint32 {
			return nil, fmt.Errorf("invalid season cost numeric cell")
		}
		values = append(values, int64(cell.Float))
	}
	return values, nil
}

func seasonCostByKey(table *pvf.CTPTable, key uint32) (SeasonCost, error) {
	var out SeasonCost
	matches := 0
	for _, cost := range table.RecordsOf("[cost]") {
		idx, err := seasonCostChild(table, cost, "[idx]")
		if err != nil {
			return out, err
		}
		numbers, err := seasonCostNumbers(idx)
		if err != nil {
			return out, err
		}
		if len(numbers) != 1 {
			return out, fmt.Errorf("invalid season cost index")
		}
		if numbers[0] != int64(key) {
			continue
		}
		matches++
		if len(cost.Cells) != 0 || len(cost.Refs) != 2 {
			return out, fmt.Errorf("unsupported season cost branches")
		}
		required, err := seasonCostChild(table, cost, "[required item]")
		if err != nil {
			return out, err
		}
		if len(required.Cells) != 0 || len(required.Refs) != 2 {
			return out, fmt.Errorf("unsupported required season cost branches")
		}
		gold, err := seasonCostChild(table, required, "[gold]")
		if err != nil {
			return out, err
		}
		values, err := seasonCostNumbers(gold)
		if err != nil {
			return out, err
		}
		if len(values) != 1 {
			return out, fmt.Errorf("invalid season gold cost")
		}
		out.Gold = uint32(max(int64(0), values[0]))
		materials, err := seasonCostChild(table, required, "[materials]")
		if err != nil {
			return out, err
		}
		values, err = seasonCostNumbers(materials)
		if err != nil {
			return out, err
		}
		if len(values) == 0 || len(values)%2 != 0 {
			return out, fmt.Errorf("invalid season material costs")
		}
		for i := 0; i < len(values); i += 2 {
			if values[i] <= 0 || values[i+1] <= 0 {
				return out, fmt.Errorf("non-positive season material cost")
			}
			out.Materials = append(out.Materials, SeasonCostMaterial{Template: uint32(values[i]), Count: uint32(values[i+1])})
		}
	}
	if matches != 1 {
		return out, fmt.Errorf("season cost key %d has %d definitions", key, matches)
	}
	return out, nil
}
