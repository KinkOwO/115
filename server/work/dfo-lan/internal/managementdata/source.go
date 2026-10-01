// Package managementdata prepares the same native catalogs used by the gateway
// for local administration commands. It has no storage or process side effects.
package managementdata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

type Flags struct {
	Mode, ArchivePath, Checksum, DropPolicy, CharacterPolicy string
	CheckOnly                                                bool
}

func Register(fs *flag.FlagSet) *Flags {
	f := new(Flags)
	fs.StringVar(&f.Mode, "catalog-source", "json", "static catalog source: json or pvf")
	fs.StringVar(&f.ArchivePath, "pvf-archive", "", "explicit inner PVF path")
	fs.StringVar(&f.Checksum, "pvf-source-checksum", "", "exact inner PVF SHA256; must match player save source")
	fs.StringVar(&f.DropPolicy, "pvf-drop-policy", "configs/pvf-drop-policy.json", "existing source-free drop/equipment selection policy")
	fs.StringVar(&f.CharacterPolicy, "pvf-character-policy", "configs/pvf-character-policy.json", "existing initial shortcut and command policy")
	fs.BoolVar(&f.CheckOnly, "check-catalogs", false, "prepare catalogs and exit before accessing storage; no character ID required")
	return f
}

// Open returns nil for the legacy JSON path. PVF requires a caller-supplied,
// validated archive checksum; it never infers or substitutes a save identity.
func (f Flags) Open() (*gamedata.Source, error) {
	if f.Mode == "json" {
		return nil, nil
	}
	return gamedata.Open(gamedata.Options{Mode: gamedata.Mode(f.Mode), ArchivePath: f.ArchivePath, ExpectedChecksum: f.Checksum})
}

func ReadPolicy(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("policy %s has trailing data", path)
	}
	return nil
}

func Characters(s *gamedata.Source, policyPath string) (catalog.Characters, error) {
	var policy catalog.CharacterRuntimePolicy
	if err := ReadPolicy(policyPath, &policy); err != nil {
		return catalog.Characters{}, err
	}
	raw, err := s.Characters("")
	if err != nil {
		return raw, err
	}
	out, err := catalog.ProjectCharacterRuntime(raw, policy)
	s.ReleaseReadCaches()
	return out, err
}

// Equipment retains the gateway's base/quest selection and attaches the same
// full native LIST resolver for grant templates outside that selection.
func Equipment(s *gamedata.Source, index catalog.ItemIndex, policy inventory.DropPolicy) (*inventory.EquipmentCatalog, error) {
	quests, err := s.Quests("")
	if err != nil {
		return nil, err
	}
	gear, err := s.EquipmentSelection(index, quests, policy)
	if err != nil {
		return nil, err
	}
	gear.Full, err = s.Equipment(index)
	s.ReleaseReadCaches()
	return gear, err
}

func Awarder(s *gamedata.Source, dropPolicyPath, bagRulesPath string) (*inventory.Awarder, error) {
	policy, err := inventory.ReadDropPolicy(dropPolicyPath)
	if err != nil {
		return nil, err
	}
	index, err := s.ItemIndex("")
	if err != nil {
		return nil, err
	}
	s.ReleaseReadCaches()
	loot, err := s.Loot(policy.MaximumLootGrade)
	if err != nil {
		return nil, err
	}
	for _, id := range policy.ExcludedLootIDs {
		delete(loot.Items, id)
	}
	if err := loot.SupplementItemIndex(index); err != nil {
		return nil, err
	}
	s.ReleaseReadCaches()
	gear, err := Equipment(s, index, policy)
	if err != nil {
		return nil, err
	}
	rules, err := inventory.LoadBagRules(bagRulesPath, s.Snapshot().Checksum)
	if err != nil {
		gear.Full.Close()
		return nil, err
	}
	return &inventory.Awarder{Catalog: loot, Rules: rules, Equipment: gear}, nil
}

func Report(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
