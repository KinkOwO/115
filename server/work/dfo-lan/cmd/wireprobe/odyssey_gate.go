package main

import (
	"fmt"
	"log"

	"dfolan/internal/game/protocol"
	"dfolan/internal/modpolicy"
)

// odysseyModeActive 报告"当前副本是奥德赛模式"。
//
// 判据与既有分支（准入、难度锁、复活额度）一致：`DungeonDefinition.Odyssey`，
// 它来自 DGN 的 `[dungeon mode script] arad odyssey`（internal/catalog/dungeons.go）。
// 这里**不**重复判 `Loaded`：调用方各自的既有校验（`lifeTokenReviveAllowed` /
// `pilotReviveAllowed`）已经查过，重复判只会多造一种"静默不生效"的可能。
func (w *worldSession) odysseyModeActive() bool {
	return w != nil && w.activeDungeon != nil && w.activeDungeon.Definition.Odyssey
}

// odysseyConsumableGate 是"奥德赛副本内禁用消耗品"的门（业主 2026-10-06 口径：
// 副本内禁止使用任何消耗品、**可以携带**；城镇不受影响）。
// 命中返回**非 nil 的空计划**（已拦截、不回复任何包），未命中返回 nil。
// 不用 UseStackableRefused 的原因见函数体：那个形状未被实机证实，回它会把客户端打崩。
//
// 客户端侧本来就是一致的：奥德赛进图不下发 NOTI1584（STACKABLE_DUNGEON_LIMIT），
// 按官方客户端实测口径"没有这一帧 = 客户端本地禁用全部消耗品"，界面是灰的；
// 这里挡的是**权威侧** —— 改过的客户端绕过界面也拿不到药。
func (w *worldSession) odysseyConsumableGate(r protocol.UseStackableRequest) []outboundPacket {
	rules := modpolicy.Odyssey()
	if !rules.BanConsumables || !w.odysseyModeActive() {
		return nil
	}
	// 为什么不回 UseStackableRefused：这个包形状**从未被实机抓包证实** ——
	// protocol.UseStackableRefused 的注释自己写着「两条 u32 的字段顺序仍需抓包确认」。
	// 2026-10-06 18:24 实机：回这个包之后客户端 1.3 秒崩退
	// （client_trace 结尾 <USERCRI>/<USERDMP>，服务端紧接着收到 CMD682 退出帧）。
	// 改为**什么都不回**：服务端不执行效果、不扣道具，规则照样成立，
	// 而客户端没有任何东西可以解析错。返回非 nil 的空计划 = 「已拦截、不回复」。
	// 等抓到实机抓包或反汇编出真正的失败形状，再换回带字段的拒绝包。
	return []outboundPacket{}
}

// odysseyReviveGate 是"奥德赛内禁止复活"的门，挂在 useCoinRevive 最前面：
// 一处挡住三级回退（奥德赛测试额度 → 背包复活币 → CERA 扣费），
// 不留"换一档还能复活"的缝。
//
// 返回非 nil 时 CMD41 的 dispatch 会回 `boosterActionRefusal(41)` = Refusal(22)，
// 客户端保持死亡态；随后既有的死亡超时流程（CMD40 → 10 秒 → N33 FAIL_CLEAR + 回城）
// 把玩家判负送回城 —— 也就是业主说的"死亡即退出副本/回城"。
func (w *worldSession) odysseyReviveGate() error {
	rules := modpolicy.Odyssey()
	if !rules.BanReviveCoin || !w.odysseyModeActive() {
		return nil
	}
	return fmt.Errorf("odyssey mode forbids revive (policy: %s)", rules.Source)
}

// odysseyImmediateDeathFail 报告"这次死亡应当**当场**判负回城"：
// 奥德赛模式 + 禁复活规则开着 + 本局确实已死亡（pilotDeath 已落到本局）。
//
// 少了任一条件就走原来的 10 秒倒计时（可复活 / 非奥德赛都不受影响）。
// 为什么值得单独一个函数：这是"死亡即回城"的唯一判据，测试直接钉它，
// 免得以后有人改了死亡流程却把这条按模式区分的规则悄悄弄丢。
func (w *worldSession) odysseyImmediateDeathFail() bool {
	if !modpolicy.Odyssey().BanReviveCoin || !w.odysseyModeActive() {
		return false
	}
	return w.pilotDeath != nil && w.activeDungeon != nil &&
		w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Dead
}

// logModPolicy 把生效中的模式规则与掉落倍率写进启动日志。
// server/AGENTS §6 的教训是"默认路径悄悄坏掉最难查"，所以这里必须有一行可核对：
// 规则没生效、或"谁把爆率改了"都必须一眼看得出来。
func logModPolicy() {
	log.Printf("odyssey mode rules: %s", modpolicy.Odyssey())
	log.Printf("drop rate rules: %s", modpolicy.Drops())
}
