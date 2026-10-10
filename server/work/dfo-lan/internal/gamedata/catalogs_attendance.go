package gamedata

import (
	"dfolan/internal/attendance"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
)

// AttendanceSource 只要「按路径读一份脚本文本」—— 与 boostup 共用同一个已打开的归档，
// 不第二次加载 PVF。
type AttendanceSource interface {
	Text(string) (string, error)
}

type attendanceSource struct{ a *pvf.Archive }

func (t attendanceSource) Text(p string) (string, error) { return t.a.ReadText(p) }

var _ attendance.TextSource = attendanceSource{}

// Attendance 读活动 331（每日签到）的源脚本。
//
// **为什么不像其它域那样按 `selected` 门禁**：这份脚本只有 470 字节，而按域走的代价是
// 改 `SupportedDomains` + 改 `configs/pvf-default.json` + **重编 dfolauncher.exe**
// （域白名单编在启动器里）—— 为一个 470 B 的只读脚本动三处发布件不划算。
// 开关仍然存在：op108 里那条 331 行由 `DFO_EVENT_INFO_ACTIVITY` 控制，
// 关掉活动就不再推 op1379、op680 也会被拒（见 cmd/wireprobe 的签到领取）。
//
// 将来若真需要按域开关，把它升级成一个 `attendance` 域即可（本函数已经是那一步的雏形）。
//
// 读失败**只降级不熔断**（与"缺源就降级 + 记 warning"的既有口径一致）：
// 活动整体不装配，玩家照常进城，不伪造内容。
func (s *Source) Attendance() (*attendance.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, fmt.Errorf("attendance import requires PVF")
	}
	return attendance.Load(attendanceSource{a: s.archive})
}

// preparePVFAttendance 装配活动 331 的目录。失败只记 warning（见 Attendance 的注释）。
func preparePVFAttendance(c *Catalogs, s *Source) {
	cat, err := s.Attendance()
	if err != nil {
		log.Printf("warning: attendance 331 disabled; PVF source read failed: %v", err)
		return
	}
	c.Attendance = cat
	log.Printf("PVF attendance 331 prepared: days=%d accumulate=%v mail_keys=%q/%q source=%s",
		len(cat.Days), cat.Accumulate, cat.MailTitleKey, cat.MailMessageKey, c.SourceChecksum)
}
