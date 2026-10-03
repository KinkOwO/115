package storage

import (
	"context"
	"dfolan/internal/db"
	"encoding/json"
	"errors"
	"fmt"

	"dfolan/internal/mail"

	"github.com/jackc/pgx/v5"
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
	_, err := s.DB.Exec(ctx, `CREATE SEQUENCE IF NOT EXISTS mailbox_id_seq;
CREATE TABLE IF NOT EXISTS character_mail (
 id bigint PRIMARY KEY DEFAULT nextval('mailbox_id_seq'),
 sender_id bigint REFERENCES characters(id),
 recipient_id bigint NOT NULL REFERENCES characters(id),
 sender_name text NOT NULL, body text NOT NULL,
 status smallint NOT NULL DEFAULT 1 CHECK(status IN (1,2,3)),
 assets jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(assets)='array'),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 deleted_at timestamptz,
 CHECK(octet_length(sender_name)<=29), CHECK(octet_length(body)<=512));
ALTER TABLE character_mail ALTER COLUMN sender_id DROP NOT NULL;
CREATE INDEX IF NOT EXISTS character_mail_inbox ON character_mail(recipient_id,id) WHERE deleted_at IS NULL;`)
	return err
}

const mailCharacterColumns = `id,account_id,wire_id,name,profession,create_request,config_version,state,created_at`

func scanMailCharacter(row pgx.Row) (Character, error) {
	var c Character
	err := row.Scan(&c.ID, &c.AccountID, &c.WireID, &c.Name, &c.Profession, &c.Request, &c.ConfigVersion, &c.State, &c.CreatedAt)
	return c, err
}
func (s *Store) MailRecipient(ctx context.Context, name string) (Character, error) {
	c, err := scanMailCharacter(s.DB.QueryRow(ctx, `SELECT `+mailCharacterColumns+` FROM characters WHERE lower(name)=lower($1) AND deleted_at IS NULL`, name))
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrMailRecipient
	}
	return c, err
}

func readMailRows(rows pgx.Rows) ([]MailMessage, error) {
	defer rows.Close()
	out := []MailMessage{}
	for rows.Next() {
		var m MailMessage
		var assets []byte
		if err := rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.SenderName, &m.Text, &m.Status, &assets, &m.ExpiresAt, &m.Deleted); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(assets, &m.Assets); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const mailColumns = `m.id,coalesce(m.sender_id,0),m.recipient_id,m.sender_name,m.body,m.status,m.assets,m.expires_at,m.deleted_at IS NOT NULL`

// MailboxDeliveryState 在同一快照中读取最新投递编号和未读数。
// 已读、领取不会产生新编号；调用方保留编号高水位，删除或过期也不会重复提醒。
func (s *Store) MailboxDeliveryState(ctx context.Context, account, id int64) (int64, uint16, error) {
	var latest int64
	var unread uint16
	err := s.DB.QueryRow(ctx, `SELECT coalesce(max(m.id),0),count(*) FILTER (WHERE m.status=1) FROM character_mail m JOIN characters c ON c.id=m.recipient_id WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL AND m.deleted_at IS NULL AND m.expires_at>now()`, account, id).Scan(&latest, &unread)
	return latest, unread, err
}

func (s *Store) Mailbox(ctx context.Context, account, id int64) ([]MailMessage, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+mailColumns+` FROM character_mail m JOIN characters c ON c.id=m.recipient_id WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL AND m.deleted_at IS NULL AND (m.expires_at>now() OR m.status=3) ORDER BY m.id`, account, id)
	if err != nil {
		return nil, err
	}
	return readMailRows(rows)
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
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback(ctx)
	var recipient int64
	err = tx.QueryRow(ctx, `SELECT id FROM characters WHERE lower(name)=lower($1) AND deleted_at IS NULL`, name).Scan(&recipient)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrMailRecipient)
	}
	if err != nil {
		return fail(err)
	}
	if recipient == id {
		return fail(ErrMailSelf)
	}
	rows, err := tx.Query(ctx, `SELECT `+mailCharacterColumns+` FROM characters WHERE id=ANY($1::bigint[]) AND deleted_at IS NULL ORDER BY id FOR UPDATE`, []int64{id, recipient})
	if err != nil {
		return fail(err)
	}
	var receiver Character
	count := 0
	for rows.Next() {
		c, e := scanMailCharacter(rows)
		if e != nil {
			rows.Close()
			return fail(e)
		}
		count++
		if c.ID == id {
			role = c
		} else {
			receiver = c
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fail(err)
	}
	if count != 2 || role.AccountID != account || role.ConfigVersion != version || receiver.ConfigVersion != version {
		return fail(fmt.Errorf("邮件角色归属或配置版本无效"))
	}
	var prior []byte
	err = tx.QueryRow(ctx, `SELECT outcome FROM character_events WHERE character_id=$1 AND event_key=$2 AND model='mail-send-v1'`, id, key).Scan(&prior)
	if err == nil {
		if err = json.Unmarshal(prior, &receipt); err != nil {
			return fail(err)
		}
		return role, receipt, false, tx.Commit(ctx)
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
	var mailCount, assetCount int
	err = tx.QueryRow(ctx, `SELECT count(*),coalesce(sum((SELECT count(*) FROM jsonb_array_elements(m.assets) a WHERE NOT coalesce((a->>'claimed')::boolean,false))),0) FROM character_mail m WHERE recipient_id=$1 AND deleted_at IS NULL AND (expires_at>now() OR status=3)`, recipient).Scan(&mailCount, &assetCount)
	if err != nil {
		return fail(err)
	}
	if mailCount >= 255 || assetCount+len(assets) > 255 {
		return fail(ErrMailFull)
	}
	for i := range assets {
		if assets[i].Claimed || (assets[i].Gold == 0 && len(assets[i].Item) == 0) || (len(assets[i].Item) > 0 && !json.Valid(assets[i].Item)) {
			return fail(fmt.Errorf("待发邮件附件无效"))
		}
		if err = tx.QueryRow(ctx, `SELECT nextval('mailbox_id_seq')`).Scan(&assets[i].ID); err != nil {
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
	err = tx.QueryRow(ctx, `INSERT INTO character_mail(sender_id,recipient_id,sender_name,body,assets,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '15 days') RETURNING id`, id, recipient, role.Name, body, encoded).Scan(&receipt.MessageID)
	if err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); err != nil {
		return fail(err)
	}
	prior, err = json.Marshal(receipt)
	if err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,'mail-send-v1',$4)`, id, key, version, prior); err != nil {
		return fail(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fail(err)
	}
	role.State = state
	return role, receipt, true, nil
}

// insertSystemMailTx inserts one system mail (sender_id NULL) inside an
// existing transaction under the mail-owned capacity and retention rules.
// It assigns the shared mailbox_id_seq IDs and returns the new message id.
func insertSystemMailTx(ctx context.Context, tx db.Tx, recipientID int64, senderName, body string, assets []mail.Asset) (int64, error) {
	if len(assets) > mail.MaxAttachments {
		return 0, fmt.Errorf("系统邮件附件过多")
	}
	var messages, existing int
	if err := tx.QueryRow(ctx, `SELECT count(*),coalesce(sum((SELECT count(*) FROM jsonb_array_elements(m.assets) a WHERE NOT coalesce((a->>'claimed')::boolean,false))),0)
 FROM character_mail m WHERE recipient_id=$1 AND deleted_at IS NULL AND (expires_at>now() OR status=3)`, recipientID).Scan(&messages, &existing); err != nil {
		return 0, err
	}
	if messages >= mail.MaxMessages || existing+len(assets) > mail.MaxUnclaimedAssets {
		return 0, ErrMailFull
	}
	for i := range assets {
		if err := tx.QueryRow(ctx, `SELECT nextval('mailbox_id_seq')`).Scan(&assets[i].ID); err != nil {
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
	var mailID int64
	err = tx.QueryRow(ctx, `INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at)
 VALUES($1,$2,$3,$4, now()+interval '15 days') RETURNING id`, recipientID, senderName, body, encoded).Scan(&mailID)
	return mailID, err
}

// CommitSystemMail creates one system mail for a recipient under the caller's
// idempotency key/model (e.g. reward-mail-v1). Returns the new message id.
func (s *Store) CommitSystemMail(ctx context.Context, account, id int64, version, key, model, senderName, body string, assets []MailAsset) (int64, bool, error) {
	var messageID int64
	_, applied, err := s.CommitCharacterEventTx(ctx, account, id, version, key, model,
		func(tx db.Tx, role Character) (json.RawMessage, json.RawMessage, error) {
			mid, err := insertSystemMailTx(ctx, tx, id, senderName, body, assets)
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
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback(ctx)
	role, err := scanMailCharacter(tx.QueryRow(ctx, `SELECT `+mailCharacterColumns+` FROM characters WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, account))
	if err != nil {
		return fail(err)
	}
	if role.ConfigVersion != version {
		return fail(fmt.Errorf("邮件角色配置版本不匹配"))
	}
	var priorModel string
	var receipt json.RawMessage
	err = tx.QueryRow(ctx, `SELECT model,outcome FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&priorModel, &receipt)
	if err == nil {
		if priorModel != model {
			return fail(fmt.Errorf("邮件操作流水冲突"))
		}
		return role, receipt, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	// 删除回调可能紧接着领取回执到达；按请求编号读取软删除记录，才能
	// 幂等应答客户端的后续删除，而不把“已领取后自动删除”误报为失败。
	rows, err := tx.Query(ctx, `SELECT `+mailColumns+` FROM character_mail m WHERE recipient_id=$1 AND ($2::boolean OR (deleted_at IS NULL AND (expires_at>now() OR status=3))) AND ($3::bigint[] IS NULL OR id=ANY($3)) ORDER BY id FOR UPDATE`, id, model == "mail-status-v1", messageIDs)
	if err != nil {
		return fail(err)
	}
	messages, err := readMailRows(rows)
	if err != nil {
		return fail(err)
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
		result, e := tx.Exec(ctx, `UPDATE character_mail SET assets=$3,status=$4,deleted_at=CASE WHEN $5 THEN now() ELSE NULL END WHERE id=$1 AND recipient_id=$2 AND deleted_at IS NULL`, m.ID, id, assets, m.Status, m.Deleted)
		if e != nil {
			return fail(e)
		}
		if result.RowsAffected() != 1 {
			return fail(fmt.Errorf("邮件状态已经改变"))
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); err != nil {
		return fail(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, key, version, model, receipt); err != nil {
		return fail(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fail(err)
	}
	role.State = state
	return role, receipt, true, nil
}
