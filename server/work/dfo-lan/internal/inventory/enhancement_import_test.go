package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnhancementTicketProjectionRetainsRepeatedFieldsAndNativeValues(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[name]"}, {Type: 8, Value: 123, Reference: "name/key"}, {Type: 3, Text: "[condition]"}, {Type: 0, Value: -1}, {Type: 3, Text: "[name]"}, {Type: 6, Value: 456, Text: "literal"}, {Type: 3, Text: "[/condition]"}}
	f := ticketFields(cells)
	if !reflect.DeepEqual(f["[name]"], []pvf.Token{{Type: 8, Text: "name/key"}, {Type: 6, Text: "literal"}}) || f["[condition]"][0].Value != -1 {
		t.Fatal(f)
	}
	if _, ok := f["[/condition]"]; !ok {
		t.Fatal("legacy section marker lost")
	}
}

func TestEnhancementPolicyRejectsPVFFieldsAndPreservesCurrentPolicies(t *testing.T) {
	p, err := readEnhancementPolicy("../../configs/pvf-enhancement-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Gold.MaxUpgradeLevel != 15 || !p.Gold.Failure.DestroyEnabled || p.Amplify.Official.FailurePenalty.TenPlus != "destroy" {
		t.Fatal("confirmed policy changed")
	}
	b, err := os.ReadFile("../../configs/pvf-enhancement-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["gold"].(map[string]any)["levels"] = []any{}
	b, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bad-policy.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readEnhancementPolicy(path); err == nil {
		t.Fatal("PVF cost data allowed in policy file")
	}
}

func TestEnhancementActivationFailsBeforePublishingAnyRule(t *testing.T) {
	previous := reinforcementTickets
	defer func() { reinforcementTickets = previous }()
	reinforcementTickets = map[uint32]reinforcementTicket{2: {Path: "sentinel"}}
	c := &EnhancementCatalog{}
	if err := c.Activate(); err == nil {
		t.Fatal("invalid catalog activated")
	}
	if reinforcementTickets[2].Path != "sentinel" {
		t.Fatal("partial rule publication")
	}
}

func TestEnhancementMatrixRefusesTruncatedSource(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[table]"}, {Type: 6, Text: "normal"}, {Type: 0, Value: 1}, {Type: 3, Text: "[/table]"}}
	if _, err := sourceMatrix(cells, "normal"); err == nil {
		t.Fatal("partial matrix accepted")
	}
	if _, err := sourceUint(pvf.Token{Type: 6, Text: "unknown"}); err == nil {
		t.Fatal("text used as cost")
	}
}
