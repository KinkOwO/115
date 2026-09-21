package main

import "testing"

func TestHandoffWornVisualsAfterEntry(t *testing.T) {
	p := entryPayloads{WornUpdate: []byte{3, 1, 0}}.packets()
	// The worn visual refresh is the last data frame. The character option
	// block (NOTI2827) is appended after it when one is configured, because
	// this client crashes on town entry when 2827 arrives early.
	end := len(p)
	if len(p) > 0 && p[len(p)-1].ID == 2827 {
		end--
	}
	if end < 1 || p[end-1].Kind != 0 || p[end-1].ID != 14 || len(p[end-1].Payload) != 3 {
		t.Fatal("visual refresh must follow world initialization")
	}
	complete := -1
	for i, q := range p[:end] {
		if q.ID == 124 {
			complete = i
		}
	}
	if complete < 0 || complete >= end-1 {
		t.Fatal("entry completion must precede visual refresh")
	}
	locked := entryPayloads{WornUpdate: []byte{3, 1, 0}, SkillLocks: []byte{1, 2, 3}}.packets()
	if last := locked[len(locked)-1]; last.Kind != 0 || last.ID != 2827 {
		t.Fatal("the character option block must be the final entry frame")
	}
}
