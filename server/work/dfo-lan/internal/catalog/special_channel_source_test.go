package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"reflect"
	"testing"
)

func TestLegionChannelEventsFollowSource(t *testing.T) {
	for _, event := range []int64{627, 1007, 1234} {
		attrs := map[uint32]ChannelAttributes{99: {IsLegion: true, Town: 204}, 119: {IsLegion: true, Town: 239}, 200: {Town: 204}}
		source := LegionContents{Contents: map[string]LegionContent{
			"first": {Recruiting: LegionAreaPoint{Town: 204}, UIInts: map[string]int64{"ENABLE_CHANNEL_TAB_EVENT_ID": event}},
			"same":  {Recruiting: LegionAreaPoint{Town: 204}, UIInts: map[string]int64{"ENABLE_CHANNEL_TAB_EVENT_ID": event}},
			"hard":  {Recruiting: LegionAreaPoint{Town: 204}, UIInts: map[string]int64{"ENABLE_CHANNEL_TAB_EVENT_ID": event + 1}},
		}}
		if err := bindLegionChannelEvents(attrs, source); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(attrs[99].OpenEvents, []uint16{uint16(event), uint16(event + 1)}) || len(attrs[119].OpenEvents) != 0 || len(attrs[200].OpenEvents) != 0 {
			t.Fatalf("incorrect source join: %+v", attrs)
		}
	}
	for _, event := range []int64{0, -2, 65536} {
		source := LegionContents{Contents: map[string]LegionContent{"bad": {Recruiting: LegionAreaPoint{Town: 204}, UIInts: map[string]int64{"ENABLE_CHANNEL_TAB_EVENT_ID": event}}}}
		if err := bindLegionChannelEvents(map[uint32]ChannelAttributes{}, source); err == nil {
			t.Fatalf("accepted event %d", event)
		}
	}
}

func TestRaidCTPEntryHierarchyAndSourceChanges(t *testing.T) {
	row := func(name string, parent int, values ...float64) pvf.CTPRecord {
		r := pvf.CTPRecord{Name: name, Parent: parent}
		for _, value := range values {
			r.Cells = append(r.Cells, pvf.CTPCell{Kind: "float", Float: value})
		}
		return r
	}
	for _, member := range []float64{12, 16} {
		table := &pvf.CTPTable{Records: []pvf.CTPRecord{
			row("[RAID MEMBER MAX]", -1, member), row("[WAITING ROOM]", -1, 228, 2),
			row("[RAID LIST WINDOW DIFFICULTY INFO]", -1), row("[INFO]", 2), row("[CHANNEL TYPE]", 3, 112),
			row("[RAID MEMBER MAX]", 3, 99), row("[CHANNEL TYPE]", -1, 200),
		}}
		values, channels, err := raidCTPEntryValues(table)
		if err != nil || !reflect.DeepEqual(values["[RAID MEMBER MAX]"], []uint32{uint32(member)}) || !reflect.DeepEqual(channels, []uint32{112}) {
			t.Fatalf("source/hierarchy mismatch: %v %v %v", values, channels, err)
		}
		for _, bad := range []float64{-1, 1.5, math.NaN(), math.Inf(1), 4294967296} {
			table.Records[0] = row("[RAID MEMBER MAX]", -1, bad)
			if _, _, err := raidCTPEntryValues(table); err == nil {
				t.Fatalf("accepted invalid integer %v", bad)
			}
		}
		table.Records[0] = row("[RAID MEMBER MAX]", -1, member)
		table.Records = append(table.Records, row("[RAID MEMBER MAX]", -1, member))
		if _, _, err := raidCTPEntryValues(table); err == nil {
			t.Fatal("accepted duplicate common header")
		}
	}
}
