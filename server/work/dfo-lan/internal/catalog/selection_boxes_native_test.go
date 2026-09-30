package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionBoxPolicyRefusesSourcePayloadAndAmbiguousScope(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"templates":[7,7]}`,
		`{"version":1,"templates":[0]}`,
		`{"version":1,"templates":[7],"boxes":{}}`,
		`{"version":1,"templates":[7]} {}`,
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
