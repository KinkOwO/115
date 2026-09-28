package protocol

import (
	"fmt"
	"time"
)

// DecodeServerTimeRequest 对应 145A31670：CMD1960 不写入任何请求字段。
func DecodeServerTimeRequest(p []byte) error {
	if len(p) != 0 {
		return fmt.Errorf("服务器时间请求必须为空")
	}
	return nil
}

// ServerTimeSuccess 对应 145257F70：成功标志后仅有一个小端 u32 秒数。
// 客户端同时记录该秒数和本机 tick，145A11A50 再按经过的秒数推进时钟。
// 145A0DAE0 做时区转换后交给 localtime32；未初始化或溢出的时间会使
// 14259BE65 的冒险团日期判断解引用空指针。不能发送毫秒或固定模板时间。
func ServerTimeSuccess(now time.Time) ([]byte, error) {
	seconds := now.Unix()
	// 148ABA030 接受 0..0x7fffd27f；两端预留一天供客户端调整时区。
	const timezoneMargin = int64(24 * time.Hour / time.Second)
	if seconds < timezoneMargin || seconds > 0x7fffd27f-timezoneMargin {
		return nil, fmt.Errorf("服务器时间 %d 超出客户端日期转换范围", seconds)
	}
	return add32([]byte{1}, uint32(seconds)), nil
}
