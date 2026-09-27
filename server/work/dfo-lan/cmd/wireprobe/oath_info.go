package main

// noti 2838 ENDKEEPER_OF_ORDER_REWARD：向客户端下发「引子 / 誓约」的两个档位。
//
// 为什么必须发（见 docs/protocol/endkeeper-of-order-primer-20260926.md §20）：
//
//	客户端单例 qword_14E6388B8 的构造器 sub_1406560D0 里硬写
//
//	    *(_DWORD *)(a1 + 88) = 72;   // primer
//	    *(_DWORD *)(a1 + 92) = 72;   // oath
//
//	而 noti 2838 的解析器 sub_140656A00 正是用 8 字节载荷（2 × u32）写这两个字段：
//
//	    v3 = 72; v4 = 72;
//	    sub_146EA0BE0(&v3, 8);        // 载荷 [0:4) -> v3, [4:8) -> v4
//	    *(_DWORD *)(v0 + 88) = v3;
//	    *(_DWORD *)(v0 + 92) = v4;
//
//	脚本（contents/2026/endkeeperoforder/.../Primer_Proc.act cell 6–9）把它们读成
//
//	    c:primer_rarity_progress_max = getPrimerGrade()   // = 单例 +88
//	    c:oath_rarity_progress_max   = getOathGrade()     // = 单例 +92
//
//	再用 max(oath, primer) 当**下标**与**闸门**：
//
//	  * 收尾动作 `[SET GROUP ACTION] end (max-40)` —— 而 `o:hp_limit` / `o:delay_time`
//	    都只有 8 项 ⇒ 合法下标 0..7 ⇒ max ∈ {40..47, 70, 71}；
//	  * `END_TRIGGER` 要 `now >= max` **两侧都成立**才开 `DIE_TRIGGER`；
//	  * 通往隐藏 BOSS 的 `summon_orthaire` 被包在 `now == max` 里。
//
//	72 越出这个值域 ⇒ 下标 32 越界 ⇒ 收尾动作选不中、不放结束动画、不 `DESTROY`、
//	**客户端永不发 C2S 39**；同时 `oath_max == 45 → nox_is_orthaire` 等四条选路分支
//	全不成立 ⇒ 奥尔泰尔永不登场。一个哨兵同时堵死三件事。
//
// 实机定案（2026-09-27 07:1x，四轮差分注入，服务端兜底开着以便退出）：
//
//	注入 (45,45) -> primer_max=45 oath_max=45，探针实时读数 45/45，c:nox_index=109019264
//	注入 (71,45) -> 71/45                                  c:nox_index=109019264
//	注入 (45,71) -> 45/71                                  c:nox_index=0
//	注入 (71,71) -> 71/71                                  c:nox_index=0
//
//	⇒ [0:4) = primer、[4:8) = oath，4/4 与实时 getter 逐字吻合；且 **只有 oath=45**
//	  会把 `c:nox_index` 选成 109019264（orderchroniclerorthaire）= 第四档「太初」的隐藏 BOSS。
//
// 档位值域（八档，与 scale_primer.mob 的 `Primer_00..07` 一一对应）：

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// oathGradeTiers 是脚本真正接受的八个档位（其它值一律让 `max-40` 越界）。
var oathGradeTiers = map[uint16]string{
	40: "normal",
	41: "rare",
	42: "unique",
	43: "legendary",
	44: "epic",
	45: "primitive",
	70: "rainbow1",
	71: "rainbow2",
}

// oathGradeDefault 默认下发 45（primitive）。
//
// 45 是「第四档 = 太初」：客户端动画 `Symptom_4/PrimevalSymptom`，也是四档里最高的
// 普通档，`max-40 = 5` 是 `o:hp_limit` 的最后一个合法下标；**且它是唯一会召唤
// 奥尔泰尔的分支**。发 70/71 会走 rainbow 分支（收尾用字面量 6/7），但四条选路
// 都不成立 ⇒ 拿不到隐藏 BOSS。
const oathGradeDefault = 45

// parseOathGrades 解析 "primer,oath"。空串 = 用默认（45,45）。
//
// 只接受八档内的值：发域外值等于把「打不死」原样复制一遍，所以宁可启动就报错。
func parseOathGrades(spec string) ([2]uint16, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return [2]uint16{oathGradeDefault, oathGradeDefault}, nil
	}
	parts := strings.Split(spec, ",")
	if len(parts) != 2 {
		return [2]uint16{}, fmt.Errorf("bad oath grades %q: want primer,oath", spec)
	}
	var out [2]uint16
	for i, part := range parts {
		v, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil {
			return [2]uint16{}, fmt.Errorf("bad oath grades %q: %w", spec, err)
		}
		if _, ok := oathGradeTiers[uint16(v)]; !ok {
			return [2]uint16{}, fmt.Errorf("bad oath grades %q: %d is outside the eight tiers "+
				"the client's script accepts (40..45, 70, 71)", spec, v)
		}
		out[i] = uint16(v)
	}
	return out, nil
}

// oathInfoPayload 是 noti 2838 的载荷：两个 little-endian u32，共 8 字节。
//
// 长度必须是 8：解析器先 `sub_146EA0BE0(&v3, 8)` 定长读一次，短了会踩客户端自己的
// 长度护栏（loc_146EA0C30 是故意的空写陷阱，2026-09-27 06:1x 用 8 字节的 2839 撞过）。
func oathInfoPayload(primer, oath uint16) []byte {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p[0:4], uint32(primer))
	binary.LittleEndian.PutUint32(p[4:8], uint32(oath))
	return p
}

// oathInfoPackets 返回进本时应随加载应答下发的档位通知。
//
// 时机很关键：必须在天平开场执行 `Primer_Proc.act` cell 6–9 之前到位，所以挂在
// 副本加载应答的同一个 plan 里（见 dungeon_flow.go 的 loading 分支）。
// 这也是一份**常驻状态**：客户端只在收到它时才会覆盖构造器里的 72/72 兜底。
func (w *worldSession) oathInfoPackets() []outboundPacket {
	primer, oath := w.oathGrades[0], w.oathGrades[1]
	return []outboundPacket{{"oath_system_grades", 0, 2838, oathInfoPayload(primer, oath)}}
}
