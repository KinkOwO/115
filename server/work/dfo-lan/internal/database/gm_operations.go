package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

// GM operations are only available through an already owned, audited grant
// transaction. They add no tables and share queries across both engines.
func (t *Tx) GMSystemMail(ctx context.Context, body string, assets []MailAsset) (int64, error) {
	if len(body) > 512 {
		return 0, fmt.Errorf("邮件正文过长")
	}
	return insertSystemMailTx(ctx, t.queries, t.characterID, "GM", body, assets)
}
func (t *Tx) GMDeleteMail(ctx context.Context, ids []int64) error {
	if len(ids) == 0 || len(ids) > 255 {
		return fmt.Errorf("请选择1至255封邮件")
	}
	rows, err := t.queries.LockMailbox(ctx, sqlcgen.LockMailboxParams{RecipientID: t.characterID, MessageIds: ids})
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return fmt.Errorf("邮件已改变或不属于当前角色")
	}
	for _, r := range rows {
		n, e := t.queries.UpdateMail(ctx, sqlcgen.UpdateMailParams{MailID: r.ID, RecipientID: t.characterID, Assets: r.Assets, Status: int32(r.Status), Deleted: true})
		if e != nil {
			return e
		}
		if n != 1 {
			return fmt.Errorf("邮件已经改变")
		}
	}
	return nil
}
func (t *Tx) GMQuests(ctx context.Context, ids []uint16, action, version string) error {
	if len(ids) == 0 || len(ids) > 100 {
		return fmt.Errorf("请选择1至100个任务")
	}
	for _, id := range ids {
		row, err := t.queries.LockQuest(ctx, sqlcgen.LockQuestParams{CharacterID: t.characterID, QuestID: int32(id)})
		if action == "quest_complete" && isNoRows(err) {
			err = t.queries.CompleteGraduationQuests(ctx, sqlcgen.CompleteGraduationQuestsParams{CharacterID: t.characterID, ConfigVersion: version, ProgressModel: "gm-complete-without-reward-v1", QuestIds: []int32{int32(id)}})
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if row.ConfigVersion != version {
			return fmt.Errorf("任务来源版本不一致")
		}
		if row.Status == "completed" && action == "quest_complete" {
			continue
		}
		if row.Status != "accepted" {
			return fmt.Errorf("任务%d不是进行中的任务", id)
		}
		switch action {
		case "quest_ready":
			_, err = t.queries.CompleteQuestObjective(ctx, sqlcgen.CompleteQuestObjectiveParams{AccountID: t.accountID, CharacterID: t.characterID, QuestID: int32(id), ConfigVersion: version, ProgressModel: row.ProgressModel})
		case "quest_complete":
			err = t.queries.CompleteGraduationQuests(ctx, sqlcgen.CompleteGraduationQuestsParams{CharacterID: t.characterID, ConfigVersion: version, ProgressModel: "gm-complete-without-reward-v1", QuestIds: []int32{int32(id)}})
		case "quest_abandon":
			_, err = t.queries.AbandonQuest(ctx, sqlcgen.AbandonQuestParams{AccountID: t.accountID, CharacterID: t.characterID, QuestID: int32(id)})
		default:
			return fmt.Errorf("未知任务操作")
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (t *Tx) GMMaterials(ctx context.Context, apply func(json.RawMessage) (json.RawMessage, error)) error {
	if err := t.queries.InitializeAccountMaterials(ctx, t.accountID); err != nil {
		return err
	}
	raw, err := t.queries.LockAccountMaterials(ctx, t.accountID)
	if err != nil {
		return err
	}
	updated, err := apply(raw)
	if err != nil {
		return err
	}
	if !json.Valid(updated) {
		return fmt.Errorf("无效晶体仓库存档")
	}
	return t.queries.SaveAccountMaterials(ctx, sqlcgen.SaveAccountMaterialsParams{AccountID: t.accountID, Counts: updated})
}
