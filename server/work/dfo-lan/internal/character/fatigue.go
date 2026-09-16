package character

import (
	"context"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"
)

type FatigueRules struct {
	DailyLimit uint16 `json:"daily_limit"`
	RoomCost   uint16 `json:"room_cost"`
	Timezone   string `json:"timezone"`
	ResetHour  int    `json:"reset_hour"`
	Source     string `json:"source"`
}
type FatigueService struct {
	Store    *storage.Store
	Rules    FatigueRules
	Location *time.Location
}

func LoadFatigueService(s *storage.Store, path string) (*FatigueService, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var r FatigueRules
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	if r.DailyLimit == 0 || r.ResetHour < 0 || r.ResetHour > 23 || r.Timezone == "" || r.Source == "" {
		return nil, fmt.Errorf("invalid fatigue policy")
	}
	l, e := time.LoadLocation(r.Timezone)
	if e != nil {
		return nil, e
	}
	return &FatigueService{s, r, l}, nil
}
func (s *FatigueService) day(now time.Time) string {
	local := now.In(s.Location)
	if local.Hour() < s.Rules.ResetHour {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02")
}
func (s *FatigueService) State(ctx context.Context, account, id int64, now time.Time) (storage.FatigueState, error) {
	return s.Store.LoadFatigue(ctx, account, id, s.day(now), s.Rules.DailyLimit)
}
func (s *FatigueService) EnterRoom(ctx context.Context, account, id int64, run string, room uint32, exempt bool, now time.Time) (storage.FatigueState, bool, error) {
	cost := s.Rules.RoomCost
	if exempt {
		cost = 0
	}
	return s.Store.ConsumeRoomFatigue(ctx, account, id, s.day(now), s.Rules.DailyLimit, run, room, cost)
}
