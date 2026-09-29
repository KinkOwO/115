package protocol

import (
	"encoding/binary"
	"fmt"
	"sort"
)

type AdventureCollectionRequest struct {
	Operation uint32
	Category  uint32
	Space     byte
	Slot      uint32
}

// 143C768F0 原样写入26字节：前13字节为未初始化前缀，不是业务字段。
// 141838121/12B/135保存栏位、槽位、分类；141837D28按动作/分类/栏位/槽位发送。
func DecodeAdventureCollection(p []byte) (AdventureCollectionRequest, error) {
	var r AdventureCollectionRequest
	if len(p) < 26 {
		return r, fmt.Errorf("图鉴登记请求不完整")
	}
	r = AdventureCollectionRequest{binary.LittleEndian.Uint32(p[13:]), binary.LittleEndian.Uint32(p[17:]), p[21], binary.LittleEndian.Uint32(p[22:])}
	if r.Operation != 0 || r.Category > 1 || r.Space != 0 || r.Slot > 65534 {
		return r, fmt.Errorf("图鉴登记动作、分类或背包槽位无效")
	}
	return r, padding(p[26:], 16)
}

// 143C6C380在成功和失败分支之前均读取12字节，不能只返回通用成功/错误字节。
// 仅动作字段参与成功刷新，其余字段保留原生初始化值2、0。
func AdventureCollectionResponse(success bool) []byte {
	p := []byte{1}
	if !success {
		p = Refusal(114)
	}
	return add32(add32(add32(p, 0), 2), 0)
}

// NOTI2425 / 143C74210：分类、已登记模板集合、已完成组合集合。
// 引导装备组0没有奖励；只有真实登记过100261068才能显示该组完成。
func AdventureCollectionGuide(equipment map[uint32]bool) []byte {
	ids := make([]uint32, 0, len(equipment))
	for id, registered := range equipment {
		if registered {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	p := add32(add32(nil, 1), uint32(len(ids)))
	for _, id := range ids {
		p = add32(p, id)
	}
	if equipment[100261068] {
		return add32(add32(p, 1), 0)
	}
	return add32(p, 0)
}
