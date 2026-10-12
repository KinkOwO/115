package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func TestDuskyFormulaOperandPreservedWithoutIntegerQuantization(t *testing.T) {
	for _, kind := range []uint32{4, 7} {
		formula := []pvf.Token{{Type: 6, Text: "source_variable"}, {Type: 2, Number: 1.5}}
		operation := NativeLegionOperation{Type: kind, SelectPool: 1, Probability: 1, MemberLimit: "party",
			Fields: map[string][]pvf.Token{"[operation formula]": formula},
			Ranges: []NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 1, Maximum: 2, Step: 0}}}
		operations := NativeLegionOperations{Modes: map[string][]uint16{"normal": {21}}, Definitions: map[uint16]NativeLegionOperation{21: operation}}
		calls := 0
		card, err := operations.DrawDuskyCard("normal", "party", "normal", 1, func(int64) (int64, error) { calls++; return 0, nil })
		if err != nil || !card.FormulaBound || card.Parameter != 0 || calls != 1 {
			t.Fatalf("formula selected an integer range: %+v %v calls=%d", card, err, calls)
		}
		if !reflect.DeepEqual(operations.Definitions[21].Fields["[operation formula]"], formula) {
			t.Fatal("mutated authored floating-point formula")
		}
		if _, err = operations.DrawCard("normal", "party", "normal", 1, func(int64) (int64, error) { return 0, nil }); err == nil {
			t.Fatal("Dusky exception leaked to another content")
		}
		operation.Fields = map[string][]pvf.Token{"[operation formula]": {{Type: 6, Text: "missing_operand"}}}
		operations.Definitions[21] = operation
		if _, err = operations.DrawDuskyCard("normal", "party", "normal", 1, func(int64) (int64, error) { return 0, nil }); err == nil {
			t.Fatal("malformed formula accepted")
		}
	}
}
