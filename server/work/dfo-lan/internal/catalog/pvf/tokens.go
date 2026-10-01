package pvf

import (
	"fmt"
	"math"
)

// Token preserves the native five-byte cell. Unknown types keep their raw
// value; consumers must not silently treat them as numeric gameplay data.
type Token struct {
	Type      byte    `json:"type"`
	Value     int32   `json:"value"`
	Text      string  `json:"text,omitempty"`
	Reference string  `json:"reference,omitempty"`
	Number    float32 `json:"number,omitempty"`
}

func (a *Archive) Tokens(path string) ([]Token, error) {
	f, ok := a.FindFile(path)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, path)
	}
	if f.DataType != 1 {
		return nil, fmt.Errorf("entry %s is not a script", path)
	}
	raw, err := a.ReadRaw(path)
	if err != nil {
		return nil, err
	}
	return a.TokensFromRaw(raw)
}

// TokensFromRaw parses already-read script bytes without a second body copy.
// Callers must verify that the source entry has DataType 1.
func (a *Archive) TokensFromRaw(raw []byte) ([]Token, error) {
	if len(raw)%5 != 0 {
		return nil, fmt.Errorf("%w: incomplete script cell", ErrInvalidArchive)
	}
	result := make([]Token, 0, len(raw)/5)
	for off := 0; off < len(raw); off += 5 {
		t := Token{Type: raw[off], Value: int32(readInt32(raw[off+1 : off+5]))}
		switch t.Type {
		case 2:
			t.Number = math.Float32frombits(uint32(t.Value))
		case 3, 6:
			t.Text = a.resolveString(int(t.Value))
		case 8:
			// Keep localization keys distinct from translated display text.
			t.Reference = a.resolveString(int(t.Value))
		}
		result = append(result, t)
	}
	return result, nil
}
