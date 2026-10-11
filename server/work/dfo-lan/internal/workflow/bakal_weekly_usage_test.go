package workflow

import (
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBakalWeeklyProjectionMatchesAdmissionAndPreservesLegacyState(t *testing.T) {
	now := time.Date(2026, 10, 7, 5, 0, 0, 0, time.UTC)
	state := json.RawMessage(`{"other":{"keep":42},"bakal_raid_rewards":{"week":"` + bakalWeekOf(now) + `","clears":1,"rewards":1,"plans":{}}}`)
	role := database.Character{ID: 7, State: state}
	before := string(role.State)
	usage, err := BakalWeeklyUsage(role, now)
	if err != nil || usage.Clears != 1 || usage.Rewards != 1 {
		t.Fatalf("legacy projection=%+v err=%v", usage, err)
	}
	if !errors.Is(BakalRaidAdmission(bakalTestRules(), role, now), ErrBakalWeeklyClearLimit) {
		t.Fatal("UI exhausted but admission allowed")
	}
	nextWeek := now.Add(7 * 24 * time.Hour)
	usage, err = BakalWeeklyUsage(role, nextWeek)
	if err != nil || usage != (BakalWeeklyCounters{}) || BakalRaidAdmission(bakalTestRules(), role, nextWeek) != nil {
		t.Fatal("week rollover did not refresh both UI and admission")
	}
	if string(role.State) != before {
		t.Fatal("read-only quota projection changed existing save")
	}
	if _, err := BakalWeeklyUsage(database.Character{State: json.RawMessage(`{"bakal_raid_rewards":[]}`)}, now); err == nil {
		t.Fatal("malformed ledger became available UI quota")
	}
}
