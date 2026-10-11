package gmicons

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestIconUsesTypedSourceAndRefusesMissingFrame(t *testing.T) {
	source := []pvf.Token{{Type: 3, Text: "[name]"}, {Type: 7, Text: "name"}, {Type: 3, Text: "[icon]"}, {Type: 7, Text: "Item/test.img"}, {Type: 0, Value: 368}, {Type: 3, Text: "[rarity]"}, {Type: 0, Value: 4}}
	path, frame, ok := iconField(source)
	if !ok || path != "Item/test.img" || frame != 368 {
		t.Fatal(path, frame, ok)
	}
	source[4] = pvf.Token{Type: 3, Text: "[rarity]"}
	if _, _, ok = iconField(source); ok {
		t.Fatal("guessed a frame from following section")
	}
}
