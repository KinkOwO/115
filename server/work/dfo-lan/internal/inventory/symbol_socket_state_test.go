package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func symbolFixture(t *testing.T) (catalog.LootCatalog, BagRules, *SymbolSocketLayout, *EmblemInlayRules, json.RawMessage) {
	t.Helper()
	source := strings.Repeat("a", 64)
	xml := `<ROOT><CONTROL>`
	for _, name := range []string{"red_symbol", "yellow_symbol", "green_symbol", "blue_symbol", "multi_symbol", "platinum", "buff_platinum"} {
		xml += fmt.Sprintf(`<IControl ID="STR:%s"><IControl ID="STR:slot_0"/><IControl ID="STR:slot_1"/></IControl>`, name)
	}
	xml += `</CONTROL></ROOT>`
	layout, e := ParseSymbolSocketLayout(source, []byte(xml))
	require.NoError(t, e)
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Items: map[uint32]catalog.LootItem{
		20001: {ID: 20001, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100},
		20002: {ID: 20002, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100},
		20003: {ID: 20003, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100},
	}}
	r := BagRules{Source: source, MissingStackLimit: 100, Slots: map[string][2]uint16{"[avatar emblem]": {289, 290}}}
	masks := &EmblemInlayRules{Source: source, Masks: map[uint32]uint16{20001: 1, 20002: 1, 20003: 8}}
	state := json.RawMessage(`{"future_character":{"value":42},"inventory":{"version":"ordinary-bag-v1","gold":777,"future_inventory":[1,2],"items":[{"slot":289,"Template":20001,"Amount":2},{"slot":290,"Template":20002,"Amount":1}]}}`)
	return c, r, layout, masks, state
}

func TestSymbolSocketExchangeRoundTripAndPreservation(t *testing.T) {
	c, r, l, m, state := symbolFixture(t)
	before := append([]byte(nil), state...)
	req := SymbolSocketRequest{Source: c.Source.Checksum, Group: "red_symbol", Socket: 0, BagSlot: 289, Template: 20001}
	equipped, e := PrepareSymbolSocketExchange(state, c, r, l, m, req, 100)
	require.NoError(t, e)
	require.Equal(t, before, []byte(state))
	saved, e := ReadSymbolSockets(equipped, l)
	require.NoError(t, e)
	require.Equal(t, uint64(1), saved.Sequence)
	require.Equal(t, uint32(20001), saved.Groups["red_symbol"][0].Template)
	b, e := ReadBag(equipped)
	require.NoError(t, e)
	require.Equal(t, uint32(1), b.Items[0].Amount)
	req.Sequence = 1
	req.ExpectedTemplate = 20001
	req.Template = 20002
	req.BagSlot = 290
	replaced, e := PrepareSymbolSocketExchange(equipped, c, r, l, m, req, 100)
	require.NoError(t, e)
	b, e = ReadBag(replaced)
	require.NoError(t, e)
	require.Len(t, b.Items, 1)
	require.Equal(t, uint32(2), b.Items[0].Amount)
	req.Sequence = 2
	req.ExpectedTemplate = 20002
	req.Template = 0
	req.BagSlot = 0
	removed, e := PrepareSymbolSocketExchange(replaced, c, r, l, m, req, 100)
	require.NoError(t, e)
	var original, after map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &original))
	require.NoError(t, json.Unmarshal(removed, &after))
	require.JSONEq(t, string(original["future_character"]), string(after["future_character"]))
	require.JSONEq(t, string(original["inventory"]), string(after["inventory"]))
	saved, e = ReadSymbolSockets(removed, l)
	require.NoError(t, e)
	require.Equal(t, uint64(3), saved.Sequence)
	require.Zero(t, saved.Groups["red_symbol"][0].Template)
}

func TestSymbolProgressStartsAtSourceMinimumAndMultiTracksLowest(t *testing.T) {
	_, _, layout, _, state := symbolFixture(t)
	layout.Rules = &SymbolEmblemRules{Tiers: map[SymbolTierKey]SymbolTier{}, Fixed: map[uint32]SymbolOptions{1: {Dealer: map[string]float64{"[equipment damage rate]": .1}}}}
	for typ := uint32(0); typ < 5; typ++ {
		layout.Rules.Tiers[SymbolTierKey{typ, 0}] = SymbolTier{Levels: map[uint32]SymbolLevel{1: {OptionIndex: 1}, 2: {OptionIndex: 1}}}
	}
	initial, e := ReadSymbolSockets(state, layout)
	require.NoError(t, e)
	require.Len(t, initial.Progress, 5)
	for _, p := range initial.Progress {
		require.Equal(t, SymbolProgress{Grade: 0, Level: 1}, p)
	}
	initial.Progress["red_symbol"] = SymbolProgress{Grade: 0, Level: 2}
	initial.Progress["multi_symbol"] = SymbolProgress{Grade: 3, Level: 99}
	raw, e := json.Marshal(map[string]any{"symbol_emblem_sockets": initial})
	require.NoError(t, e)
	next, e := ReadSymbolSockets(raw, layout)
	require.NoError(t, e)
	require.Equal(t, SymbolProgress{Grade: 0, Level: 1}, next.Progress["multi_symbol"])
	initial.Progress["red_symbol"] = SymbolProgress{Grade: 0, Level: 99}
	raw, _ = json.Marshal(map[string]any{"symbol_emblem_sockets": initial})
	_, e = ReadSymbolSockets(raw, layout)
	require.Error(t, e)
}

func TestSymbolSocketRejectsStaleAndWrongColor(t *testing.T) {
	c, r, l, m, state := symbolFixture(t)
	good := SymbolSocketRequest{Source: c.Source.Checksum, Group: "red_symbol", BagSlot: 289, Template: 20001}
	for _, change := range []func(*SymbolSocketRequest){
		func(q *SymbolSocketRequest) { q.Source = strings.Repeat("b", 64) },
		func(q *SymbolSocketRequest) { q.Sequence = 1 },
		func(q *SymbolSocketRequest) { q.ExpectedTemplate = 20002 },
		func(q *SymbolSocketRequest) { q.Group = "blue_symbol" },
		func(q *SymbolSocketRequest) { q.Socket = 2 },
		func(q *SymbolSocketRequest) { q.BagSlot = 290 },
		func(q *SymbolSocketRequest) { q.Template = 0 },
	} {
		q := good
		change(&q)
		out, e := PrepareSymbolSocketExchange(state, c, r, l, m, q, 100)
		require.Error(t, e)
		require.Nil(t, out)
	}
	equipped, e := PrepareSymbolSocketExchange(state, c, r, l, m, good, 100)
	require.NoError(t, e)
	_, e = PrepareSymbolSocketExchange(equipped, c, r, l, m, good, 100)
	require.ErrorContains(t, e, "stale")
}

func TestSymbolSocketFullBagFailureIsAtomic(t *testing.T) {
	c, r, l, m, state := symbolFixture(t)
	// Model an already-equipped distinct item and a full bag. A replacement
	// which leaves its incoming stack occupied cannot return the old item.
	ss, e := ReadSymbolSockets(state, l)
	require.NoError(t, e)
	ss.Groups["red_symbol"][0] = SymbolSocketItem{Template: 20003}
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &fields))
	fields["symbol_emblem_sockets"], e = json.Marshal(ss)
	require.NoError(t, e)
	state, e = json.Marshal(fields)
	require.NoError(t, e)
	before := append([]byte(nil), state...)
	_, e = PrepareSymbolSocketExchange(state, c, r, l, m, SymbolSocketRequest{Source: c.Source.Checksum, Group: "red_symbol", ExpectedTemplate: 20003, Template: 20001, BagSlot: 289}, 100)
	require.ErrorContains(t, e, "full")
	require.Equal(t, before, []byte(state))
}

func TestSymbolSocketExpiryIdentityIsNotMerged(t *testing.T) {
	c, r, _, _, state := symbolFixture(t)
	b, e := ReadBag(state)
	require.NoError(t, e)
	b.Items = b.Items[:1]
	b.Items[0].ExpireTime = 500
	old := SymbolSocketItem{Template: 20001}
	out, e := returnSymbolEmblem(b, c, r, old)
	require.NoError(t, e)
	require.Len(t, out.Items, 2)
	require.Equal(t, uint32(500), out.Items[0].ExpireTime)
	require.Zero(t, out.Items[1].ExpireTime)
}

func TestSymbolSocketLayoutRejectsAmbiguousResources(t *testing.T) {
	source := strings.Repeat("a", 64)
	for _, raw := range []string{`<ROOT/>`, `<ROOT><CONTROL><IControl ID="STR:red_symbol"><IControl ID="STR:slot_1"/></IControl></CONTROL></ROOT>`, `<ROOT><CONTROL><IControl ID="STR:red_symbol"><IControl ID="STR:slot_0"/><IControl ID="STR:slot_0"/></IControl></CONTROL></ROOT>`} {
		_, e := ParseSymbolSocketLayout(source, []byte(raw))
		require.Error(t, e)
	}
}

func TestSymbolSocketCurrentSourceLayout(t *testing.T) {
	path := os.Getenv("US115_SYMBOL_UI_SOURCE")
	if path == "" {
		t.Skip("explicit current CN UI source required")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	layout, err := ParseSymbolSocketLayout(strings.Repeat("a", 64), raw)
	require.NoError(t, err)
	for _, name := range []string{"red_symbol", "yellow_symbol", "green_symbol", "blue_symbol"} {
		require.EqualValues(t, 4, layout.Sockets[name].Count)
	}
	require.EqualValues(t, 6, layout.Sockets["multi_symbol"].Count)
	require.EqualValues(t, 2, layout.Sockets["platinum"].Count)
	require.EqualValues(t, 2, layout.Sockets["buff_platinum"].Count)
	// A source edit, not an executor constant, determines the slot capacity.
	changed := strings.Replace(string(raw), `ID="STR:slot_3"`, `ID="STR:slot_4"`, 1)
	_, err = ParseSymbolSocketLayout(strings.Repeat("a", 64), []byte(changed))
	require.ErrorContains(t, err, "noncontiguous")
}
