// equipmentaudit exports current source fields without assigning drop semantics.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/equipment_audit.json", "audit output")
	fameState := flag.String("fame-state", "", "只读核对名望：角色JSON文件或标准输入-，每条含name和state")
	flag.String("equipment-full", "", "兼容旧参数；名望装备只读PVF")
	flag.Parse()
	if *fameState != "" {
		if err := auditFame(*source, *fameState); err != nil {
			log.Fatal(err)
		}
		return
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	idx, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		log.Fatal(e)
	}
	rows, e := catalog.ParseIndex(idx.Cells)
	if e != nil {
		log.Fatal(e)
	}
	type row struct {
		ID     uint32
		Path   string
		Fields map[string][]pvf.Token
		SHA256 string
	}
	var selected []row
	counts := map[string]int{}
	for _, r := range rows {
		p := r.Path
		if !strings.HasPrefix(p, "equipment/") {
			p = "equipment/" + p
		}
		if strings.Contains(p, "/avatar/") {
			continue
		}
		s, e := catalog.ResolveScript(a, p)
		if e != nil {
			counts["unreadable"]++
			continue
		}
		fields := map[string][]pvf.Token{}
		name := ""
		for _, t := range s.Cells {
			if t.Type == 3 {
				name = t.Text
				continue
			}
			switch name {
			case "[name]", "[grade]", "[rarity]", "[creation rate]", "[minimum level]", "[equipment type]", "[durability]", "[attach type]":
				fields[name] = append(fields[name], t)
			}
		}
		counts["read"]++
		rate := fields["[creation rate]"]
		if len(rate) > 0 {
			counts["has_creation_rate"]++
		}
		grade := fields["[grade]"]
		level := fields["[minimum level]"]
		low := len(grade) > 0 && grade[0].Type == 0 && grade[0].Value <= 20 && grade[0].Value > 0 || len(level) > 0 && level[0].Type == 0 && level[0].Value <= 20 && level[0].Value > 0
		if low {
			selected = append(selected, row{r.ID, s.Path, fields, s.SHA256})
		}
	}
	data := map[string]any{"source": a.Snapshot(), "index_hash": idx.SHA256, "counts": counts, "rows": selected}
	b, e := json.MarshalIndent(data, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("equipment audited=%v low_grade_or_level=%d", counts, len(selected))
}

// 直接调用游戏服相同的计算器；只读输入与装备目录，不连接或修改玩家数据库。
func auditFame(prefix, path string) error {
	a, err := pvf.LoadArchive(pvf.Options{Path: prefix, MaxBytes: 1 << 30})
	if err != nil {
		return err
	}
	defer a.Close()
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		return err
	}
	full, err := inventory.OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		return err
	}
	defer full.Close()
	service := character.Service{Equipment: &inventory.EquipmentCatalog{Full: full}}
	input := os.Stdin
	if path != "-" {
		input, err = os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
	}
	decoder, encoder := json.NewDecoder(input), json.NewEncoder(os.Stdout)
	for {
		var row struct {
			Name  string          `json:"name"`
			State json.RawMessage `json:"state"`
		}
		if err = decoder.Decode(&row); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		detail, err := service.EquipmentFameBreakdown(row.State)
		if err != nil {
			return fmt.Errorf("角色%s名望核对失败：%w", row.Name, err)
		}
		if err = encoder.Encode(struct {
			Name string                  `json:"name"`
			Fame character.FameBreakdown `json:"fame"`
		}{row.Name, detail}); err != nil {
			return err
		}
	}
}
