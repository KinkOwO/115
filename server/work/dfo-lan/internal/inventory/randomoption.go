package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
)

// Current 115 magic-seal record layout inside the 181-byte item record, as
// the client consumes it: offset 13 is the sealed flag the S2C13 trace names
// isSealed, and the compact random-option block occupies 60..75 (count, three
// ids, three value_a, three value_b, type, special index 0xFF). Confirmed
// against the live unseal flow: items the client offers to unseal carry
// byte13=0 with an empty option block, and a successful unseal fills the
// block while byte13 stays 0.
const (
	RandomOptionCountOffset   = 60
	RandomOptionIDOffset      = 61
	RandomOptionValueAOffset  = 64
	RandomOptionValueBOffset  = 67
	RandomOptionTypeOffset    = 70
	RandomOptionSpecialOffset = 71
	RandomOptionSpecialNone   = 0xFF
	RandomOptionBlockEnd      = 76
	RandomOptionCapacity      = 3
	MagicSealedOffset         = 13
)

type RandomOptionData struct {
	Model  string `json:"model"`
	Source struct {
		Checksum string `json:"checksum"`
	} `json:"source"`
	TableSHA256 map[string]string `json:"table_sha256"`
	ValueRatios []struct {
		Low, High float32
		Weight    int32
	} `json:"value_ratios"`
	QuantityWeights  map[int32][][2]int32 `json:"quantity_weights"`
	OptionQuantities []struct {
		Rarity, OptionType, Level, Min, Max int32
	} `json:"option_quantities"`
	OptionGroups map[int32][][2]int32 `json:"option_groups"`
	GroupChoices []struct {
		Rarity   int32
		Group    string
		Quantity int32
		Groups   [3]int32
	} `json:"group_choices"`
	OptionTypeWeights []struct {
		Rarity, Level int32
		Weights       [3]int32
	} `json:"option_type_weights"`
	DifferentWeights []struct {
		Rarity, OptionType, Min, Max int32
	} `json:"different_weights"`
	BreakSealCosts []struct {
		Rarity, Level, Part, Cost int32
	} `json:"break_seal_costs"`
}

type randomOptionConfig = RandomOptionData

func (c *RandomOptionCatalog) ValidateSource(source string) error {
	if c == nil || len(source) != 64 || c.source != source {
		return fmt.Errorf("random option source mismatch")
	}
	return nil
}

// RandomOptionCatalog is the source-pinned projection of the client's seven
// etc/randomoption tables. Equipment membership stays in the full equipment
// catalog; this catalog only carries the roll rules.
type RandomOptionCatalog struct {
	source           string
	valueRatios      []randomOptionValueRatio
	quantityWeights  map[int32][][2]int32
	optionQuantities map[[3]int32][2]int32
	optionGroups     map[int32][][2]int32
	groupChoices     map[randomOptionChoiceKey][3]int32
	optionTypes      map[[2]int32][3]int32
	differentWeights map[[2]int32][2]int32
	breakSealCosts   map[[3]int32]int32
}

type randomOptionValueRatio struct {
	Low, High float32
	Weight    int32
}
type randomOptionChoiceKey struct {
	Rarity   int32
	Group    string
	Quantity int32
}

// RolledOption is one sealed-stat outcome in the client record layout.
type RolledOption struct {
	ID, ValueA, ValueB byte
}

// RandomOptionRoll is the full unseal outcome persisted with the item.
type RandomOptionRoll struct {
	OptionType byte
	Options    [3]RolledOption
	Count      byte
	Gold       uint32
}

func ReadRandomOptionData(path string) (RandomOptionData, error) {
	var c RandomOptionData
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func LoadRandomOptionCatalog(path, source string) (*RandomOptionCatalog, error) {
	c, err := ReadRandomOptionData(path)
	if err != nil {
		return nil, err
	}
	return NewRandomOptionCatalog(c, source)
}

func NewRandomOptionCatalog(c RandomOptionData, source string) (*RandomOptionCatalog, error) {
	if c.Model != "current115-randomoption-v1" || c.Source.Checksum != source {
		return nil, fmt.Errorf("random option source mismatch")
	}
	if len(c.ValueRatios) == 0 || len(c.QuantityWeights) == 0 || len(c.OptionQuantities) == 0 ||
		len(c.OptionGroups) == 0 || len(c.GroupChoices) == 0 || len(c.OptionTypeWeights) == 0 ||
		len(c.DifferentWeights) == 0 || len(c.BreakSealCosts) == 0 {
		return nil, fmt.Errorf("incomplete random option rules")
	}
	out := &RandomOptionCatalog{
		source:           source,
		quantityWeights:  c.QuantityWeights,
		optionGroups:     c.OptionGroups,
		optionQuantities: map[[3]int32][2]int32{},
		groupChoices:     map[randomOptionChoiceKey][3]int32{},
		optionTypes:      map[[2]int32][3]int32{},
		differentWeights: map[[2]int32][2]int32{},
		breakSealCosts:   map[[3]int32]int32{},
	}
	for _, r := range c.ValueRatios {
		if !(r.Low > 0 && r.Low <= r.High && r.High <= 1) || r.Weight <= 0 {
			return nil, fmt.Errorf("invalid option value ratio")
		}
		out.valueRatios = append(out.valueRatios, randomOptionValueRatio{r.Low, r.High, r.Weight})
	}
	for rarity, rows := range c.QuantityWeights {
		if rarity != 2 && rarity != 3 {
			return nil, fmt.Errorf("invalid quantity weight rarity %d", rarity)
		}
		seen := map[int32]bool{}
		for _, row := range rows {
			if row[0] < 1 || row[0] > RandomOptionCapacity || row[1] < 0 || seen[row[0]] {
				return nil, fmt.Errorf("invalid quantity weight row")
			}
			seen[row[0]] = true
		}
	}
	for _, r := range c.OptionQuantities {
		if r.Rarity != 2 && r.Rarity != 3 || r.OptionType < 1 || r.OptionType > 3 ||
			r.Min < 1 || r.Min > r.Max || r.Max > RandomOptionCapacity {
			return nil, fmt.Errorf("invalid option quantity row")
		}
		key := [3]int32{r.Rarity, r.OptionType, r.Level}
		if _, dup := out.optionQuantities[key]; dup {
			return nil, fmt.Errorf("duplicate option quantity row")
		}
		out.optionQuantities[key] = [2]int32{r.Min, r.Max}
	}
	for id, rows := range c.OptionGroups {
		if id <= 0 || len(rows) == 0 {
			return nil, fmt.Errorf("invalid option group %d", id)
		}
		for _, row := range rows {
			if row[0] < 1 || row[0] > 0xFF || row[1] <= 0 {
				return nil, fmt.Errorf("invalid option group %d member", id)
			}
		}
	}
	for _, r := range c.GroupChoices {
		if r.Rarity != 2 && r.Rarity != 3 || r.Group == "" || r.Quantity < 1 || r.Quantity > RandomOptionCapacity {
			return nil, fmt.Errorf("invalid option group choice")
		}
		key := randomOptionChoiceKey{r.Rarity, strings.ToLower(r.Group), r.Quantity}
		if _, dup := out.groupChoices[key]; dup {
			return nil, fmt.Errorf("duplicate option group choice")
		}
		for _, g := range r.Groups {
			if _, ok := c.OptionGroups[g]; !ok {
				return nil, fmt.Errorf("option group choice references missing group %d", g)
			}
		}
		out.groupChoices[key] = r.Groups
	}
	for _, r := range c.OptionTypeWeights {
		if r.Rarity != 2 && r.Rarity != 3 || r.Level < 1 {
			return nil, fmt.Errorf("invalid option type weight row")
		}
		key := [2]int32{r.Rarity, r.Level}
		if _, dup := out.optionTypes[key]; dup {
			return nil, fmt.Errorf("duplicate option type weight row")
		}
		if r.Weights[0] <= 0 && r.Weights[1] <= 0 && r.Weights[2] <= 0 {
			return nil, fmt.Errorf("empty option type weights")
		}
		out.optionTypes[key] = r.Weights
	}
	for _, r := range c.DifferentWeights {
		if r.Rarity != 2 && r.Rarity != 3 || r.OptionType < 1 || r.OptionType > 3 ||
			r.Min < 0 || r.Min >= r.Max || r.Max > 0xFF {
			return nil, fmt.Errorf("invalid different weight row")
		}
		out.differentWeights[[2]int32{r.Rarity, r.OptionType}] = [2]int32{r.Min, r.Max}
	}
	for _, r := range c.BreakSealCosts {
		if r.Rarity != 2 && r.Rarity != 3 || r.Level < 1 || r.Part < 1 || r.Part > 3 || r.Cost < 0 {
			return nil, fmt.Errorf("invalid break seal cost row")
		}
		out.breakSealCosts[[3]int32{r.Rarity, r.Level, r.Part}] = r.Cost
	}
	return out, nil
}

// magicSealDefinition is the per-item slice of its source .equ the roll needs.
type magicSealDefinition struct {
	Rarity int32
	Level  int32
	Group  string
}

func singleInt(def EquipmentDefinition, field string) (int32, bool) {
	rows := def.Fields[field]
	if len(rows) != 1 || rows[0].Type != 0 {
		return 0, false
	}
	return rows[0].Value, true
}

// magicSealTarget extracts the unseal rule key from an equipment definition.
// Only source rows explicitly flagged [random option] 1 qualify; everything
// else is ordinary gear the client never offers to unseal.
func magicSealTarget(def EquipmentDefinition) (magicSealDefinition, bool) {
	flagged := def.Fields["[random option]"]
	if len(flagged) != 1 || flagged[0].Type != 0 || flagged[0].Value != 1 {
		return magicSealDefinition{}, false
	}
	rarity, ok := singleInt(def, "[rarity]")
	if !ok || (rarity != 2 && rarity != 3) {
		return magicSealDefinition{}, false
	}
	level, ok := singleInt(def, "[minimum level]")
	if !ok || level < 1 {
		return magicSealDefinition{}, false
	}
	group := def.Fields["[item group name]"]
	if len(group) != 1 || group[0].Text == "" {
		return magicSealDefinition{}, false
	}
	name := strings.ToLower(strings.Trim(group[0].Text, "`[]"))
	return magicSealDefinition{rarity, level, name}, true
}

// partCategory maps the source [item group name] to the [break seal cost]
// part column: 1 weapon, 2 armor, 3 accessory. The current table charges 0
// for every row; the mapping still follows the source columns so a nonzero
// future table lands correctly. Accessory groups are the three jewelry slots
// plus the two armor-adjacent groups the selection table lists beside them.
func partCategory(group string) int32 {
	switch group {
	case "amulet", "wrist", "ring":
		return 3
	case "cl coat", "cl pants", "cl shoes", "cl shoulder", "cl waist",
		"lt coat", "lt pants", "lt shoes", "lt shoulder", "lt waist",
		"la coat", "la pants", "la shoes", "la shoulder", "la waist",
		"ha coat", "ha pants", "ha shoes", "ha shoulder", "ha waist",
		"mt coat", "mt pants", "mt shoes", "mt shoulder", "mt waist":
		return 2
	default:
		return 1
	}
}

// GroupCount reports the option pool size for startup diagnostics.
func (c *RandomOptionCatalog) GroupCount() int { return len(c.optionGroups) }

func (c *RandomOptionCatalog) weightedIndex(weights []int32, rng *rand.Rand) (int, error) {
	var total int64
	for _, w := range weights {
		if w > 0 {
			total += int64(w)
		}
	}
	if total <= 0 {
		return -1, fmt.Errorf("no positive random option weight")
	}
	roll := rng.Int63n(total)
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if roll < int64(w) {
			return i, nil
		}
		roll -= int64(w)
	}
	return -1, fmt.Errorf("random option weighted selection did not converge")
}

func (c *RandomOptionCatalog) rollValue(min, max int32, rng *rand.Rand) (byte, error) {
	weights := make([]int32, len(c.valueRatios))
	for i, r := range c.valueRatios {
		weights[i] = r.Weight
	}
	idx, e := c.weightedIndex(weights, rng)
	if e != nil {
		return 0, e
	}
	ratio := c.valueRatios[idx]
	start := int64(float32(min) * ratio.Low)
	span := int64(float32(max-min) * ratio.High)
	value := start
	if span > 0 {
		value += rng.Int63n(span)
	}
	if value < 0 || value > 0xFF {
		return 0, fmt.Errorf("rolled random option value %d outside u8", value)
	}
	return byte(value), nil
}

// Roll draws the full unseal outcome for one source-flagged item. Every table
// lookup is exact: the current PVF covers every flagged item's rarity, level
// and group (verified by the import audit), so a miss means catalog drift and
// is refused loudly instead of being guessed.
func (c *RandomOptionCatalog) Roll(def EquipmentDefinition, rng *rand.Rand) (RandomOptionRoll, error) {
	var out RandomOptionRoll
	target, ok := magicSealTarget(def)
	if !ok {
		return out, fmt.Errorf("item %d is not source magic-seal equipment", def.ID)
	}
	weightsRow, ok := c.optionTypes[[2]int32{target.Rarity, target.Level}]
	if !ok {
		return out, fmt.Errorf("item %d has no option type row (rarity %d level %d)", def.ID, target.Rarity, target.Level)
	}
	typeIdx, e := c.weightedIndex(weightsRow[:], rng)
	if e != nil {
		return out, e
	}
	optionType := int32(typeIdx + 1)
	bounds, ok := c.optionQuantities[[3]int32{target.Rarity, optionType, target.Level}]
	if !ok {
		return out, fmt.Errorf("item %d has no option quantity row", def.ID)
	}
	quantityRows := c.quantityWeights[target.Rarity]
	var candidates []int32
	var candidateWeights []int32
	for _, row := range quantityRows {
		if row[0] >= bounds[0] && row[0] <= bounds[1] && row[1] > 0 {
			candidates = append(candidates, row[0])
			candidateWeights = append(candidateWeights, row[1])
		}
	}
	qtyIdx, e := c.weightedIndex(candidateWeights, rng)
	if e != nil {
		return out, fmt.Errorf("item %d has no rollable option quantity", def.ID)
	}
	quantity := candidates[qtyIdx]
	choice, ok := c.groupChoices[randomOptionChoiceKey{target.Rarity, target.Group, quantity}]
	if !ok {
		return out, fmt.Errorf("item %d has no option group choice (rarity %d group %q quantity %d)", def.ID, target.Rarity, target.Group, quantity)
	}
	pool := append([][2]int32(nil), c.optionGroups[choice[optionType-1]]...)
	if len(pool) == 0 {
		return out, fmt.Errorf("option group %d is empty", choice[optionType-1])
	}
	valueRange, ok := c.differentWeights[[2]int32{target.Rarity, optionType}]
	if !ok {
		return out, fmt.Errorf("item %d has no option value range", def.ID)
	}
	out.OptionType = byte(optionType)
	out.Count = byte(quantity)
	seen := map[int32]bool{}
	for i := int32(0); i < quantity; i++ {
		weights := make([]int32, len(pool))
		for j, row := range pool {
			weights[j] = row[1]
		}
		idx, err := c.weightedIndex(weights, rng)
		if err != nil {
			return out, err
		}
		optionID := pool[idx][0]
		if seen[optionID] {
			return out, fmt.Errorf("option group selection repeated %d", optionID)
		}
		seen[optionID] = true
		pool = append(pool[:idx], pool[idx+1:]...)
		valueA, err := c.rollValue(valueRange[0], valueRange[1], rng)
		if err != nil {
			return out, err
		}
		valueB, err := c.rollValue(valueRange[0], valueRange[1], rng)
		if err != nil {
			return out, err
		}
		out.Options[i] = RolledOption{byte(optionID), valueA, valueB}
	}
	cost, ok := c.breakSealCosts[[3]int32{target.Rarity, target.Level, partCategory(target.Group)}]
	if !ok {
		return out, fmt.Errorf("item %d has no break seal cost row", def.ID)
	}
	if cost > 0 && uint64(cost) > math.MaxUint32 {
		return out, fmt.Errorf("break seal cost %d overflows gold", cost)
	}
	out.Gold = uint32(cost)
	return out, nil
}

// RandomOptionBlockEmpty reports whether the record's option block is
// pristine, which is how the current client presents a sealed item: source
// definitions flag [random option] 1 gear, and the live unseal requests this
// build received targeted records whose block was still all zero.
func RandomOptionBlockEmpty(record [protocol.CurrentItemRecordSize]byte) bool {
	for _, b := range record[RandomOptionCountOffset:RandomOptionBlockEnd] {
		if b != 0 {
			return false
		}
	}
	return true
}

// applyRandomOptions writes the roll into the record's option block and
// clears the sealed flag, mirroring the layout the client tooltip reads.
func applyRandomOptions(record [protocol.CurrentItemRecordSize]byte, roll RandomOptionRoll) [protocol.CurrentItemRecordSize]byte {
	for i := RandomOptionCountOffset; i < RandomOptionBlockEnd; i++ {
		record[i] = 0
	}
	record[MagicSealedOffset] = 0
	record[RandomOptionCountOffset] = roll.Count
	record[RandomOptionTypeOffset] = roll.OptionType
	record[RandomOptionSpecialOffset] = RandomOptionSpecialNone
	for i := 0; i < int(roll.Count); i++ {
		record[RandomOptionIDOffset+i] = roll.Options[i].ID
		record[RandomOptionValueAOffset+i] = roll.Options[i].ValueA
		record[RandomOptionValueBOffset+i] = roll.Options[i].ValueB
	}
	return record
}

// UnsealRandomOption breaks the magic seal on one bag equipment slot. The
// durable result is the full 181-byte record stored on the bag row, so the
// rolled options survive relogin; the same record is what NOTI14 pushes.
func (b Bag) UnsealRandomOption(c *EquipmentCatalog, rc *RandomOptionCatalog, slot uint16, rng *rand.Rand) (Bag, [protocol.CurrentItemRecordSize]byte, RandomOptionRoll, error) {
	var empty [protocol.CurrentItemRecordSize]byte
	var none RandomOptionRoll
	if c == nil || rc == nil || rng == nil {
		return b, empty, none, fmt.Errorf("unseal without catalogs")
	}
	idx := -1
	for i, item := range b.Equipment {
		if item.Slot == slot {
			idx = i
			break
		}
	}
	if idx < 0 {
		return b, empty, none, fmt.Errorf("no equipment at slot %d", slot)
	}
	item := b.Equipment[idx]
	record := EquipmentRow(item)
	if !RandomOptionBlockEmpty(record) {
		return b, empty, none, fmt.Errorf("equipment at slot %d is already unsealed", slot)
	}
	def, e := c.Definition(item.Template)
	if e != nil {
		return b, empty, none, e
	}
	if _, ok := magicSealTarget(def); !ok {
		return b, empty, none, fmt.Errorf("equipment %d cannot be unsealed", item.Template)
	}
	roll, e := rc.Roll(def, rng)
	if e != nil {
		return b, empty, none, e
	}
	if roll.Gold > 0 && b.Gold < roll.Gold {
		return b, empty, none, fmt.Errorf("insufficient gold: need %d, have %d", roll.Gold, b.Gold)
	}
	record = applyRandomOptions(record, roll)
	b.Gold -= roll.Gold
	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	// EquipmentRow re-derives slot/template/durability over the stored record;
	// persist the post-unseal record so the options and cleared seal survive.
	b.Equipment[idx].Record = append([]byte(nil), record[:]...)

	updated := EquipmentRow(b.Equipment[idx])
	return b, updated, roll, nil
}
