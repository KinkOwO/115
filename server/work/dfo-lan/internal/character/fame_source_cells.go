package character

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strconv"
)

type fameSourceSection struct {
	tag   string
	cells []pvf.Token
}

func fameSourceSections(cells []pvf.Token) []fameSourceSection {
	var out []fameSourceSection
	for _, t := range cells {
		if t.Type == 3 {
			out = append(out, fameSourceSection{tag: t.Text})
		} else if len(out) > 0 {
			out[len(out)-1].cells = append(out[len(out)-1].cells, t)
		}
	}
	return out
}

// The existing fame exporter selects the last occurrence of each field.
func fameSourceField(rows []fameSourceSection, tag string) []pvf.Token {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].tag == tag {
			return rows[i].cells
		}
	}
	return nil
}

func fameSourceBlocks(rows []fameSourceSection, tag string) [][]fameSourceSection {
	var out [][]fameSourceSection
	start := -1
	for i, row := range rows {
		if row.tag == tag {
			start = i + 1
		} else if row.tag == "[/"+tag[1:] && start >= 0 {
			out = append(out, rows[start:i])
			start = -1
		}
	}
	return out
}

func fameSourceNumbers(cells []pvf.Token) ([]int64, error) {
	out := []int64{}
	for _, t := range cells {
		var v int64
		switch t.Type {
		case 0:
			v = int64(t.Value)
		case 2:
			f := float64(math.Float32frombits(uint32(t.Value)))
			if math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f || f < math.MinInt32 || f > math.MaxUint32 {
				return nil, fmt.Errorf("non-integral fame number")
			}
			v = int64(f)
		case 6:
			var err error
			v, err = strconv.ParseInt(t.Text, 10, 64)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported fame numeric cell %d", t.Type)
		}
		out = append(out, v)
	}
	return out, nil
}

type fameInteger interface {
	~int | ~int64 | ~uint32 | ~byte
}

func fameSourcePairs[K fameInteger, V fameInteger](cells []pvf.Token) (map[K]V, error) {
	values, err := fameSourceNumbers(cells)
	if err != nil {
		return nil, err
	}
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("incomplete fame pairs")
	}
	out := map[K]V{}
	for i := 0; i < len(values); i += 2 {
		key, value := K(values[i]), V(values[i+1])
		if int64(key) != values[i] || int64(value) != values[i+1] {
			return nil, fmt.Errorf("fame pair overflow")
		}
		out[key] = value
	}
	return out, nil
}

func fameSourceTriples[K fameInteger, L fameInteger](cells []pvf.Token) (map[K]map[L]int64, error) {
	values, err := fameSourceNumbers(cells)
	if err != nil {
		return nil, err
	}
	if len(values)%3 != 0 {
		return nil, fmt.Errorf("incomplete fame triples")
	}
	out := map[K]map[L]int64{}
	for i := 0; i < len(values); i += 3 {
		key, rank := K(values[i]), L(values[i+1])
		if int64(key) != values[i] || int64(rank) != values[i+1] {
			return nil, fmt.Errorf("fame triple overflow")
		}
		if out[key] == nil {
			out[key] = map[L]int64{}
		}
		out[key][rank] = values[i+2]
	}
	return out, nil
}
