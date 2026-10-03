package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnhancementCatalog owns source data until startup activates it. Import and
// audit never mutate the process's live rule maps or player storage.
type EnhancementCatalog struct {
	ReinforcementTickets map[uint32]reinforcementTicket
	AmplifyTickets       map[uint32]reinforcementTicket
	Grimoires            amplifyGrimoireRules
	Enchant              enchantBeadConfig
	Gold                 goldRulesConfig
	Amplify              amplifyUpgradeConfig
}

type enhancementPolicy struct {
	Version       int                  `json:"version"`
	PureTemplates []uint32             `json:"pure_templates"`
	Gold          goldRulesConfig      `json:"gold"`
	Amplify       amplifyUpgradeConfig `json:"amplify"`
}

func readEnhancementJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// ReadEnhancementBaseline is an offline historical-fixture audit helper. It reads
// effective typed fields without activation and is never a runtime source.
// Descriptive JSON metadata unused by the engine is not game policy.
func ReadEnhancementBaseline(dir string) (*EnhancementCatalog, error) {
	c := &EnhancementCatalog{}
	for _, r := range []struct {
		name string
		out  any
	}{
		{"amplify-grimoire.json", &c.Grimoires}, {"enchant-beads.json", &c.Enchant},
		{"reinforcement-gold.json", &c.Gold}, {"amplify-upgrade.json", &c.Amplify},
	} {
		if err := readEnhancementJSON(filepath.Join(dir, r.name), r.out); err != nil {
			return nil, err
		}
	}
	for _, r := range []struct {
		name string
		out  *map[uint32]reinforcementTicket
	}{
		{"reinforcement-tickets.json", &c.ReinforcementTickets}, {"amplify-tickets.json", &c.AmplifyTickets},
	} {
		var doc struct {
			Version      int
			ClientSHA256 string `json:"client_sha256"`
			Items        map[uint32]reinforcementTicket
		}
		if err := readEnhancementJSON(filepath.Join(dir, r.name), &doc); err != nil {
			return nil, err
		}
		if doc.Version != 1 || len(doc.ClientSHA256) != 64 {
			return nil, fmt.Errorf("invalid ticket baseline %s", r.name)
		}
		*r.out = doc.Items
	}
	return c, c.Validate()
}

func (c *EnhancementCatalog) Validate() error {
	if c == nil {
		return fmt.Errorf("missing enhancement catalog")
	}
	for _, group := range []struct {
		rows    map[uint32]reinforcementTicket
		section string
	}{
		{c.ReinforcementTickets, "[equipment reinforcement ticket]"}, {c.AmplifyTickets, amplifyTicketSection},
	} {
		if len(group.rows) == 0 {
			return fmt.Errorf("empty enhancement ticket catalog")
		}
		for id, row := range group.rows {
			if id < 2 || row.Path == "" || len(row.Fields[group.section]) != 4 {
				return fmt.Errorf("invalid enhancement ticket %d", id)
			}
		}
	}
	if c.Grimoires.Version != 1 || len(c.Grimoires.Source) != 64 || len(c.Grimoires.Grimoires) == 0 {
		return fmt.Errorf("invalid grimoire catalog")
	}
	if c.Enchant.Version != 1 || len(c.Enchant.Beads) == 0 {
		return fmt.Errorf("invalid enchant catalog")
	}
	g := c.Gold
	if g.Version != 1 || len(g.Source) != 64 || len(g.Levels) == 0 || len(g.Gold.BaseByEquipLevel) == 0 || len(g.Gold.RarityWeight) == 0 || len(g.Gold.RarityWeight100Lv) == 0 || len(g.Gold.LevelWeight) == 0 || len(g.Materials) == 0 || g.MaxUpgradeLevel <= 0 {
		return fmt.Errorf("invalid reinforcement gold catalog")
	}
	for _, row := range g.Levels {
		if row.Level < 0 || row.MaterialCount == 0 {
			return fmt.Errorf("invalid reinforcement level %d", row.Level)
		}
	}
	if c.Amplify.Version != 1 || len(c.Amplify.Source) != 64 || len(c.Amplify.Levels) == 0 {
		return fmt.Errorf("invalid amplify cost catalog")
	}
	return nil
}

// Activate validates all six families before publishing any global rule map.
// Native protocol limits and transaction behavior remain in their existing
// consumers; this does not enable unsupported ticket levels or item classes.
func (c *EnhancementCatalog) Activate() error {
	if err := c.Validate(); err != nil {
		return err
	}
	for i := range c.Grimoires.Grimoires {
		g := &c.Grimoires.Grimoires[i]
		g.Golden = strings.Contains(strings.ToLower(g.Path), "golden")
		g.Pure = len(g.Random) > 0
		for _, w := range g.Random {
			if w.Value != 0 {
				g.Pure = false
				break
			}
		}
	}
	pure := map[uint32]bool{}
	for _, id := range c.Grimoires.PureTemplates {
		pure[id] = true
	}
	beads := map[uint32]uint32{}
	for _, b := range c.Enchant.Beads {
		if b.Card != 0 {
			beads[b.Template] = b.Card
		}
	}
	reinforcementTickets = c.ReinforcementTickets
	amplifyTickets = c.AmplifyTickets
	amplifyGrimoires = &c.Grimoires
	pureGrimoireTemplates = pure
	enchantBeadCards = beads
	goldRules = &c.Gold
	amplifyUpgradeRules = &c.Amplify
	return nil
}
