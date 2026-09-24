package protocol

import "fmt"

// DecodeCubeContractSelection 读取 145405070 发出的 CMD527 两字节请求。
// 当前实机作用域为 0，晶体索引为 0..5，0xff 表示关闭。
func DecodeCubeContractSelection(p []byte) (byte, error) {
	if len(p) < 2 || len(p) > 16 || p[0] != 0 || (p[1] > 5 && p[1] != 0xff) {
		return 0, fmt.Errorf("晶体契约选择格式无效")
	}
	for _, v := range p[2:] {
		if v != 0 {
			return 0, fmt.Errorf("晶体契约选择包含非零填充")
		}
	}
	return p[1], nil
}

// CubeContractSelectionInfo 对应 NOTI889 的 145404B30：两个 u8，
// 后一字节恢复选择，0xff 经原生分支映射为未选择。
func CubeContractSelectionInfo(selection byte) ([]byte, error) {
	p := []byte{0, selection}
	if _, err := DecodeCubeContractSelection(p); err != nil {
		return nil, err
	}
	return p, nil
}

// CubeContractSelectionReply 对应 CMD527 的 145404A10：成功后读取两个 u8。
func CubeContractSelectionReply(selection byte) ([]byte, error) {
	p, err := CubeContractSelectionInfo(selection)
	if err != nil {
		return nil, err
	}
	return append([]byte{1}, p...), nil
}
