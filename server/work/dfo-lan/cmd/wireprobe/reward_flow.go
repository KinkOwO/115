package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/reward"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"unicode/utf8"
)

// buildRewardService constructs the optional event-triggered Lua reward add-on.
// Its rules are embedded in the binary, so there is no external path to
// resolve. It never fails startup: a compile error only disables the feature
// with a warning.
func buildRewardService(store *database.Store, awarder *inventory.Awarder) *reward.Service {
	if store == nil || awarder == nil {
		return nil
	}
	service, err := reward.New(reward.Options{
		Grant: rewardGrantFunc(store, awarder),
		Mail:  rewardMailFunc(store, awarder),
		Cera:  rewardCeraFunc(store),
		Log:   func(format string, args ...any) { log.Printf(format, args...) },
	})
	if err != nil {
		log.Printf("warning: reward rules disabled: %v", err)
		return nil
	}
	log.Printf("reward rules enabled (embedded scripts)")
	return service
}

// rewardCeraFunc persists account-level cera through the audited, idempotent
// operator-grant path (admin_grants.grant_id is the idempotency gate).
func rewardCeraFunc(store *database.Store) reward.CeraFunc {
	return func(ctx context.Context, r reward.Recipient, key string, amount uint64) error {
		if amount == 0 || amount > math.MaxInt64 {
			return fmt.Errorf("reward cera amount out of range")
		}
		_, err := store.ApplyGrant(ctx, database.Grant{
			ID:        key,
			AccountID: r.AccountID,
			Cera:      int64(amount),
			Operator:  "reward",
			Reason:    "event reward",
		}, nil)
		return err
	}
}

// rewardGrantFunc commits the script's item grants through the existing
// character-event path, which makes the stable key replay-safe.
func rewardGrantFunc(store *database.Store, awarder *inventory.Awarder) reward.GrantFunc {
	return func(ctx context.Context, r reward.Recipient, key string, items []reward.ItemGrant) error {
		_, _, err := store.CommitCharacterEvent(ctx, r.AccountID, r.CharacterID, r.ConfigVersion, key, "reward-item-v1",
			func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				state := current.State
				receipt := make([]map[string]any, 0, len(items))
				for _, item := range items {
					updated, granted, grantErr := awarder.Grant(state, item.ID, item.Count)
					if grantErr != nil {
						return nil, nil, grantErr
					}
					state = updated
					receipt = append(receipt, map[string]any{"template": item.ID, "amount": item.Count, "slots": granted.Slots})
				}
				raw, marshalErr := json.Marshal(map[string]any{"items": receipt})
				if marshalErr != nil {
					return nil, nil, marshalErr
				}
				return state, raw, nil
			})
		return err
	}
}

// rewardMailFunc delivers a SYSTEM mail (sender_id NULL). Store.SendMail is
// sender-oriented and rejects self-send, so this inserts directly inside the
// same character-event transaction used by the Odyssey honor mail.
func rewardMailFunc(store *database.Store, awarder *inventory.Awarder) reward.MailFunc {
	return func(ctx context.Context, r reward.Recipient, key string, mail reward.MailReward) error {
		attachments, err := rewardMailAssets(awarder, mail)
		if err != nil {
			return err
		}
		subject := truncateUTF8(mail.Subject, 29)
		body := truncateUTF8(mail.Body, 512)
		_, _, err = store.CommitSystemMail(ctx, r.AccountID, r.CharacterID, r.ConfigVersion, key, "reward-mail-v1", subject, body, attachments)
		return err
	}
}

// rewardMailAssets builds the mail attachments. A stackable becomes a mail
// stack, anything else (equipment, pet gear) is materialised through the
// equipment catalog's reward rule so the claim path can rebuild the instance.
func rewardMailAssets(awarder *inventory.Awarder, mail reward.MailReward) ([]database.MailAsset, error) {
	if len(mail.Attachments) == 0 {
		return nil, nil
	}
	assets := make([]database.MailAsset, 0, len(mail.Attachments))
	for i, item := range mail.Attachments {
		if item.ID == 0 || item.Count == 0 {
			return nil, fmt.Errorf("reward mail attachment %d is invalid", i+1)
		}
		attachment, err := rewardMailItem(awarder, item)
		if err != nil {
			return nil, err
		}
		if _, err := attachment.Row(); err != nil {
			return nil, err
		}
		raw, err := json.Marshal(attachment)
		if err != nil {
			return nil, err
		}
		assets = append(assets, database.MailAsset{Item: raw})
	}
	return assets, nil
}

func rewardMailItem(awarder *inventory.Awarder, item reward.ItemGrant) (inventory.MailItem, error) {
	if awarder != nil && awarder.Catalog.Items[item.ID].Kind == "stackable" {
		return inventory.MailItem{Stack: &inventory.BagItem{Template: item.ID, Amount: item.Count, ExpireTime: inventory.GrantExpireTime}}, nil
	}
	if awarder == nil || awarder.Equipment == nil {
		return inventory.MailItem{}, fmt.Errorf("reward mail equipment source missing")
	}
	if item.Count != 1 {
		return inventory.MailItem{}, fmt.Errorf("reward mail equipment attachment must be count 1")
	}
	durability, err := awarder.Equipment.Reward(item.ID)
	if err != nil {
		return inventory.MailItem{}, err
	}
	return inventory.MailItem{Equipment: &inventory.BagEquipment{Template: item.ID, Durability: durability}}, nil
}

// truncateUTF8 limits s to at most limit octets without splitting a rune, so
// PostgreSQL text columns always receive valid UTF-8.
func truncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	b := []byte(s)[:limit]
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}
