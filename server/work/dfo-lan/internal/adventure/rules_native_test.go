package adventure

import (
	"dfolan/internal/catalog/pvf"
	"errors"
	"testing"
)

func TestInstalledRulesAvoidEmbeddedReadAndKeepSourceImmutable(t *testing.T) {
	baseline, err := EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	source := *baseline
	source.Experience = map[uint32]uint64{}
	for k, v := range baseline.Experience {
		source.Experience[k] = v
	}
	restore, err := InstallRules(&source)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	original := load
	load = func() (*Rules, error) { return nil, errors.New("embedded source unavailable") }
	defer func() { load = original }()
	source.Experience[50]++
	current, err := Current()
	if err != nil || current.Experience[50] != baseline.Experience[50] {
		t.Fatal("native installation fell back or was mutated", err)
	}
	if _, err := InstallRules(nil); err == nil {
		t.Fatal("nil source installed")
	}
	after, err := Current()
	if err != nil || after != current {
		t.Fatal("failed install replaced working rules", err)
	}
}

func TestExperienceNumericStringsPreserve64Bits(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[adventure exp table]"}, {Type: 0, Value: 50}, {Type: 6, Text: "171361456285"}}
	values, err := ruleNumbers(cells, "[adventure exp table]")
	if err != nil || len(values) != 2 || values[1] != 171361456285 {
		t.Fatal(values, err)
	}
	cells[2].Text = "18446744073709551616"
	if _, err := ruleNumbers(cells, "[adventure exp table]"); err == nil {
		t.Fatal("overflow accepted")
	}
}
