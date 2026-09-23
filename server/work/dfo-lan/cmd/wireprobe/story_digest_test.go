package main

import (
	"bytes"
	"testing"
)

func TestStoryDigestRestoreAndEntryOrder(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  []byte
	}{
		{`{"cinematic_skipped":[]}`, []byte{0, 0, 0, 0}},
		{`{"story_digest_level":115}`, []byte{115, 0, 0, 0}},
		{`{"story_digest_level":16777216}`, []byte{0, 0, 0, 1}},
	} {
		body, err := storyDigestRestore([]byte(tc.state))
		if err != nil || !bytes.Equal(body, tc.want) {
			t.Fatalf("restore %s: body=%x err=%v", tc.state, body, err)
		}
	}
	packets := (entryPayloads{CinematicSkips: []byte{0}, StoryDigest: []byte{115, 0, 0, 0}, Basic: []byte{1}}).packets()
	if packets[2].ID != 1352 || packets[2].Kind != 0 || packets[3].ID != 1370 || packets[3].Kind != 0 || packets[4].ID != 2 {
		t.Fatalf("wrong entry sequence: %+v", packets[:5])
	}
	if !observedGameRequest(1438) {
		t.Fatal("story digest update is not retained")
	}
}
