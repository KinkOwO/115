package gamedata

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/honeytime"
	"fmt"
	"log"
)

// HoneyTimeSource 只要「按路径读一份脚本文本」—— 与签到/成长胶囊共用同一个已打开的
// 归档，不第二次加载 PVF。
type HoneyTimeSource interface {
	Text(string) (string, error)
}

type honeyTimeSource struct{ a *pvf.Archive }

func (t honeyTimeSource) Text(p string) (string, error) { return t.a.ReadText(p) }

var _ honeytime.TextSource = honeyTimeSource{}

// HoneyTime 读活动 313（HoneyTime 每日在线奖励）的源脚本。
//
// 与 331 签到同样的取舍（见 catalogs_attendance.go）：**不按域门禁**。这份脚本不到 2 KB，
// 按域走要改 `SupportedDomains` + `configs/pvf-default.json` + **重编 dfolauncher.exe**
// （域白名单编在启动器里），为一个只读脚本动三处发布件不划算。
// 活动开关仍在：NOTI108 里那条 313 行由 `DFO_EVENT_INFO_ACTIVITY` 控制，
// 关掉之后既不推 1294 也不受理 680 领奖。
//
// 读失败**只降级不熔断**：活动整体不装配，玩家照常进城，不伪造内容。
func (s *Source) HoneyTime() (*honeytime.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, fmt.Errorf("honeytime import requires PVF")
	}
	return honeytime.Load(honeyTimeSource{a: s.archive})
}

// preparePVFHoneyTime 装配活动 313 的目录。失败只记 warning。
func preparePVFHoneyTime(c *Catalogs, s *Source) {
	cat, err := s.HoneyTime()
	if err != nil {
		log.Printf("warning: honeytime 313 disabled; PVF source read failed: %v", err)
		return
	}
	c.HoneyTime = cat
	log.Printf("PVF honeytime 313 prepared: tiers=%d period=%d loop=%d/%d offsets=%d/%d postal=%d pcroom=%v script=%s source=%s",
		len(cat.Daily), len(cat.Period), cat.Loop, cat.PeriodLoop, cat.DailyOffsetCount, cat.PeriodOffsetCount, cat.PostalType, cat.PCRoom, cat.ScriptPath, c.SourceChecksum)
}
