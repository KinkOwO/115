package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/mail"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrMailRecipient = mail.ErrRecipient
	ErrMailSelf      = mail.ErrSelf
	ErrMailFull      = mail.ErrFull
)

type MailAsset = mail.Asset
type MailMessage = mail.Message
type MailSendReceipt = mail.SendReceipt

// 只新增邮件表与序列，不修改已有角色和背包存档。附件采用同一编号
// 序列，避免正文编号和附件编号冲突；已领取的附件保留记录供审计。
func (s *Store) MigrateMailbox(ctx context.Context) error {
	return s.execMigration(ctx, "0036_mailbox.sql")
}

func (s *Store) MailRecipient(ctx context.Context, name string) (Character, error) {
	row, err := s.queries.MailRecipient(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Character{}, ErrMailRecipient
	}
	return storedCharacter(sqlcgen.CharactersRow(row)), err
}

func storedMail(row sqlcgen.MailboxRow) (MailMessage, error) {
	m := MailMessage{ID: row.ID, SenderID: row.SenderID, RecipientID: row.RecipientID,
		SenderName: row.SenderName, Text: row.Body, Status: uint16(row.Status), ExpiresAt: row.ExpiresAt, Deleted: row.Deleted}
	err := json.Unmarshal(row.Assets, &m.Assets)
	return m, err
}

// MailboxDeliveryState 在同一快照中读取最新投递编号和未读数。
// 已读、领取不会产生新编号；调用方保留编号高水位，删除或过期也不会重复提醒。
func (s *Store) MailboxDeliveryState(ctx context.Context, account, id int64) (int64, uint16, error) {
	row, err := s.queries.MailboxDeliveryState(ctx, sqlcgen.MailboxDeliveryStateParams{AccountID: account, CharacterID: id})
	if err != nil {
		return 0, 0, err
	}
	if row.Unread < 0 || row.Unread > 65535 {
		return 0, 0, fmt.Errorf("mail unread count out of range: %d", row.Unread)
	}
	return row.LatestID, uint16(row.Unread), nil
}

func (s *Store) Mailbox(ctx context.Context, account, id int64) ([]MailMessage, error) {
	rows, err := s.queries.Mailbox(ctx, sqlcgen.MailboxParams{AccountID: account, CharacterID: id})
	if err != nil {
		return nil, err
	}
	out := []MailMessage{}
	for _, row := range rows {
		m, err := storedMail(row)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// SendMail 按角色编号顺序加锁，扣除发件人资产与入箱、幂等回执在同一
// 事务内提交。收件人只新增邮件，不覆盖在线角色缓存中的背包状态。
func (s *Store) SendMail(ctx context.Context, account, id int64, version, key, name, body string,
	prepare func(Character) (json.RawMessage, []MailAsset, error)) (Character, MailSendReceipt, bool, error) {
	var role Character
	var receipt MailSendReceipt
	fail := func(err error) (Character, MailSendReceipt, bool, error) { return Character{}, receipt, false, err }
	if account <= 0 || id <= 0 || key == "" || len(key) > 200 || prepare == nil || len(body) > 512 {
		return fail(fmt.Errorf("邮件事务参数无效"))
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return fail(err)
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	recipient, err := q.MailRecipientID(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrMailRecipient)
	}
	if err != nil {
		return fail(err)
	}
	if recipient == id {
		return fail(ErrMailSelf)
	}
	rows, err := q.LockMailCharacters(ctx, []int64{id, recipient})
	if err != nil {
		return fail(err)
	}
	var receiver Character
	for _, row := range rows {
		c := storedCharacter(sqlcgen.CharactersRow(row))
		if c.ID == id {
			role = c
		} else {
			receiver = c
		}
	}
	count := len(rows)
	if count != 2 || role.AccountID != account || role.ConfigVersion != version || receiver.ConfigVersion != version {
		return fail(fmt.Errorf("邮件角色归属或配置版本无效"))
	}
	prior, err := q.MailSendReceipt(ctx, sqlcgen.MailSendReceiptParams{CharacterID: id, EventKey: key})
	if err == nil {
		if err = json.Unmarshal(prior, &receipt); err != nil {
			return fail(err)
		}
		return role, receipt, false, tx.commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	state, assets, err := prepare(role)
	if err != nil {
		return fail(err)
	}
	if !json.Valid(state) || len(assets) > 11 || len(role.Name) > 29 {
		return fail(fmt.Errorf("邮件资产或发件人名字无效"))
	}
	capacity, err := q.MailboxCapacity(ctx, recipient)
	if err != nil {
		return fail(err)
	}
	if capacity.Messages >= 255 || capacity.UnclaimedAssets+int64(len(assets)) > 255 {
		return fail(ErrMailFull)
	}
	for i := range assets {
		if assets[i].Claimed || (assets[i].Gold == 0 && len(assets[i].Item) == 0) || (len(assets[i].Item) > 0 && !json.Valid(assets[i].Item)) {
			return fail(fmt.Errorf("待发邮件附件无效"))
		}
		if assets[i].ID, err = q.NextMailID(ctx); err != nil {
			return fail(err)
		}
	}
	if assets == nil {
		assets = []MailAsset{}
	}
	encoded, err := json.Marshal(assets)
	if err != nil {
		return fail(err)
	}
	// 当前客户端 DSTR 23016 明确说明未读邮件和附件保留 15 天。
	receipt.RecipientID = recipient
	receipt.MessageID, err = q.InsertPlayerMail(ctx, sqlcgen.InsertPlayerMailParams{SenderID: pgtype.Int8{Int64: id, Valid: true}, RecipientID: recipient, SenderName: role.Name, Body: body, Assets: encoded})
	if err != nil {
		return fail(err)
	}
	if err = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return fail(err)
	}
	prior, err = json.Marshal(receipt)
	if err != nil {
		return fail(err)
	}
	if err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: "mail-send-v1", Outcome: prior}); err != nil {
		return fail(err)
	}
	if err = tx.commit(ctx); err != nil {
		return fail(err)
	}
	role.State = state
	return role, receipt, true, nil
}

// insertSystemMailTx inserts one system mail (sender_id NULL) inside an
// existing transaction under the mail-owned capacity and retention rules.
// It assigns the shared mailbox_id_seq IDs and returns the new message id.
func insertSystemMailTx(ctx context.Context, tx querySet, recipientID int64, senderName, body string, assets []mail.Asset) (int64, error) {
	if len(assets) > mail.MaxAttachments {
		return 0, fmt.Errorf("系统邮件附件过多")
	}
	q := tx
	capacity, err := q.MailboxCapacity(ctx, recipientID)
	if err != nil {
		return 0, err
	}
	if capacity.Messages >= mail.MaxMessages || capacity.UnclaimedAssets+int64(len(assets)) > mail.MaxUnclaimedAssets {
		return 0, ErrMailFull
	}
	for i := range assets {
		if assets[i].ID, err = q.NextMailID(ctx); err != nil {
			return 0, err
		}
	}
	encoded, err := json.Marshal(assets)
	if err != nil {
		return 0, err
	}
	if assets == nil {
		encoded = []byte("[]")
	}
	return q.InsertSystemMail(ctx, sqlcgen.InsertSystemMailParams{RecipientID: recipientID, SenderName: senderName, Body: body, Assets: encoded})
}

// CommitSystemMail creates one system mail for a recipient under the caller's
// idempotency key/model (e.g. reward-mail-v1). Returns the new message id.
func (s *Store) CommitSystemMail(ctx context.Context, account, id int64, version, key, model, senderName, body string, assets []MailAsset) (int64, bool, error) {
	var messageID int64
	_, applied, err := s.CommitCharacterEventTx(ctx, account, id, version, key, model,
		func(tx *Tx, role Character) (json.RawMessage, json.RawMessage, error) {
			mid, err := insertSystemMailTx(ctx, tx.queries, id, senderName, body, assets)
			if err != nil {
				return nil, nil, err
			}
			messageID = mid
			receipt, err := json.Marshal(map[string]any{"mail_id": mid})
			return role.State, receipt, err
		})
	if err != nil {
		return 0, false, err
	}
	return messageID, applied, nil
}

// MutateMailbox 将领取入包、附件已领取标记、邮件状态和操作回执原子
// 提交。所有邮件均在当前账号角色下读取，客户端不能越权指定收件人。
func (s *Store) MutateMailbox(ctx context.Context, account, id int64, version, key, model string, messageIDs []int64,
	apply func(Character, []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error)) (Character, json.RawMessage, bool, error) {
	fail := func(err error) (Character, json.RawMessage, bool, error) { return Character{}, nil, false, err }
	if account <= 0 || id <= 0 || key == "" || len(key) > 200 || (model != "mail-claim-v1" && model != "mail-status-v1") || apply == nil {
		return fail(fmt.Errorf("邮件更新参数无效"))
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return fail(err)
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	role, err := lockCharacter(ctx, tx, account, id)
	if err != nil {
		return fail(err)
	}
	if role.ConfigVersion != version {
		return fail(fmt.Errorf("邮件角色配置版本不匹配"))
	}
	prior, err := q.StoredCharacterEvent(ctx, sqlcgen.StoredCharacterEventParams{CharacterID: id, EventKey: key})
	if err == nil {
		if prior.Model != model {
			return fail(fmt.Errorf("邮件操作流水冲突"))
		}
		return role, prior.Outcome, false, tx.commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	// 删除回调可能紧接着领取回执到达；按请求编号读取软删除记录，才能
	// 幂等应答客户端的后续删除，而不把“已领取后自动删除”误报为失败。
	rows, err := q.LockMailbox(ctx, sqlcgen.LockMailboxParams{RecipientID: id, IncludeDeleted: model == "mail-status-v1", MessageIds: messageIDs})
	if err != nil {
		return fail(err)
	}
	messages := []MailMessage{}
	for _, row := range rows {
		m, err := storedMail(sqlcgen.MailboxRow(row))
		if err != nil {
			return fail(err)
		}
		messages = append(messages, m)
	}
	state, changed, receipt, err := apply(role, messages)
	if err != nil {
		return fail(err)
	}
	if !json.Valid(state) || !json.Valid(receipt) {
		return fail(fmt.Errorf("邮件事务结果无效"))
	}
	for _, m := range changed {
		assets, e := json.Marshal(m.Assets)
		if e != nil {
			return fail(e)
		}
		affected, e := q.UpdateMail(ctx, sqlcgen.UpdateMailParams{MailID: m.ID, RecipientID: id, Assets: assets, Status: int32(m.Status), Deleted: m.Deleted})
		if e != nil {
			return fail(e)
		}
		if affected != 1 {
			return fail(fmt.Errorf("邮件状态已经改变"))
		}
	}
	if err = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return fail(err)
	}
	if err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: model, Outcome: receipt}); err != nil {
		return fail(err)
	}
	if err = tx.commit(ctx); err != nil {
		return fail(err)
	}
	role.State = state
	return role, receipt, true, nil
}
