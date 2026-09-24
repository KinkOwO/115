package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

// correctionEquippedLevelKey 是装备自带的「使用等级修正」字段名，客户端把这一行显示成
// 「装备等级限制-5」。抽成常量除了消除重复字面量，还让该字符串**进入二进制** ——
// 部署时可用 `grep -a -c "correction equipped level" <exe>` 判定新逻辑是否真的编译了进去
// （改动前该串在二进制里零命中）。
const correctionEquippedLevelKey = "[correction equipped level]"

// WearableBy reports whether a source definition's own requirements admit this
// character: minimum level (including its [correction equipped level]
// adjustment), usable job, usable grow type. It is the single place those three
// rules live, so wearing a piece and being offered it as a drop cannot disagree.
//
// kind is the source [equipment type] text. The avatar family - skins
// ([skin avatar]), weapon avatars ([weapon avatar]) and the rest of the
// [* avatar] kinds - routinely ships .equ scripts with no [minimum level]
// section at all: trousers 502510504 carry "[minimum level] 1" while skin
// 502580005 has no such section. The old guard read a missing field as an
// unrecognisable definition and refused the piece, which the client renders as
// "背包已满". For those kinds a missing section means "no level requirement";
// for ordinary equipment it still means unavailable.
func WearableBy(fields map[string][]pvf.Token, kind string, job string, advancement, level byte) error {
	levels := fields["[minimum level]"]
	if len(levels) == 0 {
		if !strings.HasSuffix(kind, " avatar]") {
			return fmt.Errorf("equipment minimum level not met or unavailable")
		}
		levels = []pvf.Token{{Type: 0, Value: 0}}
	} else if len(levels) != 1 || levels[0].Type != 0 || levels[0].Value < 0 {
		return fmt.Errorf("equipment minimum level not met or unavailable")
	}
	required := levels[0].Value
	// [correction equipped level] 是装备自带的「使用等级修正」，客户端把它显示成
	// 「装备等级限制-5」；有效需求 = [minimum level] + 该修正。
	//
	// 漏读它会让客户端与服务端算法不一致：客户端按修正后的门槛放行、服务端按原始值拒绝，
	// 表现就是「客户端让你点、服务端回绝」（客户端把 0x0004 一律渲染成「背包已满」）。
	// 实机 2026-09-23：100051394 的 [minimum level]=50、修正=-5（有效 45），
	// 角色 38 级 + 霸王(PremiumConqueror) 10 = 有效 48 ≥ 45，本该能穿却被拒。
	//
	// 全量装备目录（configs/equipment-full，424,216 条）实测 3,708 条带此字段，取值只有
	// -5(2,793) / -4(287) / -2(628)，**恒为负、无一例正数**，且与 [minimum level] 呈
	// 对齐意图（115→110、105→100、100→96/98），确认语义就是"降低使用等级门槛"。
	//
	// 字段缺失或形状异常时保持旧行为（只用 [minimum level]），不引入新的拒绝理由。
	if corr := fields[correctionEquippedLevelKey]; len(corr) == 1 && corr[0].Type == 0 {
		required += corr[0].Value
	}
	if required < 0 {
		required = 0
	}
	if int32(level) < required {
		return fmt.Errorf("equipment minimum level not met or unavailable")
	}
	allowed := false
	for _, j := range fields["[usable job]"] {
		allowed = allowed || j.Text == "[all]" || j.Text == job
	}
	if !allowed {
		return fmt.Errorf("equipment profession requirement not met")
	}
	if grow := fields["[usable grow type]"]; len(grow) > 0 {
		allowed = false
		for _, g := range grow {
			if g.Type == 0 && (g.Value == -1 || g.Value == int32(advancement)) {
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("equipment advancement requirement not met")
		}
	}
	return nil
}
