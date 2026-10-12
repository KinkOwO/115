package catalog

import (
	"fmt"
	"strings"
	"testing"
)

func TestVenusOperationsTextScopesScalarTypeBeforeRelicBlock(t *testing.T) {
	for _, amount := range []uint32{1, 17} {
		source := fmt.Sprintf("[operation data set]\n[index] 2\n[type] 2\n[member limit] `party`\n[reward data]\n`expert` 123 %d 1000000 1 0\n[/reward data]\n[/operation data set]\n[relic]\n[type]\n1\n[/type]\n[/relic]\n", amount)
		ops, err := parseVenusOperations(source, []byte(source))
		if err != nil {
			t.Fatal(err)
		}
		op := ops.Definitions[2]
		if op.Type != 2 || len(op.Rewards) != 1 || op.Rewards[0].Amount != amount || op.Rewards[0].Difficulty != "expert" || len(ops.SHA256) != 64 {
			t.Fatal(op)
		}
		if _, err := parseVenusOperations(source+source, []byte(source)); err == nil {
			t.Fatal("duplicate source operation accepted")
		}
		if _, err := parseVenusOperations(strings.ReplaceAll(source, "[/operation data set]", ""), nil); err == nil {
			t.Fatal("unclosed operation accepted")
		}
	}
}
