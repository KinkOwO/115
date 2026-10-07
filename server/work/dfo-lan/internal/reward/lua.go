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
	// ents 是本 handler 收集到的角色待遇（存档字段，不是物品）。
	ents Entitlements
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
	// —— 角色待遇：**存档字段**，不是物品（挂锁 / 扩容档位 / 复活币）——
	//
	// 这些字段 grant_item / send_mail 碰不到（客户端读的是角色 state），所以单开一组能力。
	// 语义一律"累加/取高"而不是"赋值"：脚本没调就不动，调了也绝不把玩家已有的待遇改小。
	s.registerEntitlements(state)
}

// registerEntitlements installs the character-detail capabilities:
//
//	unlock_equip_slots(mask)   扩展装备栏挂锁位（按位或）
//	expand_bag(tier)           背包扩容档位（只升不降）
//	expand_avatar(tier)        时装栏扩容档位（只升不降）
//	grant_revive_coin(count)   复活币
func (s *Service) registerEntitlements(state *lua.LState) {
	collect := func() *collector {
		if s.collector == nil {
			s.collector = &collector{}
		}
		return s.collector
	}
	state.SetGlobal("unlock_equip_slots", state.NewFunction(func(l *lua.LState) int {
		mask := l.CheckNumber(1)
		if mask < 0 || mask > 255 {
			l.RaiseError("unlock_equip_slots requires a bit mask between 0 and 255")
			return 0
		}
		collect().ents.EquipSlotMask |= byte(mask)
		return 0
	}))
	state.SetGlobal("expand_bag", state.NewFunction(func(l *lua.LState) int {
		tier := l.CheckNumber(1)
		if tier < 0 || tier > 255 {
			l.RaiseError("expand_bag requires a tier between 0 and 255")
			return 0
		}
		v := byte(tier)
		collect().ents.BagTier = &v
		return 0
	}))
	state.SetGlobal("expand_avatar", state.NewFunction(func(l *lua.LState) int {
		tier := l.CheckNumber(1)
		if tier < 0 || tier > 255 {
			l.RaiseError("expand_avatar requires a tier between 0 and 255")
			return 0
		}
		v := byte(tier)
		collect().ents.AvatarTier = &v
		return 0
	}))
	state.SetGlobal("grant_revive_coin", state.NewFunction(func(l *lua.LState) int {
		count := l.CheckNumber(1)
		if count <= 0 {
			l.RaiseError("grant_revive_coin requires a positive count")
			return 0
		}
		c := collect()
		c.ents.ReviveCoins += uint32(count)
		return 0
	}))
	// —— 第二批（2026-10-06）：宠物 / 金库 / 账号材料仓 / 皮肤仓库 ——
	//
	// 参数校验只做"数是否是正数/在字节范围内"，**种类判定留给服务端**：模板到底是不是宠物本体、
	// 是不是宠物用品、是不是可入库的材料，由服务端按目录判并给出带模板号的报错 —— 这符合
	// "能不能发是服务端能力、发什么由脚本决定"。
	state.SetGlobal("grant_pet", state.NewFunction(func(l *lua.LState) int {
		template := l.CheckNumber(1)
		if template <= 0 {
			l.RaiseError("grant_pet requires a positive template id")
			return 0
		}
		c := collect()
		c.ents.Pets = append(c.ents.Pets, ItemGrant{ID: uint32(template), Count: 1})
		return 0
	}))
	state.SetGlobal("grant_pet_item", state.NewFunction(func(l *lua.LState) int {
		template := l.CheckNumber(1)
		count := l.CheckNumber(2)
		if template <= 0 || count <= 0 {
			l.RaiseError("grant_pet_item requires a positive template id and count")
			return 0
		}
		c := collect()
		c.ents.PetItems = append(c.ents.PetItems, ItemGrant{ID: uint32(template), Count: uint32(count)})
		return 0
	}))
	state.SetGlobal("expand_vault", state.NewFunction(func(l *lua.LState) int {
		space := l.CheckNumber(1)
		slots := l.CheckNumber(2)
		if slots < 0 || slots > 65535 {
			l.RaiseError("expand_vault requires slots between 0 and 65535")
			return 0
		}
		v := uint16(slots)
		c := collect()
		switch byte(space) {
		case 2:
			c.ents.Vault1Slots = &v
		case 45:
			c.ents.Vault2Slots = &v
		case 12:
			c.ents.AccountVaultSlots = &v
		default:
			l.RaiseError("expand_vault space must be 2 (vault 1), 45 (vault 2) or 12 (account vault)")
		}
		return 0
	}))
	state.SetGlobal("grant_account_material", state.NewFunction(func(l *lua.LState) int {
		template := l.CheckNumber(1)
		count := l.CheckNumber(2)
		if template <= 0 || count <= 0 {
			l.RaiseError("grant_account_material requires a positive template id and count")
			return 0
		}
		c := collect()
		if c.ents.AccountMaterialByTemplate == nil {
			c.ents.AccountMaterialByTemplate = map[uint32]uint32{}
		}
		// 按**模板**聚合，落库时再由服务端映射到账号材料仓的固定槽位
		// （服务端只认 19 个模板；不在映射里的会在落库时被拒并点名）。
		c.ents.AccountMaterialByTemplate[uint32(template)] += uint32(count)
		return 0
	}))
	state.SetGlobal("unlock_skins", state.NewFunction(func(l *lua.LState) int {
		c := collect()
		c.ents.UnlockSkins = true
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
	// 职业信息只在**知道**的时候才写进 ctx：字段缺席时 Lua 侧是 nil，规则可以
	// `if not ctx.profession then return end` 干净地跳过（见 Recipient.HasProfession 的注释）。
	if ev.Recipient.HasProfession {
		ctx.RawSetString("profession", lua.LNumber(ev.Recipient.Profession))
		ctx.RawSetString("advancement", lua.LNumber(ev.Recipient.Advancement))
	}
	state.SetGlobal("ctx", ctx)
	return state.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, ctx)
}
