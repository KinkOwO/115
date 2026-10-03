package charactercheck

import (
	"context"
	"dfolan/internal/storage"
	"errors"
	"fmt"
	"sync"
)

// Only called after the existing temporary-schema isolation check.
func fatigueChargeCheck(ctx context.Context, s, reopened *storage.Store, role storage.Character, other int64) error {
	const run = "0123456789abcdef0123456789abcdef"
	const day = "2026-09-12"
	const room = 76121
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fp, _, e := s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, day, 2, run, room, 1)
			if e == nil && fp.Used != 1 {
				e = fmt.Errorf("concurrent loading charged %d", fp.Used)
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			return e
		}
	}
	fp, fresh, e := reopened.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, day, 2, run, room, 1)
	if e != nil || fresh || fp.Used != 1 {
		return fmt.Errorf("reopen loading replay: %+v %t %v", fp, fresh, e)
	}
	if _, _, e = s.ConsumeRoomFatigue(ctx, other, role.ID, day, 2, run, room, 1); e == nil {
		return fmt.Errorf("foreign room charge accepted")
	}
	fp, fresh, e = s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, day, 2, run, 76123, 1)
	if e != nil || !fresh || fp.Used != 2 {
		return fmt.Errorf("second room consumption: %+v %v", fp, e)
	}
	if _, _, e = s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, day, 2, run, 76124, 1); !errors.Is(e, storage.ErrFatigueExhausted) {
		return fmt.Errorf("exhausted charge: %v", e)
	}
	fp, fresh, e = s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, day, 2, run, 53127, 0)
	if e != nil || !fresh || fp.Used != 2 {
		return fmt.Errorf("exempt tutorial charge: %+v %v", fp, e)
	}
	fp, fresh, e = s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, "2026-09-13", 2, run, room, 1)
	if e != nil || fresh || fp.Used != 0 {
		return fmt.Errorf("rollover retry charged an old room: %+v %v", fp, e)
	}
	fp, fresh, e = s.ConsumeRoomFatigue(ctx, role.AccountID, role.ID, "2026-09-13", 2, "abcdef0123456789abcdef0123456789", room, 1)
	if e != nil || !fresh || fp.Used != 1 {
		return fmt.Errorf("new run did not charge: %+v %v", fp, e)
	}
	fmt.Println("FATIGUE_ROOM_CHECK_PASS concurrent_retry=true reopen=true ownership=true exhaustion=true exemption=true rollover=true")
	return nil
}
