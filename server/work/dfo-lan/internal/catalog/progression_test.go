package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestExperienceThresholdsAgainstCurrentNativeReader(t *testing.T) {
	c, e := LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile("testdata/native_experience_thresholds.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct{ Thresholds []uint64 }
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(c.Thresholds, f.Thresholds) || c.Thresholds[149] <= 1<<32 {
		t.Fatal("current64-bit thresholds truncated or reordered")
	}
	for _, cells := range [][]pvf.Token{nil, {{Type: 9, Value: 1}}, {{Type: 0, Value: -1}}, {{Type: 0, Value: 5}, {Type: 0, Value: 4}}, {{Type: 9}, {Type: 0, Value: 3}}} {
		if _, e = ExperienceThresholds(cells); e == nil {
			t.Fatal("invalid table accepted")
		}
	}
	if c.Scripts["etc/serverparameter.etc"].Path != "etc/(r)serverparameter.etc" {
		t.Fatal("native source path fallback ignored")
	}
}
