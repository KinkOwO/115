package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"
)

// 装备技能栏 / 冷却提醒 / 自定义按键 的按角色快照。
//
// 外部文档（已实机确认）要求：两组 80 字节快照分表保存、2254/2256 各自更新、
// 2257 清空；建表用 `CREATE TABLE IF NOT EXISTS`，**不改写任何已有角色存档**
// （穿戴与技能仍以角色 JSON 为真源，这里只存"客户端界面上那两组快照"）。
const (
	// EquipmentSkillSnapshotColumn 是 2254 那一组（40 个 i16 技能槽位）。
	EquipmentSkillSnapshotColumn = "skills"
	// EquipmentCommandSnapshotColumn 是 2256 那一组（10 个 8 字节指令组）。
	EquipmentCommandSnapshotColumn = "commands"
)

// MigrateEquipmentSkill 建表（幂等、纯追加）。
func (s *Store) MigrateEquipmentSkill(ctx context.Context) error {
	return s.execMigration(ctx, "0007_equipment_skill.sql")
}

// SaveEquipmentSkillSnapshot 在角色行锁下保存其中一组快照。
//
// which 取 "skills" 或 "commands"；另一组保持不变（首次保存时另一组为 NULL，
// 读取时按"还没设过"处理）。
func (s *Store) SaveEquipmentSkillSnapshot(ctx context.Context, accountID, characterID int64, which string, data []byte) error {
	if accountID <= 0 || characterID <= 0 {
		return errors.New("invalid equipment skill character")
	}
	if which != EquipmentSkillSnapshotColumn && which != EquipmentCommandSnapshotColumn {
		return fmt.Errorf("unknown equipment skill snapshot %q", which)
	}
	if len(data) == 0 {
		return errors.New("empty equipment skill snapshot")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockCharacterOwnerIncludingDeleted(ctx, sqlcgen.LockCharacterOwnerIncludingDeletedParams{
		AccountID: accountID, CharacterID: characterID,
	}); err != nil {
		return err
	}
	if which == EquipmentSkillSnapshotColumn {
		err = queries.SaveEquipmentSkills(ctx, sqlcgen.SaveEquipmentSkillsParams{CharacterID: characterID, Skills: data})
	} else {
		err = queries.SaveEquipmentCommands(ctx, sqlcgen.SaveEquipmentCommandsParams{CharacterID: characterID, Commands: data})
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearEquipmentSkill 清空该角色的两组快照（2257）。
func (s *Store) ClearEquipmentSkill(ctx context.Context, accountID, characterID int64) error {
	if accountID <= 0 || characterID <= 0 {
		return errors.New("invalid equipment skill character")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err := queries.LockCharacterOwnerIncludingDeleted(ctx, sqlcgen.LockCharacterOwnerIncludingDeletedParams{
		AccountID: accountID, CharacterID: characterID,
	}); err != nil {
		return err
	}
	if err = queries.ClearEquipmentSkill(ctx, characterID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// EquipmentSkillSnapshots 读回该角色的两组快照；任一组没设过时返回 nil（调用方补零）。
func (s *Store) EquipmentSkillSnapshots(ctx context.Context, accountID, characterID int64) (skills, commands []byte, err error) {
	if accountID <= 0 || characterID <= 0 {
		return nil, nil, errors.New("invalid equipment skill character")
	}
	snapshot, err := s.queries.EquipmentSkillSnapshots(ctx, sqlcgen.EquipmentSkillSnapshotsParams{
		AccountID: accountID, CharacterID: characterID,
	})
	if err != nil {
		return nil, nil, err
	}
	return snapshot.Skills, snapshot.Commands, nil
}
