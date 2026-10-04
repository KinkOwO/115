package charactercheck

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"fmt"
)

func tutorialCheck(ctx context.Context, s *database.TestFixture, reopened *database.Store, role database.Character, other int64) error {
	if e := s.MigrateTutorial(ctx); e != nil {
		return e
	}
	for _, idx := range []byte{31, 1, 2, 31} {
		if e := s.SaveTutorialFlag(ctx, role.AccountID, role.ID, idx, true); e != nil {
			return e
		}
	}
	flags, e := reopened.TutorialFlags(ctx, role.AccountID, role.ID)
	if e != nil || !bytes.Equal(flags, []byte{1, 2, 31}) {
		return fmt.Errorf("tutorial reopen/retry failed: %v", e)
	}
	if e = s.SaveTutorialFlag(ctx, other, role.ID, 100, true); e == nil {
		return fmt.Errorf("other account changed guide state")
	}
	if _, e = reopened.TutorialFlags(ctx, other, role.ID); e == nil {
		return fmt.Errorf("other account read guide state")
	}
	if e = s.SaveTutorialFlag(ctx, role.AccountID, role.ID, 101, true); e == nil {
		return fmt.Errorf("invalid guide index accepted")
	}
	if e = s.SaveTutorialFlag(ctx, role.AccountID, role.ID, 2, false); e != nil {
		return e
	}
	flags, e = reopened.TutorialFlags(ctx, role.AccountID, role.ID)
	if e != nil || !bytes.Equal(flags, []byte{1, 31}) {
		return fmt.Errorf("explicit guide state update failed: %v", e)
	}
	fmt.Println("TUTORIAL_STATE_PASS ownership=true reopen=true retry_idempotent=true rewards_granted=false")
	return nil
}
