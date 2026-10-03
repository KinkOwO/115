package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 频道属性的**直读**真源（不经过任何导出 JSON）：
//
//	etc/clientchannelinfo.etc   每个 [client channel info] 的
//	                            [channelType] + [seriaRoomTown] + [guide dungeon index] +
//	                            [isLegion]/[isRaid]/[isPreRaid]/[isSemiRaid] + [iconIndex]
//	etc/channelslotinfo.etc     每个 [special channel slot info] 的
//	                            [attach panel]（进哪个 F7 面板）与 [drop item dungeon index]
//
// ⚠️ **两张表之间没有 type 数字 ↔ 符号名的关联字段**：channelslotinfo 用
// `CHANNEL_TEMPLE_OF_DEATH` 这类符号写 [type]，clientchannelinfo 只有数字；
// 客户端 `DFO.exe` 里也**搜不到任何 `CHANNEL_` 明文**（2026-10-02 全量搜过 246MB）。
// 所以这张映射**没有真源**，只能由本地配置按"发布的 ID 就是 type"约定给出
// （见 internal/channelrefresh + configs 的频道清单）。
//
// 唯一的例外是沉月湖：它的 slot 直接写 `[channel type] 101` 数字，本解析器两者都收。
const (
	ClientChannelInfoPath = "etc/clientchannelinfo.etc"
	ChannelSlotInfoPath   = "etc/channelslotinfo.etc"
)

// ChannelAttributes 是源里一个 `[client channel info]` 块的直读结果。
type ChannelAttributes struct {
	Type uint32 `json:"type"`

	IsLegion   bool `json:"is_legion,omitempty"`
	IsRaid     bool `json:"is_raid,omitempty"`
	IsPreRaid  bool `json:"is_pre_raid,omitempty"`
	IsSemiRaid bool `json:"is_semi_raid,omitempty"`

	Town         int    `json:"town,omitempty"`
	TownArea     int    `json:"town_area,omitempty"`
	GuideDungeon uint32 `json:"guide_dungeon,omitempty"`
	IconIndex    int    `json:"icon_index,omitempty"`

	// Panel / SlotDungeon / SlotSymbol 来自 channelslotinfo 里按 type 对上的那一块；
	// 对不上时为空（普通区域频道本来就不在 channelslotinfo 的 special 段里）。
	Panel       string `json:"panel,omitempty"`
	SlotDungeon uint32 `json:"slot_dungeon,omitempty"`
	SlotSymbol  string `json:"slot_symbol,omitempty"`
}

// ChannelDirectory 是两张频道表合并后的直读投影。
type ChannelDirectory struct {
	Source pvf.ArchiveSnapshot `json:"source"`

	ClientPath   string `json:"client_path"`
	ClientSHA256 string `json:"client_sha256"`
	ClientBytes  int    `json:"client_bytes"`

	SlotPath   string `json:"slot_path"`
	SlotSHA256 string `json:"slot_sha256"`
	SlotBytes  int    `json:"slot_bytes"`

	ByType map[uint32]ChannelAttributes `json:"by_type"`

	// SlotSymbols 是 channelslotinfo 里出现过的符号名（按出现顺序），纯诊断用：
	// 它证明"符号名在客户端资源里存在"，但不参与任何判定。
	SlotSymbols []string `json:"slot_symbols,omitempty"`
}

// Attributes 查某个频道类型的直读属性。
func (d ChannelDirectory) Attributes(channelType uint32) (ChannelAttributes, bool) {
	a, ok := d.ByType[channelType]
	return a, ok
}

// Types 返回全部已知频道类型（升序，便于日志与测试）。
func (d ChannelDirectory) Types() []uint32 {
	out := make([]uint32, 0, len(d.ByType))
	for t := range d.ByType {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ImportChannelDirectory 从内层归档直读两张频道表并合并。
func ImportChannelDirectory(a *pvf.Archive) (ChannelDirectory, error) {
	out := ChannelDirectory{
		ClientPath: ClientChannelInfoPath,
		SlotPath:   ChannelSlotInfoPath,
		ByType:     map[uint32]ChannelAttributes{},
	}
	if a == nil {
		return out, fmt.Errorf("channel directory: archive is nil")
	}
	clientRaw, clientText, err := readChannelScript(a, ClientChannelInfoPath)
	if err != nil {
		return out, err
	}
	out.ClientSHA256, out.ClientBytes = hashBytes(clientRaw), len(clientRaw)
	slotRaw, slotText, err := readChannelScript(a, ChannelSlotInfoPath)
	if err != nil {
		return out, err
	}
	out.SlotSHA256, out.SlotBytes = hashBytes(slotRaw), len(slotRaw)

	attrs, err := parseClientChannelInfo(mergeDanglingTagValues(clientText))
	if err != nil {
		return out, err
	}
	slots, symbols, err := parseChannelSlotInfo(mergeDanglingTagValues(slotText))
	if err != nil {
		return out, err
	}
	out.SlotSymbols = symbols
	for _, s := range slots {
		a, ok := attrs[s.Type]
		if !ok {
			// slot 指向的符号没有可对上的 type（没有 [channel type] 数字，或该 type 不在
			// clientchannelinfo 里）——不猜，跳过；真正要用它时由上层校验报错。
			continue
		}
		a.Panel = s.Panel
		a.SlotDungeon = s.Dungeon
		a.SlotSymbol = s.Symbol
		attrs[s.Type] = a
	}
	out.ByType = attrs
	out.Source = a.Snapshot()
	return out, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func readChannelScript(a *pvf.Archive, path string) ([]byte, string, error) {
	if _, ok := a.FindFile(path); !ok {
		return nil, "", fmt.Errorf("%s is missing from the archive", path)
	}
	raw, err := a.ReadRaw(path)
	if err != nil {
		return nil, "", err
	}
	text, err := a.ReadText(path)
	if err != nil {
		return nil, "", err
	}
	return raw, text, nil
}

func parseClientChannelInfo(text string) (map[uint32]ChannelAttributes, error) {
	root, err := parseJournalTree(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ClientChannelInfoPath, err)
	}
	sections := root.children("client channel info")
	if len(sections) == 0 {
		return nil, fmt.Errorf("%s has no [client channel info] block", ClientChannelInfoPath)
	}
	out := make(map[uint32]ChannelAttributes, len(sections))
	for _, sec := range sections {
		head := sec.child("channelType")
		if head == nil {
			return nil, fmt.Errorf("%s: a [client channel info] has no [channelType]", ClientChannelInfoPath)
		}
		channelType, ok := journalUint(nodeHead(head))
		if !ok || channelType == 0 || channelType > 255 {
			return nil, fmt.Errorf("%s: [channelType] %q is not a usable u8 type", ClientChannelInfoPath, nodeHead(head))
		}
		if _, dup := out[channelType]; dup {
			return nil, fmt.Errorf("%s: duplicate [channelType] %d", ClientChannelInfoPath, channelType)
		}
		a := ChannelAttributes{Type: channelType}
		if t := sec.child("type"); t != nil {
			a.IsLegion = nodeFlag(t, "isLegion")
			a.IsRaid = nodeFlag(t, "isRaid")
			a.IsPreRaid = nodeFlag(t, "isPreRaid")
			a.IsSemiRaid = nodeFlag(t, "isSemiRaid")
		}
		l := sec.child("logic")
		if l != nil {
			a.Town = nodeInt(l, "seriaRoomTown")
			a.TownArea = nodeInt(l, "seriaRoomArea")
		}
		// ⚠️ [guide dungeon index] 在源里**两种位置都有**：108（噩梦循环）写在 [logic] 里，
		// 107（赤红铁矿）直接挂在 [client channel info] 下。两处都查，谁先给出非零用谁。
		a.GuideDungeon = nodeUint(sec, "guide dungeon index")
		if a.GuideDungeon == 0 && l != nil {
			a.GuideDungeon = nodeUint(l, "guide dungeon index")
		}
		if u := sec.child("ui"); u != nil {
			a.IconIndex = nodeInt(u, "iconIndex")
		}
		out[channelType] = a
	}
	return out, nil
}

// channelSlot 是 channelslotinfo 里一个 [special channel slot info] 块的直读结果。
type channelSlot struct {
	// Type 只在源直接给了 [channel type] 数字时才非零（沉月湖就是这么写的）。
	Type    uint32
	Symbol  string
	Panel   string
	Dungeon uint32
}

func parseChannelSlotInfo(text string) ([]channelSlot, []string, error) {
	root, err := parseJournalTree(text)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ChannelSlotInfoPath, err)
	}
	var (
		out     []channelSlot
		symbols []string
	)
	for _, sec := range root.children("special channel slot info") {
		var s channelSlot
		if t := sec.child("type"); t != nil {
			s.Symbol = strings.Trim(nodeHead(t), "`")
			symbols = append(symbols, s.Symbol)
		}
		if t := sec.child("channel type"); t != nil {
			if n, ok := journalUint(nodeHead(t)); ok {
				s.Type = n
			}
		}
		if p := sec.child("attach panel"); p != nil {
			s.Panel = strings.Trim(nodeHead(p), "`")
		}
		if d := sec.child("drop item dungeon index"); d != nil {
			if n, ok := journalUint(nodeHead(d)); ok {
				s.Dungeon = n
			}
		}
		out = append(out, s)
	}
	return out, symbols, nil
}

// mergeDanglingTagValues 把"`[tag]` 与它的值分行"的写法并回同一行。
//
// 源里两种写法都存在：
//
//	[title] 101036479          ← 同行
//	[channelType]
//	81                         ← 分行
//
// parseJournalTree 对**没有闭合标记**的 `[tag]` 一律当叶子，于是分行时的值会落到
// **父容器**的 Values 里，叶子自己什么都读不到。这里先并回一行再交给树解析。
// 只处理"后面紧跟一行且那行不是标签"的情况，不改变任何带闭合标记的容器。
func mergeDanglingTagValues(text string) string {
	lines := strings.Split(text, "\n")
	trim := func(s string) string { return strings.TrimSpace(strings.TrimRight(s, "\r")) }
	closers := map[string]bool{}
	opens := map[string]bool{}
	for _, l := range lines {
		if name, _, open, ok := sectionTag(trim(l)); ok {
			if open {
				opens[name] = true
			} else {
				closers[name] = true
			}
		}
	}
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		cur := trim(lines[i])
		name, head, open, ok := sectionTag(cur)
		// 只有"无头、无闭合标记（即叶子）"的标签才需要并值。
		if ok && open && head == "" && !closers[name] && opens[name] && i+1 < len(lines) {
			nxt := trim(lines[i+1])
			if nxt != "" {
				if _, _, _, isTag := sectionTag(nxt); !isTag {
					out = append(out, cur+" "+nxt)
					i++
					continue
				}
			}
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

// nodeHead 取 `[name]` 节点的**首行内容**：源里两种写法都有 ——
//
//	[title] 101036479      ← 值与标签同行
//	[channelType]
//	81                     ← 值与标签分行（Head 为空，值落在 Values[0]）
//
// Head 为空时回退 Values[0]，两者都空则返回空串。
func nodeHead(n *journalNode) string {
	if n == nil {
		return ""
	}
	if s := strings.TrimSpace(n.Head); s != "" {
		return s
	}
	if len(n.Values) > 0 {
		return strings.TrimSpace(n.Values[0])
	}
	return ""
}

// nodeFlag 读一个 `[name] 0/1` 叶子并转成 bool（缺失按 false）。
func nodeFlag(n *journalNode, name string) bool {
	return nodeInt(n, name) != 0
}

// nodeInt 读一个 `[name] <int>` 叶子（缺失或非数字按 0）。
func nodeInt(n *journalNode, name string) int {
	c := n.child(name)
	if c == nil {
		return 0
	}
	v, err := strconv.Atoi(nodeHead(c))
	if err != nil {
		return 0
	}
	return v
}

// nodeUint 读一个 `[name] <uint>` 叶子（缺失或非数字按 0）。
func nodeUint(n *journalNode, name string) uint32 {
	c := n.child(name)
	if c == nil {
		return 0
	}
	v, ok := journalUint(nodeHead(c))
	if !ok {
		return 0
	}
	return v
}
