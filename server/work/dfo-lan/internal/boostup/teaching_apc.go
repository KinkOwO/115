package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const SpecialAPCPath = "etc/mycharactersapc/mycharacters_special_apc.etc"

type TeachingAPC struct {
	Index, Template, Event uint32
}

// Native142E62D70 loads these rows;142E5C590 maps their special [index] in
// NOTI1754 to a local virtual character index. Template is the AIC resource,
// not the selection packet's NPC index. Empty/reserved and non-event
// entries do not define the event's teaching companion.
func ParseTeachingAPCs(cells []pvf.Token) ([]TeachingAPC, error) {
	sections, err := sections(cells, "[character]", "[/character]")
	if err != nil {
		return nil, err
	}
	var out []TeachingAPC
	seen := map[uint32]bool{}
	for _, row := range sections {
		typ := values(row, "[type]")
		if len(typ) == 0 || typ[0].Text != "event" {
			continue
		}
		if len(typ) != 2 || typ[1].Type != 0 || typ[1].Value <= 0 {
			return nil, fmt.Errorf("invalid special APC event binding")
		}
		contents, err := one(row, "[usable contents]", "[/usable contents]")
		if err != nil {
			return nil, err
		}
		usable := false
		for _, t := range contents {
			usable = usable || t.Text == "boost up event"
		}
		if !usable {
			continue
		}
		index, err := number(row, "[index]", ^uint32(0))
		if err != nil || index == 0 || seen[index] {
			return nil, fmt.Errorf("invalid or duplicate special APC index")
		}
		template, err := number(row, "[apc index]", ^uint32(0))
		if err != nil || template == 0 {
			return nil, fmt.Errorf("invalid special APC template")
		}
		seen[index] = true
		out = append(out, TeachingAPC{Index: index, Template: template, Event: uint32(typ[1].Value)})
	}
	return out, nil
}
