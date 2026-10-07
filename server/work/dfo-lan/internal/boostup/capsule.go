package boostup

import (
	"dfolan/internal/catalog"
	"fmt"
)

type Capsule struct {
	Item, Variant uint32
	Event         uint16
	Place         string
}
type ItemSource interface {
	TemplateScript(string, uint32) (catalog.ScriptRecord, error)
}

func (c *Catalog) BindCapsules(src ItemSource) error {
	out := map[uint32]Capsule{}
	for _, g := range c.Gifts {
		if g.Event != 10017 && g.Event != 10018 {
			continue
		}
		for _, id := range g.Items {
			s, e := src.TemplateScript("stackable", id)
			if e != nil {
				return e
			}
			v := values(s.Cells, "[action type]")
			if len(v) != 3 || v[0].Text != "[boost up mode capsule]" || v[1].Type != 0 || v[1].Value < 0 || v[1].Value > 1 || v[2].Type != 0 || (v[2].Value != 10017 && v[2].Value != 10018) {
				return fmt.Errorf("capsule %d action/gift binding mismatch: values=%+v giftEvent=%d", id, v, g.Event)
			}
			place := label(s.Cells, "[action usable place]")
			if place != "[seria room]" {
				return fmt.Errorf("capsule %d unsupported source place", id)
			}
			// Buffer gift118 is opened by10018, but its source capsule's
			// action names10017 too. Gift ID/event and action event differ.
			out[id] = Capsule{Item: id, Variant: uint32(v[1].Value), Event: uint16(v[2].Value), Place: place}
		}
	}
	if len(out) == 0 {
		return fmt.Errorf("no source capsules")
	}
	c.Capsules = out
	return nil
}
func (c *Catalog) BufferEligible(job, grow byte) bool {
	for _, v := range append(append([][2]byte(nil), c.BufferJobs...), c.DualJobs...) {
		if v == [2]byte{job, grow} {
			return true
		}
	}
	return false
}
