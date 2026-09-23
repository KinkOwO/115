package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestStoryDigestRestoreAlwaysFourBytes(t *testing.T) {
	// 缺键 / 空对象 / 全零都必须产出恰好 4 字节：preparePackets 会丢弃
	// 零长载荷帧，空 body 会让 NOTI1370 静默消失（影片照播且日志无痕）。
	cases := []struct {
		name string
		raw  json.RawMessage
		want []byte
	}{
		{"missing key", json.RawMessage(`{"cinematic_skipped":[1,2,3]}`), []byte{0, 0, 0, 0}},
		{"empty object", json.RawMessage(`{}`), []byte{0, 0, 0, 0}},
		{"level zero", json.RawMessage(`{"story_digest_level":0}`), []byte{0, 0, 0, 0}},
		{"level one", json.RawMessage(`{"story_digest_level":1}`), []byte{1, 0, 0, 0}},
		{"level 115", json.RawMessage(`{"story_digest_level":115}`), []byte{0x73, 0, 0, 0}},
		{"level 256", json.RawMessage(`{"story_digest_level":256}`), []byte{0, 1, 0, 0}},
		{"level 16777216", json.RawMessage(`{"story_digest_level":16777216}`), []byte{0, 0, 0, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := storyDigestRestore(c.raw)
			if err != nil {
				t.Fatalf("storyDigestRestore(%s) error: %v", c.raw, err)
			}
			if len(got) != 4 {
				t.Fatalf("len(got)=%d, want 4 (payload must never be empty)", len(got))
			}
			if !bytes.Equal(got, c.want) {
				t.Fatalf("got %x, want %x", got, c.want)
			}
		})
	}
}

func TestStoryDigestRestoreInvalidState(t *testing.T) {
	if _, err := storyDigestRestore(json.RawMessage(`not json`)); err == nil {
		t.Fatal("expected error for unparseable state")
	}
}

func TestAdvanceStoryDigestMonotonic(t *testing.T) {
	t.Run("missing key advances from zero", func(t *testing.T) {
		state, advanced, err := advanceStoryDigest(json.RawMessage(`{"cinematic_skipped":[1]}`), 5)
		if err != nil {
			t.Fatal(err)
		}
		if !advanced {
			t.Fatal("want advanced=true when key is absent")
		}
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(state, &fields); e != nil {
			t.Fatal(e)
		}
		var lv uint32
		if e := json.Unmarshal(fields["story_digest_level"], &lv); e != nil {
			t.Fatal(e)
		}
		if lv != 5 {
			t.Fatalf("level=%d, want 5", lv)
		}
		// 其它字段必须原样保留。
		if !bytes.Contains(state, []byte(`"cinematic_skipped"`)) {
			t.Fatalf("state lost unrelated fields: %s", state)
		}
	})

	t.Run("lower level does not regress", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100,"cinematic_skipped":[9]}`)
		if _, advanced, err := advanceStoryDigest(raw, 90); err != nil || advanced {
			t.Fatalf("advanced=%v err=%v, want (false,nil)", advanced, err)
		}
	})

	t.Run("equal level does not advance", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100}`)
		if _, advanced, err := advanceStoryDigest(raw, 100); err != nil || advanced {
			t.Fatalf("advanced=%v err=%v, want (false,nil)", advanced, err)
		}
	})

	t.Run("higher level advances", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100}`)
		state, advanced, err := advanceStoryDigest(raw, 115)
		if err != nil {
			t.Fatal(err)
		}
		if !advanced {
			t.Fatal("want advanced=true")
		}
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(state, &fields); e != nil {
			t.Fatal(e)
		}
		var lv uint32
		if e := json.Unmarshal(fields["story_digest_level"], &lv); e != nil {
			t.Fatal(e)
		}
		if lv != 115 {
			t.Fatalf("level=%d, want 115", lv)
		}
	})

	t.Run("invalid state errors", func(t *testing.T) {
		if _, _, err := advanceStoryDigest(json.RawMessage(`nope`), 1); err == nil {
			t.Fatal("expected error for unparseable state")
		}
	})
}
