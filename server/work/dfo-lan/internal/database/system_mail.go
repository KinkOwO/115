package database

import (
	"context"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// systemMailSenderNameLimit 是 character_mail.sender_name 的 CHECK 上限，
// 建表语句里写死 29 字节；与玩家间邮件、荣誉邮件、矿区邮件同一口径。
const systemMailSenderNameLimit = 29

// SystemMailAssets 把「模板 + 件数」编成系统邮件附件。行构造走邮寄线唯一的
// inventory.MailItem.Row()，期限按服务端发放的统一哨兵 inventory.GrantExpireTime 打
// （写 0 会让声明过期限的物品一进背包就显示「剩余期限已过」，见 GrantExpireTime 注释）。
//
// space 是容器字节：0 普通背包，1 时装栏；其它值 MailItem.Row() 直接拒绝。
func SystemMailAssets(space byte, grants []GrantItem) ([]MailAsset, error) {
	var out []MailAsset
	for _, g := range grants {
		if g.Template < 2 || g.Amount == 0 {
			return nil, fmt.Errorf("系统邮件附件模板或数量无效")
		}
		item := inventory.MailItem{
			Stack: &inventory.BagItem{Template: g.Template, Amount: g.Amount, ExpireTime: inventory.GrantExpireTime},
			Space: space,
		}
		if _, e := item.Row(); e != nil {
			return nil, e
		}
		encoded, e := json.Marshal(item)
		if e != nil {
			return nil, e
		}
		out = append(out, MailAsset{Item: encoded})
	}
	return out, nil
}

// SendSystemMailWithCharacterUpdate 把「改角色状态」和「发系统邮件」压进同一事务：
// mutate 在角色行 FOR UPDATE 之后执行，邮件与状态一起提交。邮件编号写进事件回执，
// 事后可从 character_events.outcome 审计这一封到底是哪一封。
//
// 与 CommitSystemMail 的分工：那个只发邮件、不动状态（model 由调用方给）；
// 这个要同时改状态，model 固定 "system-mail-v1"，重放时按同一事件键读回既有邮件编号。
func (s *Store) SendSystemMailWithCharacterUpdate(ctx context.Context, account, id int64, version, key, sender, body string,
	assets []MailAsset, mutate func(Character) (json.RawMessage, error)) (int64, bool, error) {
	if mutate == nil {
		return 0, false, fmt.Errorf("系统邮件缺少状态更新函数")
	}
	if id <= 0 || len(sender) == 0 || len(sender) > systemMailSenderNameLimit || len(body) > 512 {
		return 0, false, fmt.Errorf("系统邮件发件人署名或正文无效")
	}
	var mailID int64
	_, applied, err := s.CommitCharacterEventTx(ctx, account, id, version, key, "system-mail-v1",
		func(tx *Tx, role Character) (json.RawMessage, json.RawMessage, error) {
			state, e := mutate(role)
			if e != nil {
				return nil, nil, e
			}
			if mailID, e = insertSystemMailTx(ctx, tx.queries, id, sender, body, assets); e != nil {
				return nil, nil, e
			}
			receipt, e := json.Marshal(map[string]any{"mail_id": mailID})
			return state, receipt, e
		})
	if !applied && err == nil {
		// 重放：邮件早已入箱，编号从既有回执里读回来。
		var prior struct {
			MailID int64 `json:"mail_id"`
		}
		if raw, e := s.CharacterEventReceipt(ctx, account, id, key); e == nil {
			if json.Unmarshal(raw, &prior) == nil {
				mailID = prior.MailID
			}
		}
	}
	return mailID, applied, err
}
