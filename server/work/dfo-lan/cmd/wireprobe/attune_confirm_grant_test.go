package main

import (
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// 2406 的**发放**那一半：弹窗能关只是前半，业主实机（2026-10-12 14:36 顶概率进
// maze 1）确认点 OK 之后地上/背包/邮件都没有那件 ⇒ 客户端不本地发，服务端必须自己发。
//
// 发放方式按业主定调：**不是直接入背包**，而是并进本场下一次怪物死亡掉落帧
// （noti 38）落地，拾取走已验的 CMD43 —— 顺序在实机是成立的
// （15:36:14 确认 → 15:36:32 爆掉落 → 15:36:33 起连续拾取）。

const attunementGenerated = "../../configs/attunement-rewards.generated.json"

func attuneConfirmSession(t *testing.T, run string, dungeonID, maze uint32) *worldSession {
	t.Helper()
	a, err := loot.LoadAttunementRewards(attunementGenerated)
	if err != nil {
		t.Fatal(err)
	}
	d := &dungeon.Session{RunID: run}
	d.Definition.ID = dungeonID
	d.Maze.Index = byte(maze)
	return &worldSession{
		loot:          &loot.Service{Attunement: a},
		activeDungeon: d,
		// 进本那一帧已经报过本场档（史诗砝码 100005068 的奖单就是 epic）：
		// 球色只在**后来加入的件更高**时才抬，所以夹具也得从「已经报过 44」开始。
		borderSphereGrade: 44,
		// 先把 drops 摆成「本场已经有会话」，ensureDropSession 就会直接复用（它的
		// 创建路径要整套目录/存档，不该由这条测试承担）。
		drops: loot.NewSession(catalog.LootCatalog{}, loot.Tables{}, loot.Rules{}, nil, run, 1, 1, 1),
	}
}

func attuneBody(part, set uint32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:], part)
	binary.LittleEndian.PutUint32(b[4:], set)
	return b
}

// 三个实机数据点逐一验到「记下了哪一件载体」。
func TestAttuneConfirmArmsTheSelectedCarrier(t *testing.T) {
	for _, c := range []struct {
		name        string
		part, set   uint32
		wantCarrier uint32
	}{
		{"首饰 + 黄金乡（14:36）", 2, 2, 10410316},
		{"首饰 + Dragon Arena（14:41）", 2, 3, 10410317},
		{"防具 + 黄金乡（15:36）", 1, 2, 10410340},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := attuneConfirmSession(t, "run-"+c.name, 100005068, 1)
			plan := w.answerAttuneConfirm(attuneBody(c.part, c.set))
			if len(plan) != 1 || plan[0].ID != 2406 || plan[0].Kind != 1 {
				t.Fatalf("应答 = %+v，必须仍有关窗那一帧", plan)
			}
			got := w.drops.PendingDrop()
			if len(got) != 1 || got[0].Template != c.wantCarrier || got[0].Amount != 1 {
				t.Fatalf("pending = %v, want 载体 %d×1", got, c.wantCarrier)
			}
		})
	}
}

// 武器档（part=0）：客户端不给选、源在第二层也不给概率，所以服务端按本角色筛完
// 等概率取一件（服主口径，见 loot.AttuneWeaponChoice）。这一条钉的是**接线**：
// 职业文本与转职索引真的从角色存档传到了判别器，掉出来的必定是本职业用得了的那把。
func TestAttuneConfirmPaysAWeaponTheCharacterCanUse(t *testing.T) {
	gun := uint32(104000916)
	for _, c := range []struct {
		name      string
		prof      byte
		job       string
		forbidden uint32
	}{
		{name: "鬼剑士", prof: 0, job: "[swordman]", forbidden: gun},
		{name: "神枪手", prof: 1, job: "[gunner]", forbidden: 101001152},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := attuneConfirmSession(t, "run-weapon-"+c.name, 100005068, 1)
			w.professions = catalog.Characters{Professions: map[byte]catalog.Profession{
				c.prof: {ID: c.prof, Job: c.job}}}
			w.role = database.Character{Profession: c.prof, State: json.RawMessage(`{"advancement":1}`)}
			w.loot.Equipment = weaponGearCatalog(t)
			w.loot.InstantlyOpenBoxes = weaponBoxesSource{}

			plan := w.answerAttuneConfirm(attuneBody(attuneHiddenWeaponPart, 0))
			if len(plan) != 1 || plan[0].ID != 2406 {
				t.Fatalf("关窗那一帧必须照回：%+v", plan)
			}
			got := w.drops.PendingDrop()
			if len(got) != 1 {
				t.Fatalf("武器档没有记下一件：%v", got)
			}
			if got[0].Template == c.forbidden {
				t.Fatalf("%s 拿到了别的职业的武器 %d", c.name, got[0].Template)
			}
			if !weaponJobAllows(w.loot.Equipment, c.job, got[0].Template) {
				t.Fatalf("%s 掉出 %d，本职业用不了", c.name, got[0].Template)
			}
			// 记的必须是**成品**而不是载体：那 42 件权重全 0，而 0 在通用选择器里按 1 算，
			// 直接展开会变成 42 件等概率、约 88% 掉到别的职业的武器上。
			if got[0].Template == 10409487 {
				t.Fatal("武器档记下了载体，展开会走 42 件等概率")
			}
		})
	}
}

// 源里没有的那一格（第 13 套 / 太初档没有防具）不记，也不回退到 key 0：
// 回退会让「选的套装」和「脚下掉的那件」不是同一件，而那正是这个功能在修的东西。
func TestAttuneConfirmIgnoresUndeclaredCells(t *testing.T) {
	for _, c := range []struct{ part, set uint32 }{{2, 12}, {1, 0}} {
		w := attuneConfirmSession(t, "run-undeclared", 100005068, 2)
		w.answerAttuneConfirm(attuneBody(c.part, c.set))
		if left := w.drops.PendingDrop(); len(left) != 0 {
			t.Fatalf("part=%d set=%d 在 maze 2 没有这一格，却记下了 %v", c.part, c.set, left)
		}
	}
}

// 同一场改主意（弹窗写着 Reselecting will reset…）以最后一次为准，而且只掉一件。
// 「已经跟着掉落帧出去就不许再记第二件」这一半由 loot 侧的两条测试背书：
// internal/loot/session_pending_drop_test.go 钉住发布成功后 pending 被清空，
// 而这里的门禁读的就是 pending 是否为空。
func TestAttuneConfirmRereselectReplacesUntilItDrops(t *testing.T) {
	w := attuneConfirmSession(t, "run-reselect", 100005068, 1)
	w.answerAttuneConfirm(attuneBody(2, 2))
	w.answerAttuneConfirm(attuneBody(3, 5))
	if got := w.drops.PendingDrop(); len(got) != 1 || got[0].Template != 10410355 {
		t.Fatalf("改主意后 pending = %v，want 只剩特殊装备那一件 10410355", got)
	}
}

// 不在副本上下文 / 没有调律表时，关窗那一帧照回，发放那一格不动。
func TestAttuneConfirmOutsideAnAttunementRunStillClosesTheWindow(t *testing.T) {
	w := &worldSession{}
	plan := w.answerAttuneConfirm(attuneBody(2, 2))
	if len(plan) != 1 || plan[0].ID != 2406 {
		t.Fatalf("应答 = %+v", plan)
	}
	if left := pendingAttuneHiddenDrop(w); len(left) != 0 {
		t.Fatalf("不该记下任何东西：%v", left)
	}
}

// 球色报的是**本场最高的那一件**（业主 2026-10-12 口径：本场有太初级物品 ⇒ 球色到太初；
// 对照组＝maze 0 那两场，史诗奖单 ⇒ 史诗球 + 史诗装备）。进本那一帧只看得见当时掷出的
// 奖单，而「命运的凝械」是进本**之后**才选的 —— 实机 `…_20261012_164002_…`：
// 进本帧 `grade=epic(44) -> EpicDrop(42)`，16:41:10 才记下 `tier=primeval` 那一件
// ⇒ 太初的货、史诗的球。修法＝确认时把球抬到这一件的档位，贴在关窗应答后面一批发出。
func TestAttuneConfirmRaisesTheSphereToTheItemThatJoined(t *testing.T) {
	w := attuneConfirmSession(t, "run-sphere", 100005068, 2)
	plan := w.answerAttuneConfirm(attuneBody(2, 2))
	if len(plan) != 2 {
		t.Fatalf("应答 = %+v，want 关窗那一帧 + 抬球色那一帧", plan)
	}
	if plan[0].ID != 2406 || plan[0].Kind != 1 {
		t.Fatalf("第 1 帧必须还是关窗应答：%+v", plan[0])
	}
	if plan[1].ID != 2756 || plan[1].Kind != 0 {
		t.Fatalf("第 2 帧必须是 noti 2756（掉落演出格 + 本场最高品质）：%+v", plan[1])
	}
	// 这一帧的两个值各有去处（取证见 protocol.BorderRewardInfo）：第 1 格是**演出格**
	// （40..43，太初 = 43），第 2 格是 `getHellDungeonItemMaxRarity` 读的**本场档位**
	// （40..45，太初 = 45 —— 副本里那颗球的阶段分支读的是它），第 3 格是常量下界 40。
	for i, want := range []uint32{43, 45, 40} {
		if got := binary.LittleEndian.Uint32(plan[1].Payload[i*4:]); got != want {
			t.Fatalf("位移 %d = %d, want %d", i, got, want)
		}
	}
	if w.borderSphereGrade != 45 {
		t.Fatalf("本场球色档记成 %d, want 45", w.borderSphereGrade)
	}
	// 已经报过太初，同场再确认一次（改主意）不该再把同一帧发一遍。
	if again := w.answerAttuneConfirm(attuneBody(2, 3)); len(again) != 1 || again[0].ID != 2406 {
		t.Fatalf("抬过一次就不该重发演出帧：%+v", again)
	}
}

// 后来的件与本场档**相等或更低**时不动球色：maze 1 的隐藏表就是史诗档，与史诗砝码的
// 本场档相同 ⇒ 只回关窗那一帧（三条实机数据点的测试仍是那个形状）。
func TestAttuneConfirmOnTheSameTierDoesNotResendTheSphere(t *testing.T) {
	w := attuneConfirmSession(t, "run-same-tier", 100005068, 1)
	if plan := w.answerAttuneConfirm(attuneBody(2, 2)); len(plan) != 1 || plan[0].ID != 2406 {
		t.Fatalf("同档不该多发一帧：%+v", plan)
	}
	if w.borderSphereGrade != 44 {
		t.Fatalf("球色档被改动：%d", w.borderSphereGrade)
	}
}

// ---- 武器档用的那套手搭目录（与 internal/loot/attune_weapon_test.go 同形）----

// weaponJobAllows 用**同一把判据**（inventory.UsableByJob）核对掉出来的那件，
// 免得测试自己再写一套「谁能用」而和被测代码判得不一致。
func weaponJobAllows(eq *inventory.EquipmentCatalog, job string, template uint32) bool {
	def, err := eq.Definition(template)
	if err != nil {
		return false
	}
	return inventory.UsableByJob(def.Fields, job, 1) == nil
}

func weaponGearCatalog(t *testing.T) *inventory.EquipmentCatalog {
	t.Helper()
	var rows []inventory.EquipmentDefinition
	add := func(id uint32, jobs ...string) {
		fields := map[string][]pvf.Token{"[usable job]": {}}
		for _, j := range jobs {
			fields["[usable job]"] = append(fields["[usable job]"], pvf.Token{Type: 6, Text: j})
		}
		rows = append(rows, inventory.EquipmentDefinition{ID: id,
			Path:   "equipment/weapon/" + strconv.FormatUint(uint64(id), 10) + ".equ",
			SHA256: strings.Repeat("0", 63) + strconv.FormatUint(uint64(id%10), 10),
			Fields: fields})
	}
	add(101001152, "[swordman]", "[knight]")
	add(101011318, "[swordman]", "[knight]")
	add(101040830, "[swordman]")
	add(104000916, "[gunner]")
	add(104010899, "[gunner]")
	add(109999999, "[all]")
	sum := strings.Repeat("c", 64)
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: rows}, sum)
	if err != nil {
		t.Fatal(err)
	}
	return eq
}

// weaponBoxesSource 是源里那两层：载体两条各 500000（真权重），下层组全 0 权重。
type weaponBoxesSource struct{}

func (weaponBoxesSource) RewardBox(template uint32) (loot.RewardBox, bool) {
	var group []loot.RewardBoxCandidate
	for _, id := range []uint32{101001152, 101011318, 101040830, 104000916, 104010899, 109999999} {
		group = append(group, loot.RewardBoxCandidate{Template: id, Weight: 0, Count: 1})
	}
	switch template {
	case 10409487:
		return loot.RewardBox{Pools: []loot.RewardBoxPool{{Draws: 1, Candidates: []loot.RewardBoxCandidate{
			{Template: 10409461, Weight: 500000, Count: 1},
			{Template: 10409462, Weight: 500000, Count: 1},
		}}}}, true
	case 10409461, 10409462:
		return loot.RewardBox{Pools: []loot.RewardBoxPool{{Draws: 1, Candidates: group}}}, true
	}
	return loot.RewardBox{}, false
}

func (weaponBoxesSource) Item(template uint32) bool {
	return template == 101001152 || template == 101011318 || template == 101040830 ||
		template == 104000916 || template == 104010899 || template == 109999999
}

func (s weaponBoxesSource) Container(template uint32) bool { return !s.Item(template) }

// 神兵认主（源表 `[effect index] 305`）走的是 CTP 那条线，不是 2406 那一格：奖单在
// **进本预掷**时就定了，所以职业筛必须发生在预掷之后、冻结奖单之前。这两条钉住那次接线：
// 305 不许带着载体下去，301 不许被改（那行按源就是随机武器，筛了反而偏离源）。
func TestGuaranteedWeaponRowPaysAFinishedWeapon(t *testing.T) {
	for _, c := range []struct {
		name, job string
		prof      byte
	}{
		{"鬼剑士", "[swordman]", 0},
		{"神枪手", "[gunner]", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := attuneConfirmSession(t, "run-effect305-"+c.name, 100005068, 0)
			w.professions = catalog.Characters{Professions: map[byte]catalog.Profession{
				c.prof: {ID: c.prof, Job: c.job}}}
			w.role = database.Character{Profession: c.prof, State: json.RawMessage(`{"advancement":1}`)}
			w.loot.Equipment = weaponGearCatalog(t)
			w.loot.InstantlyOpenBoxes = weaponBoxesSource{}

			tiers := loot.RunTiers{Oath: 44, OathLabel: "epic", OathEffect: attuneWeaponGuaranteedEffect,
				OathTemplate: 10409487, OathAmount: 1}
			w.resolveAttuneGuaranteedWeapon(&tiers)
			if tiers.OathTemplate == 10409487 {
				t.Fatal("305 仍是载体：统一展开会把 42 件等概率，约 88% 发到别的职业的武器")
			}
			if !weaponJobAllows(w.loot.Equipment, c.job, tiers.OathTemplate) {
				t.Fatalf("%s 掉出 %d，本职业用不了", c.name, tiers.OathTemplate)
			}
			if tiers.OathAmount != 1 || tiers.OathEffect != attuneWeaponGuaranteedEffect {
				t.Fatalf("换成品不许动别的格: %+v", tiers)
			}

			other := loot.RunTiers{Oath: 44, OathLabel: "epic", OathEffect: 301, OathTemplate: 10409467, OathAmount: 1}
			w.resolveAttuneGuaranteedWeapon(&other)
			if other.OathTemplate != 10409467 {
				t.Fatalf("301「祝福气息的武器」被改了（%d）：那一行按源不筛职业", other.OathTemplate)
			}
		})
	}
}
