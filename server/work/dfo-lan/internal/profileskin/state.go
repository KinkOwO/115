// Package profileskin owns the category-0 profile decoration state. It does
// not model avatar equipment or the unrelated NOTI1759 favorite-character UI.
package profileskin

import "fmt"

type Owned struct {
	ID      uint32 `json:"id"`
	Expires uint32 `json:"expires"`
}

type State struct {
	Version  int       `json:"version"`
	Owned    []Owned   `json:"owned"`
	Selected [3]uint32 `json:"selected"`
}

// Defaults are the public built-ins from the current Skin.lst: PartyFrame,
// PartyRequestFrame and CharacterInfoBG/default.skn. These are grants on first
// initialization only; login must subsequently read the persisted selection.
func Defaults() State {
	return State{Version: 1, Owned: []Owned{{ID: 20000}, {ID: 50000}, {ID: 60000}}, Selected: [3]uint32{20000, 50000, 60000}}
}

func (s State) Validate() error {
	if s.Version != 1 || len(s.Owned) > 65535 {
		return fmt.Errorf("invalid profile skin state version or count")
	}
	owned := make(map[uint32]bool, len(s.Owned))
	for _, item := range s.Owned {
		if item.ID == 0 || owned[item.ID] {
			return fmt.Errorf("invalid or duplicate profile skin ID")
		}
		// This initial implementation grants permanent built-ins only. Do not
		// silently interpret a timed entitlement without a proven clock domain.
		if item.Expires != 0 {
			return fmt.Errorf("timed profile skins require expiry support")
		}
		owned[item.ID] = true
	}
	for _, id := range s.Selected {
		if id == 0 || !owned[id] {
			return fmt.Errorf("selected profile skin is not owned")
		}
	}
	return nil
}
