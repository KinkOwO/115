package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

var errAdventureIdentityInvalid = errors.New("冒险团设置字段无效")

func (w *worldSession) adventureIdentityPlan(ctx context.Context) ([]outboundPacket, error) {
	p, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	name, err := protocol.AdventureNameInfo(p.Name, !p.Data.NameChosen)
	if err != nil {
		return nil, err
	}
	elite, err := w.adventureElitePayload(ctx, p)
	if err != nil {
		return nil, err
	}
	season, err := w.initialSeasonPlan(ctx)
	if err != nil {
		return nil, err
	}
	// Own-account CMD1395 is already handled after character bootstrap.
	// Restore here as well: periodic refresh waits for the first CMD35,
	// but the player can open the elite page without moving after login.
	packets := []outboundPacket{{"冒险团名称初始化", 0, 1363, name}, {"冒险团徽章恢复", 0, 1760, protocol.AdventureSymbolInfo(p.Data.SymbolID)}, {"精锐角色设置恢复", 0, 1754, elite}}
	return append(packets, season...), nil
}

func (w *worldSession) handleAdventureIdentity(ctx context.Context, id uint16, p, raw []byte, prefix string) ([]outboundPacket, error) {
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	if id == 1430 || id == 1436 {
		name, err := protocol.DecodeAdventureName(p)
		if err != nil {
			return nil, errAdventureIdentityInvalid
		}
		if id == 1436 {
			available, err := w.store.AdventureNameAvailable(ctx, w.account, name)
			if err != nil {
				return nil, err
			}
			if !available {
				return nil, database.ErrAdventureNameTaken
			}
			accepted, err := protocol.AdventureNameAccepted(name)
			if err != nil {
				return nil, err
			}
			return []outboundPacket{{"冒险团名称校验", 1, id, accepted}}, nil
		}
		key := fmt.Sprintf("adventure-name:%s:%x", prefix, sha256.Sum256(raw))
		_, profile, _, err = w.store.CommitAdventure(ctx, w.account, w.role.ID, key, func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			if a.Data.NameChosen && a.Name != name {
				return nil, nil, errAdventureIdentityInvalid
			}
			a.Name, a.Data.NameChosen = name, true
			return role.State, json.RawMessage(`{}`), nil
		})
		if err != nil {
			return nil, err
		}
		nameInfo, err := protocol.AdventureNameInfo(profile.Name, false)
		if err != nil {
			return nil, err
		}
		accepted, err := protocol.AdventureNameAccepted(profile.Name)
		if err != nil {
			return nil, err
		}
		return []outboundPacket{{"冒险团命名成功", 1, id, accepted}, {"冒险团名称保存", 0, 1363, nameInfo}}, nil
	}
	if id != 1726 {
		return nil, errAdventureIdentityInvalid
	}
	symbol, err := protocol.DecodeAdventureSymbol(p)
	if err != nil {
		return nil, errAdventureIdentityInvalid
	}
	rules, err := adventure.Current()
	if err != nil {
		return nil, err
	}
	if !rules.Symbols[symbol] {
		return nil, errAdventureIdentityInvalid
	}
	key := fmt.Sprintf("adventure-symbol:%s:%d:%x", prefix, profile.Data.SettingsRevision, sha256.Sum256(raw))
	_, profile, _, err = w.store.CommitAdventure(ctx, w.account, w.role.ID, key, func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		if a.Data.SymbolID != symbol {
			if a.Data.SettingsRevision == math.MaxUint64 {
				return nil, nil, errAdventureIdentityInvalid
			}
			a.Data.SettingsRevision++
		}
		a.Data.SymbolID = symbol
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"冒险团徽章保存", 1, id, protocol.AdventureSymbolChanged(profile.Data.SymbolID)}}, nil
}

func (client *gameConnection) dispatchAdventureIdentity(r *clientRequest) dispatchAction {
	id := r.frame.ID
	if !client.bootstrapped || client.selectedCharacterID == 0 || id != 1430 && id != 1436 && id != 1726 {
		return dispatchNext
	}
	if !r.verified {
		client.event(map[string]any{"kind": "adventure_identity_rejected", "id": id, "reason": "checksum failed"})
		return dispatchHandled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	packets, err := client.worldState.handleAdventureIdentity(ctx, id, r.plaintext, r.frame.Raw, client.purchaseSession.prefix)
	cancel()
	if err != nil {
		client.event(map[string]any{"kind": "adventure_identity_rejected", "id": id, "reason": err.Error()})
		// 14528B402 and dstr 100000051..53: 1 duplicate, 3 retry,159 invalid.
		code := uint16(3)
		if errors.Is(err, database.ErrAdventureNameTaken) {
			code = 1
		}
		if errors.Is(err, errAdventureIdentityInvalid) {
			code = 159
		}
		if id == 1726 {
			code = 13
		} // 142F9F3B9 supported failure.
		if client.output.send(1, id, []byte{0, byte(code), byte(code >> 8)}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.sendPlan(packets, client.logCharacterResponseBody) != nil {
		return dispatchClose
	}
	client.event(map[string]any{"kind": "adventure_identity_saved", "id": id, "character_id": client.selectedCharacterID})
	return dispatchHandled
}
