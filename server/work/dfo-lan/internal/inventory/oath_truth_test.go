package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"encoding/json"
	"os"
	"testing"
)

func truthRules(t *testing.T) *catalog.OathTruthRules {
	t.Helper()
	raw, err := os.ReadFile("../catalog/testdata/oath_truth.cos")
	if err != nil {
		t.Fatal(err)
	}
	r, err := catalog.ParseOathTruth(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	r.Alternatives = map[uint32][]uint32{10421371: {10421372}}
	return r
}
func truthBag(t *testing.T, seal, ore, gold uint32) json.RawMessage {
	t.Helper()
	items := []BagItem{}
	if seal > 0 {
		items = append(items, BagItem{Slot: 100, Template: 10401346, Amount: seal})
	}
	if ore > 0 {
		items = append(items, BagItem{Slot: 101, Template: 10421371, Amount: ore})
	}
	raw, err := SaveBag(json.RawMessage(`{"level":115,"future":{"unchanged":true},"oath_truth":{"level":0,"options":[0,0,0,0,0,0,0,0,0,0],"future":17}}`), Bag{Version: "ordinary-bag-v1", Gold: gold, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestTruthUpgradeCostsSwitchAndPreservation(t *testing.T) {
	r := truthRules(t)
	raw := truthBag(t, 100, 40, 0)
	vault := AccountVaultState{Items: json.RawMessage("[]")}
	state, _, err := UpgradeOathTruth(raw, vault, r, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ := ReadBag(state)
	truth, _ := ReadOathTruth(state)
	if truth.Level != 1 || truth.Options[1] != 0 || len(bag.Items) != 0 {
		t.Fatal("upgrade did not consume or open first real slot")
	}
	changed, err := SelectOathTruth(state, r, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	truth, _ = ReadOathTruth(changed)
	if truth.Options[1] != 1 || !bytes.Contains(changed, []byte(`"future":17`)) || !bytes.Contains(changed, []byte(`"unchanged":true`)) {
		t.Fatal("selection or unknown fields lost")
	}
	if _, err = SelectOathTruth(changed, r, 2, 1); err == nil {
		t.Fatal("locked slot accepted")
	}
	if _, _, err = UpgradeOathTruth(changed, vault, r, 1, 1); err == nil {
		t.Fatal("duplicate upgrade must not deduct")
	}
	for _, raw := range []json.RawMessage{truthBag(t, 99, 40, 0), truthBag(t, 100, 39, 0), truthBag(t, 100, 40, 499999)} {
		payment := uint32(1)
		if bytes.Contains(raw, []byte("499999")) {
			payment = 2
		}
		before := bytes.Clone(raw)
		if _, _, err = UpgradeOathTruth(raw, vault, r, 1, payment); err == nil || !bytes.Equal(raw, before) {
			t.Fatal("failed fee mutated input or succeeded")
		}
	}
	raw = truthBag(t, 0, 40, 500000)
	state, _, err = UpgradeOathTruth(raw, vault, r, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ = ReadBag(state)
	if bag.Gold != 0 {
		t.Fatal("gold payment")
	}
	// Four upgrades retain earlier choices and charge the current source rows.
	raw = truthBag(t, 1001, 401, 0)
	var fields, inv map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	_ = json.Unmarshal(fields["inventory"], &inv)
	inv["items"] = json.RawMessage(`[{"slot":100,"Template":10401346,"Amount":1001,"unknown_instance":17},{"slot":101,"Template":10421371,"Amount":401,"unknown_instance":18}]`)
	inv["future_inventory"] = json.RawMessage(`{"keep":true}`)
	fields["inventory"], _ = json.Marshal(inv)
	raw, _ = json.Marshal(fields)
	for level := byte(1); level <= 4; level++ {
		raw, _, err = UpgradeOathTruth(raw, vault, r, level, 1)
		if err != nil {
			t.Fatal("stage", level, err)
		}
		if level == 1 {
			raw, err = SelectOathTruth(raw, r, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	truth, _ = ReadOathTruth(raw)
	bag, _ = ReadBag(raw)
	if truth.Level != 4 || truth.Options[1] != 1 || len(bag.Items) != 2 || bag.Items[0].Amount != 1 || bag.Items[1].Amount != 1 || !bytes.Contains(raw, []byte(`"unknown_instance":17`)) || !bytes.Contains(raw, []byte(`"future_inventory":{"keep":true}`)) {
		t.Fatal("four-stage cost or opaque-instance preservation")
	}
	if _, _, err = UpgradeOathTruth(raw, vault, r, 5, 1); err == nil {
		t.Fatal("fifth stage must remain source-locked")
	}
}
func TestTruthAllVaultsAlternativeAndRollback(t *testing.T) {
	r := truthRules(t)
	raw := truthBag(t, 50, 0, 0)
	personal := [2]VaultState{{Slots: 8, Items: json.RawMessage(`[{"slot":0,"template":10401346,"amount":20}]`)}, {Slots: 24, Items: json.RawMessage(`[{"slot":0,"template":10401346,"amount":30}]`)}}
	shared := AccountVaultState{Slots: 8, Items: json.RawMessage(`[{"slot":0,"template":10421372,"amount":41,"instance_future":"keep"},{"slot":1,"template":991,"amount":1,"opaque":99}]`)}
	state, vault, personal, err := UpgradeOathTruthWithVaults(raw, shared, personal, r, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	truth, _ := ReadOathTruth(state)
	if truth.Level != 1 || !bytes.Equal(personal[0].Items, []byte("[]")) || !bytes.Equal(personal[1].Items, []byte("[]")) || !bytes.Contains(vault.Items, []byte(`"amount":1`)) || !bytes.Contains(vault.Items, []byte(`"instance_future":"keep"`)) || !bytes.Contains(vault.Items, []byte(`"opaque":99`)) {
		t.Fatal("warehouse spends or opaque fields")
	}
	shared = AccountVaultState{Gold: 500000, Items: json.RawMessage("[]")}
	state, vault, err = UpgradeOathTruth(truthBag(t, 0, 40, 0), shared, r, 1, 2)
	if err != nil || vault.Gold != 0 {
		t.Fatal("shared gold", err)
	}
	_ = state
}

func TestTruthPaymentUsesOneBasedPVFGroup(t *testing.T) {
	r := truthRules(t)
	vault := AccountVaultState{Items: json.RawMessage("[]")}
	for _, payment := range []uint32{0, 3, ^uint32(0)} {
		raw := truthBag(t, 100, 40, 500000)
		if _, _, err := UpgradeOathTruth(raw, vault, r, 1, payment); err == nil {
			t.Fatal("out-of-source payment accepted", payment)
		}
	}
	// Index 1 is the material scheme even when the wallet could fund index 2.
	state, _, err := UpgradeOathTruth(truthBag(t, 100, 40, 500000), vault, r, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := ReadBag(state)
	if err != nil || bag.Gold != 500000 || len(bag.Items) != 0 {
		t.Fatal("material group changed the wallet", err)
	}
}
