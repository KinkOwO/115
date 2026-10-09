package legion

// Native14770A410 maps names to the wire namespace; COS row order differs.
func BakalBuffType(name string) (uint32, bool) {
	names := []string{"RaidBuffAllImmune", "RaidBuffAddCoin", "RaidBuffAddDamage", "RaidBuffInvincible", "RaidBuffCampTeleport", "SparazziDragonImmune", "SkasaDragonImmune", "HismaDragonImmune", "SparazziDragonDamage", "SkasaDragonDamage", "HismaDragonDamage", "AllDragonImmune", "AddDefense", "CooltimeReduce", "StackablePotion", "AddRandomRaidBuff", "6MSparazziDragonImmune", "6MSkasaDragonImmune", "6MHismaDragonImmune", "4MSparazziDragonImmune", "4MSkasaDragonImmune", "4MHismaDragonImmune", "StrongSparazziDragonDamage", "StrongSkasaDragonDamage", "StrongHismaDragonDamage"}
	for i, n := range names {
		if name == n {
			return uint32(i), true
		}
	}
	return 25, false
}
