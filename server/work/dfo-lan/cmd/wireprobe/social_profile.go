package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/social"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// NOTI1363's native handler writes CommunityManager's own name. Both first
// open and successful profile ACKs query HTTP using that name. Initialize it
// after character entry, independently of opening the adventure window.
func (client *gameConnection) restoreSocialIdentity() error {
	if client.gatewayRuntime == nil || !client.socialProfiles || client.worldState == nil || client.selectedCharacterID == 0 || client.selectedCharacterID != client.worldState.role.ID {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	profile, err := client.worldState.prepareAdventure(ctx)
	cancel()
	if err != nil {
		client.event(map[string]any{"kind": "social_identity_restore_pending", "reason": err.Error(), "character_id": client.selectedCharacterID})
		return nil
	}
	// Unnamed accounts retain the existing adventure naming flow. Its choose
	// flag opens a dialog, so it must not be sent as a passive login restore.
	if !profile.Data.NameChosen {
		return nil
	}
	body, err := protocol.AdventureNameInfo(profile.Name, false)
	if err != nil {
		client.event(map[string]any{"kind": "social_identity_restore_pending", "reason": err.Error(), "character_id": client.selectedCharacterID})
		return nil
	}
	if err := client.output.send(0, 1363, body); err != nil {
		return err
	}
	// Do not set adventureIdentitySent: badges, elite slots and season state
	// still need their existing first-adventure-window restore.
	client.event(map[string]any{"kind": "social_identity_restored", "id": 1363, "character_id": client.selectedCharacterID, "attempt": "3/3"})
	return nil
}

func (w *worldSession) handleSocialProfile(ctx context.Context, id uint16, p, raw []byte, prefix string) ([]outboundPacket, error) {
	if w.role.ID == 0 || w.role.AccountID != w.account || w.characters == nil {
		return nil, fmt.Errorf("community character ownership unavailable")
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	if id == 1677 {
		if len(p) != 0 {
			return nil, fmt.Errorf("unexpected community refresh payload")
		}
		var groups *[8]social.Group
		if profile.Data.Social != nil {
			groups = profile.Data.Social.Groups
		}
		body, err := protocol.SocialGroupInfo(groups)
		return []outboundPacket{{"社交分组恢复", 0, 1682, body}}, err
	}
	var apply func(*social.Profile, database.Character) error
	switch id {
	case 2387:
		kind, _, err := protocol.DecodeSocialRepresentative(p)
		if err != nil {
			return nil, err
		}
		if kind != 0 {
			return nil, fmt.Errorf("season costume profile selection requires native source mapping")
		}
		apply = func(s *social.Profile, role database.Character) error {
			// CommitAdventure locks the current owned role; do not read a stale
			// world snapshot or interpret the thirteen opaque bytes as its ID.
			info, err := w.characters.SocialCharacterInfo(role)
			if err != nil {
				return err
			}
			s.RepresentativeID, s.Representative = role.ID, &info
			return nil
		}
	case 2388:
		message, err := protocol.DecodeSocialMessage(p)
		if err != nil {
			return nil, err
		}
		apply = func(s *social.Profile, _ database.Character) error { s.Message = message; return nil }
	case 1676:
		groups, err := protocol.DecodeSocialGroups(p)
		if err != nil {
			return nil, err
		}
		apply = func(s *social.Profile, _ database.Character) error { s.Groups = &groups; return nil }
	default:
		return nil, fmt.Errorf("unsupported community profile command")
	}
	key := fmt.Sprintf("community:%d:%s:%d:%x", id, prefix, profile.Data.SettingsRevision, sha256.Sum256(raw))
	_, profile, _, err = w.store.CommitAdventure(ctx, w.account, w.role.ID, key, func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		if a.Data.SettingsRevision == math.MaxUint64 {
			return nil, nil, fmt.Errorf("community settings revision overflow")
		}
		if a.Data.Social == nil {
			a.Data.Social = &social.Profile{}
		}
		if err := apply(a.Data.Social, role); err != nil {
			return nil, nil, err
		}
		a.Data.SettingsRevision++
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"社交资料保存", 1, id, []byte{1}}}
	if id == 1676 {
		body, err := protocol.SocialGroupInfo(profile.Data.Social.Groups)
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"社交分组刷新", 0, 1682, body})
	}
	return packets, nil
}

func (client *gameConnection) dispatchSocialProfile(r *clientRequest) dispatchAction {
	id := r.frame.ID
	if id != 2387 && id != 2388 && id != 1676 && id != 1677 {
		return dispatchNext
	}
	if client.gatewayRuntime == nil || !client.socialProfiles || !client.bootstrapped || client.selectedCharacterID == 0 || client.worldState == nil || client.worldState.store == nil {
		return dispatchNext
	}
	if !r.verified {
		client.event(map[string]any{"kind": "social_profile_rejected", "id": id, "reason": "checksum failed"})
		return dispatchHandled
	}
	if client.selectedCharacterID != client.worldState.role.ID {
		client.event(map[string]any{"kind": "social_profile_rejected", "id": id, "reason": "selected role mismatch"})
		return dispatchHandled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	prefix := client.peer
	if client.purchaseSession != nil {
		prefix = client.purchaseSession.prefix
	}
	packets, err := client.worldState.handleSocialProfile(ctx, id, r.plaintext, r.frame.Raw, prefix)
	cancel()
	if err != nil {
		client.event(map[string]any{"kind": "social_profile_rejected", "id": id, "reason": err.Error()})
		// These two registered native failure handlers consume 159 (invalid
		// text). Other commands' error semantics remain unclosed.
		if id == 1676 || id == 2388 {
			if client.output.send(1, id, []byte{0, 159, 0}) != nil {
				return dispatchClose
			}
		}
		// Unclosed failure contracts do not receive a fabricated success.
		return dispatchHandled
	}
	if client.sendPlan(packets, client.logCharacterResponseBody) != nil {
		return dispatchClose
	}
	client.event(map[string]any{"kind": "social_profile_saved", "id": id, "character_id": client.selectedCharacterID, "attempt": "3/3"})
	return dispatchHandled
}
