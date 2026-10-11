package main

import (
	"context"
	"dfolan/internal/reward"
	"dfolan/internal/servermod"
	"dfolan/mods"
	"reflect"
	"testing"
)

// Adding the MOD framework must preserve the established new-character
// rewards and must not auto-install the sample equipment giveaway.
func TestProductionMODRegistrationKeepsExistingCharacterRewards(t *testing.T) {
	beforeScripts, beforeIDs := servermod.RewardScripts(), servermod.Registered()
	mods.RegisterMods()
	if !reflect.DeepEqual(beforeScripts, servermod.RewardScripts()) || !reflect.DeepEqual(beforeIDs, servermod.Registered()) {
		t.Fatal("sample MOD activated in production")
	}
	grants := map[uint32]uint64{}
	var cera uint64
	mail := 0
	service, err := reward.New(reward.Options{
		Scripts: rewardScriptFS(),
		Grant: func(_ context.Context, _ reward.Recipient, _ string, items []reward.ItemGrant) error {
			for _, item := range items {
				grants[item.ID] += uint64(item.Count)
			}
			return nil
		},
		Cera: func(_ context.Context, _ reward.Recipient, _ string, n uint64) error { cera += n; return nil },
		Mail: func(_ context.Context, _ reward.Recipient, _ string, _ reward.MailReward) error { mail++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	service.CharacterCreate(context.Background(), reward.Recipient{AccountID: 1, CharacterID: 1, Name: "MOD-neutral-fixture", Level: 1, ConfigVersion: "fixture"})
	expected := map[uint32]uint64{0: 10000000, 10418035: 1000, 10418036: 1000, 3033: 10000, 3034: 10000, 3035: 10000, 3036: 10000, 3037: 10000, 3262: 10000}
	if !reflect.DeepEqual(grants, expected) || cera != 100000 || mail != 0 {
		t.Fatalf("existing rewards altered: grants=%v cera=%d mail=%d", grants, cera, mail)
	}
}

func TestTownActivityChallengeSnapshotUsesActualRecordCount(t *testing.T) {
	body, ok := buildTownEventInfoTable(true)
	if !ok {
		t.Fatal("challenge snapshot unavailable")
	}
	keys := decodeOpenEventKeys(t, body)
	if len(keys) != 24 || keys[23] != 665 {
		t.Fatalf("challenge record count or identity lost: %v", keys)
	}
}
