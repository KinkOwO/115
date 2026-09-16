package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeStoryPauseAndResume(t *testing.T) {
	b, e := os.ReadFile("testdata/native_story_pause.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		State, Kind byte
		Payload     string `json:"payload_hex"`
		Values      []byte `json:"pause_values"`
	}
	if e = json.Unmarshal(b, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		state := StoryPauseState{r.State, r.Kind}
		p, e := StoryPauseNotice(3, state)
		validEffect := len(r.Values) == 1 && r.Values[0] == 1-r.State
		if r.State == 0 && r.Kind == 3 {
			validEffect = len(r.Values) == 0
		}
		if e != nil || hex.EncodeToString(p) != r.Payload || !validEffect {
			t.Fatal("story native mismatch", e)
		}
		request := make([]byte, 16)
		request[0] = r.State
		request[1] = r.Kind
		if got, e := DecodeStoryPause(request); e != nil || got != state {
			t.Fatal("story decode", e)
		}
	}
}
