package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSelectionBoxPolicyOmittedPreservesDiscoveryAndRejectsMissingOverride(t *testing.T) {
	policy, err := ReadSelectionBoxPolicy("")
	if err != nil || !reflect.DeepEqual(policy, SelectionBoxPolicy{Version: 1}) {
		t.Fatal("omitted policy must discover source scope without a whitelist", policy, err)
	}
	if _, err := ReadSelectionBoxPolicy(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("an explicit missing override must not fall back to source discovery")
	}
}

func TestSelectionBoxPolicyRefusesSourcePayloadAndAmbiguousScope(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"templates":[7,7]}`,
		`{"version":1,"templates":[0]}`,
		`{"version":1,"whitelist":[7],"templates":[7]}`,
		`{"version":1,"templates":[7],"boxes":{}}`,
		`{"version":1,"templates":[7]} {}`,
		`{"version":2}`,
	} {
		p := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSelectionBoxPolicy(p); err == nil {
			t.Fatalf("accepted invalid scope: %s", raw)
		}
	}
}

func TestSelectionBoxPolicyAcceptsDiscoveryOnlyScope(t *testing.T) {
	// No ID list at all is the migrated shape: the source supplies the scope.
	for _, raw := range []string{
		`{"version":1}`,
		`{"version":1,"whitelist":[]}`,
		`{"version":1,"whitelist":[7,8],"provenance":"handed out by code"}`,
		`{"version":1,"templates":[7],"whitelist":[8]}`,
	} {
		p := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadSelectionBoxPolicy(p); err != nil {
			t.Fatalf("refused valid scope %s: %v", raw, err)
		}
	}
}
