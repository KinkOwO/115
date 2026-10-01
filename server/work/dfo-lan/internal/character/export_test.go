package character

var (
	AutomaticSkillFixtureForTest = autoSkillFixture
	BuffFixtureForTest           = buffFixture
	OdysseyGrowthFixtureForTest  = odysseyGrowthFixture
	AwakeningGrantFixtureForTest = loadAwakeningGrantFixture
)

func KnownSkillsForTest(s *Service, role Character, state State, tree int) (map[uint16]byte, error) {
	return s.knownSkills(role, state, tree)
}

func DependentSkillForTest(s *Service, st State, known map[uint16]byte) (uint16, int) {
	var dependent uint16
	var cost int
	for id, d := range s.Learning.index[11] {
		pre := d.Ints("[pre required skill]")
		for i := 0; i+1 < len(pre); i += 2 {
			if pre[i] == 62 && known[id] == 0 {
				if c, e := d.costForState(st, 1, known); e == nil && (dependent == 0 || id < dependent) {
					dependent, cost = id, c
				}
			}
		}
	}
	return dependent, cost
}
