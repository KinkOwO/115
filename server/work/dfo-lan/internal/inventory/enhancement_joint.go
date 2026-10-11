package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

// EnhancementItemImport accumulates only effective rules from the shared STK
// scan. Policy material paths and source cost tables are resolved at Finish.
type EnhancementItemImport struct {
	a *pvf.Archive
	c *EnhancementCatalog
}

func NewEnhancementItemImport(a *pvf.Archive, policyPath string) (*EnhancementItemImport, error) {
	if a == nil {
		return nil, fmt.Errorf("enhancement import requires PVF")
	}
	p, err := readEnhancementPolicy(policyPath)
	if err != nil {
		return nil, err
	}
	c := &EnhancementCatalog{ReinforcementTickets: map[uint32]reinforcementTicket{}, AmplifyTickets: map[uint32]reinforcementTicket{}, Gold: p.Gold, Amplify: p.Amplify}
	c.Grimoires.Version, c.Grimoires.Source = 1, a.Snapshot().Checksum
	c.Grimoires.PureTemplates = p.PureTemplates
	c.Enchant.Version, c.Enchant.Source = 1, a.Snapshot().Checksum
	return &EnhancementItemImport{a: a, c: c}, nil
}

func (b *EnhancementItemImport) Consume(s catalog.ItemScript) error {
	item, id, c := s.Item, s.Item.ID, b.c
	if item.Kind != "stackable" || !strings.HasSuffix(item.Path, ".stk") || !s.Exact {
		return nil
	}
	fields := ticketFields(s.Cells)
	if _, ok := fields["[equipment reinforcement ticket]"]; ok {
		c.ReinforcementTickets[id] = reinforcementTicket{Path: item.Path, Fields: fields}
	}
	if _, ok := fields[amplifyTicketSection]; ok {
		c.AmplifyTickets[id] = reinforcementTicket{Path: item.Path, Fields: fields}
	}
	_, expires := fields["[expiration date]"]
	if random := fields["[amplification random value]"]; len(random) > 0 {
		if len(random)%2 != 0 {
			return fmt.Errorf("incomplete grimoire weights %d", id)
		}
		rows, row := appendRuleRow(c.Grimoires.Grimoires)
		c.Grimoires.Grimoires = rows
		row.Template, row.Path, row.Expires = id, item.Path, expires
		for j := 0; j < len(random); j += 2 {
			if random[j].Type != 0 || random[j+1].Type != 0 {
				return fmt.Errorf("invalid grimoire weights %d", id)
			}
			row.Random = append(row.Random, amplifyValueWeight{Value: int(random[j].Value), Weight: int(random[j+1].Value)})
		}
	}
	if item.StackableType == "[enchant waste]" && len(fields["[monster card id]"]) > 0 {
		v := fields["[monster card id]"][0]
		if v.Type != 0 || v.Value < 0 {
			return fmt.Errorf("invalid bead card %d", id)
		}
		rows, row := appendRuleRow(c.Enchant.Beads)
		c.Enchant.Beads = rows
		row.Template, row.Path, row.Card, row.Expires = id, item.Path, uint32(v.Value), expires
	}
	return nil
}

func (b *EnhancementItemImport) Finish(index catalog.ItemIndex) (*EnhancementCatalog, error) {
	if b.a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("enhancement/PVF source mismatch")
	}
	c := b.c
	for _, rows := range [][]goldMaterialDefinition{c.Gold.Materials, c.Gold.MaterialsPendingSafePath} {
		for i := range rows {
			item, ok := index.Items[rows[i].Template]
			if !ok || item.Kind != "stackable" || item.Path == "" {
				return nil, fmt.Errorf("missing enhancement policy material %d in PVF index", rows[i].Template)
			}
			rows[i].Path = item.Path
		}
	}
	// Native LIST order need not equal template order; the original importer
	// sorted IDs before appending these externally observable rows.
	sort.Slice(c.Grimoires.Grimoires, func(i, j int) bool { return c.Grimoires.Grimoires[i].Template < c.Grimoires.Grimoires[j].Template })
	sort.Slice(c.Enchant.Beads, func(i, j int) bool { return c.Enchant.Beads[i].Template < c.Enchant.Beads[j].Template })
	if err := c.importCosts(b.a); err != nil {
		return nil, err
	}
	return c, c.Validate()
}
