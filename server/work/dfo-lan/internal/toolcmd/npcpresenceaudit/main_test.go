package npcpresenceaudit

import (
	"dfolan/internal/catalog"
	"dfolan/internal/npcpresence"
	"strings"
	"testing"
)

func TestOfflineTraceCannotTreatGapAsEmptySnapshot(t *testing.T) {
	hash := strings.Repeat("a", 64)
	index := &npcpresence.Index{Source: hash, Areas: map[string]catalog.WorldArea{"80/0": {Town: 80, Map: catalog.ScriptRecord{Path: "map/base.map", SHA256: hash}}}}
	phase := int32(-1)
	exists := true
	input := trace{Schema: 1, Source: hash, Operations: []operation{{Kind: "manager_constructed"}, {Kind: "show", NPC: 100}, {Kind: "gap"}}, Queries: []query{{Town: 80, NPC: 100, Phase: &phase, FinalRoot: "map/base.map", Instances: map[uint32]*bool{100: &exists}}}}
	r, err := evaluate(index, input)
	if err != nil || len(r) != 1 || r[0].State != npcpresence.StateUnknown || r[0].Visibility.ShowOverride != npcpresence.Unknown {
		t.Fatalf("gap restored false confidence: %+v %v", r, err)
	}
}

func TestOfflineTraceRejectsUnboundSourceAndUnmodeledOperations(t *testing.T) {
	i := &npcpresence.Index{Source: strings.Repeat("a", 64)}
	if _, err := evaluate(i, trace{Schema: 1, Source: strings.Repeat("b", 64)}); err == nil {
		t.Fatal("unbound trace source accepted")
	}
	if _, err := evaluate(i, trace{Schema: 1, Source: i.Source, Operations: []operation{{Kind: "infer_visibility_from_completed"}}}); err == nil {
		t.Fatal("unmodeled operation ignored")
	}
}
