// ProfileSkinState owns category-0 profile decoration state. It does not model avatar equipment or the unrelated NOTI1759 favorite-character UI.
package character

import (
	"encoding/binary"
	"fmt"
)

type ProfileSkinOwned struct {
	ID      uint32 `json:"id"`
	Expires uint32 `json:"expires"`
}

type ProfileSkinState struct {
	Version  int                `json:"version"`
	Owned    []ProfileSkinOwned `json:"owned"`
	Selected [3]uint32          `json:"selected"`
}

// ProfileSkinDefaults are the public built-ins from the current Skin.lst: PartyFrame,
// PartyRequestFrame and CharacterInfoBG/default.skn. These are grants on first
// initialization only; login must subsequently read the persisted selection.
func ProfileSkinDefaults() ProfileSkinState {
	return ProfileSkinState{Version: 1, Owned: []ProfileSkinOwned{{ID: 20000}, {ID: 50000}, {ID: 60000}}, Selected: [3]uint32{20000, 50000, 60000}}
}

func (s ProfileSkinState) Validate() error {
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

// RestoreProfileSkin emits cargo (NOTI1545) and selection (NOTI1546) for category 0, not
// the newer official-client full-list marker 11 which the current US readers
// reject. Cargo must precede selection: 0x1444eeca0 drops any selected ID
// absent from the owned map.
//
// 归属：本体例由 character 领域拥有（曾经的 protocol.ProfileSkinRestore），
// protocol 只保留与领域无关的布局原语，避免 protocol 反向依赖领域。
func RestoreProfileSkin(state ProfileSkinState) (cargo, selected []byte, err error) {
	if err = state.Validate(); err != nil {
		return nil, nil, err
	}
	// NOTI1545: category, first owned count, {id, expiry}, second count=0.
	cargo = make([]byte, 5+8*len(state.Owned))
	binary.LittleEndian.PutUint16(cargo[1:], uint16(len(state.Owned)))
	for i, item := range state.Owned {
		binary.LittleEndian.PutUint32(cargo[3+i*8:], item.ID)
		binary.LittleEndian.PutUint32(cargo[7+i*8:], item.Expires)
	}
	// NOTI1546: category, party frame, request frame, character background,
	// followed by the extra request-frame selection count (none in this state).
	selected = make([]byte, 15)
	for i, id := range state.Selected {
		binary.LittleEndian.PutUint32(selected[1+i*4:], id)
	}
	return cargo, selected, nil
}
