package character

import (
	"dfolan/internal/catalog"

	"testing"
)

func TestConnectionContextAcrossActorRefreshes(t *testing.T) {
	cat, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	shared := Service{Catalog: cat}
	role := Character{WireID: 3, Name: "LanTest01", Profession: 0, State: unlockState(t, 1)}
	for _, ctx := range [][2]byte{{1, 10}, {1, 6}, {0, 0}} {
		local := shared
		local.ChannelContext = ctx
		for _, build := range []func() ([]byte, error){
			func() ([]byte, error) { return local.EntryBasicProbe(role, [2]byte{}) },
			func() ([]byte, error) { return local.AppearanceProbe(role, [2]byte{}) },
			func() ([]byte, error) { return local.EntryAddition(role) },
		} {
			p, err := build()
			if err != nil {
				t.Fatal(err)
			}
			if len(p) < 5 || p[3] != ctx[0] || p[4] != ctx[1] {
				t.Fatalf("context %v payload %x", ctx, p)
			}
		}
	}
	if shared.ChannelContext != [2]byte{} {
		t.Fatal("shared context changed")
	}
}
