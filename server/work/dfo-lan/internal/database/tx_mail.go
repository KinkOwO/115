package database

import (
	"context"
	"fmt"
)

// SendSystemMail belongs to the transaction's selected character. It exposes
// no SQL or transaction completion, and leaves all capacity/retention checks
// in the existing mailbox implementation.
func (tx *Tx) SendSystemMail(ctx context.Context, sender, body string, assets []MailAsset) (int64, error) {
	if tx == nil || tx.queries == nil || tx.characterID <= 0 || sender == "" || len(sender) > systemMailSenderNameLimit || len(body) > 512 {
		return 0, fmt.Errorf("invalid transactional system mail")
	}
	return insertSystemMailTx(ctx, tx.queries, tx.characterID, sender, body, append([]MailAsset(nil), assets...))
}
