package modactor

import (
	"errors"
	"sort"
	"testing"
)

// stubHost 是一个只记账的假宿主：它让测试能验证"代理函数真的把参数传到了宿主"，
// 而不必把 cmd/wireprobe 的 lanHub 拖进 internal 包的测试里。
type stubHost struct {
	spawned []Spec
	moved   []struct {
		id     ActorID
		x, y   uint16
		motion byte
		speed  uint16
	}
	relocated []struct {
		id                  ActorID
		channel, town, area uint32
		x, y                uint16
	}
	despawned []ActorID
	poses     map[ActorID]Pose
	scenes    []Scene
	spawnErr  error
	next      ActorID
}

func (s *stubHost) Spawn(spec Spec) (ActorID, error) {
	if s.spawnErr != nil {
		return 0, s.spawnErr
	}
	s.next++
	s.spawned = append(s.spawned, spec)
	if s.poses == nil {
		s.poses = map[ActorID]Pose{}
	}
	s.poses[s.next] = Pose{Channel: spec.Channel, Town: spec.Town, Area: spec.Area, X: spec.X, Y: spec.Y, Motion: spec.Motion, Speed: spec.Speed, Placed: true}
	return s.next, nil
}

func (s *stubHost) Move(id ActorID, x, y uint16, motion byte, speed uint16) error {
	if _, ok := s.poses[id]; !ok {
		return ErrUnknownActor
	}
	type moveCall struct {
		id     ActorID
		x, y   uint16
		motion byte
		speed  uint16
	}
	s.moved = append(s.moved, moveCall{id, x, y, motion, speed})
	p := s.poses[id]
	p.X, p.Y, p.Motion, p.Speed = x, y, motion, speed
	s.poses[id] = p
	return nil
}

func (s *stubHost) Relocate(id ActorID, channel, town, area uint32, x, y uint16) error {
	if _, ok := s.poses[id]; !ok {
		return ErrUnknownActor
	}
	type relocateCall struct {
		id                  ActorID
		channel, town, area uint32
		x, y                uint16
	}
	s.relocated = append(s.relocated, relocateCall{id, channel, town, area, x, y})
	p := s.poses[id]
	p.Channel, p.Town, p.Area, p.X, p.Y = channel, town, area, x, y
	s.poses[id] = p
	return nil
}

func (s *stubHost) Despawn(id ActorID) error {
	if _, ok := s.poses[id]; !ok {
		return ErrUnknownActor
	}
	s.despawned = append(s.despawned, id)
	delete(s.poses, id)
	return nil
}

func (s *stubHost) Pose(id ActorID) (Pose, bool) {
	p, ok := s.poses[id]
	return p, ok
}

func (s *stubHost) List() []Snapshot {
	out := make([]Snapshot, 0, len(s.poses))
	for id, p := range s.poses {
		out = append(out, Snapshot{ID: id, Pose: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *stubHost) Scenes() []Scene { return s.scenes }

func installStub(t *testing.T) *stubHost {
	t.Helper()
	Uninstall()
	t.Cleanup(Uninstall)
	h := &stubHost{}
	if err := Install(h, "test"); err != nil {
		t.Fatalf("Install 失败：%v", err)
	}
	return h
}

func TestUnavailableWithoutHost(t *testing.T) {
	Uninstall()
	if Available() {
		t.Fatal("未 Install 时 Available 应为假")
	}
	if _, err := Spawn(Spec{Name: "x", Source: "m"}); !errors.Is(err, ErrNoHost) {
		t.Fatalf("Spawn 应返回 ErrNoHost，实际 %v", err)
	}
	if err := Move(1, 0, 0, 0, 0); !errors.Is(err, ErrNoHost) {
		t.Fatalf("Move 应返回 ErrNoHost，实际 %v", err)
	}
	if err := Relocate(1, 0, 0, 0, 0, 0); !errors.Is(err, ErrNoHost) {
		t.Fatalf("Relocate 应返回 ErrNoHost，实际 %v", err)
	}
	if err := Despawn(1); !errors.Is(err, ErrNoHost) {
		t.Fatalf("Despawn 应返回 ErrNoHost，实际 %v", err)
	}
	if _, ok := PoseOf(1); ok {
		t.Fatal("Pose 在无宿主时应返回 ok=false")
	}
	if got := List(); got != nil {
		t.Fatalf("List 在无宿主时应返回 nil，实际 %v", got)
	}
	if got := Scenes(); got != nil {
		t.Fatalf("Scenes 在无宿主时应返回 nil，实际 %v", got)
	}
}

func TestScenesForwardsToHost(t *testing.T) {
	h := installStub(t)
	h.scenes = []Scene{{Channel: 1, Town: 2, Area: 3, Players: 4}}
	got := Scenes()
	if len(got) != 1 || got[0] != h.scenes[0] {
		t.Fatalf("Scenes 应原样转发宿主的场景列表，实际 %+v", got)
	}
}

func TestInstallRejectsNilAndEmptySource(t *testing.T) {
	Uninstall()
	defer Uninstall()
	if err := Install(nil, "x"); err == nil {
		t.Fatal("Install(nil) 应报错")
	}
	if err := Install(&stubHost{}, "  "); err == nil {
		t.Fatal("Install 空来源应报错")
	}
}

func TestInstallTwiceIsRejected(t *testing.T) {
	installStub(t)
	if err := Install(&stubHost{}, "again"); err == nil {
		t.Fatal("重复 Install 应被拒绝（静默换宿主会让实体归属成谜）")
	}
	if Source() != "test" {
		t.Fatalf("重复 Install 不应改掉宿主来源，实际 %q", Source())
	}
}

func TestSpawnRequiresSource(t *testing.T) {
	installStub(t)
	if _, err := Spawn(Spec{Name: "无名"}); err == nil {
		t.Fatal("Spawn 缺 Source 应报错")
	}
}

func TestProxyForwardsToHost(t *testing.T) {
	h := installStub(t)
	id, err := Spawn(Spec{Name: "假人甲", Profession: 3, Level: 60, Channel: 1, Town: 2, Area: 3, X: 100, Y: 200, Source: "town.companion"})
	if err != nil {
		t.Fatalf("Spawn 失败：%v", err)
	}
	if len(h.spawned) != 1 || h.spawned[0].Name != "假人甲" {
		t.Fatalf("宿主没收到完整 Spec：%+v", h.spawned)
	}
	if err = Move(id, 150, 250, 1, 60); err != nil {
		t.Fatalf("Move 失败：%v", err)
	}
	if err = Relocate(id, 2, 9, 9, 10, 10); err != nil {
		t.Fatalf("Relocate 失败：%v", err)
	}
	p, ok := PoseOf(id)
	if !ok || p.Town != 9 || p.X != 10 {
		t.Fatalf("Relocate 后姿态不对：%+v ok=%v", p, ok)
	}
	if err = Despawn(id); err != nil {
		t.Fatalf("Despawn 失败：%v", err)
	}
	if _, ok = PoseOf(id); ok {
		t.Fatal("Despawn 之后 Pose 应查不到")
	}
	if len(List()) != 0 {
		t.Fatalf("Despawn 之后 List 应为空，实际 %+v", List())
	}
}

func TestUnknownActor(t *testing.T) {
	installStub(t)
	if err := Move(4242, 0, 0, 0, 0); !errors.Is(err, ErrUnknownActor) {
		t.Fatalf("Move 未知实体应返回 ErrUnknownActor，实际 %v", err)
	}
	if err := Despawn(4242); !errors.Is(err, ErrUnknownActor) {
		t.Fatalf("Despawn 未知实体应返回 ErrUnknownActor，实际 %v", err)
	}
}

func TestListIsStable(t *testing.T) {
	h := installStub(t)
	// 造三个，然后反复 List，顺序必须每次都一样（诊断日志要能逐行对比）。
	for i := 0; i < 3; i++ {
		if _, err := Spawn(Spec{Name: "n", Source: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	first := List()
	for i := 0; i < 5; i++ {
		got := List()
		if len(got) != len(first) {
			t.Fatalf("第 %d 次 List 条数变了", i)
		}
		for j := range got {
			if got[j].ID != first[j].ID {
				t.Fatalf("第 %d 次 List 顺序不稳定：%v vs %v", i, got[j].ID, first[j].ID)
			}
		}
	}
	if len(h.spawned) != 3 {
		t.Fatalf("宿主应收到 3 次 Spawn，实际 %d", len(h.spawned))
	}
}
