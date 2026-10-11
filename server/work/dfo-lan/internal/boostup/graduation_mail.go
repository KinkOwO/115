package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
	"unicode/utf8"
)

type GraduationMail struct {
	Item, Count  uint32
	Sender, Body string
}

func (m GraduationMail) Validate() error {
	if m.Item < 2 || m.Count == 0 || m.Sender == "" || len(m.Sender) > 29 || len(m.Body) > 512 || !utf8.ValidString(m.Sender) || !utf8.ValidString(m.Body) || strings.ContainsRune(m.Sender+m.Body, 0) {
		return fmt.Errorf("invalid graduation mail intent")
	}
	return nil
}
func parseGraduationMail(cells []pvf.Token) (*GraduationMail, error) {
	blocks, e := sections(cells, "[reserve max level reward]", "[/reserve max level reward]")
	if e != nil {
		return nil, e
	}
	if len(blocks) == 0 {
		return nil, nil
	}
	if len(blocks) != 1 {
		return nil, fmt.Errorf("duplicate graduation reward")
	}
	b := blocks[0]
	item, e := number(b, "[reward]", ^uint32(0))
	if e != nil {
		return nil, e
	}
	count, e := number(b, "[count]", ^uint32(0))
	if e != nil {
		return nil, e
	}
	sender, body := label(b, "[mail title]"), label(b, "[mail msg]")
	// Current source's localized text is empty in the exported view. These
	// are local presentation fallbacks, not invented reward identities.
	if sender == "" {
		sender = "Starter Boost"
	}
	if len(sender) > 29 {
		body = sender + "\n" + body
		sender = "Starter Boost"
	}
	if body == "" {
		body = "Starter Boost level reward."
	}
	if len(body) > 512 {
		body = body[:512]
		for !utf8.ValidString(body) {
			body = body[:len(body)-1]
		}
	}
	m := &GraduationMail{Item: item, Count: count, Sender: sender, Body: body}
	return m, m.Validate()
}
