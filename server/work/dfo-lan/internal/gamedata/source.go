// Package gamedata selects the source of static game catalogs. It never opens
// storage or rewrites source versions in player saves.
package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

type Mode string

const (
	JSON            Mode  = "json"
	PVF             Mode  = "pvf"
	DefaultMaxBytes int64 = 1024 * 1024 * 1024
)

type Options struct {
	Mode             Mode
	ArchivePath      string
	ExpectedChecksum string
	MaxBytes         int64
}

// Source owns one read-only archive shared by sequential catalog imports.
type Source struct {
	mode    Mode
	archive *pvf.Archive
}

func Open(options Options) (*Source, error) {
	if options.Mode == "" {
		options.Mode = JSON
	}
	if options.Mode != JSON && options.Mode != PVF {
		return nil, fmt.Errorf("unknown catalog source %q; expected json or pvf", options.Mode)
	}
	s := &Source{mode: options.Mode}
	if options.Mode == JSON {
		return s, nil
	}
	if strings.TrimSpace(options.ArchivePath) == "" {
		return nil, fmt.Errorf("PVF source requires an explicit inner archive path")
	}
	checksum := strings.ToLower(options.ExpectedChecksum)
	decoded, err := hex.DecodeString(checksum)
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("PVF source requires the expected SHA256 of the inner archive")
	}
	path, err := filepath.Abs(options.ArchivePath)
	if err != nil {
		return nil, err
	}
	if options.MaxBytes == 0 {
		options.MaxBytes = DefaultMaxBytes
	}
	bundle, err := pvf.Load(pvf.Options{Path: path, MaxBytes: options.MaxBytes})
	if err != nil {
		return nil, fmt.Errorf("open inner PVF: %w", err)
	}
	if actual := bundle.Snapshot().Checksum; actual != checksum {
		return nil, fmt.Errorf("inner PVF source mismatch: got %s expected %s", actual, checksum)
	}
	a, err := pvf.OpenArchive(bundle)
	if err != nil {
		return nil, fmt.Errorf("parse inner PVF: %w", err)
	}
	s.archive = a
	return s, nil
}

func (s *Source) Snapshot() pvf.ArchiveSnapshot {
	if s.archive == nil {
		return pvf.ArchiveSnapshot{}
	}
	return s.archive.Snapshot()
}

// ReleaseReadCaches may be called between import stages. Imported catalogs own
// their decoded values; clearing temporary reads does not alter those catalogs.
func (s *Source) ReleaseReadCaches() {
	if s.archive != nil {
		s.archive.ReleaseReadCaches()
	}
}

func (s *Source) Characters(path string) (catalog.Characters, error) {
	if s.mode == JSON {
		return catalog.LoadCharacters(path)
	}
	return catalog.ImportCharacters(s.archive)
}

func (s *Source) World(path string) (catalog.WorldCatalog, error) {
	if s.mode == JSON {
		return catalog.LoadWorld(path)
	}
	return catalog.ImportWorldRuntime(s.archive)
}

func (s *Source) Quests(path string) (catalog.QuestCatalog, error) {
	if s.mode == JSON {
		return catalog.LoadQuests(path)
	}
	return catalog.ImportQuests(s.archive)
}

func (s *Source) Progression(path string) (catalog.Progression, error) {
	if s.mode == JSON {
		return catalog.LoadProgression(path)
	}
	return catalog.ImportProgression(s.archive)
}

func (s *Source) ItemIndex(path string) (catalog.ItemIndex, error) {
	if s.mode == JSON {
		return catalog.LoadItemIndex(path)
	}
	return catalog.ImportItemIndex(s.archive)
}

func (s *Source) Equipment(index catalog.ItemIndex) (*inventory.FullEquipmentCatalog, error) {
	if s.mode != PVF {
		return nil, fmt.Errorf("lazy PVF equipment requires a PVF source")
	}
	return inventory.OpenPVFEquipmentCatalog(s.archive, index)
}

func (s *Source) Learning(c catalog.Characters) (*character.LearningCatalog, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("learning import requires PVF")
	}
	return character.ImportLearningCatalog(s.archive, c)
}

func (s *Source) ShopPrices(index catalog.ItemIndex) (*catalog.ShopPrices, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("price import requires PVF")
	}
	return catalog.ImportShopPrices(s.archive, index)
}

func (s *Source) ItemMaterials(index catalog.ItemIndex) (*catalog.ItemMaterials, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("material import requires PVF")
	}
	return catalog.ImportItemMaterials(s.archive, index)
}

func (s *Source) Boosters(index catalog.ItemIndex) (map[uint32]catalog.BoosterDefinition, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("booster import requires PVF")
	}
	return catalog.ImportBoosters(s.archive, index)
}

func (s *Source) ItemPeriods() (catalog.ItemPeriodCatalog, error) {
	if s.archive == nil {
		return catalog.ItemPeriodCatalog{}, fmt.Errorf("item periods import requires PVF")
	}
	return catalog.ImportItemPeriods(s.archive)
}

func (s *Source) SkinStorage() (catalog.SkinStorageCatalog, error) {
	if s.archive == nil {
		return catalog.SkinStorageCatalog{}, fmt.Errorf("skin storage import requires PVF")
	}
	return catalog.ImportSkinStorage(s.archive)
}

func (s *Source) EquipmentJournal() (catalog.EquipmentJournalRules, error) {
	if s.archive == nil {
		return catalog.EquipmentJournalRules{}, fmt.Errorf("journal import requires PVF")
	}
	return catalog.ImportEquipmentJournalRules(s.archive)
}

func (s *Source) EquipmentCreateCost() (catalog.EquipmentCreateCost, error) {
	if s.archive == nil {
		return catalog.EquipmentCreateCost{}, fmt.Errorf("create cost import requires PVF")
	}
	return catalog.ImportEquipmentCreateCost(s.archive)
}

func (s *Source) Tutorials() (catalog.TutorialCatalog, error) {
	if s.archive == nil {
		return catalog.TutorialCatalog{}, fmt.Errorf("tutorial import requires PVF")
	}
	return catalog.ImportTutorials(s.archive)
}
