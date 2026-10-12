package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"reflect"
	"testing"
)

func TestVenusRewardEndCannotClearUnownedOrLivingStage(t *testing.T) {
	for _, kind := range []string{"no dungeon", "living", "foreign completed"} {
		t.Run(kind, func(t *testing.T) {
			w := &worldSession{venus: &venusRun{choice: 1, stage: 0}}
			if kind != "no dungeon" {
				w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]}}
				if kind == "foreign completed" {
					w.activeDungeon.Definition.ID++
					w.activeDungeon.MarkSceneCompleted()
				}
			}
			old := *w.venus
			plan, events, err := w.venusRewardEnd(venusEnvelopeWithContent(0, 0, 0, 0))
			if err == nil || len(plan) != 0 || len(events) != 0 || !reflect.DeepEqual(old, *w.venus) {
				t.Fatal("unearned clear advanced", plan, events, err)
			}
		})
	}
}
