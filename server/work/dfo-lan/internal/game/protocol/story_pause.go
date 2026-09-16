package protocol

import "fmt"

type StoryPauseState struct{ State, Kind byte }

func DecodeStoryPause(p []byte) (StoryPauseState, error) {
	var r StoryPauseState
	if len(p) != 2 && len(p) != 16 {
		return r, fmt.Errorf("unsupported story pause payload")
	}
	r = StoryPauseState{p[0], p[1]}
	if r.State > 1 || r.Kind > 3 {
		return r, fmt.Errorf("unsupported story pause state")
	}
	if e := padding(p[2:], 16); e != nil {
		return r, e
	}
	return r, nil
}

// Current NOTI1701452ac690: u16 actor,u8 resume flag,u8 story kind.
// The native191 digest predicate is zero: this request has no MD5 suffix.
func StoryPauseNotice(actor uint16, r StoryPauseState) ([]byte, error) {
	if actor == 0 || actor == 65535 || r.State > 1 || r.Kind > 3 {
		return nil, fmt.Errorf("invalid story owner/state")
	}
	return append(add16(nil, actor), r.State, r.Kind), nil
}
