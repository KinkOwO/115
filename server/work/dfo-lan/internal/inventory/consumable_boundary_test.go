package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func consumableBoundaryService() *ItemService {
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	return &ItemService{Catalog: catalog.LootCatalog{
		Source: source,
		Items: map[uint32]catalog.LootItem{
			700: {ID: 700, Kind: "stackable", StackableType: "[etc]", StackLimit: 100},
			701: {ID: 701, Kind: "stackable", StackableType: "[etc]", StackLimit: 100},
		},
	}}
}

func consumableBoundaryState(t *testing.T, items []BagItem) json.RawMessage {
	t.Helper()
	state, err := SaveBag(json.RawMessage(`{"opaque":{"future":17}}`), Bag{
		Version: "ordinary-bag-v1",
		Items:   items,
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestPrepareConsumeKeepsIdentityAndUnrelatedState(t *testing.T) {
	s := consumableBoundaryService()
	source := s.Catalog.Source.SaveIdentity()
	input := consumableBoundaryState(t, []BagItem{
		{Slot: 65, Template: 700, Amount: 3},
		{Slot: 66, Template: 701, Amount: 8},
	})
	original := append(json.RawMessage(nil), input...)
	role := Role{ConfigVersion: source, State: input}
	request := protocol.UseStackableRequest{Slot: 65, Template: 700, Instance: 1234}

	key, err := s.ConsumeKey(role, request)
	if err != nil || key != "consume:65:700:1234" {
		t.Fatalf("consume event key = %q, %v", key, err)
	}
	updated, receiptJSON, premiums, err := s.PrepareConsume(role, request, false,
		func(state json.RawMessage, _ uint32, _ time.Time) (json.RawMessage, uint32, error) {
			return state, 0, nil
		}, func(uint32) (PremiumActivation, bool) { return PremiumActivation{}, false })
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("prepare mutated the caller's state")
	}
	var receipt ConsumeReceipt
	if err := json.Unmarshal(receiptJSON, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Source != source || receipt.Slot != 65 || receipt.Template != 700 || receipt.Remaining != 2 || len(premiums) != 0 {
		t.Fatalf("unexpected consume receipt: %+v premiums=%+v", receipt, premiums)
	}
	bag, err := ReadBag(updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(bag.Items) != 2 || bag.Items[0].Amount != 2 || bag.Items[1] != (BagItem{Slot: 66, Template: 701, Amount: 8}) {
		t.Fatalf("consume changed unrelated bag rows: %+v", bag.Items)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(updated, &root); err != nil || string(root["opaque"]) != `{"future":17}` {
		t.Fatalf("unknown state was not preserved: opaque=%s err=%v", root["opaque"], err)
	}

	_, _, _, err = s.PrepareConsume(role, protocol.UseStackableRequest{Slot: 99, Template: 700}, false,
		func(state json.RawMessage, _ uint32, _ time.Time) (json.RawMessage, uint32, error) {
			return state, 0, nil
		},
		func(uint32) (PremiumActivation, bool) { return PremiumActivation{}, false })
	if err == nil || !reflect.DeepEqual(input, original) {
		t.Fatal("failed consume accepted or mutated the caller's state")
	}
}

func TestPrepareConsumeReturnsContractActivationWithoutChangingReceiptIdentity(t *testing.T) {
	s := consumableBoundaryService()
	source := s.Catalog.Source.SaveIdentity()
	request := protocol.UseStackableRequest{Slot: 65, Template: 700, Instance: 91}
	state := consumableBoundaryState(t, []BagItem{{Slot: 65, Template: 700, Amount: 2, ExpireTime: protocol.MaxItemPeriod}})
	role := Role{ConfigVersion: source, State: state}
	updated, receiptJSON, premiums, err := s.PrepareConsume(role, request, false,
		func(state json.RawMessage, _ uint32, _ time.Time) (json.RawMessage, uint32, error) {
			return state, 0, nil
		},
		func(template uint32) (PremiumActivation, bool) {
			if template == 700 {
				return PremiumActivation{Type: 3, DurationSecond: 3600}, true
			}
			return PremiumActivation{}, false
		})
	if err != nil {
		t.Fatal(err)
	}
	var receipt ConsumeReceipt
	if err := json.Unmarshal(receiptJSON, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Source != source || receipt.Template != request.Template || receipt.Slot != request.Slot || len(premiums) != 1 || premiums[0] != (PremiumActivation{Type: 3, DurationSecond: 3600}) {
		t.Fatalf("contract consumption identity/activation changed: receipt=%+v premiums=%+v", receipt, premiums)
	}
	bag, err := ReadBag(updated)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 1 {
		t.Fatalf("contract item was not consumed once: bag=%+v err=%v", bag.Items, err)
	}
}

func TestPrepareBoxOpenPersistsDrawPointsAndPreservesOtherState(t *testing.T) {
	s := consumableBoundaryService()
	source := s.Catalog.Source.SaveIdentity()
	mainPrize, sectionPrize := uint32(701), uint32(702)
	s.Catalog.Items[sectionPrize] = catalog.LootItem{ID: sectionPrize, Kind: "stackable", StackableType: "[etc]", StackLimit: 100}
	slots := [2]uint16{65, 120}
	boxes, err := NewBoxCatalog(BoxCatalog{
		Source: "fixture",
		Tables: map[string]BoxTable{"9000": {
			Rate: 100, MainGroup: 1,
			Groups:      map[string][]BoxEntry{"1": {{Group: 1, Template: mainPrize, Count: 2, Weight: 100}}},
			PointStacks: []BoxPointStack{{Type: "section", Gain: 1, Max: 5, SectionReward: []BoxSectionReward{{Group: 1, Threshold: 1, Template: sectionPrize, Count: 1}}}},
		}},
		Rewards: map[string]BoxReward{
			"701": {StackableType: "[etc]", StackLimit: 100, Slots: &slots},
			"702": {StackableType: "[etc]", StackLimit: 100, Slots: &slots},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Boxes = boxes
	state := consumableBoundaryState(t, []BagItem{
		{Slot: 65, Template: 9000, Amount: 2},
		{Slot: 66, Template: 701, Amount: 5},
	})
	state = append(state[:len(state)-1], []byte(`,"box_points":{"9000":{"0":0},"9999":{"opens":7}}}`)...)
	original := append(json.RawMessage(nil), state...)
	role := Role{ConfigVersion: source, State: state}
	plan, err := s.PlanBoxOpen(role, 9000, 1)
	if err != nil || plan.Opens != 0 || !reflect.DeepEqual(state, original) {
		t.Fatalf("box planning changed state or opens: plan=%+v err=%v", plan, err)
	}
	updated, receiptJSON, premiums, err := s.PrepareBoxOpen(role, 9000, 1, plan, func(uint32) (PremiumActivation, bool) {
		return PremiumActivation{}, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, original) {
		t.Fatal("box prepare mutated the caller's state")
	}
	var receipt BoxOpenReceipt
	if err := json.Unmarshal(receiptJSON, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Box != 9000 || receipt.Opened != 1 || receipt.Source != source || len(receipt.Results) != 1 || receipt.Results[0] != (ConsumeGrant{Template: mainPrize, Count: 2, Group: 1}) || len(receipt.Milestones) != 1 || len(premiums) != 0 {
		t.Fatalf("unexpected box receipt: %+v premiums=%+v", receipt, premiums)
	}
	bag, err := ReadBag(updated)
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[uint32]uint32{}
	for _, row := range bag.Items {
		amounts[row.Template] += row.Amount
	}
	if amounts[9000] != 1 || amounts[701] != 7 || amounts[702] != 1 {
		t.Fatalf("open did not consume/grant the expected amounts: %+v", amounts)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(updated, &root); err != nil || string(root["opaque"]) != `{"future":17}` {
		t.Fatalf("unknown state was not preserved: opaque=%s err=%v", root["opaque"], err)
	}
	var boxPoints map[string]map[string]uint32
	if err := json.Unmarshal(root["box_points"], &boxPoints); err != nil {
		t.Fatal(err)
	}
	if boxPoints["9000"]["0"] != 1 || boxPoints["9000"]["opens"] != 1 || boxPoints["9999"]["opens"] != 7 {
		t.Fatalf("box counters were not committed independently: %+v", boxPoints)
	}
	nextPlan, err := s.PlanBoxOpen(Role{ConfigVersion: source, State: updated}, 9000, 1)
	if err != nil || nextPlan.Opens != 1 {
		t.Fatalf("next open did not observe the committed event counter: plan=%+v err=%v", nextPlan, err)
	}
	_, _, _, err = s.PrepareBoxOpen(Role{ConfigVersion: source, State: state}, 9000, 3, plan, func(uint32) (PremiumActivation, bool) {
		return PremiumActivation{}, false
	})
	if err == nil || !reflect.DeepEqual(state, original) {
		t.Fatal("failed box open accepted or mutated the caller's state")
	}
}
