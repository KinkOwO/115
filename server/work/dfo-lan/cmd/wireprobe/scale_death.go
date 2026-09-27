package main

// 「调律之边界 · 最终调律者」(100005014) 的定盘机关由一组 script 状态机驱动，
// 它的击杀路径要 `primer_now >= primer_max`，而两个天花板来自引擎的
// getPrimerGrade() / getOathGrade()。本机实测那两个 getter 恒返回 72，越出机制值域
// (脚本可赋 41-45/70，比较值最高 71)，于是 die_trigger 永不打开、机关永远打不死，
// 副本永远无法通关。详见 docs/protocol/endkeeper-of-order-primer-20260926.md。
//
// 这里走的是业主提的那条路：不再依赖那个参数，改由服务端在机关血量触底时
// 「宣布它死了」。做法是合成一条该机关自己的 CMD39 上报，喂给现成的
// monsterDeath —— 于是掉落/经验/任务/通关判定整条既有路径原样复用，
// 客户端也会照常收到 NOTI38 (MonsterDeathConfirmed)。
//
// 触发条件刻意保留「玩家真的把它打到血底」这一步：
//
//	客户端每秒上报一次 CMD2329 (ENUM_CMDPACKET_MONSTER_HISTORY_LOG)，
//	载荷 = u32 + char[256] 文本 (含 "HP : <值>") + 尾部若干字段 (含怪物模板号)。
//	该文本只在 safe_timer>=60 且机关正在挨打时才发；第一条样本未必是满血
//	(2026-09-27 实测有一条运行 8 个样本全部落在地板 2% 上，峰值判据会失效)。
//
// 因此血量判据 = 「本次运行见过的最高血量」的 scaleDeathFloorPercent 以下，只作兜底。
// 默认关闭：-scale-death-from-hp 或 DFO_SCALE_DEATH_FROM_HP=1 才生效。
//
// 2026-09-27 cell 级补正（取证见 docs/protocol/endkeeper-of-order-primer-20260926.md §18）：
//	1. CMD2329 的那段 [SERVER LOG MSG] 与 [DO BEHAVIOR] GO_END 在**同一个分支**里
//	   (primer_proc.act cell 2025-2068: [ON DAMAGE] + o:safe_timer>=60 + 不在 end 组)。
//	   收到一条 CMD2329 就等价于「客户端刚刚跑了收尾」——这是本机关唯一的击杀入口，
//	   所以服务端据此判死与设计意图一致。
//	2. 收尾动作 Primer_00_Normal_End.act 在 o:die_frame_normal 帧 [DESTROY] 自己，
//	   靠 [SET GROUP ACTION] end (c:primer_rarity_progress_max-40) 选中稀有度对应的
//	   END 动作；而 scale_primer.mob 的 [etc action definition] 只有 8 项下标 0..7。
//	   天花板 72 让下标变成 32 ⇒ 选不中动作 ⇒ 既不放结束动画也不 DESTROY，且
//	   [IS GROUP ACTION] end 永不置位、这条分支每挨一次打就重发一份日志(实测 20 条)。
//	3. 另一条收尾路径 [ON DAMAGE] o:getHpRate() <= 2 (primer_proc.act cell 463-495)
//	   不发日志，只能继续靠血量地板认 —— 故两组判据都留。

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"

	"dfolan/internal/game/protocol"
)

const (
	// scalePrimerTemplate 是 Scale_primer (109019266)：小深渊 map 中的索引 11 行，
	// [fixed] [boss]，也是 [clear condition] -> [hunt boss] 的通关目标。
	scalePrimerTemplate = 109019266
	// scaleOathTemplate 是同一坐标 (3168,191) 上的另一台装置 Scale_oath (109019280，
	// map 索引 12 行，[fixed] [normal])。它同样是 [fixture]、实测同样从未上报过死亡。
	// 通关路径的 roomEnemiesDead() 要求房间里不再有活着的可击杀目标，所以它必须
	// 一起退场 —— 它按「无主」(killer=65535) 处理：不给掉落、不给经验。
	scaleOathTemplate = 109019280
	// scaleStatusTextOffset / scaleStatusTextLen 是 CMD2329 载荷里那段 ASCII 文本。
	scaleStatusTextOffset = 4
	scaleStatusTextLen    = 256
	// scaleTemplateScanBytes 是尾部里找模板号时回看的字节数。
	scaleTemplateScanBytes = 24
	// scaleDeathFloorPercent 是「血量已触底」的判定线。脚本自己的死亡条件写的是
	// getHpRate() <= 2，实测地板就是 2%，这里留一点余量。
	scaleDeathFloorPercent = 5.0
)

// scaleStatusReport 是 CMD2329 里我们关心的两个字段。
type scaleStatusReport struct {
	Template uint32
	HP       float64
	// EndTrigger 是文本里 triggers 的第一个数 (o:is_end_trigger_on)。
	// 客户端进入收尾阶段后为 1；它是「这条日志与 GO_END 同分支」的旁证，不单独作为判据。
	EndTrigger bool
}

// decodeScaleStatus 从 CMD2329 载荷里取出模板号与当前血量。
// want 是本次运行里「值得关心」的模板号集合（即定盘机关的模板号）；模板号只在
// want 命中时才算解析成功，避免把别的机器人的日志当成本机关。
func decodeScaleStatus(p []byte, want map[uint32]bool) (scaleStatusReport, bool) {
	if len(p) < scaleStatusTextOffset+8 || len(want) == 0 {
		return scaleStatusReport{}, false
	}
	end := scaleStatusTextOffset + scaleStatusTextLen
	if end > len(p) {
		end = len(p)
	}
	text := p[scaleStatusTextOffset:end]
	if i := bytes.IndexByte(text, 0); i >= 0 {
		text = text[:i]
	}
	hp, ok := parseScaleHP(string(text))
	if !ok {
		return scaleStatusReport{}, false
	}
	template, ok := scaleTemplateIn(p, want)
	if !ok {
		return scaleStatusReport{}, false
	}
	return scaleStatusReport{Template: template, HP: hp, EndTrigger: parseScaleEndTrigger(string(text))}, true
}

// parseScaleHP 从日志文本里取 "HP : <float>"。
func parseScaleHP(text string) (float64, bool) {
	const key = "HP : "
	i := strings.Index(text, key)
	if i < 0 {
		return 0, false
	}
	rest := text[i+len(key):]
	if j := strings.IndexAny(rest, ", "); j >= 0 {
		rest = rest[:j]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseScaleEndTrigger 从日志文本里取 "triggers : <end>, <die>" 的第一个数。
// 取不到时返回 false（宁可不判死，也不误判）。
func parseScaleEndTrigger(text string) bool {
	const key = "triggers : "
	i := strings.Index(text, key)
	if i < 0 {
		return false
	}
	rest := text[i+len(key):]
	if j := strings.IndexAny(rest, ", "); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest) == "1"
}

// scaleTemplateIn 在载荷尾部找一个小端 u32 模板号，且必须落在 want 里。
func scaleTemplateIn(p []byte, want map[uint32]bool) (uint32, bool) {
	lo := len(p) - scaleTemplateScanBytes
	if lo < 0 {
		lo = 0
	}
	for i := lo; i+4 <= len(p); i++ {
		v := binary.LittleEndian.Uint32(p[i:])
		if want[v] {
			return v, true
		}
	}
	return 0, false
}

// scaleStatus 处理一条 CMD2329：记录「见过的最高血量」，并在血量触底时
// 直接把机关判死（合成它的 CMD39 交给 monsterDeath）。
// 默认关闭，且任何解析失败都只是「不处理」，绝不因此让整帧失败。
func (w *worldSession) scaleStatus(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if !w.scaleDeathFromHP || w.activeDungeon == nil {
		return nil, nil
	}
	// 新的一场（重进副本）必须从零开始：entity 会被重新分配，而 peak 也会变（同一模板
	// 在不同难度/房间里血量不同）。不去清零的话第二场就再也杀不掉 —— 2026-09-27 实机踩到。
	if w.scaleRun != w.activeDungeon.RunID {
		w.scaleRun = w.activeDungeon.RunID
		w.scaleHP = map[uint32]float64{}
		w.scaleForced = map[uint16]bool{}
	}
	want := map[uint32]bool{}
	var primer, sibling uint16
	for _, m := range w.activeDungeon.Monsters {
		switch m.Template {
		case scalePrimerTemplate:
			want[scalePrimerTemplate] = true
			primer = m.Entity
		case scaleOathTemplate:
			sibling = m.Entity
		}
	}
	if primer == 0 || !want[scalePrimerTemplate] {
		return nil, nil
	}
	r, ok := decodeScaleStatus(p, want)
	if !ok {
		return nil, nil
	}
	if w.scaleHP == nil {
		w.scaleHP = map[uint32]float64{}
	}
	if r.HP > w.scaleHP[r.Template] {
		w.scaleHP[r.Template] = r.HP
	}
	peak := w.scaleHP[r.Template]
	rate := 0.0
	if peak > 0 {
		rate = r.HP / peak * 100
	}
	if event != nil {
		event(map[string]any{
			"kind": "scale_status", "template": r.Template, "hp": r.HP,
			"peak_hp": peak, "rate": rate, "floor": scaleDeathFloorPercent,
			"end_trigger": r.EndTrigger,
		})
	}
	// 主判据：收到日志 = 客户端已经跑了 GO_END（该日志与 GO_END 同分支）。
	// 兜底判据：血量掉到本次运行峰值的地板以下（覆盖 getHpRate()<=2 那条不发日志的路径）。
	if !r.EndTrigger && (peak <= 0 || r.HP <= 0 || rate > scaleDeathFloorPercent) {
		return nil, nil
	}
	reason := "hp_floor"
	if r.EndTrigger {
		reason = "client_end_trigger"
	}
	if w.scaleForced == nil {
		w.scaleForced = map[uint16]bool{}
	}
	if w.scaleForced[primer] {
		return nil, nil
	}
	w.scaleForced[primer] = true
	if event != nil {
		event(map[string]any{
			"kind": "scale_death_forced", "entity": primer, "template": r.Template,
			"hp": r.HP, "peak_hp": peak, "rate": rate, "sibling": sibling, "reason": reason,
		})
	}
	// 先把同坐标的另一台装置退场：它是 [fixture]，同样永远不会死，而 roomEnemiesDead()
	// 会因此挡住通关。按「无主」处理 —— killer=65535 让 ConfirmDeath 走 unowned 分支，
	// 不给掉落也不给经验（它本来就不是给玩家砍的东西）。
	var plan []outboundPacket
	if sibling != 0 && !w.activeDungeon.Dead[sibling] {
		if _, err := w.activeDungeon.ConfirmDeath(uint32(sibling), 65535, w.role.WireID); err != nil {
			return plan, err
		}
		plan = append(plan, outboundPacket{
			"scale_sibling_confirmed", 0, 38, protocol.MonsterDeathConfirmed(sibling),
		})
	}
	// 再合成本机关自己的 CMD39 载荷（entity u32 + killer u16 + 零填充到 64B，
	// 满足 DecodeMonsterDeath 的几何要求），交给现成的死亡路径。
	body := make([]byte, 64)
	binary.LittleEndian.PutUint32(body[0:], uint32(primer))
	binary.LittleEndian.PutUint16(body[4:], w.role.WireID)
	died, err := w.monsterDeath(body, event)
	if err != nil {
		return plan, err
	}
	return append(plan, died...), nil
}
