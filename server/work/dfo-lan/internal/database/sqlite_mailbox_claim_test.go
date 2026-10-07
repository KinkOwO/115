package database

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/database/sqlcgen"
)

// Claiming a mail attachment (CMD95) calls MutateMailbox with a nil message-id set, which
// means "no id filter: read every live message" - the PostgreSQL query spells that
// `($3::bigint[] IS NULL OR id=ANY($3))`, the SQLite fork
// `(CAST(?3 AS TEXT) IS NULL OR id IN (SELECT value FROM json_each(CAST(?3 AS TEXT))))`.
//
// The adapter used to marshal that nil slice into the four-byte JSON text "null", which is
// not SQL NULL: the predicate fell through to json_each('null'), matched no row, and the
// claim read an empty mailbox. The gateway then rejected every claim with
// "邮件不存在、已过期或不属于当前角色" (errMailMissing) even though the client had just been
// handed the attachment id by the mailbox list - the reported "接收邮件提示无邮件".
func TestSQLiteMailClaimReadsTheMailboxWithoutAnIDFilter(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "claim.db"), MaxConnections: 2})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	account, err := store.DevelopmentAccount(ctx, "mail-claim")
	if err != nil {
		t.Fatal(err)
	}
	roles := make([]Character, 2)
	for i := range roles {
		roles[i], err = store.CreateCharacter(ctx, Character{
			AccountID: account, Name: fmt.Sprintf("ClaimRole%d", i),
			ConfigVersion: strings.Repeat("a", 64), Request: []byte{0, 255},
			State: json.RawMessage(`{"unknown":{"keep":true},"gold":100}`),
		}, 4)
		if err != nil {
			t.Fatal(err)
		}
	}
	receiver := roles[1]

	// A system mail with one attachment, exactly as the reward/GM path delivers it.
	if _, _, err = store.CommitSystemMail(ctx, account, receiver.ID, receiver.ConfigVersion,
		"claim-mail", "reward-mail-v1", "GM", "hello", []MailAsset{{Gold: 5}}); err != nil {
		t.Fatalf("CommitSystemMail: %v", err)
	}
	inbox, err := store.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 1 {
		t.Fatalf("inbox = %+v, %v; want one message", inbox, err)
	}
	attachment := inbox[0].Assets[0].ID
	if attachment == 0 {
		t.Fatalf("attachment id was not allocated: %+v", inbox[0].Assets)
	}

	// The claim flow resolves the client's attachment id inside the id set it is given; a
	// missing message is what the gateway turns into errMailMissing.
	seen := -1
	_, _, applied, err := store.MutateMailbox(ctx, account, receiver.ID, receiver.ConfigVersion,
		"claim-1", "mail-claim-v1", nil,
		func(c Character, messages []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error) {
			seen = len(messages)
			index := map[int64]bool{}
			for _, m := range messages {
				for _, a := range m.Assets {
					index[a.ID] = true
				}
			}
			if !index[attachment] {
				return nil, nil, nil, fmt.Errorf("邮件不存在、已过期或不属于当前角色")
			}
			changed := make([]MailMessage, 0, len(messages))
			for _, m := range messages {
				m.Status = 2
				for i := range m.Assets {
					m.Assets[i].Claimed = true
				}
				changed = append(changed, m)
			}
			return c.State, changed, json.RawMessage(`{"claimed":true}`), nil
		})
	if seen != 1 {
		t.Fatalf("the claim callback saw %d message(s) with a nil id filter; want 1", seen)
	}
	if err != nil || !applied {
		t.Fatalf("claim: applied=%v err=%v", applied, err)
	}

	// Claiming is a status change, not a delete: the message stays readable and now reads 2.
	inbox, err = store.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 1 || inbox[0].Status != 2 {
		t.Fatalf("after the claim: %+v, %v; want one read message", inbox, err)
	}
	if claimed := inbox[0].Assets[0].Claimed; !claimed {
		t.Errorf("attachment was not marked claimed: %+v", inbox[0].Assets)
	}
}

// The nil/empty distinction is semantic, not cosmetic: nil means "no filter" while an empty
// set means "match nothing" (PostgreSQL: $3 IS NULL versus id=ANY('{}')). Both must hold.
func TestSQLiteLockMailboxDistinguishesNilFromAnEmptyIDSet(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "filter.db"), MaxConnections: 2})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	account, err := store.DevelopmentAccount(ctx, "mail-filter")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID: account, Name: "FilterRole", ConfigVersion: strings.Repeat("b", 64),
		Request: []byte{1}, State: json.RawMessage(`{"gold":1}`),
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	messageID, _, err := store.CommitSystemMail(ctx, account, role.ID, role.ConfigVersion,
		"filter-mail", "reward-mail-v1", "GM", "body", []MailAsset{{Gold: 1}})
	if err != nil {
		t.Fatal(err)
	}

	all, err := store.queries.LockMailbox(ctx, sqlcgen.LockMailboxParams{RecipientID: role.ID})
	if err != nil {
		t.Fatalf("LockMailbox(nil): %v", err)
	}
	if len(all) != 1 || all[0].ID != messageID {
		t.Fatalf("LockMailbox(nil) = %d row(s), want the one live message %d", len(all), messageID)
	}
	none, err := store.queries.LockMailbox(ctx, sqlcgen.LockMailboxParams{RecipientID: role.ID, MessageIds: []int64{}})
	if err != nil {
		t.Fatalf("LockMailbox(empty): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("LockMailbox(empty set) = %d row(s); an empty id set must match nothing", len(none))
	}
	one, err := store.queries.LockMailbox(ctx, sqlcgen.LockMailboxParams{RecipientID: role.ID, MessageIds: []int64{messageID}})
	if err != nil || len(one) != 1 || one[0].ID != messageID {
		t.Errorf("LockMailbox({%d}) = %d row(s), %v; want that one message", messageID, len(one), err)
	}
}
