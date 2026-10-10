package database

import (
	"encoding/json"
	"testing"
)

func TestAdventureDocumentPreservesUnknownAndClearsKnownFields(t *testing.T) {
	var d AdventureData
	if err := json.Unmarshal([]byte(`{"future_feature":{"keep":[1,2]},"name_chosen":true,"attendance_activities":{"campaign":{"version":1,"visits":2}},"points":[1,2,3,4,5]}`), &d); err != nil {
		t.Fatal(err)
	}
	d.NameChosen = false
	d.Points = [5]uint32{}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["future_feature"]) != `{"keep":[1,2]}` || string(fields["attendance_activities"]) != `{"campaign":{"version":1,"visits":2}}` {
		t.Fatal(string(raw))
	}
	if _, exists := fields["name_chosen"]; exists {
		t.Fatal("cleared known field resurrected")
	}
}
