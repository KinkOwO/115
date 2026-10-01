package profileskin

import "encoding/binary"

// Restore emits cargo (NOTI1545) and selection (NOTI1546) for category 0, not
// the newer official-client full-list marker 11 which the current US readers
// reject. Cargo must precede selection: 0x1444eeca0 drops any selected ID
// absent from the owned map.
//
// 归属：本体例由 profileskin 拥有（曾经的 protocol.ProfileSkinRestore），
// protocol 只保留与领域无关的布局原语，避免 protocol 反向依赖领域。
func Restore(state State) (cargo, selected []byte, err error) {
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
