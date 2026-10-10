package protocol

import "fmt"

// 1452C19F0 calls 1451AA420 inside each compact equipment row. It reads a
// byte count, interleaved (id,valueA,valueB) triples, then type and special
// index iff count != 0. 1452C2491/267E apply that manager to the actual item
// via 1451A5560. The record's three columns at +61/+64/+67 are not copied
// by 14576D8B0, so the nested list must carry the saved roll instead.
// This handles ordinary unseal rolls (special index FF). Replacement rolls
// use another native four-byte branch and remain explicitly unsupported.
func tagRandomOptions(record [CurrentItemRecordSize]byte) ([]byte, error) {
	count := int(record[60])
	if count > 3 {
		return nil, fmt.Errorf("数量超过实例三格容量")
	}
	if count == 0 {
		for offset, value := range record[61:76] {
			if value != 0 && !(offset+61 == 71 && value == 0xff) {
				return nil, fmt.Errorf("空列表包含未消费的数据")
			}
		}
		return []byte{0}, nil
	}
	if record[71] != 0xff {
		return nil, fmt.Errorf("特殊替换属性布局尚未接入")
	}
	for i := count; i < 3; i++ {
		if record[61+i] != 0 || record[64+i] != 0 || record[67+i] != 0 {
			return nil, fmt.Errorf("未使用的属性格包含数据")
		}
	}
	for _, value := range record[72:76] {
		if value != 0 {
			return nil, fmt.Errorf("普通属性包含特殊替换数据")
		}
	}
	p := []byte{byte(count)}
	for i := 0; i < count; i++ {
		p = append(p, record[61+i], record[64+i], record[67+i])
	}
	return append(p, record[70], record[71]), nil
}
