package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// ticketFields preserves the Python export projection: text cells carry their
// decoded string, float cells retain raw bits, and repeated sections append.
// Closing tags remain present as empty keys just as in the legacy export.
func ticketFields(cells []pvf.Token) map[string][]pvf.Token {
	out := map[string][]pvf.Token{}
	current := ""
	for _, t := range cells {
		if t.Type == 3 {
			current = t.Text
			if _, ok := out[current]; !ok {
				out[current] = nil
			}
			continue
		}
		if current == "" {
			continue
		}
		switch t.Type {
		case 6:
			t.Value = 0
		case 8:
			t.Text = t.Reference
			t.Reference = ""
			t.Value = 0
		case 2:
			t.Number = 0
		}
		out[current] = append(out[current], t)
	}
	return out
}

func sourceSections(cells []pvf.Token) map[string][]pvf.Token {
	out := ticketFields(cells)
	for tag := range out {
		if strings.HasPrefix(tag, "[/") {
			delete(out, tag)
		}
	}
	return out
}

func appendRuleRow[T any](rows []T) ([]T, *T) {
	var zero T
	rows = append(rows, zero)
	return rows, &rows[len(rows)-1]
}

func readEnhancementPolicy(path string) (enhancementPolicy, error) {
	var p enhancementPolicy
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(b, &raw); err != nil {
		return p, err
	}
	for key := range raw {
		if key != "version" && key != "provenance" && key != "pure_templates" && key != "gold" && key != "amplify" {
			return p, fmt.Errorf("unknown enhancement policy section %s", key)
		}
	}
	allowed := map[string][]string{
		"gold":    {"materials", "materials_pending_safe_path", "safe_path_materials", "safe_path_rules", "max_upgrade_level", "material_count_semantics", "failure"},
		"amplify": {"official"},
	}
	for section, keys := range allowed {
		var obj map[string]json.RawMessage
		if err = json.Unmarshal(raw[section], &obj); err != nil {
			return p, err
		}
		for key := range obj {
			ok := false
			for _, k := range keys {
				if key == k {
					ok = true
				}
			}
			if !ok {
				return p, fmt.Errorf("enhancement policy contains source field %s.%s", section, key)
			}
		}
	}
	var gold map[string]json.RawMessage
	_ = json.Unmarshal(raw["gold"], &gold)
	for _, group := range []string{"materials", "materials_pending_safe_path"} {
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(gold[group], &rows); err != nil {
			return p, err
		}
		for _, row := range rows {
			for key := range row {
				if key != "template" && key != "name" && key != "where" && key != "note" {
					return p, fmt.Errorf("enhancement policy contains source material field %s.%s", group, key)
				}
			}
		}
	}
	var failure map[string]json.RawMessage
	_ = json.Unmarshal(gold["failure"], &failure)
	for key := range failure {
		if key != "destroy_enabled" && key != "player_measured" {
			return p, fmt.Errorf("enhancement policy contains source failure field %s", key)
		}
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	if p.Version != 1 || len(p.Gold.Failure.PlayerMeasured.SuccessRatePercentByLevel) == 0 || len(p.Amplify.Official.SuccessRatePercentByLevel) == 0 || p.Gold.SafePathRules.MaxLevel <= 0 || len(p.Gold.SafePathRules.StageRate) == 0 || p.Amplify.Official.SafeAmplify.MaxLevel <= 0 {
		return p, fmt.Errorf("incomplete enhancement policy")
	}
	return p, nil
}

func ImportEnhancements(a *pvf.Archive, index catalog.ItemIndex, policyPath string) (*EnhancementCatalog, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("enhancement/PVF source mismatch")
	}
	p, err := readEnhancementPolicy(policyPath)
	if err != nil {
		return nil, err
	}
	c := &EnhancementCatalog{ReinforcementTickets: map[uint32]reinforcementTicket{}, AmplifyTickets: map[uint32]reinforcementTicket{}, Gold: p.Gold, Amplify: p.Amplify}
	// Material selection/location and display aliases are server policy. Script
	// paths always come from the authoritative source list, never from policy.
	for _, rows := range [][]goldMaterialDefinition{c.Gold.Materials, c.Gold.MaterialsPendingSafePath} {
		for i := range rows {
			item, ok := index.Items[rows[i].Template]
			if !ok || item.Kind != "stackable" || item.Path == "" {
				return nil, fmt.Errorf("missing enhancement policy material %d in PVF index", rows[i].Template)
			}
			rows[i].Path = item.Path
		}
	}
	c.Grimoires.Version = 1
	c.Grimoires.Source = index.Source.Checksum
	c.Grimoires.PureTemplates = p.PureTemplates
	c.Enchant.Version = 1
	c.Enchant.Source = index.Source.Checksum
	ids := make([]uint32, 0, len(index.Items))
	for id, item := range index.Items {
		if item.Kind == "stackable" && strings.HasSuffix(item.Path, ".stk") {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		item := index.Items[id]
		if _, ok := a.FindFile(item.Path); !ok {
			continue
		}
		cells, err := a.Tokens(item.Path)
		if err != nil {
			return nil, err
		}
		fields := ticketFields(cells)
		if _, ok := fields["[equipment reinforcement ticket]"]; ok {
			c.ReinforcementTickets[id] = reinforcementTicket{Path: item.Path, Fields: fields}
		}
		if _, ok := fields[amplifyTicketSection]; ok {
			c.AmplifyTickets[id] = reinforcementTicket{Path: item.Path, Fields: fields}
		}
		_, expires := fields["[expiration date]"]
		if random := fields["[amplification random value]"]; len(random) > 0 {
			if len(random)%2 != 0 {
				return nil, fmt.Errorf("incomplete grimoire weights %d", id)
			}
			rows, row := appendRuleRow(c.Grimoires.Grimoires)
			c.Grimoires.Grimoires = rows
			row.Template = id
			row.Path = item.Path
			row.Expires = expires
			for j := 0; j < len(random); j += 2 {
				if random[j].Type != 0 || random[j+1].Type != 0 {
					return nil, fmt.Errorf("invalid grimoire weights %d", id)
				}
				row.Random = append(row.Random, amplifyValueWeight{Value: int(random[j].Value), Weight: int(random[j+1].Value)})
			}
		}
		if item.StackableType == "[enchant waste]" && len(fields["[monster card id]"]) > 0 {
			v := fields["[monster card id]"][0]
			if v.Type != 0 || v.Value < 0 {
				return nil, fmt.Errorf("invalid bead card %d", id)
			}
			rows, row := appendRuleRow(c.Enchant.Beads)
			c.Enchant.Beads = rows
			row.Template = id
			row.Path = item.Path
			row.Card = uint32(v.Value)
			row.Expires = expires
		}
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
	}
	if err = c.importCosts(a); err != nil {
		return nil, err
	}
	return c, c.Validate()
}

func sourceMatrix(cells []pvf.Token, kind string) ([]pvf.Token, error) {
	var rows []pvf.Token
	inside := false
	for _, t := range cells {
		if t.Type == 3 {
			if t.Text == "[table]" {
				inside = true
				rows = nil
			}
			if t.Text == "[/table]" {
				inside = false
				if len(rows) > 0 && rows[0].Text == kind {
					if (len(rows)-1)%24 != 0 {
						return nil, fmt.Errorf("invalid %s matrix width", kind)
					}
					return rows[1:], nil
				}
			}
			continue
		}
		if inside {
			rows = append(rows, t)
		}
	}
	return nil, fmt.Errorf("missing %s upgrade matrix", kind)
}

func sourceUint(t pvf.Token) (uint32, error) {
	if t.Type != 0 || t.Value < 0 {
		return 0, fmt.Errorf("invalid source integer type=%d value=%d", t.Type, t.Value)
	}
	return uint32(t.Value), nil
}

func sourceFloat(t pvf.Token) (float64, error) {
	if t.Type == 0 {
		return float64(t.Value), nil
	}
	if t.Type == 2 {
		v := float64(math.Float32frombits(uint32(t.Value)))
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("invalid numeric source cell")
}

func uintValues(cells []pvf.Token) ([]uint32, error) {
	var out []uint32
	for _, t := range cells {
		n, err := sourceUint(t)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func floatValues(cells []pvf.Token) ([]float64, error) {
	var out []float64
	for _, t := range cells {
		n, err := sourceFloat(t)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func safeConditions(cells []pvf.Token) ([]int, []string, error) {
	inside := false
	child := ""
	var levels []int
	var rarities []string
	for _, t := range cells {
		if t.Type == 3 {
			if t.Text == "[safe upgrade item condition]" {
				inside = true
				child = ""
			} else if t.Text == "[/safe upgrade item condition]" {
				inside = false
			} else if inside {
				child = t.Text
			}
			continue
		}
		if inside && child == "[level]" {
			v, err := sourceUint(t)
			if err != nil {
				return nil, nil, err
			}
			levels = append(levels, int(v))
		}
		if inside && child == "[usable rarity]" {
			if t.Type != 6 || t.Text == "" {
				return nil, nil, fmt.Errorf("invalid safe upgrade rarity")
			}
			rarities = append(rarities, t.Text)
		}
	}
	return levels, rarities, nil
}

func (c *EnhancementCatalog) importCosts(a *pvf.Archive) error {
	gold, err := a.Tokens("etc/upgrade.etc")
	if err != nil {
		return err
	}
	matrix, err := sourceMatrix(gold, "normal")
	if err != nil {
		return err
	}
	c.Gold.Version = 1
	c.Gold.Source = a.Snapshot().Checksum
	c.Gold.Table = "etc/upgrade.etc"
	for i := 0; i < len(matrix); i += 24 {
		material, err := sourceUint(matrix[i+14])
		if err != nil {
			return err
		}
		count, err := sourceUint(matrix[i+15])
		if err != nil {
			return err
		}
		c.Gold.Levels = append(c.Gold.Levels, goldLevelRule{Level: i / 24, MaterialTemplate: material, MaterialCount: count})
	}
	fields := sourceSections(gold)
	if c.Gold.Gold.BaseByEquipLevel, err = uintValues(fields["[cost]"]); err != nil {
		return err
	}
	if c.Gold.Gold.BaseByEquipLevel100Lv, err = uintValues(fields["[cost 100lv]"]); err != nil {
		return err
	}
	if c.Gold.Gold.RarityWeight, err = floatValues(fields["[cost weights by rarity]"]); err != nil {
		return err
	}
	if c.Gold.Gold.RarityWeight100Lv, err = floatValues(fields["[cost weights by rarity 100lv]"]); err != nil {
		return err
	}
	c.Gold.Gold.Base100LvFromEquipLevel = 100
	// The export takes the first weapon factor; later tables repeat [type].
	if len(fields["[type]"]) == 0 {
		return fmt.Errorf("invalid weapon price factor")
	}
	factor, err := sourceFloat(fields["[type]"][0])
	if err != nil {
		return err
	}
	c.Gold.Gold.WeaponFactor = math.Round(factor*1e6) / 1e6
	weights := fields["[cost weight by upgrade level]"]
	if len(weights)%5 != 0 {
		return fmt.Errorf("incomplete upgrade level weights")
	}
	for i := 0; i < len(weights); i += 5 {
		level, err := sourceUint(weights[i])
		if err != nil {
			return err
		}
		v, err := floatValues(weights[i+1 : i+5])
		if err != nil {
			return err
		}
		rows, row := appendRuleRow(c.Gold.Gold.LevelWeight)
		c.Gold.Gold.LevelWeight = rows
		row.Level = int(level)
		row.Weights = v
	}
	safe, err := uintValues(fields["[safe upgrade]"])
	if err != nil {
		return err
	}
	if len(safe)%8 != 0 {
		return fmt.Errorf("incomplete safe reinforcement row")
	}
	for i := 0; i < len(safe); i += 8 {
		r := safe[i : i+8]
		c.Gold.Failure.SafeUpgrade = append(c.Gold.Failure.SafeUpgrade, goldSafeUpgradeRow{Level: int(r[0]), Enabled: int(r[1]), Gold: r[2], B: int(r[3]), C: int(r[4]), Material: r[5], Count: r[6], Value: r[7]})
	}
	if c.Gold.Failure.SafeUpgradeReplaceItem, err = uintValues(fields["[safe upgrade replace item]"]); err != nil {
		return err
	}
	if c.Gold.Failure.SafeUpgradeMinLevel, c.Gold.Failure.SafeUpgradeUsableRarity, err = safeConditions(gold); err != nil {
		return err
	}
	amp, err := a.Tokens("etc/amplifyupgrade.etc")
	if err != nil {
		return err
	}
	matrix, err = sourceMatrix(amp, "amplify")
	if err != nil {
		return err
	}
	c.Amplify.Version = 1
	c.Amplify.Source = a.Snapshot().Checksum
	for i := 0; i < len(matrix); i += 24 {
		material, err := sourceUint(matrix[i+14])
		if err != nil {
			return err
		}
		count, err := sourceUint(matrix[i+15])
		if err != nil {
			return err
		}
		cols, err := uintValues(matrix[i+8 : i+10])
		if err != nil {
			return err
		}
		if i > 0 && material != c.Amplify.MaterialTemplate {
			return fmt.Errorf("amplify material varies by level")
		}
		c.Amplify.MaterialTemplate = material
		rows, row := appendRuleRow(c.Amplify.Levels)
		c.Amplify.Levels = rows
		row.Level = i / 24
		row.MaterialTemplate = material
		row.MaterialCount = count
		row.GoldColumns = cols
	}
	fields = sourceSections(amp)
	safe, err = uintValues(fields["[safe upgrade]"])
	if err != nil {
		return err
	}
	if len(safe)%8 != 0 {
		return fmt.Errorf("incomplete safe amplify row")
	}
	materials := map[uint32]bool{}
	for i := 0; i < len(safe); i += 8 {
		r := safe[i : i+8]
		rows, row := appendRuleRow(c.Amplify.SafeUpgrade)
		c.Amplify.SafeUpgrade = rows
		row.Level = int(r[0])
		row.Enabled = int(r[1])
		row.Gold = r[7]
		row.Material = r[5]
		row.Count = r[6]
		row.Value = r[7]
		materials[r[5]] = true
	}
	for id := range materials {
		c.Amplify.SafeMaterialTemplates = append(c.Amplify.SafeMaterialTemplates, id)
	}
	sort.Slice(c.Amplify.SafeMaterialTemplates, func(i, j int) bool { return c.Amplify.SafeMaterialTemplates[i] < c.Amplify.SafeMaterialTemplates[j] })
	if c.Amplify.SafeUpgradeMinLevel, c.Amplify.SafeUpgradeUsableRarity, err = safeConditions(amp); err != nil {
		return err
	}
	return nil
}
