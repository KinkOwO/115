package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type skillKey struct {
	Job byte
	ID  uint16
}

func learningBytes(d LearningDefinition) int {
	n := len(d.Path) + len(d.SHA256)
	for tag, cells := range d.Fields {
		n += len(tag) + 48*len(cells) + 64
		for _, t := range cells {
			n += len(t.Text)
		}
	}
	return n
}
func (c *LearningCatalog) EnableRuntimeDetails(a *pvf.Archive) error {
	if a == nil || c.Source.Checksum != a.Snapshot().Checksum {
		return fmt.Errorf("learning detail source mismatch")
	}
	if c.details != nil {
		return nil
	}
	refs := make(map[skillKey]catalog.ScriptRecord, len(c.Rows))
	for _, r := range c.Rows {
		refs[skillKey{r.Job, r.ID}] = catalog.ScriptRecord{Path: r.Path, SHA256: r.SHA256}
	}
	d, err := catalog.NewScriptDetails(a, refs, func(k skillKey, s catalog.ScriptRecord) LearningDefinition {
		return LearningDefinition{Job: k.Job, ID: k.ID, Path: s.Path, SHA256: s.SHA256, Fields: LearnableSkillFields(s.Cells)}
	}, learningBytes)
	if err != nil {
		return err
	}
	// Shortcut membership/type are global projections. Cost, advancement,
	// awakening and variation fields are fetched through Definition on demand.
	for i, r := range c.Rows {
		r.Fields = map[string][]pvf.Token{"[type]": append([]pvf.Token(nil), r.Fields["[type]"]...)}
		c.Rows[i] = r
		c.index[r.Job][r.ID] = r
	}
	c.details = d
	return nil
}
func (c *LearningCatalog) Definition(job byte, id uint16) (LearningDefinition, bool, error) {
	if c == nil {
		return LearningDefinition{}, false, fmt.Errorf("learning source unavailable")
	}
	d, ok := c.index[job][id]
	if !ok {
		return d, false, nil
	}
	if c.details == nil {
		return d, true, nil
	}
	d, err := c.details.Get(skillKey{job, id})
	return d, err == nil, err
}
func (c *LearningCatalog) Close() error {
	if c == nil || c.details == nil {
		return nil
	}
	return c.details.Close()
}
