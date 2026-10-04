// Package mail owns the mail envelope types, capacity/retention/postage rules
// and persistence model strings. It imports only the standard library.
package mail

import (
	"encoding/json"
	"errors"
	"time"
)

// Asset is one mail attachment slot. Item is an opaque marshalled
// inventory.MailItem and must never be parsed here. Its JSON tags are the
// persisted assets jsonb encoding.
type Asset struct {
	ID      int64           `json:"id"`
	Gold    uint32          `json:"gold,omitempty"`
	Item    json.RawMessage `json:"item,omitempty"`
	Claimed bool            `json:"claimed,omitempty"`
}

type Message struct {
	ID          int64     `json:"id"`
	SenderID    int64     `json:"sender_id"`
	RecipientID int64     `json:"recipient_id"`
	SenderName  string    `json:"sender_name"`
	Text        string    `json:"text"`
	Status      uint16    `json:"status"`
	Assets      []Asset   `json:"assets"`
	ExpiresAt   time.Time `json:"expires_at"`
	Deleted     bool      `json:"deleted,omitempty"`
}

// SendReceipt is persisted verbatim as the mail-send-v1 event outcome. It has
// NO json tags on purpose: historical rows use {"MessageID":...,"RecipientID":...}.
type SendReceipt struct {
	MessageID   int64
	RecipientID int64
}

var (
	ErrRecipient = errors.New("收件角色不存在")
	ErrSelf      = errors.New("不能给自己发送邮件")
	ErrFull      = errors.New("收件箱已满")
)

const (
	MaxMessages        = 255
	MaxUnclaimedAssets = 255
	MaxAttachments     = 11
	RetentionDays      = 15
	ModelSend          = "mail-send-v1"
	ModelClaim         = "mail-claim-v1"
	ModelStatus        = "mail-status-v1"
)

// Postage is the mail send fee: base 100, 1000 per item attachment, plus 5%
// of sent gold (floor), capped at 10000.
func Postage(gold uint32, itemCount int) uint64 {
	return 100 + uint64(itemCount)*1000 + uint64(min(gold/20, 10000))
}
