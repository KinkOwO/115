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
	// EnterFatigueOf 返回源为某副本声明的**进本消耗**（0 = 源未声明该段）。
	// 由装配方从副本目录注入，使疲劳模块不直接依赖 catalog。
	// 见 internal/catalog/dungeons.go 的 DungeonDefinition.EnterFatigue。
	EnterFatigueOf func(dungeonID uint32) uint16
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
	return &FatigueService{Store: s, Rules: r, Location: l}, nil
}
func (s *FatigueService) day(now time.Time) string {
	local := now.In(s.Location)
	if local.Hour() < s.Rules.ResetHour {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02")
}
func (s *FatigueService) Day(now time.Time) string { return s.day(now) }
func (s *FatigueService) State(ctx context.Context, account, id int64, now time.Time) (storage.FatigueState, error) {
	return s.Store.LoadFatigue(ctx, account, id, s.day(now), s.Rules.DailyLimit)
}
func (s *FatigueService) EnterRoom(ctx context.Context, account, id int64, run string, room uint32, exempt bool, now time.Time) (storage.FatigueState, bool, error) {
	cost := s.Rules.RoomCost
	if exempt {
		cost = 0
	} else if s.Store != nil && cost > 0 {
		if hasGrowth, _ := s.Store.HasActivePremium(ctx, account, storage.PremiumGrowth, now); hasGrowth {
			cost--
		}
	}
	return s.Store.ConsumeRoomFatigue(ctx, account, id, s.day(now), s.Rules.DailyLimit, run, room, cost)
}

// EnterCostFor 是**进本准入**与**记费**共用的口径：源声明优先，否则回退本地策略。
//
// 两者必须是同一个口径 —— 2026-10-01 实测过一次不一致的后果：准入按 RoomCost
// （本地策略为 0，整个检查被跳过）、记费按源声明（8 点），于是客户端已经进本、
// 正在等加载应答时才被拒（ErrFatigueExhausted），表现为「卡在加载界面出不来」。
func (s *FatigueService) EnterCostFor(dungeonID uint32) uint16 {
	if s == nil {
		return 0
	}
	if s.EnterFatigueOf != nil {
		if c := s.EnterFatigueOf(dungeonID); c > 0 {
			return c
		}
	}
	return s.Rules.RoomCost
}

// EnterRoomForDungeon 记一次房间消耗：
//   - 源声明了 [use fatigue only start dungeon] 的副本：**只在第一次进本收官方值**，
//     同一 run 的后续房间收 0（`only start` 的字面语义）；
//   - 未声明的副本：保持原有「每房间收 Rules.RoomCost」的行为，一个字节不变。
//
// 成长契约的减免对两种口径一视同仁（实机：官方 8 点 − 契约 1 = 每本 +7）。
func (s *FatigueService) EnterRoomForDungeon(ctx context.Context, account, id int64, run string, room, dungeonID uint32, exempt bool, now time.Time) (storage.FatigueState, bool, error) {
	var cost uint16
	switch {
	case exempt:
	case s.EnterFatigueOf != nil && s.EnterFatigueOf(dungeonID) > 0:
		paid, e := s.Store.RunPaidFatigue(ctx, id, run)
		if e != nil {
			return storage.FatigueState{}, false, e
		}
		if !paid {
			cost = s.EnterFatigueOf(dungeonID)
		}
	default:
		cost = s.Rules.RoomCost
	}
	if cost > 0 && s.Store != nil {
		if hasGrowth, _ := s.Store.HasActivePremium(ctx, account, storage.PremiumGrowth, now); hasGrowth {
			cost--
		}
	}
	return s.Store.ConsumeRoomFatigue(ctx, account, id, s.day(now), s.Rules.DailyLimit, run, room, cost)
}
