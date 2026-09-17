package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"io"
	"time"
)

type clientRead struct {
	frame wire.Frame
	err   error
}

// A single uninterrupted reader owns framing. Timer events never interrupt a
// partially received encrypted frame, and only the session loop writes replies.
func clientFrames(r io.Reader, done <-chan struct{}) <-chan clientRead {
	ch := make(chan clientRead)
	go func() {
		defer close(ch)
		for {
			f, e := wire.ReadClient(r)
			select {
			case ch <- clientRead{f, e}:
			case <-done:
				return
			}
			if e != nil {
				return
			}
		}
	}()
	return ch
}

func (w *worldSession) refreshDailyFatigue(now time.Time) ([]byte, error) {
	if w == nil || w.fatigue == nil || w.role.ID == 0 {
		return nil, nil
	}
	day := w.fatigue.Day(now)
	if day <= w.lastFatigueDay {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fp, e := w.fatigue.State(ctx, w.role.AccountID, w.role.ID, now)
	if e != nil {
		return nil, e
	}
	p, e := protocol.Fatigue(fp.Used, fp.Limit, fp.UsedMax)
	if e != nil {
		return nil, e
	}
	w.lastFatigueDay = fp.Day
	return p, nil
}
