package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"strings"
)

type profileFrameStore interface {
	RestoreProfileSkins(context.Context, int64, int64) (character.ProfileSkinState, error)
	SkinSelectionList(context.Context, int64, uint32) ([]uint32, error)
	ListSkins(context.Context, int64) ([]database.AccountSkin, error)
}

// Use the very same saved selection and ownership as the skin-storage window.
// Defaults and the legacy profile snapshot retain their existing save semantics;
// no new state table, skin ID band or separate content list is introduced.
func loadProfileFrames(ctx context.Context, store profileFrameStore, role character.Character,
	entries map[uint32]catalog.SkinStorageEntry) ([2]uint32, error) {
	profile, err := store.RestoreProfileSkins(ctx, role.AccountID, role.ID)
	if err != nil {
		return [2]uint32{}, err
	}
	keys, err := store.SkinSelectionList(ctx, role.ID, protocol.SkinCategoryPartyFrame)
	if err != nil {
		return [2]uint32{}, err
	}
	if len(keys) == 0 {
		return [2]uint32{profile.Selected[0], profile.Selected[1]}, nil
	}
	skins, err := store.ListSkins(ctx, role.AccountID)
	if err != nil {
		return [2]uint32{}, err
	}
	frame, _ := skinFamilyForCategory(protocol.SkinCategoryPartyFrame)
	owned := skinFamilyOwnedSet(skins, entries, frame)
	return profileFrameIDs(keys, skinByID(entries), owned), nil
}

func profileFrameIDs(keys []uint32, byID map[uint32]catalog.SkinStorageEntry, owned map[uint32]bool) [2]uint32 {
	defaults := character.ProfileSkinDefaults().Selected
	frames := [2]uint32{defaults[0], defaults[1]}
	for _, key := range keys {
		if !owned[key] {
			continue
		}
		if key == defaults[0] {
			frames[0] = key
			continue
		}
		if key == defaults[1] {
			frames[1] = key
			continue
		}
		entry, ok := byID[key]
		if !ok || entry.Family() != catalog.SkinFamilyPartyFrame {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(entry.SkinSubType)) {
		case "":
			frames[0] = key
		case "party request frame":
			frames[1] = key
		}
	}
	return frames
}

// Rebuild in town through the proven actor projection. An in-dungeon mode-0
// rebuild is unsafe (CMD19 regression): retain the selection there and let the
// next town/entry projection carry it rather than disrupt the live actor.
func (w *worldSession) profileFrameRefresh() ([]outboundPacket, error) {
	if w.activeDungeon != nil {
		return nil, nil
	}
	basic, err := w.characters.EntryBasicProbe(w.role, [2]byte{})
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"profile_frame_actor_refreshed", 0, 2, basic}}, nil
}
