package skillaudit

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func tag(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
func str(text string) pvf.Token { return pvf.Token{Type: 6, Text: text} }
func num(value int32) pvf.Token { return pvf.Token{Type: 0, Value: value} }

func wantText(t *testing.T, fields map[string][]pvf.Token, key, text string) {
	t.Helper()
	got := fields[key]
	if len(got) != 1 || got[0].Text != text {
		t.Fatalf("%s = %v, want one %q", key, got, text)
	}
}

// skill/swordman/momentaryslash.skl shape: the variant labels come first and
// the skill's own [type] sits next to [skill class].
func TestSkillTypeIgnoresVariantExplainLabels(t *testing.T) {
	cells := []pvf.Token{
		tag("[vp explain]"), tag("[explain group]"), tag("[type]"), str("vp1"),
		tag("[name]"), str("x"), tag("[/explain group]"), tag("[/vp explain]"),
		tag("[preset info]"), tag("[type]"), str("default"), tag("[/preset info]"),
		tag("[purchase cost]"), num(40),
		tag("[required level]"), num(35), tag("[required level range]"), num(2),
		tag("[type]"), str("[active]"), tag("[skill class]"), num(1),
		tag("[maximum level]"), num(70),
	}
	fields := learnableSkillFields(cells)
	wantText(t, fields, "[type]", "[active]")
	if got := fields["[required level]"]; len(got) != 1 || got[0].Value != 35 {
		t.Fatalf("[required level] = %v", got)
	}
	if got := fields["[purchase cost]"]; len(got) != 1 || got[0].Value != 40 {
		t.Fatalf("[purchase cost] = %v", got)
	}
	if got := fields["[skill class]"]; len(got) != 1 || got[0].Value != 1 {
		t.Fatalf("[skill class] = %v", got)
	}
}

// skill/priest/pandemoniumex.skl opens [vp explain] twice and closes it once.
// Those rows must still project [type]=[active] and their learning fields.
func TestSkillFieldsSurviveUnpairedExplainContainer(t *testing.T) {
	cells := []pvf.Token{
		tag("[vp explain]"), tag("[vp explain]"),
		tag("[explain group]"), tag("[type]"), str("vp1"), tag("[/explain group]"),
		tag("[explain group]"), tag("[type]"), str("vp2"), tag("[/explain group]"),
		tag("[/vp explain]"),
		tag("[purchase cost]"), num(60),
		tag("[pre required skill]"), num(123), num(1),
		tag("[type]"), str("[active]"), tag("[skill class]"), num(3),
		tag("[maximum level]"), num(50),
	}
	fields := learnableSkillFields(cells)
	wantText(t, fields, "[type]", "[active]")
	if got := fields["[purchase cost]"]; len(got) != 1 || got[0].Value != 60 {
		t.Fatalf("[purchase cost] = %v", got)
	}
	if got := fields["[pre required skill]"]; len(got) != 2 {
		t.Fatalf("[pre required skill] = %v", got)
	}
	if got := fields["[skill class]"]; len(got) != 1 || got[0].Value != 3 {
		t.Fatalf("[skill class] = %v", got)
	}
}

// skill/mage/thunderrage.skl puts a [type] default inside a damage group before
// the skill's own [type]; that label must not become the skill type.
func TestSkillTypeIgnoresDamageGroupLabel(t *testing.T) {
	cells := []pvf.Token{
		tag("[type]"), str("default"), tag("[base attack]"), num(1),
		tag("[type]"), str("[active]"), tag("[skill class]"), num(1),
	}
	fields := learnableSkillFields(cells)
	wantText(t, fields, "[type]", "[active]")
}

// Skills whose source declares no own [type] must stay without one, so the
// service keeps refusing them instead of inventing a type.
func TestSkillWithoutOwnTypeKeepsNoTypeField(t *testing.T) {
	cells := []pvf.Token{
		tag("[type]"), str("vp1"), tag("[base attack]"), num(1),
		tag("[required level]"), num(75),
	}
	fields := learnableSkillFields(cells)
	if _, ok := fields["[type]"]; ok {
		t.Fatalf("[type] = %v, want absent", fields["[type]"])
	}
	if got := fields["[required level]"]; len(got) != 1 || got[0].Value != 75 {
		t.Fatalf("[required level] = %v", got)
	}
}

// Passive rows omit [skill class], so the [active]/[passive] value has to
// identify them on its own.
func TestPassiveSkillTypeComesFromItsValue(t *testing.T) {
	cells := []pvf.Token{
		tag("[type]"), str("default"), tag("[tooltip data]"),
		tag("[type]"), str("[passive]"),
		tag("[maximum level]"), num(10),
	}
	fields := learnableSkillFields(cells)
	wantText(t, fields, "[type]", "[passive]")
}
