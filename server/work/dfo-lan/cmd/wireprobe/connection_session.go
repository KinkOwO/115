package main

import (
	"sync"
	"time"
)

// connectionSession owns resources whose lifetime is exactly one game
// connection. Gameplay state remains in worldSession; this type only owns the
// reader stop signal and periodic triggers used by the connection loop.
type connectionSession struct {
	done        chan struct{}
	mailChanges chan struct{}
	mailTicker  *time.Ticker
	dailyTicker *time.Ticker
	mineTicker  *time.Ticker
	moonTicker  *time.Ticker
	closeOnce   sync.Once
}

func newConnectionSession(moonEnabled bool) *connectionSession {
	s := &connectionSession{
		done:        make(chan struct{}),
		mailChanges: make(chan struct{}, 1),
		mailTicker:  time.NewTicker(2 * time.Second),
		dailyTicker: time.NewTicker(30 * time.Second),
		mineTicker:  time.NewTicker(time.Second),
	}
	if moonEnabled {
		s.moonTicker = time.NewTicker(250 * time.Millisecond)
	}
	return s
}

func (s *connectionSession) moonTicks() <-chan time.Time {
	if s == nil || s.moonTicker == nil {
		return nil
	}
	return s.moonTicker.C
}

func (s *connectionSession) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.done)
		s.mailTicker.Stop()
		s.dailyTicker.Stop()
		s.mineTicker.Stop()
		if s.moonTicker != nil {
			s.moonTicker.Stop()
		}
	})
}
