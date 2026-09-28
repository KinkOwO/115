package protocol

import (
	"encoding/binary"
	"fmt"
)

// AddSkinStorageAction is the CMD507 action code the client sends when using an
// `[add skin storage]` stackable (damage-font and other skin-register items).
// Live capture 2026-09-26: using 10305398/10358669 sends CMD507 with the slot
// at p[0:2] and action 169 (0xA9) at p[7:11], every other byte zero. The
// fatigue potion shares this frame with action 54.
const AddSkinStorageAction = 169

// CMD507 的动作 id。客户端对同一个 opcode 复用多种动作：54 是疲劳恢复药水，
// 169 是 [add skin storage]，197 是宠物幻化栏扩展券（模板 10309084），
// 101 是光环幻化栏扩展券（模板 10157209 / 商店同语义许可证 50006401）。
// 几种券发的都是同一个帧形状，只有动作号不同，所以只能按动作号分流。
const (
	ActionRecoverFatigue       uint32 = 54
	ActionOpenAuraSkinSlot     uint32 = 101
	ActionOpenCreatureSkinSlot uint32 = 197
)

// DecodeStackableAction parses the shared CMD507 "use stackable" frame and
// returns its slot and action code. The frame is 59 or 64 bytes; the slot is a
// u16 at offset 0, the `list` byte at offset 2 and the action a u32 at offset 7.
// The list byte is a real field each action judges for itself (the aura and
// creature skin tickets are captured with it zero, but it must not be pinned
// here), so it is skipped along with the slot and the action. The actions that
// do require list 0 - the fatigue potion and [add skin storage] - re-assert it
// in their own readers below. Every other byte
// must be zero, which also constrains the action to a single byte (54, 101, 169
// and 197 all fit).
func DecodeStackableAction(p []byte) (slot uint16, action uint32, err error) {
	if len(p) != 59 && len(p) != 64 {
		return 0, 0, fmt.Errorf("stackable action length")
	}
	slot = binary.LittleEndian.Uint16(p)
	if slot == 0 {
		return 0, 0, fmt.Errorf("unsupported stackable action")
	}
	action = binary.LittleEndian.Uint32(p[7:])
	for i, b := range p {
		// 0..2 = slot(u16) + list(u8)，7..10 = action(u32)：都是字段本身。
		if i < 3 || i == 7 {
			continue
		}
		if b != 0 {
			return 0, 0, fmt.Errorf("unsupported stackable action fields")
		}
	}
	return slot, action, nil
}

// CMD507 sender writes u16 slot, u8 list, four u32 fields and 40 bytes.
// Action54's captured request has only the slot and action populated.
func DecodeFatigueAction(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if action != 54 {
		return 0, fmt.Errorf("unsupported stackable action")
	}
	// 这两个动作的原始契约里 list（offset 2）必须为 0：通用解码器为了幻化栏券把这一
	// 位放开了，所以要在这里显式补回来，否则一个 list 被改动过的包会被当成药水/
	// 皮肤仓动作吃掉。
	if p[2] != 0 {
		return 0, fmt.Errorf("unsupported stackable action list")
	}
	return slot, nil
}

// DecodeAddSkinStorageAction accepts only the `[add skin storage]` action (169).
func DecodeAddSkinStorageAction(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if action != AddSkinStorageAction {
		return 0, fmt.Errorf("unsupported stackable action")
	}
	// 这两个动作的原始契约里 list（offset 2）必须为 0：通用解码器为了幻化栏券把这一
	// 位放开了，所以要在这里显式补回来，否则一个 list 被改动过的包会被当成药水/
	// 皮肤仓动作吃掉。
	if p[2] != 0 {
		return 0, fmt.Errorf("unsupported stackable action list")
	}
	return slot, nil
}

// DecodeQuestAirshipAction accepts the exact native CMD507 shape observed
// when the level-94 airship communicator was used in town: list 0, action 206,
// and no extra parameters. The item identity is resolved from the owned bag
// slot, not inferred from the request's other u32 fields.
func DecodeQuestAirshipAction(p []byte) (uint16, error) {
	if len(p) != 64 && len(p) != 59 {
		return 0, fmt.Errorf("airship action length")
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 || p[2] != 0 || binary.LittleEndian.Uint32(p[7:]) != 206 {
		return 0, fmt.Errorf("unsupported airship action")
	}
	for i, b := range p {
		if i < 2 || i >= 7 && i < 11 {
			continue
		}
		if b != 0 {
			return 0, fmt.Errorf("unsupported airship action fields")
		}
	}
	return slot, nil
}
