package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"reflect"
	"testing"
)

func TestJointComplexItemsLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native complex item parity")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: p, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: testPVFCacheDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const policy = "../../configs/pvf-enhancement-policy.json"
	joint, err := s.ItemCatalogs(catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: true, Skins: true, Boosters: true}, true, policy, true)
	if err != nil {
		t.Fatal(err)
	}
	if joint.Basics.ScriptsRead != uint64(len(joint.Basics.Index.Items)) {
		t.Fatal("joint scan read count", joint.Basics.ScriptsRead)
	}
	s.ReleaseReadCaches()
	skins, err := s.SkinStorage()
	if err != nil {
		t.Fatal(err)
	}
	skins.Source = joint.Basics.Skins.Source
	if !reflect.DeepEqual(skins, *joint.Basics.Skins) {
		t.Fatal("joint skin entries/missing skin order/registry labels differ")
	}
	s.ReleaseReadCaches()
	boosters, err := s.Boosters(joint.Basics.Index)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(boosters, joint.Basics.Boosters) {
		t.Fatal("joint reward pool/order/weights/counts/sealed markers differ")
	}
	s.ReleaseReadCaches()
	enhancements, err := s.Enhancements(joint.Basics.Index, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enhancements, joint.Enhancements) {
		t.Fatal("joint tickets/fields/grimoire weights/beads/cost tables/policy paths differ")
	}
	s.ReleaseReadCaches()
	fame, err := s.Fame(joint.Basics.Index)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fame, joint.Fame) {
		t.Fatal("joint fame fields/sets/formulas/exact raw source hashes differ")
	}
	t.Logf("native complex parity: scripts=%d skins=%d missing=%d boosters=%d reinforcement=%d amplify=%d grimoires=%d beads=%d fame=%d hashes=%d", joint.Basics.ScriptsRead, len(skins.Entries), len(skins.MissingSkins), len(boosters), len(enhancements.ReinforcementTickets), len(enhancements.AmplifyTickets), len(enhancements.Grimoires.Grimoires), len(enhancements.Enchant.Beads), len(fame.Items), len(fame.Sources))
}
