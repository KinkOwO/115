package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestRaidLinkedChannelAndRewardOperandsFollowSource(t *testing.T) {
	for _, sub := range []uint32{92, 203} {
		script := fmt.Sprintf("[client channel info]\n[channelType] 93\n[type]\n[isRaid] 1\n[/type]\n[logic]\n[seriaRoomTown] 186\n[/logic]\n[ui]\n[subChannelType] %d\n[mainSeparateReward] 101 202\n[subSeparateReward] 303\n[imageIndex] 27\n[/ui]\n[/client channel info]", sub)
		got, err := parseClientChannelInfo(script)
		if err != nil || got[93].SubChannelType != sub || !reflect.DeepEqual(got[93].MainSeparateReward, []uint32{101, 202}) || !reflect.DeepEqual(got[93].SubSeparateReward, []uint32{303}) || got[93].ImageIndex != 27 {
			t.Fatal("lost native UI entrance link", got, err)
		}
	}
}

func TestFogMainAndPreludeEntranceCurrentPVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current PVF required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	main, ok := dir.Attributes(93)
	if !ok || !main.IsRaid || main.SubChannelType != 92 || main.ImageIndex != 27 || !reflect.DeepEqual(main.MainSeparateReward, []uint32{100003667, 101034443}) || !reflect.DeepEqual(main.SubSeparateReward, []uint32{100003689}) {
		t.Fatal("fog main native link", main)
	}
	pre, ok := dir.Attributes(main.SubChannelType)
	if !ok || !pre.IsPreRaid || !pre.IsSemiRaid || pre.Town != 185 || main.Town != 186 {
		t.Fatal("fog/prelude native town split", main, pre)
	}
	entries, err := ImportRaidEntrances(a, &dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[main.Type]
	if entry.Path != "contents/2024/asrahanraid/etc/asrahan.etc" || entry.Waiting.TownID != 186 || entry.Waiting.AreaID != 2 || entry.Bakal != nil {
		t.Fatal("fog bound to foreign executor", entry)
	}
	title, found, err := a.LocalizedText(pvf.LocalizedRef{Table: 20, Key: "raid_channel_29"})
	if err != nil || !found || title != "雾隐之地" {
		t.Fatal("screenshot localized title", title, err)
	}
}
