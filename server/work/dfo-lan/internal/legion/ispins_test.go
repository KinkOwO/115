package legion

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Official CMD bodies (session_s4_c2s, next78 §2.5). All include the 13-byte
// envelope, exactly as the decrypted plaintext reaches the legion package.
// The 2047/2046 tails beyond the wiredecode log's cut are zero bytes (field
// table: variant A zeros after the ffff marker, B zeros after the auxiliary
// byte, 2046 zeros after the next-stage echo).
var ispinsOfficialStart = mustHex("98f55f00000000000d000000006500000000000000000000")
var ispinsOfficialOperationA = mustHex("50a9c69f01000000010000000101000000ffff" + "0000000000000000000000000000")
var ispinsOfficialOperationB = mustHex("00000000000000002900000001020000000b" + "0000000000000000000000000000")
var ispinsOfficialEnterS2 = mustHex("000000000000000000000000006500000001000000000000")
var ispinsOfficialRewardEndS1 = mustHex("00699a03000000000000000000650000000000000002" + "00000000000000000000")

func mustHex(s string) []byte {
	p, err := decodeHex(s)
	if err != nil {
		panic(err)
	}
	return p
}

func TestIspinsDecodeStartAcceptsOfficialBody(t *testing.T) {
	req, err := DecodeIspinsStart(ispinsOfficialStart)
	if err != nil {
		t.Fatalf("official start body rejected: %v", err)
	}
	if req.Argument != IspinsContentID || req.BodyLength != 24 {
		t.Fatalf("start = %+v", req)
	}
}

func TestIspinsDecodeOperationSelect(t *testing.T) {
	a, err := DecodeIspinsOperationSelect(ispinsOfficialOperationA)
	if err != nil {
		t.Fatalf("official operation A rejected: %v", err)
	}
	if a.Variant != 1 || a.Token != 0x9fc6a950 {
		t.Fatalf("operation A = %+v", a)
	}
	b, err := DecodeIspinsOperationSelect(ispinsOfficialOperationB)
	if err != nil {
		t.Fatalf("official operation B rejected: %v", err)
	}
	if b.Variant != 2 || b.Counter != 0x29 || b.Auxiliary != 0x0b {
		t.Fatalf("operation B = %+v", b)
	}
}

func TestIspinsDecodeEnterAndRewardEnd(t *testing.T) {
	enter, err := DecodeIspinsEnter(ispinsOfficialEnterS2)
	if err != nil {
		t.Fatalf("official enter rejected: %v", err)
	}
	if enter.Stage != 1 || enter.BodyLength != 24 {
		t.Fatalf("enter = %+v", enter)
	}
	reward, next, err := DecodeIspinsRewardEnd(ispinsOfficialRewardEndS1)
	if err != nil {
		t.Fatalf("official reward-end rejected: %v", err)
	}
	if reward.Stage != 0 || next != 2 {
		t.Fatalf("reward-end = %+v next=%d", reward, next)
	}
}

func TestIspinsInfoTemplatesAreCompleteAndSized(t *testing.T) {
	want := []string{
		"initial",
		"wait0", "wait1", "wait2", "wait3",
		"chosen0", "chosen1", "chosen2", "chosen3",
		"dungeon0", "dungeon1", "dungeon2", "dungeon3",
		"clear0", "clear1", "clear2", "clear3",
		"final", "leave",
	}
	for _, name := range want {
		p, err := IspinsInfoPayload(name, [5]byte{})
		if err != nil {
			t.Fatalf("state %q: %v", name, err)
		}
		if len(p) != 88 {
			t.Fatalf("state %q: %d bytes, want 88", name, len(p))
		}
		// The place/state bytes the state machine keys on (next78 §2.1).
		var wantPlace, wantState byte = 0xff, 0x06
		switch name {
		case "initial":
			wantState = 0x01
		case "dungeon0", "dungeon1", "dungeon2", "dungeon3":
			wantPlace = 0x02
			wantState = 0x02
		case "chosen0", "chosen1", "chosen2", "chosen3":
			wantState = 0x02
		case "clear0", "clear1", "clear2", "clear3":
			wantState = 0x02
		case "final":
			wantState = 0x03
		case "leave":
			wantState = 0x05
		}
		if p[2] != wantPlace || p[3] != wantState {
			t.Fatalf("state %q: place=%02x state=%02x, want %02x/%02x", name, p[2], p[3], wantPlace, wantState)
		}
	}
	if _, err := IspinsInfoPayload("nonexistent", [5]byte{}); err == nil {
		t.Fatal("unknown state accepted")
	}
}

func TestIspinsInfoStageRecords(t *testing.T) {
	// wait0: nothing cleared; stage 0 (dungeon index 2) current, rest locked.
	p, err := IspinsInfoPayload("wait0", [5]byte{})
	if err != nil {
		t.Fatal(err)
	}
	wantRecords := [4][2]byte{{2, 0}, {3, 2}, {0, 2}, {1, 2}}
	for i, want := range wantRecords {
		idx := binary.LittleEndian.Uint32(p[19+8*i:])
		status := binary.LittleEndian.Uint32(p[23+8*i:])
		if idx != uint32(want[0]) || status != uint32(want[1]) {
			t.Fatalf("wait0 record %d = (%d,%d), want %v", i, idx, status, want)
		}
	}
	// wait3: stages 0..2 cleared, stage 3 current.
	p, _ = IspinsInfoPayload("wait3", [5]byte{})
	wantRecords = [4][2]byte{{2, 1}, {3, 1}, {0, 1}, {1, 0}}
	for i, want := range wantRecords {
		idx := binary.LittleEndian.Uint32(p[19+8*i:])
		status := binary.LittleEndian.Uint32(p[23+8*i:])
		if idx != uint32(want[0]) || status != uint32(want[1]) {
			t.Fatalf("wait3 record %d = (%d,%d), want %v", i, idx, status, want)
		}
	}
}

func TestIspinsInfoNonceRewrite(t *testing.T) {
	var nonce [5]byte
	copy(nonce[:], []byte{1, 2, 3, 4, 5})
	p, err := IspinsInfoPayload("wait0", nonce)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p[79:84], nonce[:]) {
		t.Fatalf("nonce bytes = %x", p[79:84])
	}
}

func TestIspinsEntryCharacterInfo(t *testing.T) {
	login, err := IspinsEntryCharacterInfo(true, [4]bool{}, [5]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if len(login) != 272 || login[0] != 0 {
		t.Fatalf("login frame = %d bytes, first %02x", len(login), login[0])
	}
	run, err := IspinsEntryCharacterInfo(false, [4]bool{true, false, true, false}, [5]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if run[0] != 1 || run[2] != 1 || run[4] != 0 || run[6] != 1 || run[8] != 0 {
		t.Fatalf("stage flags = %x", run[2:9])
	}
	all, err := IspinsEntryCharacterInfo(false, [4]bool{true, true, true, true}, [5]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if all[2] != 1 || all[4] != 1 || all[6] != 1 || all[8] != 1 {
		t.Fatalf("all-stages flags = %x", all[2:9])
	}
}

// next79 §28（2026-10-03 九测）：官服 s4 四阶段结算帧（490/585/668/777）的
// 标记位形态完全一致：idx21/69=0x7f、idx93=0x39、idx94=0x32、idx117=00、
// 53/85 保持 00。§24 的 idx21/53/85=0x7f、idx117=0x39 是错位写法，导致
// 客户端结算面板门禁不放行（1654 不发→op=2060→682 闪退，九测实证
// 整链其余帧均已 byte-equal 后仍闪退）。
func TestIspinsEntryCharacterInfoSettlementFlags(t *testing.T) {
	for _, flags := range [][4]bool{
		{true, false, false, false},
		{true, true, false, false},
		{true, true, true, false},
		{true, true, true, true},
	} {
		p, err := IspinsEntryCharacterInfo(false, flags, [5]byte{})
		if err != nil {
			t.Fatal(err)
		}
		if p[21] != 0x7f || p[69] != 0x7f {
			t.Fatalf("flags %v: bytes 21/69 = %02x/%02x, want 7f/7f", flags, p[21], p[69])
		}
		if p[93] != 0x39 || p[94] != 0x32 {
			t.Fatalf("flags %v: bytes 93/94 = %02x/%02x, want 39/32", flags, p[93], p[94])
		}
		if p[53] != 0 || p[85] != 0 || p[117] != 0 {
			t.Fatalf("flags %v: bytes 53/85/117 = %02x/%02x/%02x, want 00/00/00", flags, p[53], p[85], p[117])
		}
	}
	// 无阶段结算的 run 基线保持帧 374 全零形态。
	base, err := IspinsEntryCharacterInfo(false, [4]bool{}, [5]byte{})
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{21, 53, 69, 85, 93, 94, 117} {
		if base[i] != 0 {
			t.Fatalf("uncleared base byte %d = %02x, want 00", i, base[i])
		}
	}
}

// next79 §19（2026-10-03）：待机入场 N2254 的 0/2/4/6/8 五个 u16 是本周 stage
// 完成标记。官服 s4（10-02，待机后成功建队并全程作战）的待机 announce 帧 257
// 旗标全零 + 21/117=0x7f（与 login 帧同形）；switch 抓包（10-03，本周已打满）
// 帧 331/997 旗标全开，replay 该形导致面板「参赛 2/3 奖励 0/1」并本地拦截建队。
// 待机体必须用全零旗标形：与 run 基底只差 21/117 两个 7f，与 login 基底 0..255
// 完全一致。
func TestIspinsStandbyEntryCharacterInfo(t *testing.T) {
	p, err := IspinsStandbyEntryCharacterInfo([5]byte{})
	if err != nil || len(p) != 272 {
		t.Fatal(len(p), err)
	}
	if p[0] != 0 || p[2] != 0 || p[4] != 0 || p[6] != 0 || p[8] != 0 {
		t.Fatalf("standby head flags = %x, want all zero", p[0:9])
	}
	if p[21] != 0x7f || p[117] != 0x7f {
		t.Fatalf("flag bytes 21/117 = %02x/%02x, want 7f/7f", p[21], p[117])
	}
	run, _ := IspinsEntryCharacterInfo(false, [4]bool{}, [5]byte{})
	login, _ := IspinsEntryCharacterInfo(true, [4]bool{}, [5]byte{})
	// 只比 0..255：三个模板在 256..260 的 nonce 各不相同，不属于体差异。
	var vsRun, vsLogin []int
	for i := 0; i < 256; i++ {
		if p[i] != run[i] {
			vsRun = append(vsRun, i)
		}
		if p[i] != login[i] {
			vsLogin = append(vsLogin, i)
		}
	}
	// 相对 run 基底：offset 0 清零（run 自带入场标记）+ 21/117 两个 7f。
	if len(vsRun) != 3 || vsRun[0] != 0 || vsRun[1] != 21 || vsRun[2] != 117 {
		t.Fatalf("standby vs run base differs at %v, want [0 21 117]", vsRun)
	}
	// 相对 login 基底：0..255 完全一致。
	if len(vsLogin) != 0 {
		t.Fatalf("standby vs login base differs at %v, want none", vsLogin)
	}
}

func TestIspinsAcks(t *testing.T) {
	var nonce [5]byte
	copy(nonce[:], []byte{0x3b, 0xff, 0xf7, 0x7e, 0x43})
	// f377: 01 00000000 3bfff77e43 00*6.
	ack := IspinsStartAck(nonce)
	if len(ack) != 16 || ack[0] != 1 || !bytes.Equal(ack[5:10], nonce[:]) {
		t.Fatalf("start ack = %x", ack)
	}
	// f439-style ACK2045 for stage 0.
	ack = IspinsEnterAck(0, nonce)
	if len(ack) != 24 || ack[0] != 1 || ack[5] != 0x65 || ack[9] != 0 {
		t.Fatalf("enter ack = %x", ack)
	}
	// ACK2046 echoes stage and (stage+2)%4.
	ack = IspinsRewardEndAck(3, 1, nonce)
	if len(ack) != 32 || ack[9] != 3 || ack[13] != 1 {
		t.Fatalf("reward-end ack = %x", ack)
	}
	// f431 ACK2047-A carries the LE timestamp at 12.
	ack = IspinsOperationAckA(0x6abf662d, nonce)
	if len(ack) != 32 || ack[1] != 1 || ack[5] != 0xff || ack[6] != 0xff {
		t.Fatalf("operation ack A = %x", ack)
	}
	if binary.LittleEndian.Uint32(ack[12:]) != 0x6abf662d {
		t.Fatalf("operation ack A timestamp = %x", ack[12:16])
	}
	// f433 ACK2047-B echoes the auxiliary byte.
	ack = IspinsOperationAckB(0x0b, nonce)
	if len(ack) != 32 || ack[1] != 2 || ack[5] != 0x0b || ack[17] != 0xf0 {
		t.Fatalf("operation ack B = %x", ack)
	}
	// ACK2059 replay.
	ack = IspinsReviveAck()
	if len(ack) != 16 || ack[0] != 1 || ack[6] != 0xed {
		t.Fatalf("revive ack = %x", ack)
	}
}

func TestIspinsRewardPayloads(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		basic, err := IspinsBasicClearReward(stage)
		if err != nil {
			t.Fatalf("stage %d basic: %v", stage, err)
		}
		raw := basic
		if len(raw) != 7772 {
			t.Fatalf("stage %d native basic body %d bytes, reader requires 7772", stage, len(raw))
		}
		if item := binary.LittleEndian.Uint32(raw[1600:]); item != 0x009DB1BC {
			t.Fatalf("stage %d basic item = %x", stage, item)
		}
		if count := binary.LittleEndian.Uint32(raw[1604:]); count != 20 {
			t.Fatalf("stage %d basic count = %d", stage, count)
		}
		if raw[7760] != IspinsStageTokens[stage][0] || raw[7761] != IspinsStageTokens[stage][1] {
			t.Fatalf("stage %d basic tail = %x", stage, raw[7760:7762])
		}
		// The N31 head must equal the N2252 tail.
		n31 := IspinsDungeonClearEnabled(stage)
		if !bytes.Equal(n31[0:2], raw[7760:7762]) {
			t.Fatalf("stage %d N31/basic token mismatch", stage)
		}
		additional, err := IspinsAdditionalClearReward(stage)
		if err != nil {
			t.Fatalf("stage %d additional: %v", stage, err)
		}
		rawAdd := additional
		if len(rawAdd) != 2405 {
			t.Fatalf("stage %d native additional body %d bytes, reader requires 2405", stage, len(rawAdd))
		}
		for i, rec := range IspinsAdditionalRewards[stage] {
			off := i * 40
			flag := byte(0)
			if i == 0 {
				flag = 1
			}
			if rawAdd[off] != flag {
				t.Fatalf("stage %d record %d flag = %d", stage, i, rawAdd[off])
			}
			if item := binary.LittleEndian.Uint32(rawAdd[off+1:]); item != rec.Item {
				t.Fatalf("stage %d record %d item = %x", stage, i, item)
			}
			if rawAdd[off+5] != rec.Count {
				t.Fatalf("stage %d record %d count = %d", stage, i, rawAdd[off+5])
			}
			if rawAdd[off+9] != 3 {
				t.Fatalf("stage %d record %d constant = %d", stage, i, rawAdd[off+9])
			}
		}
		// S1 and S4 carry five records, S2/S3 four.
		wantRecords := 4
		if stage == 0 || stage == 3 {
			wantRecords = 5
		}
		if rawAdd[wantRecords*40] != 0 || rawAdd[wantRecords*40+1] != 0 {
			t.Fatalf("stage %d trailing records not zero", stage)
		}
	}
}

func TestIspinsDecodeChangeOperationLive(t *testing.T) {
	p := mustHex("0000000000000000000000000004000000ffff00000000000000000000000000")
	r, err := DecodeIspinsOperationSelect(p)
	if err != nil || r.Variant != 4 {
		t.Fatal(r, err)
	}
	for _, off := range []int{14, 15, 16, 17, 18} {
		bad := append([]byte(nil), p...)
		bad[off] ^= 1
		if _, err := DecodeIspinsOperationSelect(bad); err == nil {
			t.Fatalf("invalid reset field at %d accepted", off)
		}
	}
}

func TestIspinsOperationNoticeReplay(t *testing.T) {
	p := IspinsOperationNotice()
	if len(p) != 16 || p[0] != 0x0b || p[2] != 0x01 {
		t.Fatalf("operation notice = %x", p)
	}
}

func TestIspinsStoryPause(t *testing.T) {
	var nonce [5]byte
	copy(nonce[:], []byte{0x34, 0xbc, 0x2e, 0x0a, 0x42})
	p := IspinsStoryPause(false, nonce)
	if len(p) != 16 || p[0] != 0xed || p[2] != 0 || p[3] != 1 || !bytes.Equal(p[4:9], nonce[:]) {
		t.Fatalf("story pause = %x", p)
	}
	p = IspinsStoryPause(true, nonce)
	if p[2] != 1 {
		t.Fatalf("story resume = %x", p)
	}
}
