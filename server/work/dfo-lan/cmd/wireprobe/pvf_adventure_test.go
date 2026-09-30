package main

import (
	"dfolan/internal/adventure"
	"os"
	"testing"
)

func TestPVFAdventureLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete adventure source parity")
	}
	c, err := preparePVFCoreCatalogs("adventure", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.installAdventureRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	actual, err := adventure.Current()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.adventureRules, actual); err != nil {
		t.Fatal(err)
	}
	if actual.MaxLevel != 50 || actual.Experience[50] != 171361456285 || actual.Experience[60] != 369956531543 || len(actual.Shops) != 3 {
		t.Fatal("adventure source scope changed")
	}
	t.Log("native experience levels", len(actual.Experience), "shops", len(actual.Shops), "unique items", len(actual.Items))
}

func TestAdventureAuditProvenanceAllowanceIsNarrow(t *testing.T) {
	old, err := adventure.EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	direct := *old
	direct.SourceChecksum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	if err := auditAdventureRules(old, &direct); err != nil {
		t.Fatal(err)
	}
	if old.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" {
		t.Fatal("audit rewrote embedded provenance")
	}
	unknown := *old
	unknown.SourceChecksum = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if auditAdventureRules(&unknown, &direct) == nil {
		t.Fatal("unknown provenance accepted")
	}
	direct.Sources = map[string]string{"etc/adventurersystem/adventurersystem2018.etc": "wrong-source-hash"}
	if auditAdventureRules(old, &direct) == nil {
		t.Fatal("changed source hash suppressed")
	}
}
