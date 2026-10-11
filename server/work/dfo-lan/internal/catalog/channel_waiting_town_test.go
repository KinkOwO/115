package catalog

import (
	"fmt"
	"testing"
)

func TestChannelWaitingTownFollowsUIWithLogicPrecedence(t *testing.T) {
	for _, town := range []int{83, 98, 321} {
		text := fmt.Sprintf("[client channel info]\n[channelType]\n67\n[/channelType]\n[ui]\n[seriaRoomTown]\n%d\n[/seriaRoomTown]\n[seriaRoomArea]\n2\n[/seriaRoomArea]\n[/ui]\n[/client channel info]", town)
		attrs, err := parseClientChannelInfo(text)
		if err != nil || attrs[67].Town != town || attrs[67].TownArea != 2 {
			t.Fatalf("ui source town=%d got=%+v err=%v", town, attrs[67], err)
		}
	}
	text := "[client channel info]\n[channelType]\n67\n[/channelType]\n[logic]\n[seriaRoomTown]\n123\n[/seriaRoomTown]\n[seriaRoomArea]\n1\n[/seriaRoomArea]\n[/logic]\n[ui]\n[seriaRoomTown]\n456\n[/seriaRoomTown]\n[/ui]\n[/client channel info]"
	attrs, err := parseClientChannelInfo(text)
	if err != nil || attrs[67].Town != 123 || attrs[67].TownArea != 1 {
		t.Fatalf("logic precedence %+v %v", attrs, err)
	}
}
