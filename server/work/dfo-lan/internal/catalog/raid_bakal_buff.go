package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

func parseBakalBuffDefinitions(ts []pvf.Token) (map[string]BakalBuffDefinition, error) {
	out := map[string]BakalBuffDefinition{}
	var b *BakalBuffDefinition
	var next uint32
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		switch t.Text {
		case "[PARTY BUFF]", "[RAID BUFF]":
			if b != nil {
				return nil, fmt.Errorf("nested native buff")
			}
			b = &BakalBuffDefinition{Index: next, Raid: t.Text == "[RAID BUFF]"}
			next++
		case "[TYPE]":
			if b == nil || i+1 >= len(ts) || ts[i+1].Type != 6 {
				return nil, fmt.Errorf("missing buff type")
			}
			b.Kind = ts[i+1].Text
			i++
		case "[COOLTIME]", "[DURATION]", "[SERVER VALUE]":
			if b == nil || i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value < 0 {
				return nil, fmt.Errorf("invalid native buff scalar")
			}
			v := uint32(ts[i+1].Value)
			i++
			if t.Text == "[COOLTIME]" {
				b.Cooltime = v
			} else if t.Text == "[DURATION]" {
				b.Duration = v
			} else {
				b.ServerValue = v
			}
		case "[APPENDAGE VALUE]":
			if b == nil {
				return nil, fmt.Errorf("appendage outside buff")
			}
			for i++; i < len(ts) && ts[i].Type == 0; i++ {
				b.Appendage = append(b.Appendage, ts[i].Value)
			}
			i--
		case "[/PARTY BUFF]", "[/RAID BUFF]":
			if b == nil || b.Kind == "" {
				return nil, fmt.Errorf("incomplete native buff")
			}
			if _, exists := out[b.Kind]; exists {
				return nil, fmt.Errorf("duplicate native buff")
			}
			out[b.Kind] = *b
			b = nil
		}
	}
	if b != nil || next != 25 {
		return nil, fmt.Errorf("native buff table does not match client 25-type namespace")
	}
	return out, nil
}
