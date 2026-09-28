package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// navigation_room.map's source role/gathering gate actor, not Ica or a
// reward boss. gate_open.act enters its last action after the party gathers.
const ApocalypseNavigationGate uint32 = 109019067

// EntryPlan binds the imported legion content to the actual dungeon catalog.
// It is not a second copy of the phase map table, nor permission to treat every
// room as a clear. The shared run owner still requires real loading/death proof.
type EntryPlan struct {
	Source           string
	Waiting          catalog.LegionAreaPoint
	Recruiting       catalog.LegionAreaPoint
	MinimumLevel     byte
	SelectionTimeout time.Duration
	Destinations     [6]uint32 // source dungeon table, NOT stage order
	Bosses           [6]uint32
	PhaseSeconds     [6]time.Duration
}

func BuildEntryPlan(contents catalog.LegionContents, operations *catalog.ApocalypseCatalog, dungeons catalog.DungeonCatalog) (EntryPlan, error) {
	var out EntryPlan
	c, ok := contents.Contents["Apocalypse"]
	// 生成物与副本目录必须同代。原实现（捐赠者主线）在这里调自己的 resources.AcceptsGeneration
	// 做「代次等价放行」；本仓库没有 resources 包，按本仓库既有口径改为严格同一 checksum。
	// 若你的部署用「汉化 PVF + 另一代导出配置」的组合，请把这一行换成你自己的等价判定，
	// 不要删掉这道检查：军团入场计划里的目标/坐标/时钟全部来自这两份数据。
	if !ok || !c.Complete || contents.Source.Checksum == "" || contents.Source.Checksum != dungeons.Source.Checksum || operations == nil {
		return out, fmt.Errorf("apocalypse source content/dungeon generation unavailable")
	}
	if c.LimitLevel != 115 || c.LastPhase != 5 || len(c.Dungeons) != 6 || c.Waiting.Town != 239 || c.Waiting.Area != 2 || c.Recruiting.Town != 239 || c.Recruiting.Area != 1 || c.SelectLimit <= 0 || c.SelectLimit > 300 {
		return out, fmt.Errorf("unsupported apocalypse content shape")
	}
	out.Source = contents.Source.Checksum
	out.MinimumLevel = byte(c.LimitLevel)
	out.Waiting, out.Recruiting = c.Waiting, c.Recruiting
	out.SelectionTimeout = time.Duration(c.SelectLimit) * time.Second
	for i, row := range c.Dungeons {
		if row.Dungeon == 0 {
			return EntryPlan{}, fmt.Errorf("apocalypse phase%d lacks its source dungeon", i)
		}
		if err := dungeon.ValidateSelection(dungeons, protocol.DungeonSelection{ID: row.Dungeon, Party: 65535}, out.MinimumLevel, nil); err != nil {
			return EntryPlan{}, fmt.Errorf("apocalypse phase%d: %w", i, err)
		}
		out.Destinations[i] = row.Dungeon
		out.Bosses[i] = row.Boss
	}
	if len(operations.PhaseClock) != 6 {
		return EntryPlan{}, fmt.Errorf("apocalypse source clock missing")
	}
	for i, v := range operations.PhaseClock {
		if v.Phase != int64(i) || v.Seconds <= 0 || v.Seconds > 3600 {
			return EntryPlan{}, fmt.Errorf("apocalypse phase clock invalid")
		}
		out.PhaseSeconds[i] = time.Duration(v.Seconds * float64(time.Second))
	}
	return out, nil
}

// Current14069BCC0 reads the stage record's first byte, then passes that
// destination index to the source virtual+96 lookup. Stage and destination
// must never be collapsed into the same index. This function resolves only;
// the shared owner must separately authorize the current stage/room/portal.
func (p EntryPlan) ResolveStageDestination(stage uint32, targets [6]byte) (uint32, error) {
	if stage >= uint32(len(targets)) {
		return 0, fmt.Errorf("apocalypse stage out of range")
	}
	index := targets[stage]
	if int(index) >= len(p.Destinations) || p.Destinations[index] == 0 {
		return 0, fmt.Errorf("apocalypse stage has no declared destination")
	}
	return p.Destinations[index], nil
}

// Current native difficulty-name reader140699750 looks up wireChoice+1.
// Thus the CTP indices1/2/3/5 are NOT the 2045 entry-stage field.
func PlanForChoice(cat *catalog.ApocalypseCatalog, clock *ApocalypseClock, choice byte) (*RunPlan, error) {
	if choice == 255 {
		return nil, fmt.Errorf("no apocalypse difficulty selected")
	}
	return BuildRunPlan(cat, clock, uint32(choice)+1)
}
