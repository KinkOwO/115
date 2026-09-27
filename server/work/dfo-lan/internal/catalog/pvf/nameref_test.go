package pvf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// 这些测试固定的是"ID 才是权威标识"这条契约：
//
//   - 定位走文件名主干的十进制 ID，不参考任何文本；
//   - [name] 的文本随客户端（原版英文 / 汉化中文）变化，变化后按 ID 仍能定位；
//   - 引用点一旦不再指向 name_<id>，KeyMatchesID 转 false，工具的改写能被发现。

func TestParseLocalizedRef(t *testing.T) {
	cases := []struct {
		in    string
		table int
		key   string
		ok    bool
	}{
		{"<13::name_10362480>", 13, "name_10362480", true},
		{"<3::name_100401592>", 3, "name_100401592", true},
		{"< 3 :: name_x >", 3, "name_x", true},
		{"  <13::name_10362480>\t", 13, "name_10362480", true},
		{"Rare Weapon", 0, "", false},
		{"", 0, "", false},
		{"<13:name_x>", 0, "", false},
		{"<x::name_x>", 0, "", false},
		{"<13::>", 0, "", false},
		{"name_10362480", 0, "", false},
	}
	for _, c := range cases {
		got, ok := ParseLocalizedRef(c.in)
		if ok != c.ok {
			t.Fatalf("ParseLocalizedRef(%q) ok=%v, want %v", c.in, ok, c.ok)
		}
		if !ok {
			continue
		}
		if got.Table != c.table || got.Key != c.key {
			t.Fatalf("ParseLocalizedRef(%q) = {table:%d key:%q}, want {table:%d key:%q}",
				c.in, got.Table, got.Key, c.table, c.key)
		}
		if got.Raw == "" {
			t.Fatalf("ParseLocalizedRef(%q) dropped the raw text", c.in)
		}
	}
}

func TestItemIDFromScriptName(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
		ok   bool
	}{
		{"10362480.stk", 10362480, true},
		{"100401592.equ", 100401592, true},
		{"10362480.STK", 10362480, true},
		// 同号段的其它数字文件不是物品：扩展名不在白名单。
		{"100001016.map", 0, false},
		{"100005067.dgn", 0, false},
		{"n_string.lst", 0, false},
		// 名字不是纯十进制 ID。
		{"name_10362480.stk", 0, false},
		{"*.stk", 0, false},
		{".stk", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		id, ok := ItemIDFromScriptName(c.name)
		if ok != c.ok || id != c.id {
			t.Fatalf("ItemIDFromScriptName(%q) = (%d,%v), want (%d,%v)", c.name, id, ok, c.id, c.ok)
		}
	}
}

// nameRefFixture 造一个自包含归档：
//
//	stackable/10362001/10362480.stk  -> [name] = <13::name_10362480>
//	list/n_string.lst                -> 3 = String/equipment.uv.str, 13 = String/Stackable.uv.str
//	string/stackable.uv.str          -> 由 tableText 提供（模拟原版 / 汉化）
func nameRefFixture(t *testing.T, tableText string) *Archive {
	t.Helper()
	return nameRefFixtureWithRef(t, tableText, "<13::name_10362480>")
}

// nameRefFixtureWithRef 同 nameRefFixture，但可以指定 [name] 的引用字符串，
// 用来构造"引用点被改写"的脚本。
func nameRefFixtureWithRef(t *testing.T, tableText, refText string) *Archive {
	t.Helper()

	strA := []byte{}
	add := func(s string) int {
		off := len(strA)
		strA = append(strA, s...)
		strA = append(strA, 0)
		return off << 1 // 偶数 magic = ANSI 池，偏移 = magic>>1
	}
	labelMagic := add("[name]")
	refMagic := add(refText)
	equipPathMagic := add("String/equipment.uv.str")
	stkPathMagic := add("String/Stackable.uv.str")

	cell := func(typ byte, val int) []byte {
		b := make([]byte, 5)
		b[0] = typ
		binary.LittleEndian.PutUint32(b[1:], uint32(int32(val)))
		return b
	}

	item := append(cell(3, labelMagic), cell(8, refMagic)...)
	// n_string.lst：交替的 (序号, 路径) 单元。序号用真值
	// （3 = equipment、13 = Stackable，见 list/n_string.lst）。
	lst := append(cell(0, 3), cell(3, equipPathMagic)...)
	lst = append(lst, cell(0, 13)...)
	lst = append(lst, cell(3, stkPathMagic)...)
	table := encodeUTF16LETest(tableText)

	chunk := append(append(append([]byte{}, item...), lst...), table...)
	itemOff, lstOff, tableOff := 0, len(item), len(item)+len(lst)

	a := &Archive{
		format: FormatDFO20260901,
		strA:   strA,
		header: pvfHeader{fileCount: 3, groupCount: 1},
	}
	a.bodyOff = headerSize + 3*fileItemSize + 1*groupItemSize
	a.data = make([]byte, a.bodyOff)

	var buf bytes.Buffer
	z := zlib.NewWriter(&buf)
	if _, err := z.Write(chunk); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	encoded := buf.Bytes()
	decryptProtected("mAIn", encoded)
	a.data = append(a.data, encoded...)
	a.groups = []groupItem{{compressedSize: len(a.data) - a.bodyOff, originalSize: len(chunk)}}
	a.header.bodySize = len(a.data) - a.bodyOff
	binary.LittleEndian.PutUint32(a.header.plain[32:], uint32(a.header.bodySize))

	a.items = []fileItem{
		{chunkIndex: 0, dataOffset: itemOff, dataSize: len(item), dataType: 1},
		{chunkIndex: 0, dataOffset: lstOff, dataSize: len(lst), dataType: 1},
		{chunkIndex: 0, dataOffset: tableOff, dataSize: len(table), dataType: 3},
	}
	a.files = []File{
		{Index: 0, Name: "10362480.stk", ArchivePath: "stackable/10362001/10362480.stk", DataType: 1, Size: len(item)},
		{Index: 1, Name: "n_string.lst", ArchivePath: "list/n_string.lst", DataType: 1, Size: len(lst)},
		{Index: 2, Name: "Stackable.uv.str", ArchivePath: "string/stackable.uv.str", DataType: 3, Size: len(table)},
	}
	a.pathIdx = map[string]int{
		pathKey("stackable/10362001/10362480.stk"): 0,
		pathKey("list/n_string.lst"):               1,
		pathKey("string/stackable.uv.str"):         2,
	}
	return a
}

func encodeUTF16LETest(s string) []byte {
	out := []byte{}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func TestItemNameRefByIDResolvesLocalizedText(t *testing.T) {
	// 本机同形：英文表那条 "Rare Weapon (Actual Booster Issue X, Issued as Opened O)"。
	a := nameRefFixture(t, "name_10362480>Rare Weapon (Test)\r\n")

	ref, ok, err := a.ItemNameRefByID(10362480)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ID 10362480 应当定位到 [name] 引用点")
	}
	if ref.Path != "stackable/10362001/10362480.stk" {
		t.Fatalf("path=%q", ref.Path)
	}
	if !ref.IsRef {
		t.Fatal("[name] 应当是 <N::key> 引用")
	}
	if ref.Ref.Table != 13 || ref.Ref.Key != "name_10362480" {
		t.Fatalf("ref = {table:%d key:%q}, want {13, name_10362480}", ref.Ref.Table, ref.Ref.Key)
	}
	if !ref.KeyMatchesID {
		t.Fatal("引用的键恰好是 name_<id> 时 KeyMatchesID 应为 true")
	}
	text, ok, err := a.LocalizedText(ref.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || text != "Rare Weapon (Test)" {
		t.Fatalf("text = (%q,%v), want (\"Rare Weapon (Test)\",true)", text, ok)
	}
	name, ok, err := a.LocalizedNameByID(10362480)
	if err != nil || !ok || name != "Rare Weapon (Test)" {
		t.Fatalf("LocalizedNameByID = (%q,%v,%v)", name, ok, err)
	}
}

// TestItemNameRefIsIDKeyedNotNameKeyed 是本文件存在的理由：
// 同一件物品换成汉化文本后，文本变了，按 ID 定位必须仍然成立。
func TestItemNameRefIsIDKeyedNotNameKeyed(t *testing.T) {
	zh := nameRefFixture(t, "name_10362480>稀有武器（实际开出礼盒）\r\n")
	en := nameRefFixture(t, "name_10362480>Rare Weapon (Test)\r\n")

	zhName, ok, err := zh.LocalizedNameByID(10362480)
	if err != nil || !ok {
		t.Fatalf("汉化表下按 ID 定位失败：%v", err)
	}
	enName, ok, err := en.LocalizedNameByID(10362480)
	if err != nil || !ok {
		t.Fatalf("原版表下按 ID 定位失败：%v", err)
	}
	if zhName == enName {
		t.Fatal("两份表文本相同，用例没有区分力")
	}

	// 两个版本里 ID、路径、引用点必须完全一致 —— 只有文本不同。
	zhRef, _, _ := zh.ItemNameRefByID(10362480)
	enRef, _, _ := en.ItemNameRefByID(10362480)
	if zhRef.Path != enRef.Path || zhRef.Ref != enRef.Ref || zhRef.ValueIdx != enRef.ValueIdx {
		t.Fatalf("引用点随文本变化了：zh=%+v en=%+v", zhRef.Ref, enRef.Ref)
	}
	if !zhRef.KeyMatchesID || !enRef.KeyMatchesID {
		t.Fatal("两个版本的 KeyMatchesID 都应为 true")
	}
}

// TestItemNameRefDetectsRewrittenReference 覆盖"引用点被工具改写"的情形：
// 键不再是 name_<id> 时，按 ID 定位仍成功（ID 来自文件表），但 KeyMatchesID
// 必须转 false，改写才能被发现。
func TestItemNameRefDetectsRewrittenReference(t *testing.T) {
	a := nameRefFixtureWithRef(t, "name_10362480>Rare Weapon (Test)\r\n", "<13::name_99999999>")
	ref, ok, err := a.ItemNameRefByID(10362480)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("定位仍应成功：ID 由文件表决定，不依赖引用内容")
	}
	if ref.Ref.Key != "name_99999999" {
		t.Fatalf("key=%q，用例没构造出改写后的引用", ref.Ref.Key)
	}
	if ref.KeyMatchesID {
		t.Fatal("引用被改写后 KeyMatchesID 必须为 false")
	}
	// 表里没有这个键，取文本必须明确失败，而不是回落到猜。
	if _, ok, err := a.LocalizedText(ref.Ref); err != nil || ok {
		t.Fatalf("未知键应当 miss：ok=%v err=%v", ok, err)
	}
}

func TestItemNameRefsByIDsSkipsUnknownAndNonItems(t *testing.T) {
	a := nameRefFixture(t, "name_10362480>Rare Weapon (Test)\r\n")
	m, err := a.ItemNameRefsByIDs([]uint32{10362480, 999999})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("命中 %d 条，只应有 10362480", len(m))
	}
	if _, ok := m[10362480]; !ok {
		t.Fatal("缺 10362480")
	}
	if m[10362480].ID != 10362480 {
		t.Fatal("命中项应回填 ID")
	}
	empty, err := a.ItemNameRefsByIDs(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("空入参应返回空表且无错：%v %d", err, len(empty))
	}
}

func TestStringTablesParsesOrderedList(t *testing.T) {
	a := nameRefFixture(t, "name_10362480>x\r\n")
	tables, err := a.StringTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 {
		t.Fatalf("表数 = %d, want 2", len(tables))
	}
	if tables[3] != "String/equipment.uv.str" || tables[13] != "String/Stackable.uv.str" {
		t.Fatalf("表索引错位：%v", tables)
	}
}

// TestItemNameRefsByPathsCoversNamedScripts 固定"ID 与文件名无关"这条实测结论：
// stackable/gold.stk 的 ID 是 0，只能靠目录给的 ID→路径映射定位，
// 且此时引用的键（name_10362480）与 ID（0）不同，KeyMatchesID 必须为 false。
func TestItemNameRefsByPathsCoversNamedScripts(t *testing.T) {
	a := nameRefFixture(t, "name_10362480>Rare Weapon (Test)\r\n")

	// 文件名主干约定对命名脚本无能为力。
	if id, ok := ItemIDFromScriptName("gold.stk"); ok {
		t.Fatalf("命名脚本不该按文件名给出 ID，却得到 %d", id)
	}
	if _, ok, _ := a.ItemNameRefByID(0); ok {
		t.Fatal("ID 0 不该被文件名约定命中")
	}

	m, err := a.ItemNameRefsByPaths(map[uint32]string{0: "stackable/10362001/10362480.stk"})
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := m[0]
	if !ok {
		t.Fatal("按 ID→路径映射应当定位成功")
	}
	if ref.ID != 0 {
		t.Fatalf("回填的 ID = %d, want 0", ref.ID)
	}
	if ref.KeyMatchesID {
		t.Fatal("引用键是 name_10362480 而 ID 是 0，不该声明 KeyMatchesID")
	}
	// 路径版取名字应当走通，且不依赖 ID 约定。
	name, ok, err := a.LocalizedNameByPath("stackable/10362001/10362480.stk")
	if err != nil || !ok || name != "Rare Weapon (Test)" {
		t.Fatalf("LocalizedNameByPath = (%q,%v,%v)", name, ok, err)
	}
}
