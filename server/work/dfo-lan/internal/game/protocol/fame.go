package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// CharacterFameValue为NOTI2257。1452FAA89将2257绑定到1452C9640，读取固定10字节。
// +2进入145F05F60（当前名望）；+6进入145F06750及145F059D0。
// 后者写入14EF2CB58映射，141440144经145EFFB60读取并显示maxFameValue。
func CharacterFameValue(actor uint16, current, highest uint32) ([]byte, error) {
	if actor == 0 || actor == 65535 || current > math.MaxInt32 || highest > math.MaxInt32 || highest < current {
		return nil, fmt.Errorf("角色名望通知参数无效")
	}
	p := make([]byte, 10)
	binary.LittleEndian.PutUint16(p, actor)
	binary.LittleEndian.PutUint32(p[2:], current)
	binary.LittleEndian.PutUint32(p[6:], highest)
	return p, nil
}
