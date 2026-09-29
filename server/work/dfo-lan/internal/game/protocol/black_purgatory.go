package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// BlackPurgatoryRemaining 编码NOTI537的副本ID、每日剩余、每周剩余。
// 1452FA235注册537→1452CB580；其两个u8分别写角色+11FB0/+11FC0。
// 14416120B/14416121E在发送建队请求前检查这两个计数，缺省0会直接拦截。
// 原生reader把100000521映射到原版配置中的黑鸦四模式共享组。
func BlackPurgatoryRemaining(daily, weekly byte) []byte {
	p := binary.LittleEndian.AppendUint32(nil, 100000521)
	return append(p, daily, weekly)
}

// DecodeBlackPurgatoryParty 解码142C7BBF0发送的CMD12。
// 142AB75F0将半团本类型3映射为队伍类型7；其后的u16=5才是小队模式。
func DecodeBlackPurgatoryParty(p []byte) ([]byte, error) {
	if len(p) < 36 {
		return nil, fmt.Errorf("黑鸦建队请求不完整")
	}
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n > 63 || n < 0 || n > len(p)-36 {
		return nil, fmt.Errorf("黑鸦队伍名称长度无效")
	}
	at := 6 + n
	if p[0] > 1 || p[1] != 0 || p[at] != 1 || p[at+9] != 7 ||
		binary.LittleEndian.Uint16(p[at+10:]) != 5 || p[at+29] != 0 {
		return nil, fmt.Errorf("请创建黑鸦之境小队模式队伍")
	}
	if err := padding(p[at+30:], 16); err != nil {
		return nil, err
	}
	name := p[6:at]
	if bytes.IndexByte(name, 0) >= 0 {
		return nil, fmt.Errorf("黑鸦队伍名称含无效字符")
	}
	return bytes.Clone(name), nil
}

// BlackPurgatorySoloParty 按1452F2620的本频道单成员分支构造NOTI9。
// 与旧99字节记录不同，保留146D78070的名字长度和146D77F50的扩展长度。
// 1452F2DDB与1452F3C25均写队伍类型，后者不能把小队类型覆盖为普通队伍。
// 子模式写入队伍对象+0x84；1452F62EE经142AC2650设置黑鸦管理器+0x10，
// 142A29390据此查出小队副本100000527。142ABC169确认模式5的队伍容量为1。
func BlackPurgatorySoloParty(actor uint16, channel [2]byte, name []byte) ([]byte, error) {
	if actor == 0 || actor == 65535 || channel[0] == 0 || channel[1] == 0 || len(name) > 63 {
		return nil, fmt.Errorf("黑鸦队伍身份或名称无效")
	}
	p := make([]byte, 107+len(name))
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], 9999)
	binary.LittleEndian.PutUint16(p[4:], 1)
	copy(p[6:8], channel[:])
	binary.LittleEndian.PutUint32(p[14:], uint32(len(name)))
	copy(p[18:], name)
	q := p[len(name):]
	q[20] = 1 // 容量，不把精锐AI计为在线玩家。
	q[25] = 5 // 与原生建队发送器的普通难度字段一致。
	q[29], q[95] = 7, 7
	binary.LittleEndian.PutUint16(q[30:], 5)
	copy(q[46:54], []byte{1, 1, 2, 4, 7, 7, 7, 7})
	q[69] = 1
	binary.LittleEndian.PutUint16(q[71:], actor)
	// 末尾扩展类型0不更新各玩法次数，不能借用沉月湖的41及测试次数。
	return p, nil
}

func BlackPurgatoryPartyGone(channel [2]byte) []byte {
	p := make([]byte, 12)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], 9999)
	binary.LittleEndian.PutUint16(p[4:], 1)
	copy(p[6:8], channel[:])
	p[9], p[10] = 3, 1
	return p
}

// BlackPurgatoryEntryInfo 仅描述新挑战的准备、开场和首次进图状态。
// 142A274D0依次读u8/u8/8*u32/u8/u32，共39字节；+6为剩余秒数。
// 新挑战未击杀、未救援、未使用复活，计数及复活冷却均为0。
// 1经142A2CA20建立开场对象，2表示进行中；不能直接跳过1初始化。
func BlackPurgatoryEntryInfo(phase byte, remaining uint32) []byte {
	p := make([]byte, 39)
	p[0] = phase
	binary.LittleEndian.PutUint32(p[6:], remaining)
	return p
}
