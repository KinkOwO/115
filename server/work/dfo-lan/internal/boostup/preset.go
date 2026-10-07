package boostup

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
)

type SkillRank struct{ Skill, Level uint16 }
type SkillOption struct {
	Skill  uint16
	Option byte
}
type SkillPreset struct {
	Version, Job, Grow       byte
	Level                    uint16
	SP                       uint32
	Skills                   []SkillRank
	Evolutions, Enhancements []SkillOption
	QuickSlots               []uint16
	Tail                     []byte // source extension preserved, never guessed as another active tree
}

// Decode the PVF's skill sharing code, not a hand-maintained per-job skill list.
func DecodePreset(encoded string) (SkillPreset, error) {
	var out SkillPreset
	if len(encoded) > 262144 {
		return out, fmt.Errorf("oversized preset")
	}
	compressed, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return out, e
	}
	z, e := zlib.NewReader(bytes.NewReader(compressed))
	if e != nil {
		return out, e
	}
	raw, e := io.ReadAll(io.LimitReader(z, 65537))
	closeErr := z.Close()
	if e != nil {
		return out, e
	}
	if closeErr != nil {
		return out, closeErr
	}
	if len(raw) > 65536 || len(raw) < 16 || raw[0] != 0xf3 || raw[len(raw)-1] != 0xf3 {
		return out, fmt.Errorf("invalid skill preset envelope")
	}
	r := bytes.NewReader(raw[1 : len(raw)-1])
	read := func(v any) {
		if e == nil {
			e = binary.Read(r, binary.LittleEndian, v)
		}
	}
	read(&out.Version)
	read(&out.Job)
	read(&out.Grow)
	var marker byte
	var format uint16
	read(&marker)
	read(&out.Level)
	read(&out.SP)
	read(&format)
	if e != nil || out.Version != 3 && out.Version != 4 || marker != 3 || format != 1 || out.Level == 0 {
		return out, fmt.Errorf("unsupported preset header")
	}
	if out.Version == 4 {
		var n byte
		read(&n)
		if int(n) > r.Len() {
			return out, fmt.Errorf("truncated preset name")
		}
		name := make([]byte, n)
		_, e = io.ReadFull(r, name)
		var nameFlag byte
		read(&nameFlag)
		if e != nil {
			return out, e
		}
	}
	var count uint16
	read(&count)
	if count > 1024 {
		return out, fmt.Errorf("too many preset skills")
	}
	seen := map[uint16]bool{}
	for i := 0; i < int(count); i++ {
		var v SkillRank
		read(&v.Skill)
		read(&v.Level)
		if e != nil {
			return out, e
		}
		// Keep all16 source bits. Some presets contain values above255;
		// applying them to the byte-sized learned-rank field requires a
		// separate source/native interpretation, not truncation here.
		if v.Skill == 0 || seen[v.Skill] {
			return out, fmt.Errorf("invalid preset skill row%d id%d level%d duplicate=%v", i, v.Skill, v.Level, seen[v.Skill])
		}
		seen[v.Skill] = true
		out.Skills = append(out.Skills, v)
	}
	options := func(max byte) ([]SkillOption, error) {
		var n byte
		read(&n)
		if n > max {
			return nil, fmt.Errorf("too many preset options")
		}
		var rows []SkillOption
		for i := byte(0); i < n; i++ {
			var v SkillOption
			read(&v.Skill)
			read(&v.Option)
			if e != nil {
				return nil, e
			}
			if v.Skill == 0 || v.Option > 2 {
				return nil, fmt.Errorf("invalid preset option")
			}
			rows = append(rows, v)
		}
		return rows, e
	}
	out.Evolutions, e = options(5)
	if e != nil {
		return out, e
	}
	out.Enhancements, e = options(3)
	if e != nil {
		return out, e
	}
	var slots byte
	read(&slots)
	if e != nil || slots > 28 {
		return out, fmt.Errorf("invalid preset quickslot count")
	}
	for i := byte(0); i < slots; i++ {
		var id uint16
		read(&id)
		out.QuickSlots = append(out.QuickSlots, id)
	}
	if e != nil {
		return out, e
	}
	out.Tail, e = io.ReadAll(r)
	return out, e
}
