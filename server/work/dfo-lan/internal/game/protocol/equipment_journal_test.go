package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// 实机样本：服主 2026-09-29 在装备库 4 个栏目各收藏一次 + 若干取消，共 8 条 CMD2264。
// 这里钉住 5 条有代表性的（含一次"取消到全零"），把线格式锁死。
// 字节按**空格分隔**写：与日志里的 `plain_hex` 一一对应，肉眼可核，也避免手抄漏位。
var equipmentFavoriteSamples = []struct {
	name     string
	bytes    string
	category byte
	slots    [3]uint32
}{
	// 下面 8 条的 hex 全部**逐字节取自** 2026-09-29 实机会话
	// runtime/roles_persist_..._013013_046940_next37/events.jsonl 里 CMD2264 的 plain_hex，
	// 未经手抄。期望值按 +17 起点解读，且都应能对回源表 `[part set index]`（16201..16213）
	// 或栏目内行号 —— 对不回源表就说明偏移又错了。
	{
		"套装 收藏#1",
		"23 fa d1 44 01 00 00 00 00 ab 44 7f 01 00 00 00 00 4b 3f 00 00 00 00 00 00 00 00 00 00 00 00 00",
		0, [3]uint32{16203, 0, 0},
	},
	{
		"套装 收藏#2",
		"23 fa d1 44 01 00 00 00 00 ab 44 7f 01 00 00 00 00 4b 3f 00 00 53 3f 00 00 00 00 00 00 00 00 00",
		0, [3]uint32{16203, 16211, 0},
	},
	{
		"套装 满三槽",
		"23 fa d1 44 01 00 00 00 c0 3a 47 19 01 00 00 00 00 49 3f 00 00 4b 3f 00 00 53 3f 00 00 00 00 00",
		0, [3]uint32{16201, 16203, 16211},
	},
	{
		"套装 取消一槽",
		"23 fa d1 44 01 00 00 00 c0 3a 47 19 01 00 00 00 00 49 3f 00 00 4b 3f 00 00 00 00 00 00 00 00 00",
		0, [3]uint32{16201, 16203, 0},
	},
	{
		"换栏目 类别3",
		"80 20 a9 59 01 00 00 00 b9 fd 7a 48 01 03 00 00 00 54 3f 00 00 00 00 00 00 00 00 00 00 00 00 00",
		3, [3]uint32{16212, 0, 0},
	},
	{
		"类别1 收藏行 1",
		"80 20 a9 59 01 00 00 00 b9 fd 7a 48 01 01 00 00 00 01 00 00 00 00 00 00 00 00 00 00 00 00 00 00",
		1, [3]uint32{1, 0, 0},
	},
	{
		"类别1 取消到全零",
		"80 20 a9 59 01 00 00 00 b9 fd 7a 48 01 01 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00",
		1, [3]uint32{0, 0, 0},
	},
	{
		"类别4",
		"80 20 a9 59 01 00 00 00 b9 fd 7a 48 01 04 00 00 00 4b 3f 00 00 00 00 00 00 00 00 00 00 00 00 00",
		4, [3]uint32{16203, 0, 0},
	},
}

func TestDecodeEquipmentFavoriteRequestLiveSamples(t *testing.T) {
	for _, s := range equipmentFavoriteSamples {
		b, e := hex.DecodeString(strings.ReplaceAll(s.bytes, " ", ""))
		if e != nil {
			t.Fatalf("%s: bad hex: %v", s.name, e)
		}
		if len(b) != EquipmentFavoriteRequestSize {
			t.Fatalf("%s: sample is %d bytes, want %d", s.name, len(b), EquipmentFavoriteRequestSize)
		}
		r, e := DecodeEquipmentFavoriteRequest(b)
		if e != nil {
			t.Fatalf("%s: %v", s.name, e)
		}
		if r.Category != s.category {
			t.Fatalf("%s: category = %d, want %d", s.name, r.Category, s.category)
		}
		if r.Slots != s.slots {
			t.Fatalf("%s: slots = %#v, want %#v", s.name, r.Slots, s.slots)
		}
		if r.Kind != 1 {
			t.Fatalf("%s: kind = %d, want 1", s.name, r.Kind)
		}
	}
}

func TestDecodeEquipmentFavoriteRequestRejects(t *testing.T) {
	// 长度必须精确 32
	if _, e := DecodeEquipmentFavoriteRequest(make([]byte, 31)); e == nil {
		t.Fatalf("31-byte body must be rejected")
	}
	if _, e := DecodeEquipmentFavoriteRequest(make([]byte, 33)); e == nil {
		t.Fatalf("33-byte body must be rejected")
	}
	// 类别越界（源里只有 0..4）—— 这一类必须硬拒：我们没有地方存它。
	bad := make([]byte, EquipmentFavoriteRequestSize)
	bad[12] = 1
	bad[13] = 5
	if _, e := DecodeEquipmentFavoriteRequest(bad); e == nil {
		t.Fatalf("category 5 must be rejected")
	}
	// Kind(+12) 是未定字段：放行但暴露给调用方，不在这里假拒绝（假拒绝会变成"点了没反应"）。
	bad[13] = 0
	bad[12] = 7
	got, e := DecodeEquipmentFavoriteRequest(bad)
	if e != nil {
		t.Fatalf("kind 7 should be decoded, not rejected: %v", e)
	}
	if got.Kind != 7 {
		t.Fatalf("kind = %d, want it passed through", got.Kind)
	}
}

// 空正文的帧会被服务端的 preparePackets 丢掉（`!90` 的教训），所以 2264 的应答必须非空。
func TestEquipmentFavoriteReplyIsNotEmpty(t *testing.T) {
	if len(EquipmentFavoriteReply()) == 0 {
		t.Fatalf("2264 reply must not be empty; empty bodies are dropped before send")
	}
	if got := EquipmentFavoriteReply(); !bytes.Equal(got, []byte{1}) {
		t.Fatalf("reply = %#v, want the generic success prefix {1}", got)
	}
}

func TestEquipmentJournalCreateReplySize(t *testing.T) {
	got := EquipmentJournalCreateReply(1, 0)
	if len(got) != EquipmentJournalCreateReplySize {
		t.Fatalf("2265 reply = %d bytes, want %d (handler reads exactly 6)",
			len(got), EquipmentJournalCreateReplySize)
	}
	// handler 只读 [4] 与 [5]；前 4 字节是保留位。
	if got[0] != 0 || got[1] != 0 || got[2] != 0 || got[3] != 0 || got[4] != 1 || got[5] != 0 {
		t.Fatalf("reply layout = %#v", got)
	}
}

func TestEquipmentJournalBodySizeAndTail(t *testing.T) {
	counts := map[uint32]uint32{
		100401592: 3, // 微光星蕴石
		100401599: 0, // 已登记但 0 份：必须保留
		100051285: 99,
	}
	favorites := map[uint32][]uint32{
		0: {0x3f4900, 0x3f4b00, 0x3f5300},
		3: {0x3f5400},
	}
	body, e := EquipmentJournalBody(counts, favorites)
	if e != nil {
		t.Fatalf("body: %v", e)
	}
	if len(body) != EquipmentJournalBodySize {
		t.Fatalf("body = %d bytes, want %d", len(body), EquipmentJournalBodySize)
	}
	// 表长 = 2048×8，尾 60 B
	if got := len(body) - EquipmentJournalTableSlots*8; got != EquipmentJournalCategoryCount*EquipmentJournalCategorySlots*4 {
		t.Fatalf("tail = %d bytes, want 60", got)
	}
	back, favs, e := DecodeEquipmentJournalBody(body)
	if e != nil {
		t.Fatalf("decode: %v", e)
	}
	if len(back) != len(counts) {
		t.Fatalf("decoded %d rows, want %d (count 0 rows must survive)", len(back), len(counts))
	}
	for k, v := range counts {
		if back[k] != v {
			t.Fatalf("template %d = %d, want %d", k, back[k], v)
		}
	}
	if _, ok := back[100401599]; !ok {
		t.Fatalf("registered-with-zero row 100401599 must be present")
	}
	if len(favs) != len(favorites) {
		t.Fatalf("decoded %d categories, want %d", len(favs), len(favorites))
	}
	for cat, want := range favorites {
		got := favs[cat]
		if len(got) != len(want) {
			t.Fatalf("category %d = %v, want %v", cat, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("category %d slot %d = %#x, want %#x", cat, i, got[i], want[i])
			}
		}
	}
}

func TestEquipmentJournalBodyRuleGuards(t *testing.T) {
	// 模板 0 不是合法键（它是"空行"哨兵）
	if _, e := EquipmentJournalBody(map[uint32]uint32{0: 5}, nil); e == nil {
		t.Fatalf("template 0 must be rejected")
	}
	// 超过 2048 个模板就装不下（不静默截断）
	tooMany := map[uint32]uint32{}
	for i := 0; i < EquipmentJournalTableSlots+1; i++ {
		tooMany[uint32(1000000+i)] = 1
	}
	if _, e := EquipmentJournalBody(tooMany, nil); e == nil {
		t.Fatalf("%d templates must not fit", len(tooMany))
	}
	// 每类最多编 3 槽：第 4 项被裁掉但不报错（存量里的第 4 槽由调用方记诊断）
	body, e := EquipmentJournalBody(nil, map[uint32][]uint32{0: {1, 2, 3, 4}})
	if e != nil {
		t.Fatalf("body: %v", e)
	}
	_, favs, e := DecodeEquipmentJournalBody(body)
	if e != nil {
		t.Fatalf("decode: %v", e)
	}
	if got := favs[0]; len(got) != 3 || got[2] != 3 {
		t.Fatalf("category 0 = %v, want the first three", got)
	}
	// 长度必须精确
	if _, _, e := DecodeEquipmentJournalBody(make([]byte, EquipmentJournalBodySize-1)); e == nil {
		t.Fatalf("short body must be rejected")
	}
}

// 客户端按 2048 行逐行读、模板 0 跳过（不是结束）；份数 0 的行仍有效。
// 这条用"全部 2048 槽都填满"的极端形状验证编码器不会越界或提前收尾。
func TestEquipmentJournalBodyFullTable(t *testing.T) {
	counts := map[uint32]uint32{}
	for i := 0; i < EquipmentJournalTableSlots; i++ {
		counts[uint32(100000000+i)] = uint32(i % 100)
	}
	body, e := EquipmentJournalBody(counts, nil)
	if e != nil {
		t.Fatalf("full table: %v", e)
	}
	back, _, e := DecodeEquipmentJournalBody(body)
	if e != nil {
		t.Fatalf("decode: %v", e)
	}
	if len(back) != EquipmentJournalTableSlots {
		t.Fatalf("decoded %d rows, want %d", len(back), EquipmentJournalTableSlots)
	}
}


// ---------------------------------------------------------------- CMD2259

// 实机 2259（装备库「制作 / 变换」）样本：2026-09-29 三次会话共 5 帧，去重后 3 条。
// hex 由脚本从 events.jsonl 的 plain_hex **直接生成**（不手抄），空格分隔便于肉眼核对。
// 期望槽号来自 configs/equipment-wear.full-candidate.json 的 slots（记录 i ↔ 槽 12+i），
// 期望模板必须是物品索引里的真实模板 —— 对不回源表就说明偏移又错了。
var equipmentCraftSamples = []struct {
	name      string
	bytes     string
	panel     uint32
	context   uint32
	action    byte
	pay       byte
	slots     []uint32
	templates []uint32
}{
	{
		"装备栏目 / 头肩（两次点击字节相同）",
		"a4 00 00 00 00 00 00 00 36 e8 ec 46 01 01 00 00 00 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 23 79 01 66 2f f8 05 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 00 01 00 00 00 00 00 00 00 00 00 00 00",
		164,
		0x46ece836, 1, 1, // 装备变换窗口 · [12]=1 变换 · [13]=1 付法 1
		[]uint32{15},
		[]uint32{100151142},
	},
	{
		"装备栏目 / 腰带",
		"a4 00 00 00 00 00 00 00 36 e8 ec 46 01 01 00 00 00 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 23 79 01 9a f2 f8 05 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 00 01 00 00 00 00 00 00 00 00 00 00 00",
		164,
		0x46ece836, 1, 1, // 装备变换窗口 · [12]=1 变换 · [13]=1 付法 1
		[]uint32{18},
		[]uint32{100201114},
	},
	{
		"武器栏目 / 巨剑",
		"c5 00 00 00 00 00 00 00 36 e8 ec 46 01 01 00 00 00 03 0c 00 13 9d 05 06 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 00 01 00 00 00 00 00 00 00 00 00 00 00",
		197,
		0x46ece836, 1, 1, // 装备变换窗口 · [12]=1 变换 · [13]=1 付法 1
		[]uint32{12},
		[]uint32{101031187},
	},
	{
		// 2026-09-29 13:51:39（会话 …829743）：**装备生成**（[12]=0）、玩家点付款方式 1（金币）。
		"装备栏目 / 耳环 r6（生成 · 付法 1 = 金币）",
		"00 00 00 00 00 00 00 00 f9 f2 5f 00 00 01 00 00 00 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 23 79 01 90 d8 fb 05 01 00 00 00 00 00 00 00 00 00 00 00 00",
		0,
		0x5ff2f9, 0, 1,
		[]uint32{25},
		[]uint32{100391056},
	},
	{
		// 2026-09-29 13:52:01（同一会话）：同样 [12]=0，但 **[13]=2** —— 玩家点的是
		// 巡礼之印（`10401346`）那支。这一帧就是"[13] 不是常量"的判据本身。
		"装备栏目 / 套装（生成 · 付法 2 = 巡礼之印）",
		"00 00 00 00 00 00 00 00 f9 f2 5f 00 00 02 00 00 00 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 23 79 01 82 48 fb 05 2e 00 00 ff ff ff ff 2e 00 00 ff ff ff ff 01 00 00 00 00 00 00 00 00 00 00 00 00",
		0,
		0x5ff2f9, 0, 2,
		[]uint32{23},
		[]uint32{100354178},
	},
}

func TestDecodeEquipmentCraftRequestLiveSamples(t *testing.T) {
	for _, s := range equipmentCraftSamples {
		b, e := hex.DecodeString(strings.ReplaceAll(s.bytes, " ", ""))
		if e != nil {
			t.Fatalf("%s: bad hex: %v", s.name, e)
		}
		r, e := DecodeEquipmentCraftRequest(b)
		if e != nil {
			t.Fatalf("%s: %v", s.name, e)
		}
		if r.Panel != s.panel {
			t.Fatalf("%s: panel = %d, want %d", s.name, r.Panel, s.panel)
		}
		// u32@8 是**来源窗口**而不是常量：装备变换 = 0x46ece836、装备生成 = 0x5ff2f9
		// （5 条样本全部符合）。
		if r.Context != s.context {
			t.Fatalf("%s: context = %#x, want %#x", s.name, r.Context, s.context)
		}
		// u8@12 = 动作（0 生成 / 1 变换）；u8@13 = **付款方式序号**（1 起）。
		// ⚠️ `[13]` 曾被当成常量 1（老注释写"实测恒 1/1"）—— 2026-09-29 13:52 那帧 = 2
		// 推翻了它，且玩家确认那次点的就是另一支付法（巡礼之印）。
		if r.Reserved != 0 || r.Action != s.action || r.PayOption != s.pay {
			t.Fatalf("%s: reserved/action/pay = %d/%d/%d, want 0/%d/%d",
				s.name, r.Reserved, r.Action, r.PayOption, s.action, s.pay)
		}
		slots, templates := r.Wanted()
		if len(slots) != len(s.slots) || len(templates) != len(s.templates) {
			t.Fatalf("%s: want %d selected, got slots=%v templates=%v",
				s.name, len(s.slots), slots, templates)
		}
		for i := range slots {
			if slots[i] != s.slots[i] || templates[i] != s.templates[i] {
				t.Fatalf("%s: #%d = slot %d template %d, want slot %d template %d",
					s.name, i, slots[i], templates[i], s.slots[i], s.templates[i])
			}
		}
		// 空槽的 3 字节字段必须正好是 0x2e（46）—— 偏移错一位这里就会炸。
		for i, en := range r.Entries {
			if en.Template == EquipmentCraftEmptyTemplate && en.Group != 0x2e {
				t.Fatalf("%s: empty entry #%d group = %d, want 46", s.name, i, en.Group)
			}
		}
	}
}

func TestEquipmentCraftLayoutAddsUp(t *testing.T) {
	// 布局必须精确铺满正文：17 + 14×7 + 13 = 128。改任何一个常量都要在这里对上。
	got := EquipmentCraftHeaderSize + EquipmentCraftEntryCount*EquipmentCraftEntrySize + EquipmentCraftTrailerSize
	if got != EquipmentCraftBodySize {
		t.Fatalf("craft layout adds up to %d, want %d", got, EquipmentCraftBodySize)
	}
	if EquipmentCraftSlotBase+EquipmentCraftEntryCount-1 != 25 {
		t.Fatalf("slot range ends at %d, want 25 ([earring])",
			EquipmentCraftSlotBase+EquipmentCraftEntryCount-1)
	}
}

func TestDecodeEquipmentCraftRequestRejects(t *testing.T) {
	if _, e := DecodeEquipmentCraftRequest(make([]byte, 127)); e == nil {
		t.Fatalf("127-byte body must be rejected")
	}
	if _, e := DecodeEquipmentCraftRequest(make([]byte, 129)); e == nil {
		t.Fatalf("129-byte body must be rejected")
	}
	// 全零正文 = 14 个"模板 0"的槽：不能报错，但也不能算被点选。
	r, e := DecodeEquipmentCraftRequest(make([]byte, EquipmentCraftBodySize))
	if e != nil {
		t.Fatalf("all-zero body: %v", e)
	}
	if slots, templates := r.Wanted(); len(slots) != 0 || len(templates) != 0 {
		t.Fatalf("all-zero body reported selected slots %v / templates %v", slots, templates)
	}
}

func TestEquipmentCraftReplyIsExactlySixBytes(t *testing.T) {
	// handler sub_145277D00 用 sub_146EA0BE0(&v12, 6) 精确读 6 字节：
	// u8@4 选窗口（非 0 → 3937；0 → 2145），u8@5 是它的子分支。
	// 正文 = 1 字节成功前缀 + 6 字节窗口指令。
	if EquipmentCraftPayloadSize != 6 || EquipmentCraftReplySize != 7 {
		t.Fatalf("craft sizes = %d/%d, want 6/7", EquipmentCraftPayloadSize, EquipmentCraftReplySize)
	}
	got := EquipmentCraftReply(1, 0)
	if len(got) != 7 {
		t.Fatalf("craft reply is %d bytes, want 7", len(got))
	}
	// body[0] 是收包分发器要吃掉的成功标志；少了它 handler 会读到越界并崩客户端。
	if got[0] != 1 {
		t.Fatalf("craft reply success prefix = %d, want 1", got[0])
	}
	if got[5] != 1 || got[6] != 0 {
		t.Fatalf("craft reply window/variant = %d/%d, want 1/0", got[5], got[6])
	}
	if got[1] != 0 || got[2] != 0 || got[3] != 0 || got[4] != 0 {
		t.Fatalf("craft reply reserved u32 must stay zero, got %v", got[1:5])
	}
}
