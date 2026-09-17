package main

import "os"

func odysseyRewardsEnabled() bool {
	return os.Getenv("DFO_ODYSSEY_REWARDS_PILOT") == "1" || os.Getenv("DFO_ODYSSEY_REWARDS_RELEASE") == "1"
}

func odysseyTemporaryCreditsEnabled() bool {
	return os.Getenv("DFO_ODYSSEY_REWARDS_PILOT") == "1" || (odysseyRewardsEnabled() && os.Getenv("DFO_ODYSSEY_TEMPORARY_CREDITS") == "1")
}
