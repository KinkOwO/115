package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestElitePilotProfileAcceptsOnlyExplicitAttunementScope(t *testing.T) {
	for _, value := range []string{`"attunement"`, `"all"`, `"1"`, `true`, `1`, `null`} {
		t.Run(value, func(t *testing.T) {
			module := t.TempDir()
			path := filepath.Join(module, "profile.json")
			if err := os.WriteFile(path, []byte(`{"binary":"bin/x.exe","environment":{"DFO_ELITE_COMPANION_PILOT":`+value+`}}`), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := LoadProfile(path, module)
			if value != `"attunement"` && value != `"all"` {
				if err == nil {
					t.Fatal("accepted broader or malformed pilot policy")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Env["DFO_ELITE_COMPANION_PILOT"] != strings.Trim(value, `"`) {
				t.Fatal("lost explicit pilot scope")
			}
		})
	}
}
