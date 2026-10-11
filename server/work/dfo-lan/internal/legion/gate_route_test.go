package legion

import (
	"reflect"
	"testing"
)

func TestFixedGateRouteEndsAtSourceTarget(t *testing.T) {
	unreferenced, err := CompileGateRules([]int64{900, 0, 0, 0, 0}, []int64{1, 2, 3, -1, -1})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := unreferenced.LinearRoute(); err == nil {
		t.Fatal("unreferenced gateflow authorized route")
	}
	for _, flow := range [][]int64{{1, 2, 3, -1, -1}, {1, 2, 4, -1, -1}} {
		rules, err := CompileGateRules([]int64{900, 1, 0, 0, 0}, flow)
		if err != nil {
			t.Fatal(err)
		}
		route, linear, err := rules.LinearRoute()
		if err != nil || !linear || !reflect.DeepEqual(route, []byte{1, 2, byte(flow[2])}) {
			t.Fatal(route, linear, err)
		}
	}
	rules, err := CompileGateRules([]int64{300, 1, 2, 0, 0}, []int64{1, 3, 4, 5, -1, 2, 4, 5, -1, -1})
	if err != nil {
		t.Fatal(err)
	}
	if route, linear, err := rules.LinearRoute(); err != nil || linear || route != nil {
		t.Fatal("branching route inferred from phase clock")
	}
	for _, flow := range [][]int64{{1, 2, 2, -1, -1}, {1, 2, -1, 3, -1}, {1, 0, 3, -1, -1}} {
		rules, err := CompileGateRules([]int64{900, 1, 0, 0, 0}, flow)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = rules.LinearRoute(); err == nil {
			t.Fatal("ambiguous linear flow accepted")
		}
	}
}

func TestStableMainRouteUsesPrimarySourceFlowNotClock(t *testing.T) {
	for _, end := range []int64{4, 5} {
		rules, err := CompileGateRules([]int64{300, 3, 1, 0, 0, 180, 3, 0, 0, 0}, []int64{1, 3, end, -1, -1, 3, 2, 3, end, -1})
		if err != nil {
			t.Fatal(err)
		}
		path, ok, err := rules.StableMainRoute(1)
		if err != nil || !ok || !reflect.DeepEqual(path, []byte{1, 2, 3, byte(end)}) {
			t.Fatal(path, ok, err)
		}
		rules.Schedule[1].FlowKeys[0] = 1
		if _, ok, _ := rules.StableMainRoute(1); ok {
			t.Fatal("changing primary gate treated as fixed")
		}
	}
}
