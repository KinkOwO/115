package inventory

import "encoding/json"

// Role is the character projection consumed by inventory rules. It carries no
// persistence identity or lifecycle fields; workflows retain the complete role.
type Role struct {
	AccountID     int64
	Profession    byte
	ConfigVersion string
	State         json.RawMessage
}

// VaultState is the saved state of one personal vault.
type VaultState struct {
	Slots         uint16
	Items         json.RawMessage
	ConfigVersion string
}

// AccountVaultState is the account-shared vault state.
type AccountVaultState struct {
	Slots uint16
	Gold  uint32
	Items json.RawMessage
}
