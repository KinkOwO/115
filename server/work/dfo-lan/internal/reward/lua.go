package reward

import (
	"fmt"
	"io/fs"

	lua "github.com/yuin/gopher-lua"
)

// collector accumulates the grants/mails a single handler invocation emits.
// It is stored on the Service and only touched under the service mutex.
type collector struct {
	items []ItemGrant
	mails []MailReward
	cera  uint64
}

// script is one loaded *.lua file plus the handlers it registered with on().
// Handlers are captured per script, so a later script cannot shadow an earlier
// one's registration.
type script struct {
	name     string
	handlers map[string]*lua.LFunction
}

// knownEvents are the event names on() accepts. Only these are dispatched.
var knownEvents = map[string]bool{
	string(EventLevelUp):         true,
	string(EventQuestComplete):   true,
	string(EventCharacterCreate): true,
}

// loadScripts creates the single LState, registers the Go globals once, then
// loads each script source so the on() registrations are captured per script.
func (s *Service) loadScripts(src fs.FS, files []string) error {
	state := lua.NewState()
	s.state = state
	s.register(state)
	for _, name := range files {
		data, err := fs.ReadFile(src, name)
		if err != nil {
			state.Close()
			s.state = nil
			return fmt.Errorf("reward: read %s: %w", name, err)
		}
		s.pending = map[string]*lua.LFunction{}
		if err := state.DoString(string(data)); err != nil {
			state.Close()
			s.state = nil
			return fmt.Errorf("reward: load %s: %w", name, err)
		}
		s.scripts = append(s.scripts, script{name: name, handlers: s.pending})
	}
	s.pending = nil
	return nil
}

// register installs the Lua-facing reward API. on() registers an event handler
// while a script is loading; grant_item/send_mail/grant_cera append to the
// current collector and never touch persistence directly.
func (s *Service) register(state *lua.LState) {
	state.SetGlobal("on", state.NewFunction(func(l *lua.LState) int {
		name := l.CheckString(1)
		fn := l.CheckFunction(2)
		if !knownEvents[name] {
			l.RaiseError("unknown reward event %q (want level_up, quest_complete or character_create)", name)
			return 0
		}
		if s.pending == nil {
			s.pending = map[string]*lua.LFunction{}
		}
		s.pending[name] = fn
		return 0
	}))
	state.SetGlobal("grant_item", state.NewFunction(func(l *lua.LState) int {
		id := l.CheckNumber(1)
		count := l.CheckNumber(2)
		// id 0 is the character gold stack (inventory.Bag.Add), not an item, so
		// it is a valid grant; only a negative id or non-positive count is
		// rejected. The awarder resolves other ids against the catalog.
		if id < 0 || count <= 0 {
			l.RaiseError("grant_item requires a non-negative id and positive count")
			return 0
		}
		if s.collector == nil {
			s.collector = &collector{}
		}
		s.collector.items = append(s.collector.items, ItemGrant{ID: uint32(id), Count: uint32(count)})
		return 0
	}))
	state.SetGlobal("send_mail", state.NewFunction(func(l *lua.LState) int {
		subject := l.CheckString(1)
		body := l.CheckString(2)
		m := MailReward{Subject: subject, Body: body}
		switch l.GetTop() {
		case 2:
			// Notification mail without attachments.
		case 3:
			if value := l.Get(3); value != lua.LNil {
				table, ok := value.(*lua.LTable)
				if !ok {
					l.RaiseError("send_mail attachments must be a table")
					return 0
				}
				attachments, err := readAttachments(l, table)
				if err != nil {
					l.RaiseError("%s", err.Error())
					return 0
				}
				m.Attachments = attachments
			}
		default:
			l.RaiseError("send_mail expects (subject, body) or (subject, body, attachments)")
			return 0
		}
		if s.collector == nil {
			s.collector = &collector{}
		}
		s.collector.mails = append(s.collector.mails, m)
		return 0
	}))
	state.SetGlobal("grant_cera", state.NewFunction(func(l *lua.LState) int {
		amount := l.CheckNumber(1)
		if amount <= 0 {
			l.RaiseError("grant_cera requires a positive amount")
			return 0
		}
		if s.collector == nil {
			s.collector = &collector{}
		}
		s.collector.cera += uint64(amount)
		return 0
	}))
}

// readAttachments reads a Lua array of { id = template, count = amount }.
// A malformed entry or more than the mail cap (11) is an error.
func readAttachments(l *lua.LState, table *lua.LTable) ([]ItemGrant, error) {
	n := table.Len()
	if n > 11 {
		return nil, fmt.Errorf("too many attachments (%d, max 11)", n)
	}
	out := make([]ItemGrant, 0, n)
	for i := 1; i <= n; i++ {
		entry, ok := table.RawGetInt(i).(*lua.LTable)
		if !ok {
			return nil, fmt.Errorf("attachment %d must be a { id = ..., count = ... } table", i)
		}
		id := lua.LVAsNumber(entry.RawGetString("id"))
		count := lua.LVAsNumber(entry.RawGetString("count"))
		if float64(id) <= 0 || float64(count) <= 0 {
			return nil, fmt.Errorf("attachment %d requires positive id and count", i)
		}
		out = append(out, ItemGrant{ID: uint32(id), Count: uint32(count)})
	}
	return out, nil
}

// callHandler builds the ctx table, publishes it as the global ctx and calls
// the protected handler.
func (s *Service) callHandler(fn *lua.LFunction, ev Event) error {
	state := s.state
	ctx := state.NewTable()
	ctx.RawSetString("type", lua.LString(string(ev.Type)))
	ctx.RawSetString("level", lua.LNumber(ev.Recipient.Level))
	ctx.RawSetString("quest_id", lua.LNumber(ev.QuestID))
	ctx.RawSetString("character_id", lua.LNumber(ev.Recipient.CharacterID))
	ctx.RawSetString("account_id", lua.LNumber(ev.Recipient.AccountID))
	ctx.RawSetString("name", lua.LString(ev.Recipient.Name))
	state.SetGlobal("ctx", ctx)
	return state.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, ctx)
}
