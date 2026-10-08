package pvp

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// realMakeHex 是 2026-10-08 客户端实机点「创建房间」时发出的真实 CMD50 明文
// （来自服务端 events.jsonl 的 client_frame.id=50 / plain_hex）。
//
//	00 | 11 00 00 00 | "In Arena Training" | 00 00 | 00 | 01 | 00 | 00 00 00 00 00
//	NameType  NameLen       Name(17)         Map    Pwd  Special Flag  尾部 5 字节
const realMakeHex = "0011000000496e204172656e6120547261696e696e6700000001000000000000"

func TestParseMakeRealClientSample(t *testing.T) {
	b, err := hex.DecodeString(realMakeHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 32 {
		t.Fatalf("real sample is %d bytes, want 32", len(b))
	}
	q, err := ParseMake(b)
	if err != nil {
		t.Fatalf("ParseMake: %v", err)
	}
	if q.NameType != 0 {
		t.Errorf("NameType = %d, want 0", q.NameType)
	}
	if string(q.Name) != "In Arena Training" {
		t.Errorf("Name = %q, want %q", q.Name, "In Arena Training")
	}
	if q.Map != 0 {
		t.Errorf("Map = %d, want 0", q.Map)
	}
	if q.SpecialMode != 1 {
		t.Errorf("SpecialMode = %d, want 1 (普通练习房间)", q.SpecialMode)
	}
	if q.Flag != 0 {
		t.Errorf("Flag = %d, want 0", q.Flag)
	}
	if len(q.Password) != 0 {
		t.Errorf("Password = %v, want empty", q.Password)
	}
}

func TestParseMakeToleratesExtraTail(t *testing.T) {
	// 115 客户端比 90US 的 reader 多 5 字节尾部；解析器不得因此报错。
	b, _ := hex.DecodeString(realMakeHex)
	if _, err := ParseMake(b); err != nil {
		t.Fatalf("尾部字节把解析搞挂了：%v", err)
	}
	// 反过来：截断到 90US 的长度也必须能解析（防御旧客户端/测试样本）。
	if _, err := ParseMake(b[:27]); err != nil {
		t.Fatalf("90US 长度（27B）样本解析失败：%v", err)
	}
}

func testIdentity(n uint16) Identity { return Identity{UserID: n, Generation: 1} }

// realOrdinaryMakeHex 是 2026-10-08 客户端在决斗场城镇点「创建房间」时发出的
// 普通房间请求（8 字节，SpecialMode=0）。同一次会话里还发过 0d... 的变体，
// 只有 NameType 不同（预设房间名序号），其余字段一致。
const realOrdinaryMakeHex = "0500000000000000"

func TestParseMakeOrdinaryRoomSample(t *testing.T) {
	b, _ := hex.DecodeString(realOrdinaryMakeHex)
	if len(b) != 8 {
		t.Fatalf("sample is %d bytes, want 8", len(b))
	}
	q, err := ParseMake(b)
	if err != nil {
		t.Fatalf("ParseMake: %v", err)
	}
	if q.NameType != 5 {
		t.Errorf("NameType = %d, want 5", q.NameType)
	}
	if q.Map != 0 {
		t.Errorf("Map = %d, want 0", q.Map)
	}
	if q.SpecialMode != 0 {
		t.Errorf("SpecialMode = %d, want 0 (普通房间)", q.SpecialMode)
	}
	if len(q.Password) != 0 || q.Flag != 0 {
		t.Errorf("Pwd=%v Flag=%d, want 空/0", q.Password, q.Flag)
	}
	// NameType != 0 ⇒ 不读名字，所以 Create 里 Name 为空是正常的。
	if len(q.Name) != 0 {
		t.Errorf("Name = %q, want empty（预设名不随包下发）", q.Name)
	}
}

func TestCreateOrdinaryRoomAcceptsPredetNameType(t *testing.T) {
	var m Manager
	b, _ := hex.DecodeString(realOrdinaryMakeHex)
	q, err := ParseMake(b)
	if err != nil {
		t.Fatal(err)
	}
	room, err := m.Create(testIdentity(9), 21, q)
	if err != nil {
		t.Fatalf("普通房间建房被拒：%v", err)
	}
	if room.Mode != NormalMode {
		t.Errorf("Mode = %d, want NormalMode(%d)", room.Mode, NormalMode)
	}
	if room.NameType != 5 {
		t.Errorf("NameType = %d, want 5", room.NameType)
	}
	// 普通房间 1..7 号位保持可加入（EmptySeat），不是练习房的 ClosedSeat。
	if room.Seats[1].State != EmptySeat {
		t.Errorf("seat1 = %d, want EmptySeat(%d)", room.Seats[1].State, EmptySeat)
	}
	// RoomList 对 NameType != 0 不写名字，长度应比带名字的短。
	rl := RoomList([]Room{room})
	want := 2 + 2 + 1 + 1 + 1 + 2 + 1 + 8*3 + 1 + 4
	if len(rl) != want {
		t.Errorf("RoomList len = %d, want %d（NameType!=0 时不带名字）", len(rl), want)
	}
}

func TestCreatePracticeRoomShape(t *testing.T) {
	var m Manager
	req := MakeRequest{NameType: 0, Name: []byte("In Arena Training"), Map: 0, SpecialMode: 1}
	room, err := m.CreatePractice(testIdentity(7), 21, req)
	if err != nil {
		t.Fatalf("CreatePractice: %v", err)
	}
	if room.Mode != PracticeMode {
		t.Errorf("Mode = %d, want %d", room.Mode, PracticeMode)
	}
	if room.State != Waiting {
		t.Errorf("State = %d, want Waiting(%d)", room.State, Waiting)
	}
	if room.Channel != 21 {
		t.Errorf("Channel = %d, want 21", room.Channel)
	}
	if room.Seats[0].Owner.UserID != 7 || room.Seats[0].State != Waiting {
		t.Errorf("seat0 = %+v, want host waiting", room.Seats[0])
	}
	for i := 1; i < SeatCount; i++ {
		if room.Seats[i].State != ClosedSeat {
			t.Errorf("seat%d state = %d, want ClosedSeat(%d)", i, room.Seats[i].State, ClosedSeat)
		}
	}
}

func TestCreatePracticeRejectsWrongSpecialMode(t *testing.T) {
	var m Manager
	for _, sm := range []byte{0, 2, 3} {
		if _, err := m.CreatePractice(testIdentity(7), 21, MakeRequest{SpecialMode: sm}); err == nil {
			t.Errorf("SpecialMode=%d 应被拒绝", sm)
		}
	}
}

func TestWireBuilderLengths(t *testing.T) {
	var m Manager
	room, err := m.CreatePractice(testIdentity(3), 21, MakeRequest{Name: []byte("abc"), SpecialMode: 1})
	if err != nil {
		t.Fatal(err)
	}

	// EnterSuccess = success 字节 + 8 个准备标志。
	es := EnterSuccess(room)
	if len(es) != 9 {
		t.Errorf("EnterSuccess len = %d, want 9", len(es))
	}
	if es[0] != 1 {
		t.Errorf("EnterSuccess[0] = %d, want 1 (success)", es[0])
	}

	// Seats = u16 ID + u8 Mode + u8 count + 8×(u8 idx,u8 state,u16 uid)
	seats := Seats(room)
	if len(seats) != 2+1+1+8*4 {
		t.Errorf("Seats len = %d, want %d", len(seats), 2+1+1+8*4)
	}
	if seats[3] != SeatCount {
		t.Errorf("Seats count byte = %d, want %d", seats[3], SeatCount)
	}
	// 0 号座位 uid = 房主，1 号座位是 ClosedSeat（uid=255）。
	if got := binary.LittleEndian.Uint16(seats[6:8]); got != 3 {
		t.Errorf("seat0 uid = %d, want 3", got)
	}
	if got := binary.LittleEndian.Uint16(seats[10:12]); got != uint16(EmptySeat) {
		t.Errorf("seat1 uid = %d, want %d", got, EmptySeat)
	}

	// RoomState = u16 ID + u8 State + u8 Manager + u16 Map + u8 Mode + u32
	rs := RoomState(room)
	if len(rs) != 2+1+1+2+1+4 {
		t.Errorf("RoomState len = %d, want %d", len(rs), 2+1+1+2+1+4)
	}

	// RoomList = u16 count + 每条房间记录
	rl := RoomList([]Room{room})
	if got := binary.LittleEndian.Uint16(rl[:2]); got != 1 {
		t.Errorf("RoomList count = %d, want 1", got)
	}
	wantRoom := 2 + 1 + (4 + len(room.Name)) + 1 + 1 + 2 + 1 + 8*3 + 1 + 4
	if len(rl) != 2+wantRoom {
		t.Errorf("RoomList len = %d, want %d", len(rl), 2+wantRoom)
	}
}

func TestReadyStartsWhenAllOccupiedSeatsReady(t *testing.T) {
	var m Manager
	host := testIdentity(1)
	guest := testIdentity(2)

	// 普通房间（SpecialMode=0 走 Create），这样 1 号位是开放的。
	room, err := m.Create(host, 21, MakeRequest{Name: []byte("room"), SpecialMode: 0})
	if err != nil {
		t.Fatal(err)
	}
	room, err = m.Join(guest, 21, room.ID, nil)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if room.SeatOf(guest) < 0 {
		t.Fatal("guest 没坐上座位")
	}

	// 只有一个人准备 → 不开打。
	if _, started, err := m.Ready(host, true); err != nil || started {
		t.Errorf("单人 prepare 不应开打（started=%v err=%v）", started, err)
	}
	// 两个人都准备 → 开打。
	got, started, err := m.Ready(guest, true)
	if err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if !started {
		t.Error("全员准备后应当开打")
	}
	if got.State != Fighting {
		t.Errorf("State = %d, want Fighting(%d)", got.State, Fighting)
	}
}

func TestLeaveTransfersManagerAndDeletesEmptyRoom(t *testing.T) {
	var m Manager
	host := testIdentity(1)
	guest := testIdentity(2)
	room, _ := m.Create(host, 21, MakeRequest{Name: []byte("r"), SpecialMode: 0})
	room, _ = m.Join(guest, 21, room.ID, nil)
	if room.Manager != 0 {
		t.Fatalf("Manager = %d, want 0", room.Manager)
	}
	after, left := m.Leave(host)
	if !left {
		t.Fatal("Leave 应返回 true")
	}
	if after.Manager != byte(after.SeatOf(guest)) {
		t.Errorf("房主离开后 Manager = %d, want 房客座位 %d", after.Manager, after.SeatOf(guest))
	}
	// 最后一个人走 → 房间消失。
	if _, left := m.Leave(guest); !left {
		t.Fatal("最后一个离开也应返回 true")
	}
	if snap := m.Snapshot(21); len(snap) != 0 {
		t.Errorf("空房间应被删除，剩 %d 个", len(snap))
	}
}

func TestSetSeatAndModeRealSampleShapes(t *testing.T) {
	// 实机 2026-10-08 样本：
	//   cmd 52 = 4 字节 `seat state 00 00`（比 90US 多 2 字节尾）
	//   cmd 54 = 16 字节 `mode 00 00 00` + 12 字节 0（mode 是 u32，且取过 3）
	var m Manager
	host := testIdentity(4)
	room, err := m.Create(host, 21, MakeRequest{NameType: 8, Map: 0})
	if err != nil {
		t.Fatal(err)
	}

	// `00 03` → 房主把自己座位设成 3。
	room, err = m.SetSeat(host, 0x00, 0x03)
	if err != nil {
		t.Fatalf("SetSeat(0,3) 应被接受：%v", err)
	}
	if room.Seats[0].State != 3 {
		t.Errorf("seat0 state = %d, want 3", room.Seats[0].State)
	}
	// `01 fe` → 关闭 1 号座位。
	room, err = m.SetSeat(host, 0x01, ClosedSeat)
	if err != nil {
		t.Fatalf("SetSeat(1,ClosedSeat) 应被接受：%v", err)
	}
	if room.Seats[1].State != ClosedSeat {
		t.Errorf("seat1 state = %d, want ClosedSeat(%d)", room.Seats[1].State, ClosedSeat)
	}

	// cmd 54 取过 1 和 3；90US 只允许 1/2，所以 3 必须放行。
	for _, mode := range []byte{1, 3} {
		if _, err := m.SetMode(host, mode); err != nil {
			t.Errorf("SetMode(%d) 应被接受：%v", mode, err)
		}
	}
	// 越界仍要拒；6/10 是服务端内部模式，不允许客户端设置。
	for _, mode := range []byte{0, 5, 10} {
		if _, err := m.SetMode(host, mode); err == nil {
			t.Errorf("SetMode(%d) 应被拒绝", mode)
		}
	}
}

func TestSetModeRejectedOnPracticeRoom(t *testing.T) {
	var m Manager
	host := testIdentity(5)
	if _, err := m.CreatePractice(host, 21, MakeRequest{Name: []byte("x"), SpecialMode: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetMode(host, 1); err == nil {
		t.Error("练习房间不应接受 CMD54 改模式")
	}
}
