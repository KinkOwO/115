package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
)

// noti 2859 BOUNDARY_OF_ATTUNEMENT_REWARD —— 调律之边界的奖励通知。
//
// ## 这条包的形状（L0 证据）
//
// `opcodes.tsv`（dfocap 从进程内存取的名字表）给出名字与 `table_slot_va`；
// 派发表在 .data 的 BSS 里（运行时才填），所以改用**注册调用**的机器码形状反查：
//
//	lea  r8, <handler>
//	mov  edx, <packet id>
//	call sub_14599D5D0        ; S→C（noti）注册
//
// 用已知的 `2838 → sub_140656A00` 自检通过后，得到 **2859 → sub_1406B18D0**。
// 它的完整反编译（`runtime/ida-attunement/probe7-out.txt`）就是载荷的全部依据：
//
//	__int64 sub_1406B18D0()
//	{
//	  int v1, v2, v3;
//	  v1 = 0; v2 = 72; v3 = 72;      // 短包时的兜底
//	  sub_146EA0BE0(&v1, 12);         // 定长读 12 字节：v1=packet[0:4] v2=[4:8] v3=[8:12]
//	  *(_DWORD *)(sub_1406B1BD0() + 80) = v2;
//	  *(_DWORD *)(sub_1406B1BD0() + 84) = v3;
//	  *(_DWORD *)(sub_1406B1BD0() + 88) = v1;
//	  return sub_1406B1BD0();
//	}
//
// ⇒ **载荷 = 12 字节 = 3 × little-endian u32**；它们写进同一个 240 字节的
// 「调律模块」实例（`sub_1406B1520` 是它的 init，注册了 noti 2756/2859 与 cmd 2343/2406）。
// 模块 init 里的初值是 `+80 = 72`、`+84 = 72`、`+88 = -1`；
// **我们从来没发过这条包**，所以这三个字段一直是初值 —— 这就是「这块还没接线」。
//
// ## 三个 u32 各是什么：**未取证**
//
// 已排除的路径（见 next176 §8.2）：模块 vtable（24 槽位只有 3 个实函数，都不读这三个偏移）、
// getter `sub_1406B1BD0` 的 19 个调用方、以及 `sub_1406B19C0` / `sub_1406B1C50` /
// `sub_1406B1D10` / `sub_1406B2980`。按项目硬约束「禁止猜包」，**默认一个字节都不发**，
// 只提供这个诊断开关，由业主在实机上一组组试值、看珠子/演出怎么变。
//
// ## 与 noti 2838 的关系
//
// 2838（ENDKEEPER_OF_ORDER_REWARD，边界之守）是两个 u32 → 小深渊那条线；
// 2859 的**第一个 u32 落在同一个位移（+88）**，说明两者共享同一套「档位」语义的两个入口。
// 所以试值时优先按 2838 的两个数（引子 / 誓约）去猜前两位，第三位待定 ——
// 但**不因此默认发送**：写错档位会让客户端的珠子演出与实际掉落实质性不符，比不发更糟。

// attuneConfirmAckBody 是 CMD2406 下行应答的正文。
//
// 分发器在把包交给处理器**之前**替处理器读掉体首：sub_1459A1BB0 的 kind==1 分支里
// `v55[0]=0; sub_146EA09F0(v55,1)` 先读 1 字节 flag，且**只在 flag==0** 时再用
// `sub_146EA1920(&Time_1)` 读一个 u16；这两个值才是处理器
// `sub_1406B1860(unsigned char flag, unsigned short)` 的实参（包号本身走
// `*(u16*)(frame+1)` 去查 CMD 表 qword_14E6836F8）。所以正文至少 3 字节。
//
// flag 取 0：处理器会先关旧实例（sub_1406B4440 → 窗口 vtable+264，要求「尚未提交」
// 闩 win+1816==0），再无条件跑类级收尾 sub_146694510(mgr, 3843, 1)。非 0 只跑收尾。
// 0 两条路都覆盖。
var attuneConfirmAckBody = []byte{0, 0, 0}

// attuneHiddenWeaponPart 是隐藏表里**不给套装下拉**的那一格（部位 0 = 武器，
// 客户端下拉第 1 项，dstr 101038052 "Weapon"）。
//
// 它和另外三档不是一套机制，源给的事实：
//   - 载体 10409487(史诗) / 10409488(太初) 是 `[booster]`，`[etc] 1` 下挂两条各
//     500000 ⇒ 先掷「普通 / 传承」，落到的组 21377..21380 各 **42 件、权重全是 0**；
//   - 42 件按号段分 9 个职业系（101/102/104/106/108/109/114/116/117），同系 4~5 件
//     是不同武器子型，`[usable grow type]` 全空 ⇒ **职业筛不完，源第二层没有规则**；
//   - 客户端这一档**不给选**：实机 16:11:32 那条 `part=0 set=0`，弹窗里 Weapon
//     没有可选项（业主口径），所以第二个 u32 在武器格恒 0。
//
// 第二层因此是**服主 policy**（业主 2026-10-12「就按鬼剑士系 掉一把」）：按本角色
// 职业筛剩的那 5 件里等概率取一件，实现见 loot.AttuneWeaponChoice，消费点唯一
// （attuneWeaponTemplate），日志会念出筛剩几件。
//
// ⚠️ 这一层**不能交给通用的礼包展开**：全 0 权重不代表掷不出来，`poolWeight` 把 0 当
// 1 用 ⇒ 42 件等概率，约 88% 会掉到别的职业的武器上。必须先按职业筛。
const attuneHiddenWeaponPart = 0

// answerAttuneConfirm 回 CMD2406 BOUNDARY_OF_ATTUNEMENT_HIDDEN_SELECT。
//
// 玩家在大深渊/调律之边界里点「Confirm Selected Set/Part」的 OK，客户端的按钮回调
// sub_1406B30C0 **只发包、不关窗**（发完把 win+1816 置 1 防重复提交），而关掉这个窗
// 的唯一入口就是上面那个注册在 CMD 表上的处理器 —— 且只有**信封 kind==1** 的帧才会
// 查那张表。本仓此前对 2406 一帧都不回，实机表现就是「点 OK 弹窗不消失」
// （2026-10-12 会话 04:30:18 / 04:42:57 两条 id=2406，服务端零应答）。
//
// 上行正文那两个 u32 的语义已经用实机钉住（见 loot.HiddenSelection 的三点数据）：
// 第 1 个是部位大类、第 2 个是套装号。客户端自己**不落地任何一件**（业主实机：
// 弹窗能关了、地上/背包/邮件都没有），所以发放是服务端这一格。
func (w *worldSession) answerAttuneConfirm(plaintext []byte) []outboundPacket {
	plan := []outboundPacket{{"attune_confirm_ack", 1, 2406, attuneConfirmAckBody}}
	if len(plaintext) < 8 {
		log.Printf("attune confirm (cmd 2406): body=%s too short to carry (part, set)", hex.EncodeToString(plaintext))
		return plan
	}
	part := binary.LittleEndian.Uint32(plaintext[0:])
	set := binary.LittleEndian.Uint32(plaintext[4:])
	log.Printf("attune confirm (cmd 2406): body=%s part=%d set=%d", hex.EncodeToString(plaintext), part, set)
	frames, err := w.armAttuneHiddenDrop(part, set)
	if err != nil {
		log.Printf("attune confirm (cmd 2406): part=%d set=%d not paid: %v", part, set, err)
		return plan
	}
	// 那件比本场奖单高时，同一批里还要跟上一帧把球色抬上去（见 raiseBorderSphereGrade）。
	return append(plan, frames...)
}

// pendingAttuneHiddenDrop 返回本场还没搭上下一次掉落帧的那一件（没有则 nil）。
// 只读，供结算那一处的「确认了却没掉」告警和测试用。
func pendingAttuneHiddenDrop(w *worldSession) []loot.Award {
	if w == nil || w.drops == nil {
		return nil
	}
	return w.drops.PendingDrop()
}

// armAttuneHiddenDrop 把玩家所选那一格记进掉落会话，等本场下一帧掉落带出。
//
// 另外三档（防具/首饰/特殊装备）记的是**源给的载体**，展开与掷哪一件都交给掉落
// 那条统一的包装步骤；武器档要在这里就定成一件（见 loot.AttuneWeaponChoice），
// 因为那张组 42 件权重全 0，走通用选择器会把所有人都发成同一把。
//
// 返回的帧是「球色要抬到这一件的档位」那一帧（不需要抬就为空），调用方贴在关窗应答后面发。
func (w *worldSession) armAttuneHiddenDrop(part, set uint32) ([]outboundPacket, error) {
	if w.activeDungeon == nil || w.loot == nil || !w.loot.Attunement.Enabled() {
		return nil, fmt.Errorf("not inside a dungeon with an attunement table")
	}
	dungeon := w.activeDungeon.Definition.ID
	maze := uint32(w.activeDungeon.Maze.Index)
	sel, ok := w.loot.Attunement.HiddenSelection(dungeon, maze, part, set)
	if !ok {
		return nil, fmt.Errorf("hidden table has no cell for dungeon %d maze %d part %d set %d", dungeon, maze, part, set)
	}
	w.ensureDropSession()
	if w.drops == nil {
		return nil, fmt.Errorf("drop session unavailable")
	}
	// 同一场的第二次确认**只在该件还没落地时**顶掉前一次：弹窗自己写着「Reselecting
	// will reset the selected Part and Set」，所以改了主意要以最后一次为准；
	// 而它已经跟着掉落帧掉出去了，就绝不能再记第二件 —— 那才是一次确认掉两件的真风险。
	if w.attuneHiddenRun == w.activeDungeon.RunID && len(w.drops.PendingDrop()) == 0 {
		return nil, fmt.Errorf("already paid in this run")
	}
	award := loot.Award{Template: sel.Carrier, Amount: 1}
	if part == attuneHiddenWeaponPart {
		template, err := w.attuneWeaponTemplate(sel.Carrier)
		if err != nil {
			return nil, err
		}
		award = loot.Award{Template: template, Amount: 1}
	}
	w.drops.SetPendingDrop(award)
	w.attuneHiddenRun = w.activeDungeon.RunID
	// 念出来：这一件是玩家选的、我们记下了，但它要等下一帧掉落才落地；
	// 事后对不上账时这一行是唯一能说清「哪一格、哪一场、什么档」的记录。
	log.Printf("attune hidden selected: dungeon %d maze %d part %d set %d tier=%s pays %d, waiting for the next drop frame",
		dungeon, maze, part, set, sel.Tier, award.Template)
	return w.raiseBorderSphereGrade(loot.TierGradeValue(sel.Tier)), nil
}

// attuneWeaponGuaranteedEffect 是 epic.ctp `[additional drop table]` 里**神兵认主**那一行的
// `[effect index]`（源表的行号；只有 epic 那张表有这一行）。
const attuneWeaponGuaranteedEffect = 305

// resolveAttuneGuaranteedWeapon 把「神兵认主」掷中的那张**载体**就地换成本角色用得了的那一件。
//
// 这一行的载体与「命运的凝械」武器格是同一张（`10409487` 史诗 / `10409488` 太初），业主口径
// 这一行发的就是**本基础职业那一把**。不换成就交给统一展开：它落到组 21377..21380 是 42 件、
// 权重全 0，而 `poolWeight` 把 0 当 1 用 ⇒ 等于 42 件等概率、约 88% 发成别的职业的武器
// （2026-10-12 实机两场 305 都掉成圣职者的 `Soul Hunter` / `Shooting Star Rosary`）。
//
// 判据认**源表的行号**而不是物品号：301「祝福气息的武器」按源就是随机武器，那行不筛。
func (w *worldSession) resolveAttuneGuaranteedWeapon(tiers *loot.RunTiers) {
	if tiers.OathEffect != attuneWeaponGuaranteedEffect || tiers.OathTemplate == 0 {
		return
	}
	template, err := w.attuneWeaponTemplate(tiers.OathTemplate)
	if err != nil {
		// 走到这里最常见的是这个职业在那 42 件里筛剩 0 件。把玩家挡在副本外不是解决办法，
		// 所以保留载体交给通用展开，但「为什么没筛」必须当场念出来。
		log.Printf("attune guaranteed weapon (effect %d) kept carrier %d: %v",
			attuneWeaponGuaranteedEffect, tiers.OathTemplate, err)
		return
	}
	tiers.OathTemplate = template
}

// attuneWeaponTemplate 把武器格定成**这一角色的**那一件。
func (w *worldSession) attuneWeaponTemplate(carrier uint32) (uint32, error) {
	prof, ok := w.professions.Professions[w.role.Profession]
	if !ok {
		return 0, fmt.Errorf("weapon row needs the character profession (%d)", w.role.Profession)
	}
	var state struct {
		Advancement byte `json:"advancement"`
	}
	if err := json.Unmarshal(w.role.State, &state); err != nil {
		return 0, fmt.Errorf("weapon row needs the character advancement: %w", err)
	}
	src := w.loot.InstantlyOpenBoxes
	if src == nil {
		src = w.loot.RewardBoxes
	}
	seed, err := randomSeed()
	if err != nil {
		return 0, err
	}
	template, eligible, _, err := loot.AttuneWeaponChoice(seed, src, w.loot.Equipment, prof.Job, state.Advancement, carrier)
	if err != nil {
		return 0, err
	}
	// 候选数必须念出来：第二层是服主口径而不是源口径，哪天业主想知道「这行为什么是这把」,
	// 这一行是唯一说明「本职业筛剩几件、等概率取了一件」的现场证据。
	log.Printf("attune hidden weapon: job=%s advancement=%d carrier %d left %d usable candidate(s) %v, paying %d",
		prof.Job, state.Advancement, carrier, len(eligible), eligible, template)
	return template, nil
}

// attunementRewardPayloadSize 是 noti 2859 的载荷长度，**必须**是 12。
//
// 依据是解析器的定长读：`sub_146EA0BE0(&v1, 12)` 一次读满 12 字节，
// 和 2838 的 `(…, 8)` 是同一个成对形状（2838 的长度护栏在
// cmd/wireprobe/oath_info.go 的注释里记着：loc_146EA0C30 是故意的空写陷阱，
// 2026-09-27 用 8 字节的 2839 撞过）。长度不对一律拒绝，不给客户端留下错位的半包。
const attunementRewardPayloadSize = 12

// attunementRewardSpec 解析 `-attunement-reward` / `DFO_ATTUNEMENT_REWARD` 的值。
//
//	""              不发（默认，客户端保留模块里的 72/72/-1）
//	"@<路径>"        **每次进本重读**这个文件；空文件（或只有注释）= 不发
//	其它            直接当载荷：24 位十六进制（空格/逗号/0x 都可），或 3 个十进制 u32
//
// 「文件」这一档是为实机试值加的：三个字段的语义没定，得一组组试，而**换一个值不该
// 重启一次服务端**（重启要连带重启客户端，一轮三分钟）。用文件的话，改完存盘，
// 下一次进本就按新值发 —— 一轮之内可以把候选值挨个试完，日志逐次记下用了哪个值。
//
// 文件内容按「去掉 # 注释行后的第一行」读，写法与命令行一致（例如 `44,43,0`）。
// 让它容错到这种程度是刻意的：试值文件是给人手改的，写错必须马上在日志里说话。
func attunementRewardSpec(spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "@") {
		return parseAttunementReward(trimmed)
	}
	path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
	if path == "" {
		return nil, fmt.Errorf("empty payload file path after @")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		body = line
		break
	}
	if body == "" {
		// 空文件（或只有注释）= 这一段不发。这样「关掉注入」也不用重启。
		return nil, nil
	}
	payload, err := parseAttunementReward(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return payload, nil
}

// attunementRewardFile 把 -attunement-reward 的值里那条 @路径 抠出来（没有则空）。
// 只在日志与诊断里用，解析仍以 attunementRewardSpec 为准。
func attunementRewardFile(spec string) string {
	trimmed := strings.TrimSpace(spec)
	if !strings.HasPrefix(trimmed, "@") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
}

// parseAttunementReward 解析一份具体载荷（不含 @ 文件形式）。
//
// 两种写法，都是为了实机试值方便：
//
//	原始载荷：24 位十六进制（空格、逗号、0x 前缀都忽略）  → 直接当 12 字节
//	三个十进制：如 "44,43,0"                              → 各按 little-endian u32 打包
//
// 留空返回 nil，表示不发送（默认）。
func parseAttunementReward(spec string) ([]byte, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return nil, nil
	}
	compact := strings.NewReplacer(" ", "", "\t", "", ",", "", ":", "", "-", "").Replace(trimmed)
	compact = strings.TrimPrefix(strings.TrimPrefix(compact, "0x"), "0X")
	if len(compact) == attunementRewardPayloadSize*2 {
		raw, err := hex.DecodeString(compact)
		if err != nil {
			return nil, fmt.Errorf("not a hex payload: %w", err)
		}
		return raw, nil
	}
	parts := strings.Split(trimmed, ",")
	if len(parts) != attunementRewardPayloadSize/4 {
		return nil, fmt.Errorf("want %d hex bytes or %d comma separated u32 values, got %q",
			attunementRewardPayloadSize, attunementRewardPayloadSize/4, spec)
	}
	out := make([]byte, attunementRewardPayloadSize)
	for i, part := range parts {
		// 用带符号解析：模块 init 里 `+88` 的初值就是 **-1**，试值时要能照着写
		// （-1 按二补数写成全 F）。非负值仍按 u32 走，范围按 32 位校验。
		n, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || n < math.MinInt32 || n > math.MaxUint32 {
			return nil, fmt.Errorf("field %d: %q is outside the u32 range (a negative value such as -1 is written two's complement)", i, part)
		}
		binary.LittleEndian.PutUint32(out[i*4:], uint32(int64(n)))
	}
	return out, nil
}

// describeAttunementReward 把 12 字节拆成三个 u32 打日志，并对「不在客户端八档阶梯里」
// 的值提一句醒 —— 它不是错误（试值本来就可能故意用 0 或 72 去看兜底行为），
// 但值得在日志里写明，免得事后来回翻代码猜这个数是什么。
func describeAttunementReward(raw []byte) string {
	if len(raw) != attunementRewardPayloadSize {
		return fmt.Sprintf("%d bytes (unexpected length)", len(raw))
	}
	fields := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		v := binary.LittleEndian.Uint32(raw[i*4:])
		name := oathGradeTiers[uint16(v)]
		if name == "" {
			name = "not-a-client-tier"
		}
		fields = append(fields, fmt.Sprintf("[%d]=%d(%s)", i, v, name))
	}
	return fmt.Sprintf("%s hex=%s", strings.Join(fields, " "), hex.EncodeToString(raw))
}

// logAttunementRewardInjection 在启动期把注入内容打出来。
//
// 放启动期是有先例的教训：`-omen-info` 以前在「进频道」那一刻才解析，
// 写错一个字符会 log.Fatalf 在加载日志的最底下，现象是「启动游戏进不去频道」，极难定位。
func logAttunementRewardInjection(spec string, raw []byte) {
	if strings.TrimSpace(spec) == "" {
		return
	}
	if file := attunementRewardFile(spec); file != "" {
		if len(raw) == 0 {
			log.Printf("attunement reward (noti 2859): armed from %s, currently OFF (empty or comment-only file)", file)
			return
		}
		log.Printf("attunement reward (noti 2859): armed from %s, currently %s (re-read on every dungeon entry)",
			file, describeAttunementReward(raw))
		return
	}
	log.Printf("attunement reward (noti 2859): injecting %s", describeAttunementReward(raw))
}

// noteAttunementGroundDrops 把**源领主死亡那一格**两头并排念出来：掉落会话实际付出去
// 的那一摞（载体号）与落在地上的那些件（成品号）。
//
// 为什么非得加这一行：誓约/事件那一支是进本掷定、源领主死亡才落地的，中间要过统一
// 开箱展开（载体 → 成品号），所以「进本日志念的模板号」和「地上出现的物品号」**天然不是
// 同一个数字**，只靠档位那一行对不回 `[additional drop table]` 的 12 行。此前
// 「天平亮了、地上什么都没有」那一族故障（`voidsoul`/`primestella` 被当档位 0 跳过）
// 就是没有任何一行能证明这件东西到底落没落。
//
// 触发器用「本场第一次看到非空实付」而不是实体号比对：封包里的 entity 是实体序号，
// 与 `Definition.SourceBoss`（怪物模板号）不是一个东西，外层比不出「这是不是源领主」。
func (w *worldSession) noteAttunementGroundDrops(rows []protocol.SceneDrop) {
	if w == nil || w.activeDungeon == nil || w.drops == nil || w.loot == nil ||
		w.loot.Attunement == nil || !w.loot.Attunement.Enabled() {
		return
	}
	paid := w.drops.AttunementPaid
	run := w.activeDungeon.RunID
	if len(paid) == 0 || w.attuneGroundRun == run {
		return
	}
	w.attuneGroundRun = run
	owed := w.drops.Tiers
	out := make([]string, 0, len(paid))
	for _, a := range paid {
		out = append(out, fmt.Sprintf("%dx%d", a.Template, a.Amount))
	}
	ground := make([]string, 0, len(rows))
	for _, d := range rows {
		// 181 B 物品记录：+2 模板号，+6 数量（见 protocol.OrdinaryItem）。
		ground = append(ground, fmt.Sprintf("%dx%d",
			binary.LittleEndian.Uint32(d.Item[2:6]), binary.LittleEndian.Uint32(d.Item[6:10])))
	}
	log.Printf("attunement paid vs ground: dungeon %d owed effect=%d item=%d x%d tier=%s -> paid [%s] | ground %d item(s) [%s]",
		w.activeDungeon.Definition.ID, owed.OathEffect, owed.OathTemplate, owed.OathAmount,
		owed.OathLabel, strings.Join(out, " "), len(rows), strings.Join(ground, " "))
}
