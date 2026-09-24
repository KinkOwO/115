package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"flag"
	"fmt"
	"sort"
	"strings"
)

func main() {
	source := flag.String("source", "", "read-only inner PVF")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		panic(e)
	}
	shop, e := catalog.ResolveScript(a, "etc/(r)cerashop.etc")
	if e != nil {
		panic(e)
	}
	type sec struct {
		name  string
		start int
		end   int
	}
	sections := []sec{}
	cur := sec{name: "", start: -1, end: -1}
	for i, t := range shop.Cells {
		if t.Type != 3 {
			continue
		}
		if strings.HasPrefix(t.Text, "[/") {
			cur.end = i
			sections = append(sections, cur)
			cur = sec{name: "", start: -1, end: -1}
			continue
		}
		cur = sec{name: t.Text, start: i + 1, end: len(shop.Cells)}
	}
	find := func(n string) *sec {
		for i := range sections {
			if sections[i].name == n {
				return &sections[i]
			}
		}
		return nil
	}
	stats := func(name string, width int, needPriceCol int) {
		s := find(name)
		n := s.end - s.start
		fmt.Printf("--- %s rows=%d\n", name, n/width)
		for col := 0; col < width; col++ {
			vals := map[int32]int{}
			typeCnt := map[byte]int{}
			sample := []string{}
			for at := s.start + col; at+width <= s.end; at += width {
				t := shop.Cells[at]
				typeCnt[t.Type]++
				if t.Type == 6 {
					if len(sample) < 4 {
						sample = append(sample, t.Text)
					}
				} else {
					vals[t.Value]++
				}
			}
			desc := ""
			if len(vals) <= 10 {
				keys := []int{}
				for k := range vals {
					keys = append(keys, int(k))
				}
				sort.Ints(keys)
				for _, k := range keys {
					desc += fmt.Sprintf(" %d×%d", k, vals[int32(k)])
				}
			} else {
				desc = fmt.Sprintf(" distinct=%d", len(vals))
			}
			fmt.Printf("  col%2d types=%v %s %v\n", col, typeCnt, desc, sample)
		}
		_ = needPriceCol
	}
	stats("[dont trade avatar]", 14, 5)
	stats("[package]", 13, 4)
}
