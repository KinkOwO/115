// Package reward is a lightweight, event-triggered Lua reward add-on.
//
// Domains only notify after a business success; the reward rules live in the
// embedded scripts/*.lua files. It deliberately avoids an event bus, a rule
// engine or dependency injection: the whole surface is New + LevelUp +
// QuestComplete + CharacterCreate.
//
// The Lua scripts are compiled into the binary (go:embed), so the add-on needs
// no external file path. Options.Scripts can override the embedded set (tests,
// or a future operator-supplied directory).
//
// This package must not import internal/database or internal/character, because
// character imports reward. Persistence is injected via GrantFunc/MailFunc/CeraFunc.
package reward

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// bundledScripts is the default rule set compiled into the executable.
//
//go:embed scripts/*.lua
var bundledScripts embed.FS

// GrantFunc persists a set of item grants for one recipient under a stable key.
type GrantFunc func(ctx context.Context, r Recipient, key string, items []ItemGrant) error

// MailFunc persists one system mail for a recipient under a stable key.
type MailFunc func(ctx context.Context, r Recipient, key string, mail MailReward) error

// CeraFunc persists an account-level cera grant under a stable key.
type CeraFunc func(ctx context.Context, r Recipient, key string, amount uint64) error

type Options struct {
	// Scripts overrides the embedded scripts. nil uses the bundled rule set.
	Scripts fs.FS
	Grant   GrantFunc
	Mail    MailFunc
	Cera    CeraFunc
	Log     func(format string, args ...any)
}

// Service implements Notifier over one gopher-lua LState. All Lua execution is
// serialized because a single LState is not concurrency-safe.
type Service struct {
	mu        sync.Mutex
	state     *lua.LState
	scripts   []script
	grant     GrantFunc
	mail      MailFunc
	cera      CeraFunc
	log       func(format string, args ...any)
	collector *collector
	// pending collects the handlers a script registers through on() while that
	// script is being loaded; it is only touched during New.
	pending map[string]*lua.LFunction
}

const maxKeyLength = 200

// New loads every *.lua in Scripts (or the embedded default when nil) into one
// LState. No scripts, or a script that fails to compile, is an error; the
// caller decides whether that disables the feature.
func New(opts Options) (*Service, error) {
	src := opts.Scripts
	if src == nil {
		sub, err := fs.Sub(bundledScripts, "scripts")
		if err != nil {
			return nil, fmt.Errorf("reward: embedded scripts: %w", err)
		}
		src = sub
	}
	files, err := fs.Glob(src, "*.lua")
	if err != nil {
		return nil, fmt.Errorf("reward: list scripts: %w", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("reward: no lua scripts")
	}
	s := &Service{grant: opts.Grant, mail: opts.Mail, cera: opts.Cera, log: opts.Log}
	if s.log == nil {
		s.log = func(string, ...any) {}
	}
	if err := s.loadScripts(src, files); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) LevelUp(ctx context.Context, r Recipient) {
	s.dispatch(ctx, Event{Type: EventLevelUp, Recipient: r})
}

func (s *Service) QuestComplete(ctx context.Context, r Recipient, questID uint16) {
	s.dispatch(ctx, Event{Type: EventQuestComplete, Recipient: r, QuestID: questID})
}

func (s *Service) CharacterCreate(ctx context.Context, r Recipient) {
	s.dispatch(ctx, Event{Type: EventCharacterCreate, Recipient: r})
}

// dispatch runs each script's handler for the event, then executes the grants
// and mails that handler collected. A missing handler is skipped; a runtime
// error is logged and the remaining scripts still run.
func (s *Service) dispatch(ctx context.Context, ev Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	handler := string(ev.Type)
	for _, sc := range s.scripts {
		fn := sc.handlers[handler]
		if fn == nil {
			continue
		}
		s.collector = &collector{}
		if err := s.callHandler(fn, ev); err != nil {
			s.logf("reward: script %s %s failed: %v", sc.name, handler, err)
			s.collector = nil
			continue
		}
		s.flush(ctx, ev, sc.name, s.collector)
	}
	s.collector = nil
}

// flush performs the collected capabilities. A nil callback skips that
// capability with a log line; persistence errors are logged, never returned.
func (s *Service) flush(ctx context.Context, ev Event, scriptName string, c *collector) {
	if c == nil {
		return
	}
	if len(c.items) > 0 {
		key := baseKey(ev, scriptName) + ":item"
		switch {
		case s.grant == nil:
			s.logf("reward grant/mail capability is not configured")
		case len(key) > maxKeyLength:
			s.logf("reward: grant key too long for %s", scriptName)
		default:
			if err := s.grant(ctx, ev.Recipient, key, c.items); err != nil {
				s.logf("reward: grant %s failed: %v", key, err)
			}
		}
	}
	for i, m := range c.mails {
		key := baseKey(ev, scriptName) + ":mail"
		if i > 0 {
			key = fmt.Sprintf("%s:mail:%d", baseKey(ev, scriptName), i)
		}
		switch {
		case s.mail == nil:
			s.logf("reward grant/mail capability is not configured")
		case len(key) > maxKeyLength:
			s.logf("reward: mail key too long for %s", scriptName)
		default:
			if err := s.mail(ctx, ev.Recipient, key, m); err != nil {
				s.logf("reward: mail %s failed: %v", key, err)
			}
		}
	}
	if c.cera > 0 {
		key := baseKey(ev, scriptName) + ":cera"
		switch {
		case s.cera == nil:
			s.logf("reward cera capability is not configured")
		case len(key) > maxKeyLength:
			s.logf("reward: cera key too long for %s", scriptName)
		default:
			if err := s.cera(ctx, ev.Recipient, key, c.cera); err != nil {
				s.logf("reward: cera %s failed: %v", key, err)
			}
		}
	}
}

// baseKey is stable so a replayed event dedups in the character event table.
// The script filename (not the full path) is the discriminator.
func baseKey(ev Event, scriptName string) string {
	discriminator := path.Base(scriptName)
	if ev.Type == EventQuestComplete {
		return fmt.Sprintf("reward:quest_complete:%s:%d", discriminator, ev.QuestID)
	}
	if ev.Type == EventCharacterCreate {
		return fmt.Sprintf("reward:character_create:%s:%d", discriminator, ev.Recipient.CharacterID)
	}
	return fmt.Sprintf("reward:level_up:%s:%d", discriminator, ev.Recipient.Level)
}

func (s *Service) logf(format string, args ...any) {
	if s != nil && s.log != nil {
		s.log(format, args...)
	}
}
