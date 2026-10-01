package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestScriptWarpRecordSourceSignedFields(t *testing.T) {
	r := [18]byte{1, 0, 0, 0, 5, 5, 210, 0, 240, 0, 255, 255}
	if err := validateScriptWarpRecord(r, []int32{210, 240, -1}, []int32{0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	for i := range r {
		bad := r
		bad[i]++
		if err := validateScriptWarpRecord(bad, []int32{210, 240, -1}, []int32{0, 0, 0}); err == nil {
			t.Fatalf("altered record byte %d accepted", i)
		}
	}
	if err := validateScriptWarpRecord(r, []int32{65536, 240, -1}, []int32{0, 0, 0}); err == nil {
		t.Fatal("source coordinate overflow accepted")
	}
	if _, err := scriptWarpGrid([]pvf.Token{{Type: 0, Value: -1}, {Type: 0, Value: 0}}); err == nil {
		t.Fatal("negative grid accepted")
	}
}
