package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

// tutorialRoute resolves this character's own starting route from the source
// tutorial flow table. An event-only route is never selected for an ordinary
// character, and a job without a route simply has none.
func (w *worldSession) tutorialRoute() (uint32, [2]uint16, uint32, uint32, error) {
	var none [2]uint16
	if w.tutorials == nil {
		return 0, none, 0, 0, fmt.Errorf("tutorial routes are not loaded")
	}
	var state character.State
	if e := json.Unmarshal(w.role.State, &state); e != nil {
		return 0, none, 0, 0, e
	}
	job := w.service.Catalog.Source.Checksum
	_ = job
	profession, ok := w.professions.Professions[w.role.Profession]
	if !ok {
		return 0, none, 0, 0, fmt.Errorf("character profession absent from source")
	}
	f, e := w.tutorials.Normal(profession.Job, state.Advancement)
	if e != nil {
		return 0, none, 0, 0, e
	}
	return f.Dungeon, f.Position, f.Town, f.Area, nil
}

// authorizeTutorial accepts a tutorial entry only for this character's own
// source route, and only while the route is still owed. It never resets or
// replays a route a character has already been through.
func (w *worldSession) authorizeTutorial(requested uint32) error {
	if w.dungeons == nil || w.tutorialDungeons == nil {
		return fmt.Errorf("tutorial dungeons are not loaded")
	}
	route, _, _, _, e := w.tutorialRoute()
	if e != nil {
		return e
	}
	if route != requested {
		return fmt.Errorf("dungeon %d is not this character's source starting route", requested)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stage, _, e := w.store.BirthStage(ctx, w.account, w.role.ID)
	if e != nil {
		return e
	}
	if stage >= storage.BirthComplete {
		return fmt.Errorf("starting route already finished")
	}
	return nil
}

// selectTutorial opens the owned starting dungeon for a tutorial CMD16.
func (w *worldSession) selectTutorial(requested uint32) (*dungeon.Session, []outboundPacket, error) {
	if e := w.authorizeTutorial(requested); e != nil {
		return nil, nil, e
	}
	s, e := dungeon.SelectTutorial(*w.tutorialDungeons, requested)
	if e != nil {
		return nil, nil, e
	}
	seed, e := randomSeed()
	if e != nil {
		return nil, nil, e
	}
	start, e := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Seed: seed, Map: s.Room.Map, Monsters: s.Monsters, EncodeCreateTrigger: monsterCreateTriggerEnabled()})
	if e != nil {
		return nil, nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e = w.store.AdvanceBirth(ctx, w.account, w.role.ID, storage.BirthEntered, requested); e != nil {
		return nil, nil, e
	}
	w.inTutorial = true
	return s, []outboundPacket{
		{"tutorial_select_ack", 1, 16, []byte{1}},
		{"dungeon_info_sent", 0, 28, protocol.DungeonInfo(protocol.DungeonInfoState{ID: requested, Maze: s.Maze.Index, Boss: s.Maze.Boss})},
		{"dungeon_start_map_sent", 0, 29, start},
	}, nil
}

// settleTutorialReturn closes the starting route once, on whichever way out
// of the dungeon the character takes, and moves it to the town position the
// source route names rather than the dungeon gate an ordinary leave restores.
// The stage only ever moves forward, so a reconnect cannot replay the route.
func (w *worldSession) settleTutorialReturn() error {
	_, position, town, area, e := w.tutorialRoute()
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e = w.store.AdvanceBirth(ctx, w.account, w.role.ID, storage.BirthComplete, 0); e != nil {
		return e
	}
	next := storage.WorldPosition{Town: town, Area: area, X: position[0], Y: position[1]}
	if e = w.service.ValidatePosition(w.level, w.odyssey, next); e != nil {
		return e
	}
	saved, e := w.store.SaveWorld(ctx, w.account, w.role.ID, w.state, next)
	if e != nil {
		return e
	}
	w.state, w.inTutorial = saved, false
	return nil
}
