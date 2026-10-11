package legion

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
)

func TestStagedLegionNativeSourcePlan(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("real archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		c, ok := contents.Contents[name]
		if !ok || c.NativeOperations == nil {
			t.Fatal("source content missing", name)
		}
		weeks := []int{-1}
		if len(c.NativeOperations.WeeklyDungeons) > 0 {
			weeks = nil
			for i := range c.NativeOperations.WeeklyDungeons {
				weeks = append(weeks, i)
			}
		}
		for _, week := range weeks {
			r, err := NewStagedRun(&contents, name, week, StagedOperationMode{"normal", "party", "normal"})
			if err != nil {
				t.Fatal(name, week, err)
			}
			for stage := 0; stage <= int(c.LastPhase); stage++ {
				card, err := c.NativeOperations.DrawCard("normal", "party", "normal", 0, func(n int64) (int64, error) { return 0, nil })
				if err != nil {
					t.Fatal(name, stage, err)
				}
				if err = r.Offer(&contents, []catalog.NativeLegionCard{card}); err != nil {
					t.Fatal(err)
				}
				if err = r.Select(card.Index); err != nil {
					t.Fatal(err)
				}
				d, err := r.Entry(stage)
				if err != nil {
					t.Fatal(err)
				}
				run := name + string(rune('A'+stage))
				rewards, e := catalog.NativePhaseRewards(&contents, name, d.Dungeon, card.Index, "normal", "normal")
				if e != nil || len(rewards.Basic) == 0 || len(rewards.Additional) == 0 {
					t.Fatal("source reward dimension", rewards, e)
				}
				if err = r.Enter(stage, d.Dungeon, run); err != nil {
					t.Fatal(err)
				}
				if err = r.Complete(run, d.Dungeon); err != nil {
					t.Fatal(err)
				}
				if err = r.RewardEnd(run); err != nil {
					t.Fatal(err)
				}
				t.Logf("content=%s source=%s weekrow=%d stage=%d dungeon=%d boss=%d operation=%d fixed=%v parameter=%v native_client_started=false", name, contents.Source.Checksum, week, stage, d.Dungeon, d.Boss, card.Index, card.FixedValue, card.Parameter)
			}
			if !r.Finished() {
				t.Fatal("source sequence incomplete", name, week)
			}
		}
		if len(c.NativeOperations.ClearRewards) == 0 {
			t.Fatal("source operation clear reward not parsed", name)
		}
		for index, def := range c.NativeOperations.Definitions {
			if len(def.Rewards) == 0 {
				t.Fatal("source operation reward not parsed", name, index)
			}
		}
	}
}
