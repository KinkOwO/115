package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/world"
	"encoding/json"
	"fmt"
	"reflect"
)

// ordinaryBoostChannel 是本树的「普通频道」判据：每连接 world 服务按真实频道号
// 克隆（ChannelID 0..11 都是普通城镇频道），特殊玩法频道由频道目录行 Type
// 区分（73=黑鸦小队，106=血矿），与 donor 的 ChannelID==0 语义不同。
func (w *worldSession) ordinaryBoostChannel() bool {
	return w.channelType != 73 && w.channelType != 106
}

// Admit precisely this source-defined step, not every event permission.
// The base catalog is shared/read-only; this connection gets a value copy.
func boostWorldTarget(base *world.Service, c *boostup.Catalog, st boostup.State, level byte, ordinary bool) (*world.Service, database.WorldPosition, error) {
	var target database.WorldPosition
	if base == nil || c == nil || !st.Activated || st.Training.Finished || st.Training.Step == 0 || int(st.Training.Step) > len(c.Steps) || !ordinary {
		return nil, target, fmt.Errorf("boost town needs owned active ordinary-channel training")
	}
	step := c.Steps[int(st.Training.Step)-1]
	key := catalog.AreaKey(c.Town, uint32(step.Area))
	area, ok := base.Catalog.Areas[key]
	if !ok {
		return nil, target, fmt.Errorf("boost step area missing")
	}
	ids, conditions, opens, closes := 0, 0, 0, 0
	var event, condition int32
	for i, t := range area.Definition {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[check event condition]":
			opens++
		case "[/check event condition]":
			closes++
		case "[event id]", "[condition]":
			if i+1 >= len(area.Definition) || area.Definition[i+1].Type != 0 {
				return nil, target, fmt.Errorf("invalid source event permission")
			}
			if t.Text == "[event id]" {
				ids++
				event = area.Definition[i+1].Value
			} else {
				conditions++
				condition = area.Definition[i+1].Value
			}
		}
	}
	if ids != 1 || conditions != 1 || opens != 1 || closes != 1 || event != int32(boostup.EventID) || condition != int32(st.Training.Step) {
		return nil, target, fmt.Errorf("source permission is not this training step")
	}
	var pending []string
	for _, p := range area.Pending {
		switch p {
		case "unsupported permission [check event condition]", "unsupported permission [/check event condition]", "unsupported permission [event id]", "unsupported permission [condition]":
		default:
			pending = append(pending, p)
		}
	}
	area.Pending = pending
	found := false
	for i, t := range area.Definition {
		if t.Type != 6 || t.Text != area.Kind || i+2 >= len(area.Definition) {
			continue
		}
		x, y := area.Definition[i+1], area.Definition[i+2]
		if x.Type == 0 && y.Type == 0 && x.Value >= 0 && x.Value <= 65535 && y.Value >= 0 && y.Value <= 65535 {
			target = database.WorldPosition{Town: c.Town, Area: uint32(step.Area), X: uint16(x.Value), Y: uint16(y.Value)}
			found = true
			break
		}
	}
	if !found {
		return nil, target, fmt.Errorf("source boost area lacks entry coordinates")
	}
	clone := *base
	clone.Catalog.Areas = make(map[string]catalog.WorldArea, len(base.Catalog.Areas))
	for k, a := range base.Catalog.Areas {
		clone.Catalog.Areas[k] = a
	}
	clone.Catalog.Areas[key] = area
	// 胶囊教学城镇（Seria 房）是普通城镇区，奥德赛入场等级不适用。
	if err := clone.ValidatePosition(level, false, target); err != nil {
		return nil, target, err
	}
	return &clone, target, nil
}

// Activation and Origin are durable before this idempotent scene handoff.
// If the world write or connection fails, next entry retries the same step
// without charging another capsule or restoring someone else's coordinates.
func (w *worldSession) enterBoostWorld(ctx context.Context, role database.Character, level byte, spawn database.WorldPosition) (database.WorldState, bool, error) {
	var none database.WorldState
	if w.boostup == nil {
		return none, false, nil
	}
	if role.ID == 0 || role.AccountID != w.account {
		return none, true, fmt.Errorf("boost world character owner mismatch")
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return none, false, e
	}
	if !st.Activated {
		return none, false, nil
	}
	base := w.service
	if w.boostWorldBase != nil {
		base = w.boostWorldBase
	}
	if base == nil || w.store == nil {
		return none, true, fmt.Errorf("boost world unavailable on this channel")
	}
	ordinary := w.ordinaryBoostChannel()
	if !ordinary {
		if st.Training.Finished {
			return none, false, nil
		}
		return none, true, fmt.Errorf("active boost training cannot enter a special channel town")
	}
	var origin database.WorldPosition
	if len(st.Origin) == 0 || json.Unmarshal(st.Origin, &origin) != nil || origin.Town == w.boostup.Town {
		return none, true, fmt.Errorf("boost ordinary return point missing/invalid")
	}
	if e = base.ValidatePosition(level, character.OdysseyMember(role), origin); e != nil {
		return none, true, e
	}
	selected, target := base, origin
	if !st.Training.Finished {
		selected, target, e = boostWorldTarget(base, w.boostup, st, level, ordinary)
		if e != nil {
			return none, true, e
		}
	}
	// 本树 LoadWorld 的 version 与 characters.config_version 同语义（契约身份），
	// world.Service.Enter 也传 SaveIdentity()；donor 基线传内层 Checksum，移植时
	// 必须换掉，否则 :148 的回读比较在生产存档上恒不匹配。
	saved, e := w.store.LoadWorld(ctx, role.AccountID, role.ID, w.worldStorageType(), origin, base.Catalog.Source.SaveIdentity())
	if e != nil {
		return none, true, e
	}
	if st.Training.Finished && saved.Position.Town != w.boostup.Town {
		return none, false, nil
	}
	if !st.Training.Finished && saved.Position.Town == target.Town && saved.Position.Area == target.Area && selected.ValidatePosition(level, character.OdysseyMember(role), saved.Position) == nil {
		target = saved.Position // do not teleport a relogging actor across its own current room
	}
	if saved.ConfigVersion != base.Catalog.Source.SaveIdentity() {
		return none, true, fmt.Errorf("boost world source mismatch")
	}
	if !reflect.DeepEqual(saved.Position, target) {
		saved, e = w.store.SaveWorld(ctx, role.AccountID, role.ID, w.worldStorageType(), saved, target)
		if e != nil {
			return none, true, e
		}
	}
	w.boostWorldBase = base
	w.service = selected
	return saved, true, nil
}

// Native C36 moves between source step areas after N2638 advances the owned
// step. It is NOT an arbitrary town teleport, nor a claim/completion proof.
func (w *worldSession) boostAreaTransition(r protocol.AreaChangeRequest) (database.WorldPosition, bool, error) {
	old := w.state.Position
	if w.boostup == nil || old.Town != w.boostup.Town {
		return old, false, nil
	}
	fail := func(e error) (database.WorldPosition, bool, error) { return old, true, e }
	if w.activeDungeon != nil || r.PreviousTown != old.Town || uint32(r.PreviousArea) != old.Area {
		return fail(fmt.Errorf("stale boost area transition"))
	}
	st, e := boostup.ReadState(w.role.State)
	if e != nil {
		return fail(e)
	}
	if !st.Activated {
		return fail(fmt.Errorf("boost area belongs to another role"))
	}
	base := w.service
	if w.boostWorldBase != nil {
		base = w.boostWorldBase
	}
	if st.Training.Finished {
		var origin database.WorldPosition
		if len(st.Origin) == 0 || json.Unmarshal(st.Origin, &origin) != nil || r.Town != origin.Town || r.Area != origin.Area {
			return fail(fmt.Errorf("boost graduation return differs from saved origin"))
		}
		if e = base.ValidatePosition(w.level, w.odyssey, origin); e != nil {
			return fail(e)
		}
		w.service = base
		return origin, true, nil
	}
	scoped, target, e := boostWorldTarget(base, w.boostup, st, w.level, w.ordinaryBoostChannel())
	if e != nil {
		return fail(e)
	}
	if r.Town != target.Town || r.Area != target.Area {
		return fail(fmt.Errorf("boost target is not current authorized step"))
	}
	target.X, target.Y = r.X, r.Y
	if e = scoped.ValidatePosition(w.level, w.odyssey, target); e != nil {
		return fail(e)
	}
	w.boostWorldBase = base
	w.service = scoped
	return target, true, nil
}

func (w *worldSession) boostTownRefresh(ctx context.Context) ([]outboundPacket, error) {
	var st character.State
	if e := json.Unmarshal(w.role.State, &st); e != nil {
		return nil, e
	}
	saved, handled, e := w.enterBoostWorld(ctx, w.role, st.Level, w.state.Position)
	if e != nil {
		return nil, e
	}
	if !handled {
		return nil, fmt.Errorf("boost scene without activation")
	}
	w.state = saved
	own, e := w.userAreaPayload()
	if e != nil {
		return nil, e
	}
	area, e := w.areaPayload()
	if e != nil {
		return nil, e
	}
	state, e := protocol.UserState(w.role.WireID, protocol.UserStateTown)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"boost_town_user", 0, 23, own}, {"boost_town_area", 0, 24, area}, {"boost_town_state", 0, 3, state}}, nil
}
