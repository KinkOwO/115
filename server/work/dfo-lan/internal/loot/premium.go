package loot

// PremiumActivation is the account contract capability consumed by loot.
type PremiumActivation struct {
	Type           uint8
	DurationSecond int64
}

// Premium preserves the existing receipt JSON consumed by loot clients.
type Premium struct {
	Type            uint8 `json:"type"`
	EndTime         int64 `json:"end_time"`
	RemainingSecond int64 `json:"remaining_seconds"`
}
