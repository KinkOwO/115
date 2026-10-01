package pvf

// 本文件把"物品 ID"与"PVF 里 [name] 段的引用点"对上，并从引用点走到本地化文本表。
//
// 为什么以 ID 为键：物品的 ID 是独立编号，[name] 引用里就带着它；而 [name] 渲染出
// 的文本会随客户端/汉化补丁变化 —— 同一件 10362480 在本机（英文表）是
// "Rare Weapon (Actual Booster Issue X, Issued as Opened O)"，汉化后是中文。
// 用文本当标识，在未汉化的客户端上必然失效，所以定位一律以 ID 为键，文本只作显示。
//
// ID 与文件名的关系（2026-09-27 实测，易踩）：
//   - 多数物品的脚本文件名主干就是 ID，例如 stackable/10362001/10362480.stk → 10362480；
//   - 但命名脚本不是：stackable/gold.stk 的 ID 是 0（[name] = <13::name_0> = "Gold"）。
//     这类必须由调用方给出 ID→路径映射（见 ItemNameRefsByPaths）。
//   ItemIDFromScriptName 只实现"文件名主干是十进制"这一种，不要把它当成 ID 的唯一定义。
//
// [name] 的值是一个本地化引用 "<N::key>"：N 是 list/n_string.lst 的表序号，
// key 形如 name_<id>。本机实测：
//
//	N=3  → String/equipment.uv.str  装备 100401592 用 <3::name_100401592>
//	N=13 → String/Stackable.uv.str  堆叠物 10362480 用 <13::name_10362480>
//
// 即 ID → [name] 引用点 → n_string.lst → 文本表 → 文本。
//
// ⚠️ key 并不总是 name_<本物品ID>。采样 400 件里 6 件不是（约 1.5%），实测三种形态：
//
//	name_<id>       常见形态（343/400）
//	name_<别的id>   装扮/继承复用别的 ID 的名字，例如 500310025 → <3::name_100312235>
//	chn_name_<id>   平行键体系，见下
//
// chn_ 前缀（本机实测 equipment 表 43,159 条 chn_name_、Stackable 表 148,386 条
// chn_*，其中 50,806 条空值）：像 name_/explain_ 一样成对存在（chn_name_ /
// chn_explain_），且 400330331 这类 ID **只有 chn_name_、没有 name_**。
// 因此"key == name_<ID>"不是不变量，只当自洽信号用（见 KeyMatchesID）；
// 取文本必须按引用里给出的键，不能自己拼 name_<id>。
//
// 由 ID 到文本要经过文件表 → 脚本 → 引用点 → 表 → 文本，任一环缺失都返回
// 明确的 found=false，而不是回落到"用名字模糊匹配"。

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// StringTableListPath 是"表序号 → 文本表路径"清单，38 类 String/<类别>.uv.str。
const StringTableListPath = "list/n_string.lst"

// nameLabel 是物品脚本里名字段的标签。
const nameLabel = "[name]"

// ItemScriptExtensions 是物品脚本的扩展名白名单。
//
// 只用"扩展名 + 纯十进制文件名主干"来认物品，避免把同号段的数字文件名
// （地图 .map、副本 .dgn 等）误当物品。
var ItemScriptExtensions = []string{".stk", ".equ"}

// LocalizedRef 是脚本里的一个 "<N::key>" 本地化引用。
type LocalizedRef struct {
	Table int    `json:"table"` // list/n_string.lst 的表序号
	Key   string `json:"key"`   // 形如 name_<id>
	Raw   string `json:"raw"`   // 原样，例如 "<13::name_10362480>"
}

// ParseLocalizedRef 解析 "<N::key>"。不是引用时返回 ok=false，调用方应把
// 入参当作普通文本。
func ParseLocalizedRef(s string) (LocalizedRef, bool) {
	t := strings.TrimSpace(s)
	if len(t) < 5 || t[0] != '<' || t[len(t)-1] != '>' {
		return LocalizedRef{}, false
	}
	body := t[1 : len(t)-1]
	i := strings.Index(body, "::")
	if i <= 0 {
		return LocalizedRef{}, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(body[:i]))
	if err != nil {
		return LocalizedRef{}, false
	}
	key := strings.TrimSpace(body[i+2:])
	if key == "" {
		return LocalizedRef{}, false
	}
	return LocalizedRef{Table: n, Key: key, Raw: t}, true
}

// NameRef 是一个物品 [name] 段的引用点。
type NameRef struct {
	// ID 是按 ID 定位时填的十进制物品 ID；按路径定位时为 0。
	ID uint32 `json:"id,omitempty"`
	// Path 是脚本在归档内的路径。
	Path string `json:"path"`
	// LabelIdx / ValueIdx 是 [name] 标签与取值 token 在脚本里的序号。
	LabelIdx int `json:"label_index"`
	ValueIdx int `json:"value_index"`
	// Token 是取值单元本身（Type=8 的本地化键、Type=3/6 的文本）。
	Token Token `json:"token"`
	// Raw 是取值单元解析出的字符串，例如 "<13::name_10362480>"。
	Raw string `json:"raw"`
	// Ref 是解析后的引用；IsRef=false 时 Ref 无意义。
	Ref   LocalizedRef `json:"ref"`
	IsRef bool         `json:"is_ref"`
	// KeyMatchesID 表示引用的键恰好是 name_<ID>。实测约 1.5% 的物品不是
	// （装扮复用别的 ID 的名字、或使用 chn_name_ 平行键），所以它只是
	// "引用点是否仍按最常见的约定指向"的自洽信号，不是硬不变量。
	KeyMatchesID bool `json:"key_matches_id"`
}

// ItemNameRef 按脚本路径定位 [name] 引用点。
//
// found=false 表示脚本里没有 [name] 段（不是所有条目的名字都放在这里）。
func (a *Archive) ItemNameRef(scriptPath string) (NameRef, bool, error) {
	if a == nil {
		return NameRef{}, false, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	tokens, err := a.Tokens(scriptPath)
	if err != nil {
		return NameRef{}, false, err
	}
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].Text != nameLabel {
			continue
		}
		v := tokens[i+1]
		raw := v.Text
		if raw == "" {
			raw = v.Reference
		}
		if raw == "" && v.Type != 0 && v.Type != 2 {
			raw = a.ResolveString(int(v.Value))
			if err := a.poolError(); err != nil {
				return NameRef{}, false, err
			}
		}
		ref, isRef := ParseLocalizedRef(raw)
		return NameRef{
			Path: scriptPath, LabelIdx: i, ValueIdx: i + 1,
			Token: v, Raw: raw, Ref: ref, IsRef: isRef,
		}, true, nil
	}
	return NameRef{}, false, nil
}

// ItemNameRefByID 按物品 ID 定位 [name] 引用点。
func (a *Archive) ItemNameRefByID(id uint32) (NameRef, bool, error) {
	m, err := a.ItemNameRefsByIDs([]uint32{id})
	if err != nil {
		return NameRef{}, false, err
	}
	r, ok := m[id]
	return r, ok, nil
}

// ItemNameRefsByIDs 用"文件名主干即 ID"这一约定定位 [name] 引用点。
//
// 只覆盖文件名是 <id>.<ext> 的脚本。命名脚本（如 stackable/gold.stk，其 ID 是 0）
// 不在其中 —— 那类调用方有 ID→路径映射时请用 ItemNameRefsByPaths。
//
// 单次扫描是刻意的：归档有 565 万条文件，逐 ID 扫描会把批量调用变成 O(n·m)。
// 返回的 map 只含真的定位到引用点的 ID；脚本缺失或没有 [name] 段的 ID 不出现。
func (a *Archive) ItemNameRefsByIDs(ids []uint32) (map[uint32]NameRef, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	if len(ids) == 0 {
		return map[uint32]NameRef{}, nil
	}
	want := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	paths := make(map[uint32]string, len(ids))
	for i := 0; i < a.FileCount(); i++ {
		f := a.fileAt(i)
		id, ok := ItemIDFromScriptName(f.Name)
		if !ok {
			continue
		}
		if _, need := want[id]; !need {
			continue
		}
		if _, dup := paths[id]; dup {
			continue // 同名文件先出现的优先
		}
		paths[id] = f.ArchivePath
	}
	return a.ItemNameRefsByPaths(paths)
}

// ItemNameRefsByPaths 对给定的 ID→脚本路径映射逐个定位 [name] 引用点。
//
// 这是 ID 权威性的最完整形态：ID 由调用方（物品目录/items.index.json）给出，
// 路径只用来找脚本，引用的键与文本都不参与定位。
func (a *Archive) ItemNameRefsByPaths(paths map[uint32]string) (map[uint32]NameRef, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	out := make(map[uint32]NameRef, len(paths))
	for id, p := range paths {
		ref, ok, err := a.ItemNameRef(p)
		if err != nil {
			return out, err
		}
		if !ok {
			continue
		}
		ref.ID = id
		ref.KeyMatchesID = ref.IsRef && ref.Ref.Key == "name_"+strconv.FormatUint(uint64(id), 10)
		out[id] = ref
	}
	return out, nil
}

// ItemIDFromScriptName 从脚本文件名取出物品 ID —— 仅覆盖"文件名主干即 ID"这一约定。
//
// 规则：主干的每一个字节都是 ASCII 十进制数字，且扩展名在 ItemScriptExtensions
// 白名单里。ID 是权威标识，因此这里不参考文件路径里的任何文本。
//
// 命名脚本（stackable/gold.stk，ID 为 0）返回 ok=false：它们的 ID 来自物品目录，
// 不要用文件名猜。
func ItemIDFromScriptName(name string) (uint32, bool) {
	ext := strings.ToLower(path.Ext(name))
	if !itemScriptExtAllowed(ext) {
		return 0, false
	}
	stem := name[:len(name)-len(ext)]
	if stem == "" {
		return 0, false
	}
	for i := 0; i < len(stem); i++ {
		if stem[i] < '0' || stem[i] > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseUint(stem, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func itemScriptExtAllowed(ext string) bool {
	for _, want := range ItemScriptExtensions {
		if ext == want {
			return true
		}
	}
	return false
}

// StringTables 读取 list/n_string.lst，返回"表序号 → 表路径"。
func (a *Archive) StringTables() (map[int]string, error) {
	if a == nil {
		return nil, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	tokens, err := a.Tokens(StringTableListPath)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(tokens)/2)
	pending := -1
	for _, t := range tokens {
		if t.Text != "" {
			if pending >= 0 {
				out[pending] = normalizePath(t.Text)
				pending = -1
			}
			continue
		}
		pending = int(t.Value)
	}
	return out, nil
}

// LocalizedText 按引用取出文本表里的文本。
//
// found=false 表示表序号不在清单里，或表里没有这个键 —— 两种都不回落到猜。
// 文本内容取决于当前客户端（原版英文 / 汉化中文），因此只可用于显示。
func (a *Archive) LocalizedText(ref LocalizedRef) (string, bool, error) {
	if a == nil {
		return "", false, fmt.Errorf("%w: archive is nil", ErrInvalidArchive)
	}
	tables, err := a.StringTables()
	if err != nil {
		return "", false, err
	}
	tablePath, ok := tables[ref.Table]
	if !ok {
		return "", false, nil
	}
	text, err := a.ReadText(tablePath)
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		i := strings.Index(line, ">")
		if i <= 0 {
			continue
		}
		if strings.TrimSpace(line[:i]) == ref.Key {
			return line[i+1:], true, nil
		}
	}
	return "", false, nil
}

// LocalizedNameByID 是 ID → [name] 引用点 → 文本表的完整一趟。
//
// 依赖"文件名主干即 ID"这一约定，因此不覆盖命名脚本；那类请先用目录解析出
// 路径再调 LocalizedNameByPath。
//
// 注意返回的是当前客户端里的文本；它因汉化与否而不同，不要拿它做物品识别。
// 第二个返回值表示这一趟是否走通（脚本/段/引用/表/键都命中）。
func (a *Archive) LocalizedNameByID(id uint32) (string, bool, error) {
	ref, ok, err := a.ItemNameRefByID(id)
	if err != nil || !ok {
		return "", false, err
	}
	return a.localizedTextOf(ref)
}

// LocalizedNameByPath 按脚本路径取 [name] 的文本（不经 ID 约定，覆盖命名脚本）。
func (a *Archive) LocalizedNameByPath(scriptPath string) (string, bool, error) {
	ref, ok, err := a.ItemNameRef(scriptPath)
	if err != nil || !ok {
		return "", false, err
	}
	return a.localizedTextOf(ref)
}

func (a *Archive) localizedTextOf(ref NameRef) (string, bool, error) {
	if !ref.IsRef {
		// [name] 直接是文本（少见），原样返回。
		return ref.Raw, ref.Raw != "", nil
	}
	return a.LocalizedText(ref.Ref)
}
