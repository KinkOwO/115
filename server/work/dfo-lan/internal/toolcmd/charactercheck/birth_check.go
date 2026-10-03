package charactercheck

import (
	"context"
	"dfolan/internal/storage"
	"fmt"
)

// birthCheck covers the one-time starting route. The property that matters
// most is the backfill: a character that already existed when this table was
// introduced must come out complete, because the project never resets or
// replays progress a character already has.
func birthCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	// role was created before MigrateBirth runs here, standing in for every
	// character that predates the table.
	if e := s.MigrateBirth(ctx); e != nil {
		return e
	}
	stage, _, e := reopened.BirthStage(ctx, role.AccountID, role.ID)
	if e != nil {
		return e
	}
	if stage != storage.BirthComplete {
		return fmt.Errorf("pre-existing character was sent back to the starting route: stage=%d", stage)
	}
	// A second migration must not disturb a recorded stage.
	if e = s.MigrateBirth(ctx); e != nil {
		return e
	}
	if stage, _, e = reopened.BirthStage(ctx, role.AccountID, role.ID); e != nil || stage != storage.BirthComplete {
		return fmt.Errorf("repeat migration changed a recorded stage: %d %v", stage, e)
	}
	if _, _, e = reopened.BirthStage(ctx, other, role.ID); e == nil {
		return fmt.Errorf("other account read the starting route")
	}

	// Now exercise the newly-created path on this same isolated character by
	// clearing its backfilled row, without adding a character to the schema.
	if _, e = s.DB.Exec(ctx, `DELETE FROM character_birth WHERE character_id=$1`, role.ID); e != nil {
		return e
	}
	if e = s.StartBirth(ctx, role.AccountID, role.ID); e != nil {
		return e
	}
	if e = s.StartBirth(ctx, role.AccountID, role.ID); e == nil {
		return fmt.Errorf("starting route recorded twice")
	}
	if stage, _, e = reopened.BirthStage(ctx, role.AccountID, role.ID); e != nil || stage != storage.BirthPending {
		return fmt.Errorf("new character does not owe a starting route: %d %v", stage, e)
	}
	// A restart must not re-mark a character that is mid-route as complete.
	if e = s.MigrateBirth(ctx); e != nil {
		return e
	}
	if stage, _, e = reopened.BirthStage(ctx, role.AccountID, role.ID); e != nil || stage != storage.BirthPending {
		return fmt.Errorf("migration overwrote a pending starting route: %d %v", stage, e)
	}
	if e = s.StartBirth(ctx, other, role.ID); e == nil {
		return fmt.Errorf("other account started the route")
	}

	applied, e := s.AdvanceBirth(ctx, role.AccountID, role.ID, storage.BirthEntered, 7115)
	if e != nil || !applied {
		return fmt.Errorf("starting route entry not recorded: %v", e)
	}
	stage, dungeon, e := reopened.BirthStage(ctx, role.AccountID, role.ID)
	if e != nil || stage != storage.BirthEntered || dungeon != 7115 {
		return fmt.Errorf("starting route entry not durable: %d %d %v", stage, dungeon, e)
	}
	if applied, e = s.AdvanceBirth(ctx, other, role.ID, storage.BirthComplete, 0); e != nil || applied {
		return fmt.Errorf("other account advanced the starting route: %v", e)
	}
	if applied, e = s.AdvanceBirth(ctx, role.AccountID, role.ID, storage.BirthComplete, 0); e != nil || !applied {
		return fmt.Errorf("starting route completion not recorded: %v", e)
	}
	// The stage is one-way: a replayed entry after completion must not reopen
	// the route, which is what would let a reconnect replay the tutorial.
	if applied, e = s.AdvanceBirth(ctx, role.AccountID, role.ID, storage.BirthEntered, 7115); e != nil || applied {
		return fmt.Errorf("finished starting route was reopened: %v", e)
	}
	if stage, _, e = reopened.BirthStage(ctx, role.AccountID, role.ID); e != nil || stage != storage.BirthComplete {
		return fmt.Errorf("starting route did not settle: %d %v", stage, e)
	}
	fmt.Println("BIRTH_ROUTE_PASS backfill_complete=true pending_recorded=true restart_safe=true one_way=true ownership=true replay_blocked=true")
	return nil
}
