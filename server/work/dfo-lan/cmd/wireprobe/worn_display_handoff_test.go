package main

import "testing"

func TestHandoffWornVisualsAfterEntry(t *testing.T) {
	p := entryPayloads{WornUpdate: []byte{3, 1, 0}}.packets()
	if last := p[len(p)-1]; last.Kind != 0 || last.ID != 14 || len(last.Payload) != 3 {
		t.Fatal("visual refresh must follow world initialization")
	}
	complete := -1
	for i, q := range p {
		if q.ID == 124 {
			complete = i
		}
	}
	if complete < 0 || complete >= len(p)-1 {
		t.Fatal("entry completion must precede visual refresh")
	}
}
