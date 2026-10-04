package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/quest"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// QuestState retains the storage API while the quest domain owns its schema.
type QuestState = quest.QuestState

var _ quest.Store = (*Store)(nil)

func (s *Store) MigrateQuests(ctx context.Context) error {
	return s.execMigration(ctx, "0023_quests.sql")
}
func (s *Store) AcceptQuest(ctx context.Context, account, characterID int64, qid uint16, version string, minLevel, maxLevel uint32, prerequisites []uint32, initial uint32, model string) (QuestState, error) {
	var groups [][]uint32
	if len(prerequisites) > 0 {
		groups = [][]uint32{prerequisites}
	}
	return s.AcceptQuestGroups(ctx, account, characterID, qid, version, minLevel, maxLevel, groups, initial, model)
}

// AcceptQuestGroups checks alternative prerequisite sections while holding the
// character row lock. Each group requires every quest in that group.
func (s *Store) AcceptQuestGroups(ctx context.Context, account, characterID int64, qid uint16, version string, minLevel, maxLevel uint32, groups [][]uint32, initial uint32, model string) (QuestState, error) {
	out := QuestState{ID: qid, Status: "accepted", ConfigVersion: version, Progress: initial, ProgressModel: model}
	if qid == 0 || qid == 65535 || len(version) != 64 || model == "" || model == "legacy-zero" {
		return out, errors.New("invalid quest")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	raw, e := queries.LockOwnedCharacterState(ctx, sqlcgen.LockOwnedCharacterStateParams{AccountID: account, CharacterID: characterID})
	if e != nil {
		return out, e
	}
	var state struct {
		Level uint32 `json:"level"`
	}
	if e = json.Unmarshal(raw, &state); e != nil {
		return out, e
	}
	if state.Level < minLevel || state.Level > maxLevel {
		return out, errors.New("quest level condition not met")
	}
	prior, e := queries.Quest(ctx, sqlcgen.QuestParams{CharacterID: characterID, QuestID: int32(qid)})
	if e == nil {
		out.Status, out.Progress, out.ConfigVersion, out.ProgressModel = prior.Status, uint32(prior.Progress), prior.ConfigVersion, prior.ProgressModel
		if out.Status != "accepted" || out.ConfigVersion != version || out.ProgressModel != model {
			return out, errors.New("quest is completed or requires configuration migration")
		}
		return out, tx.commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if len(groups) > 0 {
		matched := false
		for _, group := range groups {
			if len(group) == 0 {
				return out, errors.New("empty prerequisite group")
			}
			complete := true
			for _, pre := range group {
				done, err := queries.HasCompletedQuest(ctx, sqlcgen.HasCompletedQuestParams{CharacterID: characterID, QuestID: int64(pre)})
				if err != nil {
					return out, err
				}
				if !done {
					complete = false
					break
				}
			}
			if complete {
				matched = true
				break
			}
		}
		if !matched {
			return out, fmt.Errorf("quest prerequisite alternatives incomplete")
		}
	}
	e = queries.AcceptQuest(ctx, sqlcgen.AcceptQuestParams{CharacterID: characterID, QuestID: int32(qid), ConfigVersion: version, Progress: int64(initial), ProgressModel: model})
	if e != nil {
		return out, e
	}
	return out, tx.commit(ctx)
}
func (s *Store) AbandonQuest(ctx context.Context, account, characterID int64, qid uint16) error {
	changed, e := s.queries.AbandonQuest(ctx, sqlcgen.AbandonQuestParams{AccountID: account, CharacterID: characterID, QuestID: int32(qid)})
	if e != nil {
		return e
	}
	if changed != 1 {
		return errors.New("quest is not active for this character")
	}
	return nil
}

// MarkMeetNPCQuest records the native meet-NPC progress transition for an
// owned accepted quest. SQL ownership stays here so quest rules do not need to
// know the character_quests table shape or its ownership predicates.
func (s *Store) MarkMeetNPCQuest(ctx context.Context, account, characterID int64, qid uint16, version, model string) error {
	changed, err := s.queries.MarkMeetNPCQuest(ctx, sqlcgen.MarkMeetNPCQuestParams{AccountID: account, CharacterID: characterID, QuestID: int32(qid), ConfigVersion: version, ProgressModel: model})
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("NPC quest is not active for this owner")
	}
	return nil
}

// ClearQuests marks the given story quests as already completed for the
// character, without ever touching rows that exist (a quest the player truly
// has in progress or finished keeps its state). Rows are auditable and
// precisely reversible through progress_model='odyssey-skip-v1'; the call is
// idempotent, so repeated logins only fill the gaps.
func (s *Store) ClearQuests(ctx context.Context, account, characterID int64, version string, ids []uint16) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(version) != 64 {
		return 0, errors.New("invalid quest config version")
	}
	changed, e := s.queries.ClearQuests(ctx, sqlcgen.ClearQuestsParams{AccountID: account, CharacterID: characterID, QuestIds: questIDsToInt32(ids), ConfigVersion: version})
	if e != nil {
		return 0, e
	}
	return int(changed), nil
}

func questIDsToInt32(ids []uint16) []int32 {
	out := make([]int32, len(ids))
	for i, id := range ids {
		out[i] = int32(id)
	}
	return out
}

// ClearActQuests completes only existing accepted rows. It never inserts a
// quest, so successors unlocked by this update remain unaccepted and visible
// to the normal quest list flow. The character lock keeps the batch atomic.
func (s *Store) ClearActQuests(ctx context.Context, account, characterID int64, version string, ids []uint16) (int, error) {
	if len(version) != 64 {
		return 0, errors.New("invalid quest config version")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if _, err = queries.LockCharacterOwner(ctx, sqlcgen.LockCharacterOwnerParams{AccountID: account, CharacterID: characterID}); err != nil {
		return 0, err
	}
	changed, err := queries.ClearActQuests(ctx, sqlcgen.ClearActQuestsParams{CharacterID: characterID, QuestIds: questIDsToInt32(ids), ConfigVersion: version})
	if err != nil {
		return 0, err
	}
	if changed != int64(len(ids)) {
		return 0, errors.New("accepted quest set changed or requires source migration")
	}
	if err = tx.commit(ctx); err != nil {
		return 0, err
	}
	return int(changed), nil
}
