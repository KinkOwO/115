package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"testing"
)

// 二觉（stage 2）在 75 级解锁，但 .chr 的二觉段里有一批 85 级技能；三觉同理
// （100 级解锁，段里有 95 级技能）。ApplyAwakening 在觉醒当下只写等级已达标的
// 授予，75~84 级二觉的角色因此永远拿不到那批技能——实机表现就是"二觉技能没有
// 根据等级自动加点"。修复让授予与 automaticSkills（转职段起始技能）一样，从存档
// 的等级/觉醒阶段现算，等级达标即生效，也不需要改写已有存档。
func loadAwakeningGrantFixture(t *testing.T) (*Service, catalog.Characters) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	return &Service{Catalog: c, Learning: l}, c
}

type lateGrant struct {
	job   byte
	prof  catalog.Profession
	adv   byte
	id    uint16
	rank  byte
	level int
}

// lateSecondAwakeningGrants 收集"二觉段里有、但自身 [required level] 高于二觉
// 门槛 75"的授予，正是被 ApplyAwakening 跳过的那批。
func lateSecondAwakeningGrants(t *testing.T, s *Service, c catalog.Characters) []lateGrant {
	t.Helper()
	var out []lateGrant
	for job, prof := range c.Professions {
		for adv, stages := range prof.AwakeningSkills {
			if adv == 0 {
				// 无转职分支的职业（creator mage / demonic swordman）其觉醒阶段
				// 另有 WireAdvancement 门槛，不在本次缺陷范围。
				continue
			}
			grants := stages[2]
			for i := 0; i+1 < len(grants); i += 2 {
				id, rank := uint16(grants[i]), byte(grants[i+1])
				d, ok := s.Learning.index[job][id]
				if !ok {
					t.Fatalf("job%d skill%d missing from the learning catalog", job, id)
				}
				levels := d.Ints("[required level]")
				if len(levels) != 1 {
					t.Fatalf("job%d skill%d [required level] = %v", job, id, levels)
				}
				if levels[0] <= 75 || levels[0] > 115 {
					continue
				}
				out = append(out, lateGrant{job: job, prof: prof, adv: adv, id: id, rank: rank, level: levels[0]})
			}
		}
	}
	return out
}

func TestSecondAwakeningLateGrantFollowsLevel(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	samples := lateSecondAwakeningGrants(t, s, c)
	if len(samples) == 0 {
		t.Fatal("目录里没有 75 级以上门槛的二觉授予样本")
	}
	for _, g := range samples {
		role := storage.Character{Profession: g.job, ConfigVersion: c.Source.Checksum}
		early := State{Level: 75, Advancement: g.adv, Awakening: 2, SourceSHA256: g.prof.RawSHA256, InitialSkills: g.prof.InitialSkills}
		granted, e := s.awakeningSkills(role, early)
		if e != nil {
			t.Fatal(e)
		}
		if early.Level != 75 {
			t.Fatal("fixture level mutated")
		}
		// 75 级二觉：技能自己的 [required level] 还没到，授予必须挂起而不是先给后退。
		if granted[g.id] != 0 {
			t.Fatalf("job%d adv%d skill%d 在 75 级就被授予（需要 %d 级）", g.job, g.adv, g.id, g.level)
		}
		// 等级达标：同一份存档自动得到授予。
		late := early
		late.Level = byte(g.level)
		granted, e = s.awakeningSkills(role, late)
		if e != nil {
			t.Fatal(e)
		}
		if granted[g.id] != g.rank {
			t.Fatalf("job%d adv%d skill%d 在 %d 级未授予：%v", g.job, g.adv, g.id, g.level, granted[g.id])
		}
		// 阶段没到（只有一觉）时不得授予。
		unawakened := late
		unawakened.Awakening = 1
		granted, e = s.awakeningSkills(role, unawakened)
		if e != nil {
			t.Fatal(e)
		}
		if granted[g.id] != 0 {
			t.Fatalf("job%d adv%d skill%d 在二觉前被授予", g.job, g.adv, g.id)
		}
	}
	t.Logf("二觉延迟授予样本 %d 个", len(samples))
}

// 端到端：同一份存档，等级从 75 提到技能门槛后，下发用的 knownSkills 直接带上
// 该技能；等级不足时不下发。这条路径就是客户端技能树/进城技能帧看到的集合。
func TestSecondAwakeningLateGrantReachesKnownSkills(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	samples := lateSecondAwakeningGrants(t, s, c)
	if len(samples) == 0 {
		t.Fatal("目录里没有 75 级以上门槛的二觉授予样本")
	}
	g := samples[0]
	role := storage.Character{Profession: g.job, ConfigVersion: c.Source.Checksum}
	st := State{Level: 75, Advancement: g.adv, Awakening: 2, SourceSHA256: g.prof.RawSHA256, InitialSkills: g.prof.InitialSkills}
	known, e := s.knownSkills(role, st, 0)
	if e != nil {
		t.Fatal(e)
	}
	if known[g.id] != 0 {
		t.Fatalf("job%d skill%d 在 75 级进入 known", g.job, g.id)
	}
	st.Level = byte(g.level)
	known, e = s.knownSkills(role, st, 0)
	if e != nil {
		t.Fatal(e)
	}
	if known[g.id] != g.rank {
		t.Fatalf("job%d skill%d 在 %d 级未进入 known：%v", g.job, g.id, g.level, known[g.id])
	}
}

// 全目录扫描：115 级三觉角色必须拿到 .chr 觉醒段里的每一条授予，且 skillRows
// 能把这些行（含只有觉醒矩阵的技能）完整下发，不因缺槽位或缺定义而拒绝。
func TestAwakeningGrantsCoverTheWholeSource(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	checked := 0
	for job, prof := range c.Professions {
		role := storage.Character{Profession: job, ConfigVersion: c.Source.Checksum}
		st := State{Level: 115, Awakening: 3, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills}
		for adv, stages := range prof.AwakeningSkills {
			if len(stages[1]) == 0 || len(stages[2]) == 0 || len(stages[3]) == 0 {
				continue
			}
			st.Advancement = adv
			if _, e := s.skillRows(role, st, 0); e != nil {
				t.Fatalf("job%d adv%d: %v", job, adv, e)
			}
			granted, e := s.awakeningSkills(role, st)
			if e != nil {
				t.Fatal(e)
			}
			for stage := byte(1); stage <= 3; stage++ {
				grants := stages[stage]
				for i := 0; i+1 < len(grants); i += 2 {
					id, rank := uint16(grants[i]), byte(grants[i+1])
					if granted[id] != rank {
						t.Fatalf("job%d adv%d stage%d skill%d: 115 级未授予 rank%d（%v）", job, adv, stage, id, rank, granted[id])
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no awakening grants checked")
	}
	t.Logf("115 级三觉覆盖 %d 条觉醒授予", checked)
}

// 觉醒授予是白给的等级：Reset（CMD483）只能退玩家花 SP 买的部分，授予本身必须
// 原样留在存档里，SP 不变。
func TestAwakeningGrantStaysOnReset(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	samples := lateSecondAwakeningGrants(t, s, c)
	if len(samples) == 0 {
		t.Fatal("目录里没有 75 级以上门槛的二觉授予样本")
	}
	g := samples[0]
	role := storage.Character{Profession: g.job, ConfigVersion: c.Source.Checksum}
	st := State{
		Level:         byte(g.level),
		Advancement:   g.adv,
		Awakening:     2,
		SourceSHA256:  g.prof.RawSHA256,
		InitialSkills: g.prof.InitialSkills,
		LearnedSkills: [2]map[uint16]byte{{g.id: g.rank}, {}},
		SkillSlots:    [2]map[uint16]uint16{{g.id: 0}, {}},
	}
	if e := s.resetAutoState(t.Context(), role, &st, 0, 1); e != nil {
		t.Fatal(e)
	}
	if st.LearnedSkills[0][g.id] != g.rank {
		t.Fatalf("觉醒授予被清掉: %v", st.LearnedSkills[0])
	}
	if st.SkillPoints[0] != 0 {
		t.Fatalf("觉醒授予被退成 SP: %d", st.SkillPoints[0])
	}
}
