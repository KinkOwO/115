package boostup

import (
	"encoding/json"
	"fmt"
)

// Per-character, explicitly chosen by the user. Do not put these claims in
// account state or clear them when the actor reconnects or PVF text changes.
type State struct {
	Challenge         *ChallengeState `json:"challenge,omitempty"`
	PendingLevelBonus *GraduationMail `json:"pending_level_bonus_mail,omitempty"`
	LevelBonusSent    bool            `json:"level_bonus_mail_sent,omitempty"`
	PendingMail       *GraduationMail `json:"pending_graduation_mail,omitempty"`
	MailSent          bool            `json:"graduation_mail_sent,omitempty"`
	Origin            json.RawMessage `json:"origin,omitempty"` // server-owned ordinary WorldPosition before activation
	Version           int             `json:"version"`
	Gifts             map[uint16]bool `json:"gifts"`
	Activated         bool            `json:"activated"`
	Variant           uint32          `json:"variant"`
	Training          Training        `json:"training"`
}

func ReadState(raw json.RawMessage) (State, error) {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return State{}, e
	}
	if fields == nil {
		return State{}, fmt.Errorf("character state is not an object")
	}
	s := State{Version: 1, Gifts: map[uint16]bool{}}
	if b, ok := fields["boost_up115"]; ok {
		if e := json.Unmarshal(b, &s); e != nil {
			return State{}, e
		}
		if s.Version != 1 || s.Variant > 1 {
			return State{}, fmt.Errorf("invalid boost state")
		}
		if s.Gifts == nil {
			s.Gifts = map[uint16]bool{}
		}
	}
	return s, nil
}
func WriteState(raw json.RawMessage, s State) (json.RawMessage, error) {
	if s.Version != 1 || s.Variant > 1 {
		return nil, fmt.Errorf("invalid boost state")
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	b, e := json.Marshal(s)
	if e != nil {
		return nil, e
	}
	fields["boost_up115"] = b
	return json.Marshal(fields)
}
