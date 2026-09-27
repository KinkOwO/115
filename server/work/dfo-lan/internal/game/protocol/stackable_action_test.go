package protocol

import (
	"encoding/hex"
	"strings"
	"testing"
)

// stackableFrame is the live CMD507 body: slot u16 at 0, action u32 at 7, every
// other byte zero. Captured 2026-09-26 while using damage-font consumables.
func stackableFrame(slot uint16, action byte, size int) []byte {
	p := make([]byte, size)
	p[0] = byte(slot)
	p[1] = byte(slot >> 8)
	p[7] = action
	return p
}

// addSkinStorageCapture is the 64-byte plain body the client sent for bag slot
// 71 holding template 10358669.
var addSkinStorageCapture = "4700" + strings.Repeat("00", 5) + "a9" + strings.Repeat("00", 56)

func TestAddSkinStorageCaptured(t *testing.T) {
	if len(addSkinStorageCapture) != 128 {
		t.Fatal("vector length")
	}
	p, e := hex.DecodeString(addSkinStorageCapture)
	if e != nil {
		t.Fatal(e)
	}
	slot, action, e := DecodeStackableAction(p)
	if e != nil || slot != 71 || action != 169 {
		t.Fatal(slot, action, e)
	}
	if got, e := DecodeAddSkinStorageAction(p); e != nil || got != 71 {
		t.Fatal(got, e)
	}
	// The fatigue path must keep refusing the skin frame, and the skin path the
	// fatigue frame, so the two uses of CMD507 can never cross.
	if _, e := DecodeFatigueAction(p); e == nil {
		t.Fatal("fatigue decoder accepted action 169")
	}
	q := stackableFrame(66, 54, 64)
	if _, e := DecodeAddSkinStorageAction(q); e == nil {
		t.Fatal("skin decoder accepted action 54")
	}
	if got, e := DecodeFatigueAction(q); e != nil || got != 66 {
		t.Fatal(got, e)
	}
	if _, _, e := DecodeStackableAction(stackableFrame(71, 169, 63)); e == nil {
		t.Fatal("short frame accepted")
	}
	mutated := append([]byte{}, p...)
	mutated[len(mutated)-1] = 1
	if _, _, e := DecodeStackableAction(mutated); e == nil {
		t.Fatal("nonzero tail accepted")
	}
}

// AddSkinStorageVector is the 64-byte plain body the client sent for bag slot 71
// holding template 10358669.
var AddSkinStorageVector = "4700" + strings.Repeat("00", 6) + "a9" + strings.Repeat("00", 55)

// 实机 2026-09-26T11:02:34Z 点「Expand Creature Skin slots?」的 OK 后服务端抓到
// 的明文（CMD507，frame.Raw[13:] 共 64 字节）：slot=81, list=0, action=197(0xC5)，
// 其余全 0。当时宠物券正好摆在 81 格；券的归属由同会话 client_trace 的
// "Creature Skin Slot Expansion Ticket(10309084) : SlotIndex(80/81)" 佐证，
// 所以动作 id 是跟着券走的，不能按槽位记。
var creatureSkinSlotCapture = "5100" + strings.Repeat("00", 5) + "c5" + strings.Repeat("00", 56)

func TestStackableActionCreatureSkinTicketCaptured(t *testing.T) {
	p, e := hex.DecodeString(creatureSkinSlotCapture)
	if e != nil || len(p) != 64 {
		t.Fatal(len(p), e)
	}
	slot, action, e := DecodeStackableAction(p)
	if e != nil || slot != 81 || action != ActionOpenCreatureSkinSlot {
		t.Fatalf("decoded %d %d %v", slot, action, e)
	}
	// action 54 的读取器必须继续拒绝这一包：否则疲劳药水路径会误吃幻化券。
	if _, e := DecodeFatigueAction(p); e == nil {
		t.Fatal("fatigue reader accepted a skin slot ticket")
	}
	if _, e := DecodeAddSkinStorageAction(p); e == nil {
		t.Fatal("skin-storage reader accepted a pet-skin-slot ticket")
	}
	// 只有 slot / list / action 三处允许非零，其余字节任一处被改动都必须整包
	// 拒绝。list 位由各动作自行判定，故不在这里做变异。
	for _, at := range []int{3, 4, 5, 6, 11, 15, 40, 63} {
		q := append([]byte{}, p...)
		q[at] = 255
		if _, _, e := DecodeStackableAction(q); e == nil {
			t.Fatalf("mutation %d accepted", at)
		}
	}
}
