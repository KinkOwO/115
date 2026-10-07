package boostup

import "fmt"

// DisjointMissionAdvanced 用**服务端自己刚提交的分解事实**推进 `[mission][type] disjoint`
// 关卡（直升训练第九关）。
//
// 源里这一关只有 `[type] disjoint` 一条条件、**没有件数**：`live/event/kor/2026/0326_boostup/boostup.evt`
// 的 `[step info][no] 9 → [mission]` 段除 `[guide actions] open equipment journal 1` 之外不含
// `[condition]`（2026-10-03 从内层 PVF 逐字段核对），所以一次真正删掉装备的分解就算完成。
//
// 条件不满足时**原样返回、changed=false、不报错**：调用方是分解流程 —— 其它关卡的分解、
// 还没领奖励（Phase!=2）的分解都必须照常成功，不能被事件层拦下来。
func (c *Catalog) DisjointMissionAdvanced(s State, disassembled bool) (State, bool, error) {
	if c == nil || !disassembled || !s.Activated {
		return s, false, nil
	}
	if _, e := c.current(s.Training); e != nil {
		// 没有可用的进行中关卡（未激活/越界/已结束）：不是分解的错误。
		return s, false, nil
	}
	row := c.Steps[int(s.Training.Step)-1]
	if row.Mission != "disjoint" || s.Training.Phase != 2 || !s.Training.Claimed[s.Training.Step] {
		return s, false, nil
	}
	next, e := c.MissionCompleted(s.Training, row.Mission)
	if e != nil {
		return s, false, fmt.Errorf("boost disjoint mission: %w", e)
	}
	out := s
	out.Training = next
	return out, true, nil
}
