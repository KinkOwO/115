package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Read both explicit archives; never touch installed resources or player data.
// This compares the actual encoded candidate, not the intermediate build plan.
func TestLilwenRoomCandidateNative(t *testing.T) {
	path, basePath := os.Getenv("US115_TEST_LILWEN_PVF"), os.Getenv("US115_TEST_LILWEN_BASE_PVF")
	if path == "" || basePath == "" {
		t.Skip("set explicit Lilwen candidate and original US inner archive")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: path, DerivedCacheDir: "-"})
	require.NoError(t, err)
	defer s.Close()
	b, err := Open(Options{Mode: PVF, ArchivePath: basePath, DerivedCacheDir: "-"})
	require.NoError(t, err)
	defer b.Close()
	semantic := func(ts []pvf.Token) []pvf.Token {
		out := append([]pvf.Token(nil), ts...)
		for i := range out {
			if out[i].Type == 3 || out[i].Type == 6 || out[i].Type == 8 {
				out[i].Value = 0
			}
		}
		return out
	}
	room := "map/common/(r)gate_seria.map"
	before, err := b.archive.Tokens(room)
	require.NoError(t, err)
	after, err := s.archive.Tokens(room)
	require.NoError(t, err)
	var newAt = -1
	for i, cell := range after {
		if cell.Type == 0 && cell.Value == 400001880 {
			require.Equal(t, -1, newAt)
			newAt = i
		}
	}
	require.GreaterOrEqual(t, newAt, 0)
	require.GreaterOrEqual(t, len(after), newAt+5)
	require.Equal(t, "[right]", after[newAt+1].Text)
	require.EqualValues(t, 700, after[newAt+2].Value)
	require.EqualValues(t, 153, after[newAt+3].Value)
	require.Zero(t, after[newAt+4].Value)
	restored := append(append([]pvf.Token{}, after[:newAt]...), after[newAt+5:]...)
	require.Equal(t, semantic(before), semantic(restored), "all original room content must survive")
	oldIndex, err := b.archive.Tokens("list/npc.lst")
	require.NoError(t, err)
	index, err := s.archive.Tokens("list/npc.lst")
	require.NoError(t, err)
	require.Len(t, index, len(oldIndex)+2)
	require.Equal(t, semantic(oldIndex), semantic(index[:len(oldIndex)]))
	require.EqualValues(t, 400001880, index[len(oldIndex)].Value)
	npcPath := index[len(oldIndex)+1].Text
	npc, err := s.archive.Tokens(npcPath)
	require.NoError(t, err)
	faceRefs, dialogues := 0, 0
	for i, cell := range npc {
		if cell.Type == 3 {
			require.NotContains(t, []string{"[role]", "[npc role data]", "[open visible event]", "[event id]"}, cell.Text)
			if cell.Text == "[dialog]" {
				dialogues++
			}
			if cell.Text == "[default npc visible]" {
				require.EqualValues(t, 1, npc[i+1].Value)
			}
		}
		if cell.Type == 6 && strings.HasSuffix(strings.ToLower(cell.Text), "_lilwen_face.img") {
			faceRefs++
			require.EqualValues(t, 88, npc[i+1].Value)
		}
	}
	require.Equal(t, 4, dialogues)
	require.Equal(t, 4, faceRefs)
	w, err := s.World("")
	require.NoError(t, err)
	require.NotEmpty(t, w.NPCPlaces[400001880], "runtime world importer must include the source placement")
	rows := catalog.PhaseNPCsFromScript(catalog.ScriptRecord{Cells: after})
	var found bool
	for _, row := range rows {
		if row.ID == 400001880 {
			require.EqualValues(t, 700, row.X)
			require.EqualValues(t, 153, row.Y)
			found = true
		}
	}
	require.True(t, found)
	t.Logf("source=%s room=(700,153,0) original_map/index_preserved=true dialogues=4 shared_images_replaced=false live_acceptance=false", s.Snapshot().Checksum)
}
