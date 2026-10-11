// Package social contains account community settings, not character growth rules.
package social

// CharacterInfo is the req_type=6 character_info record consumed by
// 140477410. All strings are required: the native reader calls strlen on them.
// The avatar vectors have exactly twelve entries; the reader has no upper bound.
type CharacterInfo struct {
	MemberName          string     `json:"member_name"`
	ServerGroup         byte       `json:"server_group"`
	AvatarColor1        [12]uint16 `json:"avatar_color1"`
	AvatarColor2        [12]uint16 `json:"avatar_color2"`
	AvatarIndex         [12]uint32 `json:"avatar_index"`
	GrowthType          byte       `json:"growth_type"`
	Job                 byte       `json:"job"`
	StateMessage        string     `json:"state_message"`
	WeaponIndex         uint32     `json:"weapon_index"`
	WeaponSkinIndex     uint32     `json:"weapon_skin_index"`
	CharacterName       string     `json:"charac_name"`
	ExpertJob           uint32     `json:"expert_job"`
	ExpertJobExperience uint32     `json:"expert_job_exp"`
	Fame                uint32     `json:"fame_value"`
	Level               byte       `json:"level"`
	Reputation          uint32     `json:"reputation"`
}

type Group struct {
	Color uint32 `json:"color"`
	Order byte   `json:"order"`
	Name  string `json:"name"`
}

// Profile is an optional additive field in account_adventures.data. Old saves
// need no migration; these account settings are not stored in character JSON.
type Profile struct {
	RepresentativeID int64          `json:"representative_id,omitempty"`
	Representative   *CharacterInfo `json:"representative,omitempty"`
	Message          string         `json:"message,omitempty"`
	Groups           *[8]Group      `json:"groups,omitempty"`
}
