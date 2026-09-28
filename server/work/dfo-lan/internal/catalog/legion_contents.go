package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LegionContents 是 contents/system/legionsystem/legionsystem.cos 的导出物：
// 军团内容表（末世录 / 维纳斯 / 达斯岛 / 千海之空 … 各自一块 [legion data set]）。
//
// 为什么单独导：军团入口的**几乎全部**判据都在这张表里，而不在 apocalypse.ctp：
//   - [recruiting area info] / [waiting area info]  → 频道的招募区与等候区坐标
//   - [dungeon info data]                           → 每个**阶段**的副本号与 boss 号
//   - [entry data] [weekly enter count]/[phase reward count] → 入口门禁⑤⑥读的次数真源
//   - [enter data]                                  → 难度×单双人 的名声门槛（如末世录 73993）
//   - [incount dungeon index]                       → 计数键（与 REWARD_ITEM_DUNGEON_INDEX 同值）
//   - [ui data][int list] ENABLE_CHANNEL_TAB_EVENT_ID → 分享版所说的 channel OpenEvent（末世录 = 1007）
//
// 字段名一律保留源里的写法，**不翻译语义**；不确定含义的值（如 [dungeon info data] 行尾两个数）
// 按位置保留为 Tail，不命名。
type LegionContents struct {
	Source   pvf.ArchiveSnapshot      `json:"source"`
	Path     string                   `json:"path"`
	SHA256   string                   `json:"sha256"`
	Bytes    int                      `json:"bytes"`
	Contents map[string]LegionContent `json:"contents"`
}

type LegionContent struct {
	Name         string            `json:"name"`
	LimitLevel   int64             `json:"limit_level"`
	LastPhase    int64             `json:"last_phase"`
	NeedFatigue  int64             `json:"need_fatigue"`
	Recruiting   LegionAreaPoint   `json:"recruiting_area"`
	Waiting      LegionAreaPoint   `json:"waiting_area"`
	MovieTime    int64             `json:"movie_time"`
	SelectLimit  int64             `json:"operation_select_limit_seconds"`
	QuestIndex   []int64           `json:"quest_index"`
	Dungeons     []LegionDungeon   `json:"dungeon_info_data"`
	WeeklyEnter  int64             `json:"weekly_enter_count"`
	PhaseReward  int64             `json:"phase_reward_count"`
	EnterData    []LegionEnterRow  `json:"enter_data"`
	InCountIndex int64             `json:"incount_dungeon_index"`
	UIInts       map[string]int64  `json:"ui_int_list"`
	Rewards      []LegionRewardRow `json:"reward_data"`
	TingRewards  []LegionRewardRow `json:"ting_reward_data"`

	// Complete/Missing 报告"入口侧消费需要的字段"是否齐全。源里各内容块的字段并不一致，
	// 所以目录忠实取值、由使用方按 Complete 把关，而不是在这里假装整张表都齐。
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing,omitempty"`
}

// LegionAreaPoint 是 [recruiting area info] / [waiting area info] 的 (town, area, x, y)。
type LegionAreaPoint struct {
	Town uint32 `json:"town"`
	Area uint32 `json:"area"`
	X    uint16 `json:"x"`
	Y    uint16 `json:"y"`
}

// LegionDungeon 是 [dungeon info data] 的一行，按**阶段**排列（第 0 行 = phase 0）：
// 名称引用、副本号、boss 号，以及行尾两个未解意义的数（按位置保留）。
type LegionDungeon struct {
	NameRef string  `json:"name_ref"`
	Dungeon uint32  `json:"dungeon"`
	Boss    uint32  `json:"boss"`
	Tail    []int64 `json:"tail"`
}

// LegionEnterRow 是 [enter data] 的一行：难度、单双人、两个未定值、名声下限/上限。
type LegionEnterRow struct {
	Difficulty string  `json:"difficulty"`
	PartyType  string  `json:"party_type"`
	Values     []int64 `json:"values"`
	FameLow    int64   `json:"fame_low"`
	FameHigh   int64   `json:"fame_high"`
}

// LegionRewardRow 是 [reward data]/[ting reward data] 的一行：难度、档位、权重、物品、数量。
type LegionRewardRow struct {
	Difficulty string  `json:"difficulty"`
	Kind       string  `json:"kind"`
	Values     []int64 `json:"values"`
}

const LegionContentsPath = "contents/system/legionsystem/legionsystem.cos"

// LoadLegionContents 读已导出的目录。
func LoadLegionContents(path, source string) (LegionContents, error) {
	var c LegionContents
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	// 生成物声明的代次必须与调用方期望的一致。捐赠者主线在这里用自己 catalog 包里的
	// SourceMismatch（默认不校验、可用开关收紧）；本补丁保持上游的严格口径，不把
	// 「默认关掉代次校验」带进来。若你的部署确实要跨代（汉化 PVF + 另一代导出），
	// 请在这里换成你自己的等价判定并保留一道检查。
	if c.Source.Checksum == "" || c.Source.Checksum != source {
		return c, fmt.Errorf("legion contents generation mismatch")
	}
	if len(c.Contents) == 0 {
		return c, fmt.Errorf("legion contents are empty")
	}
	return c, nil
}

// ImportLegionContents 从内层归档读取并解析军团内容表。
func ImportLegionContents(a *pvf.Archive) (LegionContents, error) {
	out := LegionContents{Source: a.Snapshot(), Path: LegionContentsPath}
	if _, ok := a.FindFile(LegionContentsPath); !ok {
		return out, fmt.Errorf("%s is missing from the archive", LegionContentsPath)
	}
	raw, e := a.ReadRaw(LegionContentsPath)
	if e != nil {
		return out, e
	}
	sum := sha256.Sum256(raw)
	out.SHA256, out.Bytes = hex.EncodeToString(sum[:]), len(raw)
	text, e := a.ReadText(LegionContentsPath)
	if e != nil {
		return out, e
	}
	out.Contents, e = ParseLegionContents(text)
	return out, e
}

// ParseLegionContents 解析文本 COS。语法与 PVF 脚本文本一致：
//
//	[name] 可选同行值
//	 子块或数据行
//	[/name]
//
// 只要有一块缺关键字段就报错，避免拿半张表当完整表用。
func ParseLegionContents(text string) (map[string]LegionContent, error) {
	root, e := parseLegionBlocks(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "root")
	if e != nil {
		return nil, e
	}
	out := map[string]LegionContent{}
	for _, set := range root.children {
		if set.name != "legion data set" {
			continue
		}
		c, e := parseLegionContent(set)
		if e != nil {
			return nil, e
		}
		if c.Name == "" {
			return nil, fmt.Errorf("legion data set without [contents]")
		}
		out[c.Name] = c
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no [legion data set] block")
	}
	return out, nil
}

// parseLegionContent 解析一个 [legion data set] 块。
//
// **不做"必须齐全"的强校验**：源里各内容块的字段并不一致 —— 例如
// ForestOfAwakeningNormal 没有 [dungeon info data]、Dimension Cloister 的
// "行数 == last phase+1" 也不成立。所以这里忠实取值，缺什么就空什么，另外用
// Missing 列出"我们消费时真正需要的字段"缺了哪些，由导入器打印、由使用方把关。
func parseLegionContent(set *legionBlock) (LegionContent, error) {
	var c LegionContent
	var err error
	if c.Name, _ = set.stringField("contents"); c.Name == "" {
		return c, nil
	}
	var missing []string

	if c.LimitLevel, err = set.optionalIntField("limit level"); err != nil {
		return c, err
	} else if !set.has("limit level") {
		missing = append(missing, "[limit level]")
	}
	if c.LastPhase, err = set.optionalIntField("last phase"); err != nil {
		return c, err
	} else if !set.has("last phase") {
		missing = append(missing, "[last phase]")
	}
	if c.NeedFatigue, err = set.optionalIntField("need fatigue"); err != nil {
		return c, err
	}
	if c.MovieTime, err = set.optionalIntField("movie time"); err != nil {
		return c, err
	}
	if c.SelectLimit, err = set.optionalIntField("operation select limit time"); err != nil {
		return c, err
	}
	if c.InCountIndex, err = set.optionalIntField("incount dungeon index"); err != nil {
		return c, err
	}
	c.QuestIndex = set.intListField("quest index")
	for _, spec := range []struct {
		field string
		into  *LegionAreaPoint
	}{{"recruiting area info", &c.Recruiting}, {"waiting area info", &c.Waiting}} {
		if !set.has(spec.field) {
			missing = append(missing, "["+spec.field+"]")
			continue
		}
		v, e := set.areaField(spec.field)
		if e != nil {
			return c, e
		}
		*spec.into = v
	}
	if c.Dungeons, err = set.dungeonRows(); err != nil {
		return c, err
	}
	if len(c.Dungeons) == 0 {
		missing = append(missing, "[dungeon info data]")
	}
	// 不把"行数 == last phase + 1"当契约：源里并不总是成立
	// （Dimension Cloister：last phase 2 却有 5 行；Apocalypse：last phase 5、6 行）。
	// 末世录那 6 行与阶段 0..5 对应，是同块内 [boss tooltip info] 的
	// 1phase..6phase（Apocalypse_Boss_Name_00..05）佐证的，不能推广到所有内容。
	if entry := set.child("entry data"); entry != nil {
		if c.WeeklyEnter, err = entry.intField("weekly enter count"); err != nil {
			return c, err
		}
		if c.PhaseReward, err = entry.intField("phase reward count"); err != nil {
			return c, err
		}
	} else {
		missing = append(missing, "[entry data]")
	}
	if c.EnterData, err = set.enterRows(); err != nil {
		return c, err
	}
	if ui := set.child("ui data"); ui != nil {
		if c.UIInts, err = ui.uiIntList(); err != nil {
			return c, err
		}
	}
	if c.Rewards, err = set.rewardRows("reward data"); err != nil {
		return c, err
	}
	if c.TingRewards, err = set.rewardRows("ting reward data"); err != nil {
		return c, err
	}
	c.Missing = missing
	c.Complete = len(missing) == 0
	return c, nil
}

// —— 下面是块解析与取值工具 ——

type legionBlock struct {
	name     string
	head     string
	lines    []string
	children []*legionBlock
}

func (b *legionBlock) child(name string) *legionBlock {
	for _, c := range b.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func (b *legionBlock) text(name string) (string, bool) {
	c := b.child(name)
	if c == nil {
		return "", false
	}
	if strings.TrimSpace(c.head) != "" {
		return strings.TrimSpace(c.head), true
	}
	return strings.TrimSpace(strings.Join(c.lines, " ")), true
}

func (b *legionBlock) has(name string) bool {
	_, ok := b.text(name)
	return ok
}

func (b *legionBlock) stringField(name string) (string, bool) {
	v, ok := b.text(name)
	if !ok {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(v), "`"), true
}

func (b *legionBlock) intField(name string) (int64, error) {
	v, ok := b.text(name)
	if !ok {
		return 0, fmt.Errorf("missing [%s]", name)
	}
	return strconv.ParseInt(strings.Fields(v)[0], 10, 64)
}

func (b *legionBlock) optionalIntField(name string) (int64, error) {
	v, ok := b.text(name)
	if !ok {
		return 0, nil
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return 0, nil
	}
	return strconv.ParseInt(fields[0], 10, 64)
}

func (b *legionBlock) intListField(name string) []int64 {
	c := b.child(name)
	if c == nil {
		return nil
	}
	var out []int64
	for _, line := range c.lines {
		for _, f := range strings.Fields(line) {
			if n, e := strconv.ParseInt(f, 10, 64); e == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

func (b *legionBlock) areaField(name string) (LegionAreaPoint, error) {
	v, ok := b.text(name)
	if !ok {
		return LegionAreaPoint{}, fmt.Errorf("missing [%s]", name)
	}
	f := strings.Fields(v)
	if len(f) != 4 {
		return LegionAreaPoint{}, fmt.Errorf("[%s] has %d fields, want 4", name, len(f))
	}
	var n [4]int64
	for i, s := range f {
		x, e := strconv.ParseInt(s, 10, 64)
		if e != nil {
			return LegionAreaPoint{}, fmt.Errorf("[%s] field %d: %w", name, i, e)
		}
		n[i] = x
	}
	if n[0] <= 0 || n[1] < 0 || n[2] < 0 || n[3] < 0 || n[2] > 65535 || n[3] > 65535 {
		return LegionAreaPoint{}, fmt.Errorf("[%s] out of range: %v", name, n)
	}
	return LegionAreaPoint{Town: uint32(n[0]), Area: uint32(n[1]), X: uint16(n[2]), Y: uint16(n[3])}, nil
}

// dungeonRows 取 [dungeon info data]。块不存在时返回空表（源里确实有内容块没有这段），
// 是否可用由 LegionContent.Missing 报告。
func (b *legionBlock) dungeonRows() ([]LegionDungeon, error) {
	c := b.child("dungeon info data")
	if c == nil {
		return nil, nil
	}
	var out []LegionDungeon
	for _, line := range c.lines {
		f := strings.Fields(line)
		if len(f) < 3 {
			return nil, fmt.Errorf("[dungeon info data] row %q", line)
		}
		row := LegionDungeon{NameRef: f[0]}
		var e error
		if row.Dungeon, e = parseU32(f[1], "dungeon"); e != nil {
			return nil, e
		}
		// boss 允许为 0：末世录第一行实测就是 "100005112 0 5 26 43"（该阶段没有独立 boss 号）。
		if row.Boss, e = parseU32Zero(f[2], "boss"); e != nil {
			return nil, e
		}
		for _, s := range f[3:] {
			n, e := strconv.ParseInt(s, 10, 64)
			if e != nil {
				return nil, fmt.Errorf("[dungeon info data] tail %q", s)
			}
			row.Tail = append(row.Tail, n)
		}
		out = append(out, row)
	}
	return out, nil
}

func (b *legionBlock) enterRows() ([]LegionEnterRow, error) {
	c := b.child("enter data")
	if c == nil {
		return nil, nil
	}
	var out []LegionEnterRow
	for _, line := range c.lines {
		f := strings.Fields(line)
		if len(f) < 4 {
			return nil, fmt.Errorf("[enter data] row %q", line)
		}
		row := LegionEnterRow{Difficulty: strings.Trim(f[0], "`"), PartyType: strings.Trim(f[1], "`")}
		for _, s := range f[2:] {
			n, e := strconv.ParseInt(s, 10, 64)
			if e != nil {
				return nil, fmt.Errorf("[enter data] value %q", s)
			}
			row.Values = append(row.Values, n)
		}
		if len(row.Values) >= 2 {
			row.FameLow, row.FameHigh = row.Values[len(row.Values)-2], row.Values[len(row.Values)-1]
		}
		out = append(out, row)
	}
	return out, nil
}

func (b *legionBlock) rewardRows(name string) ([]LegionRewardRow, error) {
	c := b.child(name)
	if c == nil {
		return nil, nil
	}
	var out []LegionRewardRow
	for _, section := range c.children {
		if section.name != "data" {
			continue
		}
		for _, line := range section.lines {
			f := strings.Fields(line)
			if len(f) < 2 {
				return nil, fmt.Errorf("[%s] row %q", name, line)
			}
			row := LegionRewardRow{Difficulty: strings.Trim(f[0], "`"), Kind: strings.Trim(f[1], "`")}
			for _, s := range f[2:] {
				n, e := strconv.ParseInt(s, 10, 64)
				if e != nil {
					return nil, fmt.Errorf("[%s] value %q", name, s)
				}
				row.Values = append(row.Values, n)
			}
			out = append(out, row)
		}
	}
	return out, nil
}

func (b *legionBlock) uiIntList() (map[string]int64, error) {
	list := b.child("int list")
	if list == nil {
		return nil, nil
	}
	out := map[string]int64{}
	for _, line := range list.lines {
		f := strings.Fields(line)
		if len(f) < 2 {
			return nil, fmt.Errorf("[int list] row %q", line)
		}
		n, e := strconv.ParseInt(f[len(f)-1], 10, 64)
		if e != nil {
			return nil, fmt.Errorf("[int list] value %q", f[len(f)-1])
		}
		key := strings.Trim(strings.Join(f[:len(f)-1], " "), "`")
		out[key] = n
	}
	return out, nil
}

func parseU32Zero(s, what string) (uint32, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n < 0 || n > 4294967295 {
		return 0, fmt.Errorf("[%s] value %q out of range", what, s)
	}
	return uint32(n), nil
}

func parseU32(s, what string) (uint32, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 || n > 4294967295 {
		return 0, fmt.Errorf("[%s] value %q out of range", what, s)
	}
	return uint32(n), nil
}

// parseLegionBlocks 把行切成块树。
//
// 关键区分：这张表里**既有块也有单行字段** —— "[limit level] 115" 是字段（没有闭标签），
// "[dungeon info data]" 才是块。判据是文件里是否存在对应的 "[/name]"：只有出现过闭标签的
// 名字才入栈；其余按"带同行值的字段"挂到当前块下，不改变嵌套。
// trimLine 去掉行首尾的空白与 NUL 填充。PVF 的文本 COS 整份以 NUL 结尾（实测最后一行的
// 原貌是闭标签后面还跟着一个 0x00 字节），只用 TrimSpace 会让闭标签永远匹配不上。
func trimLine(s string) string { return strings.Trim(strings.TrimSpace(s), string(rune(0))) }

// closeTagName 取闭标签的名字：剥掉 "[/" 前缀与结尾的 ']'（源里该行还可能带尾随空格）。
func closeTagName(line string) string {
	name := trimLine(strings.TrimPrefix(trimLine(line), "[/"))
	return trimLine(strings.TrimRight(name, "]"))
}

func parseLegionBlocks(lines []string, open string) (*legionBlock, error) {
	closing := map[string]bool{}
	for _, raw := range lines {
		line := trimLine(raw)
		if strings.HasPrefix(line, "[/") {
			closing[closeTagName(line)] = true
		}
	}
	root := &legionBlock{name: "root"}
	stack := []*legionBlock{root}
	for n, raw := range lines {
		line := trimLine(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[/") {
			name := closeTagName(line)
			if len(stack) == 1 || stack[len(stack)-1].name != name {
				return nil, fmt.Errorf("line %d: closing tag [/%s] does not match open block", n+1, name)
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated tag %q", n+1, line)
			}
			name := trimLine(line[1:end])
			b := &legionBlock{name: name, head: trimLine(line[end+1:])}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, b)
			if closing[name] {
				stack = append(stack, b)
			}
			continue
		}
		cur := stack[len(stack)-1]
		cur.lines = append(cur.lines, line)
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("unclosed block [%s]", stack[len(stack)-1].name)
	}
	return root, nil
}
