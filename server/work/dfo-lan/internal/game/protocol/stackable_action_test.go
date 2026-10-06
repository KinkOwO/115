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

// boostCapsuleCapture 是实机 2026-10-04 17:56:59 使用直升胶囊的 64 字节正文：
// 背包格 65、list 0、动作 337（0x151，占 p[7:8] 两字节）。动作号是 u32，它自己的
// 高位不是「未知字段」——按单字节动作校验会把胶囊拒死成 unsupported stackable
// action fields。
var boostCapsuleCapture = "4100" + strings.Repeat("00", 5) + "5101" + strings.Repeat("00", 55)

func TestBoostCapsuleCapturedActionIsU32(t *testing.T) {
	p, e := hex.DecodeString(boostCapsuleCapture)
	if e != nil || len(p) != 64 {
		t.Fatal(len(p), e)
	}
	slot, action, e := DecodeStackableAction(p)
	if e != nil || slot != 65 || action != CapsuleAction {
		t.Fatal(slot, action, e)
	}
	r, e := DecodeBoostCapsule115(p)
	if e != nil || r.Slot != 65 || r.Space != 0 {
		t.Fatal(r, e)
	}
	// 疲劳药水仍然不能接手这一帧，两条 507 线路不互串。
	if _, e := DecodeFatigueAction(p); e == nil {
		t.Fatal("fatigue decoder accepted the capsule frame")
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

// 实机 2026-09-26 从背包直接使用光环幻化栏扩展券（模板 10157209，摆在 81 格）时抓到
// 的 CMD507（64 字节）：slot=81、list=0、action=101(0x65)，其余全 0。同会话
// client_trace 把它记成 "Skin Slot Unlocker (Aura)(10157209) : SlotIndex(81)"。
//
// 它和宠物券（197）是同一帧形状、只有动作号不同，所以只能按动作号分流：按槽位或按
// 券模板都认不出来 —— 10157209 的物品脚本里根本没有 [action type] 段。
var auraSkinSlotCapture = "5100" + strings.Repeat("00", 5) + "65" + strings.Repeat("00", 56)

func TestStackableActionAuraSkinTicketCaptured(t *testing.T) {
	if ActionOpenAuraSkinSlot != 101 {
		t.Fatalf("aura action is %d, want 101", ActionOpenAuraSkinSlot)
	}
	p, e := hex.DecodeString(auraSkinSlotCapture)
	if e != nil || len(p) != 64 {
		t.Fatal(len(p), e)
	}
	slot, action, e := DecodeStackableAction(p)
	if e != nil || slot != 81 || action != ActionOpenAuraSkinSlot {
		t.Fatalf("decoded %d %d %v", slot, action, e)
	}
	// 其它读取器不得吃下这一包，否则会走错路径（疲劳药水 / 皮肤仓库 / 宠物券）。
	if _, e := DecodeFatigueAction(p); e == nil {
		t.Fatal("fatigue reader accepted an aura ticket")
	}
	if _, e := DecodeAddSkinStorageAction(p); e == nil {
		t.Fatal("skin-storage reader accepted an aura ticket")
	}
	// 光环券与宠物券必须解出**不同**的动作号，否则两条路会互相顶掉。
	creature, e := hex.DecodeString(creatureSkinSlotCapture)
	if e != nil {
		t.Fatal(e)
	}
	if _, a, e := DecodeStackableAction(creature); e != nil || a != ActionOpenCreatureSkinSlot {
		t.Fatalf("creature ticket -> %d %v", a, e)
	}
}

// list 字节（offset 2）是字段本身：光环与宠物券实机都发 0，但它不能被钉死为 0 ——
// 上游哪天多发一个 list 值，整包就会被丢弃，表现与「这条命令没实现」一模一样。
func TestStackableActionAcceptsListByte(t *testing.T) {
	q := stackableFrame(81, 101, 64)
	q[2] = 1
	slot, action, e := DecodeStackableAction(q)
	if e != nil || slot != 81 || action != ActionOpenAuraSkinSlot {
		t.Fatalf("non-zero list byte rejected: %d %d %v", slot, action, e)
	}
}
