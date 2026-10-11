package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

// The protocol enum identifies a requested mode; the current PVF decides
// which modes the channel exposes. Never substitute normal rules for an
// unavailable single/match/hard mode.
func delezieModeForTeam(entry catalog.RaidEntrance, channel uint32, request protocol.RaidCreateRequest) (*catalog.DelezieModeRules, error) {
	difficulty, err := request.DelezieMode115()
	if err != nil {
		return nil, err
	}
	mode, err := entry.Delezie.ModeForChannel(channel, catalog.DelezieDifficulty(difficulty))
	if err != nil {
		return nil, err
	}
	if mode.Difficulty != catalog.DelezieDifficulty(difficulty) || mode.Phase == nil || mode.Phase.Table == nil {
		return nil, fmt.Errorf("native Delezie mode source is incomplete or mismatched")
	}
	return mode, nil
}
