package database

import (
	"context"
	"encoding/json"
	"fmt"
)

// AttendanceEvidence is an existing committed receipt, never a new reward.
type AttendanceEvidence struct {
	Key     string          `json:"key"`
	Outcome json.RawMessage `json:"outcome"`
}

// RecoverMissingAttendance only fills an absent campaign. It keeps current
// account and character state and never sends mail, changes items or deletes data.
func (s *Store) RecoverMissingAttendance(ctx context.Context, account, id int64, campaign, prefix string, recover func([]AttendanceEvidence) (json.RawMessage, error)) error {
	if s == nil || len(campaign) != 64 || prefix == "" || recover == nil {
		return fmt.Errorf("attendance recovery requires owned campaign")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	role, err := lockCharacter(ctx, tx, account, id)
	if err != nil {
		return err
	}
	p, err := lockAdventure(ctx, tx, role)
	if err != nil {
		return err
	}
	if len(p.Data.Attendance[campaign]) != 0 {
		return tx.commit(ctx)
	}
	raw, err := tx.queries().AttendanceEvidence(ctx, account, prefix)
	if err != nil {
		return err
	}
	var evidence []AttendanceEvidence
	if err = json.Unmarshal(raw, &evidence); err != nil {
		return err
	}
	state, err := recover(evidence)
	if err != nil {
		return err
	}
	if len(state) != 0 {
		if !json.Valid(state) {
			return fmt.Errorf("invalid recovered attendance")
		}
		if p.Data.Attendance == nil {
			p.Data.Attendance = map[string]json.RawMessage{}
		}
		p.Data.Attendance[campaign] = state
		if err = saveAdventure(ctx, tx, account, p); err != nil {
			return err
		}
	}
	return tx.commit(ctx)
}
