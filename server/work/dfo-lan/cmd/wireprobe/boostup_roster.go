package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"fmt"
	"time"
)

func selectionRosterPackets(ctx context.Context, s *character.Service, account int64, fatigue *character.FatigueService, activity *boostup.Catalog) ([]outboundPacket, error) {
	if s == nil || s.Store == nil || account <= 0 {
		return nil, fmt.Errorf("owned roster service required")
	}
	var packets []outboundPacket
	if activity != nil {
		roles, e := s.Store.Characters(ctx, account)
		if e != nil {
			return nil, e
		}
		for _, r := range roles {
			if r.AccountID != account {
				return nil, fmt.Errorf("foreign boost roster role")
			}
		}
		marker, e := boostRosterForRoles(roles, database.Character{})
		if e != nil {
			return nil, e
		}
		packets = append(packets, outboundPacket{"boost_roster_restored", 0, 2639, marker})
	}
	p, e := s.ListWithFatigue(ctx, account, fatigue, time.Now())
	if e != nil {
		return nil, e
	}
	return append(packets, outboundPacket{"character_roster_snapshot", 0, 2, p}), nil
}

func boostLoginRoster(ctx context.Context, s *database.Store, account int64, activity *boostup.Catalog) ([]outboundPacket, error) {
	if activity == nil {
		return nil, nil
	}
	if s == nil || account <= 0 {
		return nil, fmt.Errorf("boost login roster owner missing")
	}
	roles, e := s.Characters(ctx, account)
	if e != nil {
		return nil, e
	}
	marker, e := boostRosterForRoles(roles, database.Character{})
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"boost_roster_restored", 0, 2639, marker}}, nil
}
