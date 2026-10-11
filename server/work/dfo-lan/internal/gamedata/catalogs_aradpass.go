package gamedata

import (
	"dfolan/internal/aradpass"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
)

// AradPassSource 只要「按路径读一份脚本文本」—— 与签到/HoneyTime/成长胶囊共用同一个
// 已打开的归档，不第二次加载 PVF。
type AradPassSource interface {
	Text(string) (string, error)
}

type aradPassSource struct{ a *pvf.Archive }

func (t aradPassSource) Text(p string) (string, error) { return t.a.ReadText(p) }

var _ aradpass.TextSource = aradPassSource{}

// AradPassScript 是**本仓 PVF 里当前在期的那一季**战令脚本。
//
// 为什么写死一条路径而不是像别的域那样走配置：
//
//	① 战令每季一个新目录（`live/else/univ/<年>/<月日>_<年>_aradpass_seasonN/`），
//	   本机 PVF 里有 37 份 `newaradpassevent.evt`（2022 kor 一季一份 + 2023 起的 univ 各季）；
//	② 换赛季 = 换这一行（+ 重编），与"换期改 op108 那一行"是同一种运维动作；
//	③ 走配置要新增 flag ⇒ 启动器白名单/`pvf-default.json` 域列表跟着动（本仓踩过：
//	   未知 profile 键会让 `LoadProfile` 报错、**服务端起不来**）。
//
// ★ 选取判据是脚本自己的 `[event period]`：season3 = `2026-08-04 09:00:00` →
// `2026-10-20 09:00:00`，**覆盖当前运营日**。换季时按同样口径挑"到期日最靠后的那一份"。
const AradPassScript = "live/else/univ/2026/0804_2026_aradpass_season3/newaradpassevent.evt"

// AradPass 读战令的源脚本。
//
// 与 331/313 同样的取舍：**不按域门禁**（读失败只降级不熔断）—— 活动整体不装配，
// 玩家照常进城，不伪造内容。
func (s *Source) AradPass() (*aradpass.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, fmt.Errorf("arad pass import requires PVF")
	}
	text, err := aradPassSource{a: s.archive}.Text(AradPassScript)
	if err != nil {
		return nil, err
	}
	cat, err := aradpass.Parse(text)
	if err != nil {
		return nil, err
	}
	cat.Path = AradPassScript
	return cat, nil
}

// preparePVFAradPass 装配战令目录。失败只记 warning（见 AradPass 的注释）。
func preparePVFAradPass(c *Catalogs, s *Source) {
	cat, err := s.AradPass()
	if err != nil {
		log.Printf("warning: arad pass disabled; PVF source read failed: %v", err)
		return
	}
	log.Printf("PVF arad pass prepared: levels=%d maxExp=%d normal=%d royal=%d missions=%d/%d weekly_cap=%d purchase_exp=%d pass_item=%v source=%s",
		len(cat.Levels), cat.MaxExp(), len(cat.RewardsFor(aradpass.TrackNormal)), len(cat.RewardsFor(aradpass.TrackRoyal)),
		len(cat.NormalMissions), len(cat.SeasonMissions), cat.WeeklyGainableExp, cat.PurchaseExp, cat.PassItem, c.SourceChecksum)
	c.AradPass = cat
}
