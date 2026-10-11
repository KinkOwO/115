package database

import (
	"context"
	"dfolan/internal/social"
	"encoding/json"
)

// SocialCharacterInfo is read-only, including for missing accounts. Do not call
// LoadAdventure here: that method intentionally ensures a profile with a write.
func (s *Store) SocialCharacterInfo(ctx context.Context, names []string) ([]social.CharacterInfo, error) {
	rows := make([]social.CharacterInfo, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		raw, err := s.queries.AdventureDataByName(ctx, name)
		if isNoRows(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var data AdventureData
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
		info := social.CharacterInfo{MemberName: name}
		if data.Social != nil {
			if data.Social.Representative != nil {
				info = *data.Social.Representative
			}
			info.MemberName, info.StateMessage = name, data.Social.Message
		}
		rows = append(rows, info)
	}
	return rows, nil
}
