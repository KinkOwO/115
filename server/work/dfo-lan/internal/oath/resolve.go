package oath

import (
	"errors"
	"fmt"
	"sort"
)

// Effect is a content-defined runtime contribution of an oath option or
// crystal. The resolver never invents effect keys or operations.
type Effect struct {
	EffectKey         string `json:"effect_key"`
	SourceInstanceKey string `json:"source_instance_key"`
	Target            string `json:"target"`
	Operation         string `json:"operation"`
	Value             int64  `json:"value"`
	Conditions        string `json:"conditions,omitempty"`
}

type OptionDefinition struct {
	ID      int      `json:"id"`
	Effects []Effect `json:"effects"`
}

type CoreDefinition struct {
	ID            string             `json:"id"`
	Slot          uint16             `json:"slot"`
	UnlockLevel   int                `json:"unlock_level"`
	DefaultOption int                `json:"default_option"`
	Options       []OptionDefinition `json:"options"`
}

type CrystalDefinition struct {
	ID           string   `json:"id"`
	AllowedSlots []uint16 `json:"allowed_slots"`
	Effects      []Effect `json:"effects"`
}

// Catalog is immutable content for one client/PVF version.
type Catalog struct {
	Version     string                       `json:"version"`
	UnlockLevel int                          `json:"unlock_level"`
	Cores       map[string]CoreDefinition    `json:"cores"`
	Crystals    map[string]CrystalDefinition `json:"crystals"`
}

type CoreInstance struct {
	InstanceKey  string `json:"instance_key"`
	DefinitionID string `json:"definition_id"`
	Slot         uint16 `json:"slot"`
}

type CrystalInstance struct {
	InstanceKey  string `json:"instance_key"`
	DefinitionID string `json:"definition_id"`
	Slot         uint16 `json:"slot"`
}

type BoundOption struct {
	CoreInstanceKey string `json:"core_instance_key"`
	SelectedOption  int    `json:"selected_option"`
	Revision        int64  `json:"revision"`
}

type CharacterSnapshot struct {
	CharacterKey string            `json:"character_key"`
	Level        int               `json:"level"`
	Revision     int64             `json:"revision"`
	Core         *CoreInstance     `json:"core,omitempty"`
	Crystals     []CrystalInstance `json:"crystals,omitempty"`
	BoundOption  *BoundOption      `json:"bound_option,omitempty"`
}

type RuntimeSnapshot struct {
	CharacterKey           string   `json:"character_key"`
	SnapshotRevision       int64    `json:"snapshot_revision"`
	Unlocked               bool     `json:"unlocked"`
	CoreInstanceKey        string   `json:"core_instance_key,omitempty"`
	SelectedOption         *int     `json:"selected_option,omitempty"`
	ActiveCrystalInstances []string `json:"active_crystal_instances,omitempty"`
	ResolvedEffects        []Effect `json:"resolved_effects,omitempty"`
}

var (
	ErrInvalidCatalog   = errors.New("invalid oath catalog")
	ErrInvalidSnapshot  = errors.New("invalid oath snapshot")
	ErrInvalidSelection = errors.New("invalid oath selection")
)

// Resolve produces an absolute, deterministic runtime snapshot. It is
// side-effect free so the same state can feed combat and a verified protocol
// adapter without creating a second source of truth.
func Resolve(snapshot CharacterSnapshot, catalog Catalog) (RuntimeSnapshot, error) {
	out := RuntimeSnapshot{CharacterKey: snapshot.CharacterKey, SnapshotRevision: snapshot.Revision}
	if catalog.Version == "" || catalog.UnlockLevel < 0 || catalog.Cores == nil || catalog.Crystals == nil {
		return out, ErrInvalidCatalog
	}
	if snapshot.CharacterKey == "" || snapshot.Revision <= 0 {
		return out, ErrInvalidSnapshot
	}
	if snapshot.Core == nil {
		out.Unlocked = snapshot.Level >= catalog.UnlockLevel
		return out, nil
	}
	core := *snapshot.Core
	if core.InstanceKey == "" || core.DefinitionID == "" || core.Slot == 0 {
		return out, ErrInvalidSnapshot
	}
	def, ok := catalog.Cores[core.DefinitionID]
	if !ok || def.ID != core.DefinitionID || def.Slot != core.Slot || len(def.Options) == 0 {
		return out, fmt.Errorf("%w: core %q", ErrInvalidSnapshot, core.DefinitionID)
	}
	unlockLevel := catalog.UnlockLevel
	if def.UnlockLevel > unlockLevel {
		unlockLevel = def.UnlockLevel
	}
	out.Unlocked = snapshot.Level >= unlockLevel
	out.CoreInstanceKey = core.InstanceKey
	if !out.Unlocked {
		return out, nil
	}

	option, err := optionDefinition(def, def.DefaultOption)
	if err != nil {
		return out, err
	}
	selected := def.DefaultOption
	if snapshot.BoundOption != nil {
		if snapshot.BoundOption.CoreInstanceKey != core.InstanceKey || snapshot.BoundOption.Revision <= 0 {
			return out, ErrInvalidSelection
		}
		selected = snapshot.BoundOption.SelectedOption
		option, err = optionDefinition(def, selected)
		if err != nil {
			return out, err
		}
	}
	out.SelectedOption = &selected

	effects := make([]Effect, 0, len(option.Effects))
	for _, effect := range option.Effects {
		if effect.EffectKey == "" || effect.Target == "" || effect.Operation == "" {
			return out, fmt.Errorf("%w: option effect is incomplete", ErrInvalidCatalog)
		}
		effect.SourceInstanceKey = core.InstanceKey
		effects = append(effects, effect)
	}
	seenSlots := map[uint16]bool{core.Slot: true}
	seenInstances := map[string]bool{core.InstanceKey: true}
	for _, crystal := range snapshot.Crystals {
		if crystal.InstanceKey == "" || crystal.DefinitionID == "" || crystal.Slot == 0 {
			return out, ErrInvalidSnapshot
		}
		if seenSlots[crystal.Slot] || seenInstances[crystal.InstanceKey] {
			return out, fmt.Errorf("%w: duplicate crystal identity or slot", ErrInvalidSnapshot)
		}
		crystalDef, ok := catalog.Crystals[crystal.DefinitionID]
		if !ok || crystalDef.ID != crystal.DefinitionID {
			return out, fmt.Errorf("%w: crystal %q", ErrInvalidSnapshot, crystal.DefinitionID)
		}
		if !allowedSlot(crystalDef.AllowedSlots, crystal.Slot) {
			return out, fmt.Errorf("%w: crystal %q at slot %d", ErrInvalidSnapshot, crystal.DefinitionID, crystal.Slot)
		}
		seenSlots[crystal.Slot] = true
		seenInstances[crystal.InstanceKey] = true
		out.ActiveCrystalInstances = append(out.ActiveCrystalInstances, crystal.InstanceKey)
		for _, effect := range crystalDef.Effects {
			if effect.EffectKey == "" || effect.Target == "" || effect.Operation == "" {
				return out, fmt.Errorf("%w: crystal effect is incomplete", ErrInvalidCatalog)
			}
			effect.SourceInstanceKey = crystal.InstanceKey
			effects = append(effects, effect)
		}
	}
	sort.Strings(out.ActiveCrystalInstances)
	sort.SliceStable(effects, func(i, j int) bool {
		a, b := effects[i], effects[j]
		if a.EffectKey != b.EffectKey {
			return a.EffectKey < b.EffectKey
		}
		if a.SourceInstanceKey != b.SourceInstanceKey {
			return a.SourceInstanceKey < b.SourceInstanceKey
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.Operation != b.Operation {
			return a.Operation < b.Operation
		}
		if a.Value != b.Value {
			return a.Value < b.Value
		}
		return a.Conditions < b.Conditions
	})
	out.ResolvedEffects = effects
	return out, nil
}

func optionDefinition(def CoreDefinition, id int) (OptionDefinition, error) {
	for _, option := range def.Options {
		if option.ID == id {
			return option, nil
		}
	}
	return OptionDefinition{}, fmt.Errorf("%w: option %d", ErrInvalidSelection, id)
}

func allowedSlot(slots []uint16, slot uint16) bool {
	for _, allowed := range slots {
		if allowed == slot {
			return true
		}
	}
	return false
}
