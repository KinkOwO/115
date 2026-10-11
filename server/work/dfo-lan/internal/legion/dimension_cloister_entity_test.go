package legion

import (
	"testing"
)

// 次元回廊：进图帧列里的 N29 编号 == 客户端 CMD39/CMD117 上报的编号。
//
// 这一组钉住**编号真源**：官服 s30 抓包里三界各自的 N29（#786/#860/#942）第 1 条
// 记录的实体，与同界客户端的 CMD39（#489/#520/#558）**逐值相同**。会话怪物的编号
// 必须对齐到这里，否则客户端上报的死亡在 `ConfirmDeath`/`BossCheck` 里查不到。
//
// 见 dimension_cloister_entity.go 顶部（2026-10-10 实机根因）。
func TestDimCloisterStartMapEntitiesMatchOfficialEntities(t *testing.T) {
	// 官服 s30 三界的实体编号，三处同值（下面三个来源在抓包里逐值一致）：
	//
	//	N29  START_MAP（s2c #786 / #860 / #942）记录区 +6
	//	CMD39 DIE_MONSTER（c2s #489 / #520 / #558）首 4 字节
	//	N115  BOSS_DIE_CHECK 应答（s2c #827 / #900 / #984）@2
	want := []uint16{0x09ff, 0x07e7, 0x0b8d}
	for stage := range DimCloisterStageDungeons {
		vectors, err := GetDimCloisterEntryVectorsForStage(stage)
		if err != nil {
			t.Fatalf("stage %d: %v", stage, err)
		}
		entities, err := DimCloisterStartMapEntities(vectors)
		if err != nil {
			t.Fatalf("stage %d 解析 N29 失败: %v", stage, err)
		}
		if len(entities) != 1 {
			t.Fatalf("stage %d 怪物 %d 只, want 1（官服三帧 N29 都是 1 条 rank3 记录）: %v",
				stage, len(entities), entities)
		}
		if entities[0] != want[stage] {
			t.Fatalf("stage %d（副本 %d）N29 实体 = %#04x, want %#04x",
				stage, DimCloisterStageDungeons[stage], entities[0], want[stage])
		}
	}
}

// TestDimCloisterStartMapRecordLayout 逐字节核对 N29 记录区的偏移：
// entity@+6、template@+8、level@+12、rank@+13、team@+17，条数在 @36。
//
// 第 5 界那一帧的正文（官服 #942）与 `protocol.StartMap` 的构造字段一一对上，
// 包括本仓自己的 `.dgn` 服务端表里那只 BOSS 的模板号 109015480（= 0x068b7fb8）。
func TestDimCloisterStartMapRecordLayout(t *testing.T) {
	vectors, err := GetDimCloisterEntryVectorsForStage(2)
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	for _, v := range vectors {
		if v.Kind == 0 && v.ID == DimCloisterStartMapID {
			body = v.Body
		}
	}
	if len(body) == 0 {
		t.Fatal("第 5 界帧列里没有 N29")
	}
	if got := body[n29MonsterCountAt]; got != 1 {
		t.Fatalf("条数 = %d, want 1", got)
	}
	record := body[n29RecordFrom:]
	if len(record) < n29RecordSize {
		t.Fatalf("记录区只有 %d 字节", len(record))
	}
	if got := uint16(record[0]) | uint16(record[1])<<8; got != 0 { // spawnOrder
		t.Fatalf("spawnOrder = %d, want 0", got)
	}
	if got := uint16(record[6]) | uint16(record[7])<<8; got != 0x0b8d { // entity
		t.Fatalf("entity = %#04x, want 0x0b8d（官服 CMD39 #558 = 8d0b0000）", got)
	}
	if got := uint32(record[8]) | uint32(record[9])<<8 | uint32(record[10])<<16 | uint32(record[11])<<24; got != 109015480 { // template
		t.Fatalf("template = %d, want 109015480（= .dgn 里第 5 界那只 BOSS）", got)
	}
	if got := record[12]; got != 0x8c {
		t.Fatalf("level = %#02x, want 0x8c", got)
	}
	if got := record[13]; got != 3 {
		t.Fatalf("rank = %d, want 3", got)
	}
	if got := uint32(record[17]) | uint32(record[18])<<8 | uint32(record[19])<<16 | uint32(record[20])<<24; got != 100 { // team
		t.Fatalf("team = %d, want 100", got)
	}
}

// testRoster 是 `DimCloisterArenaRoster` 的最小实现（会话怪物表只需要编号这一列）。
type testRoster struct {
	entities []uint16
	applied  []uint16
	failSet  bool
}

func (r *testRoster) LegionMonsterEntities() []uint16 {
	return append([]uint16(nil), r.entities...)
}

func (r *testRoster) SetLegionMonsterEntities(entities []uint16) error {
	if r.failSet {
		return errTestRosterRejected
	}
	r.applied = append([]uint16(nil), entities...)
	return nil
}

var errTestRosterRejected = &rosterRejectedError{}

type rosterRejectedError struct{}

func (*rosterRejectedError) Error() string { return "roster rejected" }

// TestDimCloisterAlignArenaEntitiesReplacesSessionIds 钉住对齐动作本身：
// 会话原先自己编的号（4096 起）必须被 N29 给的官服号取代。
func TestDimCloisterAlignArenaEntitiesReplacesSessionIds(t *testing.T) {
	roster := &testRoster{entities: []uint16{4096}}
	applied, err := DimCloisterAlignArenaEntities(roster, []uint16{0x0b8d}, 4097)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 || applied[0] != 0x0b8d {
		t.Fatalf("applied = %v, want [0x0b8d]", applied)
	}
	if len(roster.applied) != 1 || roster.applied[0] != 0x0b8d {
		t.Fatalf("会话怪物表 = %v, want [0x0b8d]", roster.applied)
	}
}

// TestDimCloisterAlignArenaEntitiesRefusesMismatch 数量对不上时**一个字节都不改**。
//
// 宁可退化到旧行为（客户端报的号查不到 ⇒ 不通关），也不要错配到别的怪身上
// —— 那会变成「随便打一只杂兵就清关」。
func TestDimCloisterAlignArenaEntitiesRefusesMismatch(t *testing.T) {
	roster := &testRoster{entities: []uint16{4096, 4097}}
	if _, err := DimCloisterAlignArenaEntities(roster, []uint16{0x0b8d}, 4098); err == nil {
		t.Fatal("2 只对 1 只必须报错")
	}
	if roster.applied != nil {
		t.Fatalf("报错时不许改动会话表，got %v", roster.applied)
	}
	if _, err := DimCloisterAlignArenaEntities(&testRoster{entities: nil}, []uint16{1}, 0); err == nil {
		t.Fatal("空怪物表必须报错")
	}
	if _, err := DimCloisterAlignArenaEntities(nil, []uint16{1}, 0); err == nil {
		t.Fatal("缺少会话表必须报错")
	}
	if _, err := DimCloisterAlignArenaEntities(&testRoster{entities: []uint16{4096}}, []uint16{0, 1}, 0); err == nil {
		t.Fatal("无效编号必须报错")
	}
	roster = &testRoster{entities: []uint16{4096}, failSet: true}
	if _, err := DimCloisterAlignArenaEntities(roster, []uint16{0x0b8d}, 0); err == nil {
		t.Fatal("会话拒绝写入必须把错误透出来")
	}
}

// TestDimCloisterStartMapEntitiesRejectsBrokenBody 坏正文必须报错，而不是静默返回空表
// （静默返回空表会让对齐退化，问题又变成「BOSS 死了不通关」那种查不出来的形态）。
func TestDimCloisterStartMapEntitiesRejectsBrokenBody(t *testing.T) {
	cases := map[string]DimCloisterVector{
		"正文太短": {Kind: 0, ID: DimCloisterStartMapID, Body: make([]byte, n29BodyMin-1)},
		"条数为零": {Kind: 0, ID: DimCloisterStartMapID, Body: func() []byte {
			b := make([]byte, 64)
			b[n29MonsterCountAt] = 0
			return b
		}()},
		"条数太多": {Kind: 0, ID: DimCloisterStartMapID, Body: func() []byte {
			b := make([]byte, 64)
			b[n29MonsterCountAt] = 200
			return b
		}()},
		"记录被截断": {Kind: 0, ID: DimCloisterStartMapID, Body: func() []byte {
			b := make([]byte, n29RecordFrom+n29RecordSize)
			b[n29MonsterCountAt] = 2
			return b
		}()},
	}
	for name, vector := range cases {
		if _, err := DimCloisterStartMapEntities([]DimCloisterVector{vector}); err == nil {
			t.Fatalf("%s：必须报错", name)
		}
	}
	if _, err := DimCloisterStartMapEntities(nil); err == nil {
		t.Fatal("帧列里没有 N29：必须报错")
	}
}
