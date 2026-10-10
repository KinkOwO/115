package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Current1452AE370 (registered at1452BB718) reads two u32s. It converts both to ms
// for the scene and stores [startSeconds,startSeconds+durationSeconds] in
// the mode clock pair. This is NOT two absolute timestamps or a u64 value.
//
// 2026-10-09 IDA 复核（L0，analysis/tasks/next189）：
//   * 注册形态 `sub_1459A3DD0(registry, opcode=0x5C2, handler, 0)` ⇒ **不带尺寸参数**，
//     所以 8 字节与 16 字节都不会被拒（handler 只读 2 个 u32；短包缺的字段按 0 处理）。
//   * 非 106 频道：线值是**秒**（客户端 `esi=limit*1000 ; ebp=start*1000` 给场景）；
//     106 频道：线值是**毫秒**（直接传，并折回秒写时钟）。
//   * 写时钟前有一道门禁 `sub_1459AC240(world)`：要求 `[world+0x11B0]` 在
//     `Etc/clientChannelInfo.etc` 表里**且**该频道记录的**字段 12**存在；
//     字段 12 = `[isSpecialRegionChannel]`（字段号 = 装载器内联串比较顺序，去块标记后 0-based）。
//     沉月湖（频道 101）该值为 1 ⇒ 门禁通过。**不过门禁就完全不写时钟**（UI 就停在 00:00）。
//   * 时钟对 = 单人军团管理器 `qword_14E683C40` 的 +0x1F8 / +0x200，写入者
//     `sub_142AC2760` 的**唯一调用方就是 N1474**；读取者 `sub_142AB4570`（一次取 16 字节），
//     其消费方把它算成 `max([+0x200] - 当前秒, 0)`（除 3600 得小时、除 60 得分钟）
//     ⇒ 单位是**秒**，负数钳 0。
func LegionDungeonTimeout115(start time.Time, limit time.Duration) ([]byte, error) {
	seconds := start.Unix()
	if start.IsZero() || seconds <= 0 || seconds > 0x7fffffff || limit <= 0 || limit%time.Second != 0 || limit > time.Hour || seconds+int64(limit/time.Second) > 0x7fffffff {
		return nil, fmt.Errorf("invalid legion owned clock range")
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p, uint32(limit/time.Second))
	binary.LittleEndian.PutUint32(p[4:], uint32(seconds))
	return p, nil
}
