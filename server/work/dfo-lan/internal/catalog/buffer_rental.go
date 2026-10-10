package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

const (
	BufferRentalCSSource     = "contents/2026/mercenarygroup/etc/csbufferrentalsystem.ctp"
	BufferRentalClientSource = "contents/2026/mercenarygroup/etc/bufferrentalsystem.ctp"
)

// BufferRentalRules is the source projection of the two tables consumed by
// 1474B3EA0 and 1402AC230. It does not invent rental records or buff statistics.
// Account level, registration protocol and reward settlement require their
// respective native contracts before these values can execute transactions.
type BufferRentalRules struct {
	SourceChecksum                                     string
	CS, Client                                         *pvf.CTPTable
	Channels                                           map[uint32]BufferRentalChannel
	MaxDispatchCount                                   uint32
	Commission, RegistrationSlots                      []BufferRentalTier
	StatCaps                                           map[string][2]uint32
	AwakeningCoolTimeMS, BasicStartTimeMS              uint32
	TargetStat, TargetDamage, BasicDefense, BasicSpeed uint32
	Skills                                             map[uint32]map[string]BufferRentalSkill
	SourceAPIURL                                       string
}

type BufferRentalChannel struct {
	Index, MercenaryLevel, Fame uint32
	FeeMin, FeeMax              uint32
	NPCs                        []uint32
}

// BufferRentalTier retains the ordered pairs in [commission rate] and
// [registable count]. It does not infer account level from character level.
type BufferRentalTier struct{ Level, Value uint32 }

// Parameters retains cells 1..4 of [buff skill info], read at 1402ACD30..D8D.
// Only the tuple layout is closed; downstream formulas are a separate contract.
type BufferRentalSkill struct{ Parameters [4]uint32 }

func ImportBufferRental(a *pvf.Archive) (*BufferRentalRules, error) {
	if a == nil {
		return nil, fmt.Errorf("nil buffer rental archive")
	}
	cs, err := a.CTP(BufferRentalCSSource)
	if err != nil {
		return nil, err
	}
	client, err := a.CTP(BufferRentalClientSource)
	if err != nil {
		return nil, err
	}
	r, err := readBufferRental(cs, client)
	if err != nil {
		return nil, err
	}
	r.SourceChecksum = a.Snapshot().Checksum
	return r, nil
}

func readBufferRental(cs, client *pvf.CTPTable) (*BufferRentalRules, error) {
	if cs == nil || client == nil {
		return nil, fmt.Errorf("buffer rental requires both native tables")
	}
	r := &BufferRentalRules{CS: cs, Client: client, Channels: map[uint32]BufferRentalChannel{}, StatCaps: map[string][2]uint32{}, Skills: map[uint32]map[string]BufferRentalSkill{}}
	csRows := rentalRows{cs}
	channels, err := csRows.root("[rentable channel]")
	if err != nil {
		return nil, err
	}
	blocks, err := csRows.children(channels, "[channel]")
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%s: empty rentable channel list", cs.Path)
	}
	for _, block := range blocks {
		values := map[string][]uint32{}
		for _, name := range []string{"[index]", "[mercenary level]", "[fame]", "[rental fee]", "[rentalable npc]"} {
			row, e := csRows.child(block, name)
			if e != nil {
				return nil, e
			}
			n := 1
			if name == "[rental fee]" {
				n = 2
			}
			if name == "[rentalable npc]" {
				n = -1
			}
			values[name], e = rentalU32(cs, row, n)
			if e != nil {
				return nil, e
			}
		}
		ch := BufferRentalChannel{Index: values["[index]"][0], MercenaryLevel: values["[mercenary level]"][0], Fame: values["[fame]"][0], FeeMin: values["[rental fee]"][0], FeeMax: values["[rental fee]"][1], NPCs: values["[rentalable npc]"]}
		if ch.Index == 0 || ch.FeeMin > ch.FeeMax {
			return nil, fmt.Errorf("%s: invalid channel/fee range at row %d", cs.Path, block.Index)
		}
		if _, duplicate := r.Channels[ch.Index]; duplicate {
			return nil, fmt.Errorf("%s: duplicate channel %d", cs.Path, ch.Index)
		}
		r.Channels[ch.Index] = ch
	}
	if r.MaxDispatchCount, err = csRows.scalar("[max dispatch count]"); err != nil {
		return nil, err
	}
	if r.Commission, err = csRows.tiers("[commission rate]"); err != nil {
		return nil, err
	}
	if r.RegistrationSlots, err = csRows.tiers("[registable count]"); err != nil {
		return nil, err
	}
	caps, err := csRows.root("[stat cap]")
	if err != nil {
		return nil, err
	}
	capRows, err := csRows.children(caps, "[cap info]")
	if err != nil {
		return nil, err
	}
	for _, row := range capRows {
		kind, values, e := rentalTuple(cs, row, 2)
		if e != nil {
			return nil, e
		}
		if _, exists := r.StatCaps[kind]; exists {
			return nil, fmt.Errorf("%s: duplicate cap kind %s", cs.Path, kind)
		}
		r.StatCaps[kind] = [2]uint32{values[0], values[1]}
	}
	if len(r.StatCaps) != 2 {
		return nil, fmt.Errorf("%s: basic/awakening caps missing", cs.Path)
	}
	clRows := rentalRows{client}
	if r.AwakeningCoolTimeMS, err = clRows.scalar("[awakening buff cool time]"); err != nil {
		return nil, err
	}
	if r.BasicStartTimeMS, err = clRows.scalar("[basic buff start time]"); err != nil {
		return nil, err
	}
	for _, group := range []struct {
		name   string
		fields map[string]*uint32
	}{
		{"[buff target stat]", map[string]*uint32{"[stat]": &r.TargetStat, "[damage]": &r.TargetDamage}},
		{"[basic stat]", map[string]*uint32{"[defense]": &r.BasicDefense, "[speed]": &r.BasicSpeed}},
	} {
		block, e := clRows.root(group.name)
		if e != nil {
			return nil, e
		}
		for name, target := range group.fields {
			row, e := clRows.child(block, name)
			if e != nil {
				return nil, e
			}
			values, e := rentalU32(client, row, 1)
			if e != nil {
				return nil, e
			}
			*target = values[0]
		}
	}
	skills, err := clRows.root("[buff skill list]")
	if err != nil {
		return nil, err
	}
	jobs, err := clRows.children(skills, "[data]")
	if err != nil {
		return nil, err
	}
	for _, block := range jobs {
		row, e := clRows.child(block, "[job]")
		if e != nil {
			return nil, e
		}
		values, e := rentalU32(client, row, 1)
		if e != nil {
			return nil, e
		}
		job := values[0]
		if _, exists := r.Skills[job]; exists {
			return nil, fmt.Errorf("%s: duplicate buff job %d", client.Path, job)
		}
		kinds := map[string]BufferRentalSkill{}
		rows, e := clRows.children(block, "[buff skill info]")
		if e != nil {
			return nil, e
		}
		for _, row := range rows {
			kind, values, e := rentalTuple(client, row, 4)
			if e != nil {
				return nil, e
			}
			if _, exists := kinds[kind]; exists {
				return nil, fmt.Errorf("%s: duplicate skill kind %s for job %d", client.Path, kind, job)
			}
			kinds[kind] = BufferRentalSkill{Parameters: [4]uint32{values[0], values[1], values[2], values[3]}}
		}
		if len(kinds) != 2 {
			return nil, fmt.Errorf("%s: job %d needs basic/awakening skills", client.Path, job)
		}
		r.Skills[job] = kinds
	}
	if len(r.Skills) == 0 {
		return nil, fmt.Errorf("%s: no buff skills", client.Path)
	}
	api, err := clRows.root("[api url]")
	if err != nil {
		return nil, err
	}
	if len(api.Cells) != 1 || api.Cells[0].Kind != "name" || api.Cells[0].Name == "" {
		return nil, fmt.Errorf("%s: invalid source API URL", client.Path)
	}
	r.SourceAPIURL = api.Cells[0].Name
	return r, nil
}

type rentalRows struct{ t *pvf.CTPTable }

func (r rentalRows) root(name string) (pvf.CTPRecord, error) {
	var found []pvf.CTPRecord
	for _, row := range r.t.Records {
		if row.Name == name && row.Parent == -1 {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		return pvf.CTPRecord{}, fmt.Errorf("%s: expected one root %s, got %d", r.t.Path, name, len(found))
	}
	return found[0], nil
}

// Follow references, including their parent/name checks, rather than collecting
// every similarly named row in unrelated CTP subtrees.
func (r rentalRows) children(parent pvf.CTPRecord, name string) ([]pvf.CTPRecord, error) {
	var found []pvf.CTPRecord
	refs := 0
	seen := map[uint64]bool{}
	for _, ref := range parent.Refs {
		if ref.Name != name {
			continue
		}
		refs++
		for _, index := range ref.Values {
			if index >= uint64(len(r.t.Records)) || seen[index] {
				return nil, fmt.Errorf("%s: invalid/duplicate %s reference %d", r.t.Path, name, index)
			}
			seen[index] = true
			row := r.t.Records[index]
			if row.Parent != parent.Index || row.Name != name || row.Index != int(index) {
				return nil, fmt.Errorf("%s: %s reference %d crosses CTP ownership", r.t.Path, name, index)
			}
			found = append(found, row)
		}
	}
	if refs != 1 {
		return nil, fmt.Errorf("%s: row %d needs one %s reference group", r.t.Path, parent.Index, name)
	}
	return found, nil
}

func (r rentalRows) child(parent pvf.CTPRecord, name string) (pvf.CTPRecord, error) {
	rows, err := r.children(parent, name)
	if err != nil {
		return pvf.CTPRecord{}, err
	}
	if len(rows) != 1 {
		return pvf.CTPRecord{}, fmt.Errorf("%s: row %d needs one %s child", r.t.Path, parent.Index, name)
	}
	return rows[0], nil
}

func (r rentalRows) scalar(name string) (uint32, error) {
	row, err := r.root(name)
	if err != nil {
		return 0, err
	}
	values, err := rentalU32(r.t, row, 1)
	if err != nil {
		return 0, err
	}
	return values[0], nil
}

func (r rentalRows) tiers(name string) ([]BufferRentalTier, error) {
	row, err := r.root(name)
	if err != nil {
		return nil, err
	}
	values, err := rentalU32(r.t, row, -1)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 || len(values)%2 != 0 {
		return nil, fmt.Errorf("%s: %s needs level/value pairs", r.t.Path, name)
	}
	var out []BufferRentalTier
	for i := 0; i < len(values); i += 2 {
		if i > 0 && values[i] <= values[i-2] {
			return nil, fmt.Errorf("%s: unordered/duplicate %s level", r.t.Path, name)
		}
		out = append(out, BufferRentalTier{Level: values[i], Value: values[i+1]})
	}
	return out, nil
}

func rentalTuple(t *pvf.CTPTable, row pvf.CTPRecord, count int) (string, []uint32, error) {
	if len(row.Cells) != count+1 || row.Cells[0].Kind != "name" {
		return "", nil, fmt.Errorf("%s: malformed %s tuple at row %d", t.Path, row.Name, row.Index)
	}
	kind := row.Cells[0].Name
	if kind != "basic" && kind != "awakening" {
		return "", nil, fmt.Errorf("%s: unknown buff kind %q", t.Path, kind)
	}
	row.Cells = row.Cells[1:]
	values, err := rentalU32(t, row, count)
	return kind, values, err
}

func rentalU32(t *pvf.CTPTable, row pvf.CTPRecord, count int) ([]uint32, error) {
	if count >= 0 && len(row.Cells) != count {
		return nil, fmt.Errorf("%s: %s row %d needs %d numeric cells", t.Path, row.Name, row.Index, count)
	}
	values := make([]uint32, len(row.Cells))
	for i, cell := range row.Cells {
		v := cell.Float
		if cell.Kind != "float" || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > math.MaxUint32 || math.Trunc(v) != v {
			return nil, fmt.Errorf("%s: %s row %d cell %d is not u32", t.Path, row.Name, row.Index, i)
		}
		values[i] = uint32(v)
	}
	return values, nil
}
