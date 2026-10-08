package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
)

// avatarSocketOptionsSize 是客户端时装徽章孔扩展的固定长度：
// 5 槽 × (u16 socket 类型 + u32 已嵌徽章模板) = 30 字节（小端）。
// 打孔（avatar_socket.go）、镶嵌（emblem_inlay.go）和这里补默认孔
// 必须写同一个布局，客户端显示的孔**完全来自该字段**。
const avatarSocketOptionsSize = 30

// avatarPlatinumSocket 是 [S socket]（白金徽章孔）的掩码。客户端徽章目标掩码
// 语义见 emblem_inlay_rules.go（"M excludes S (platinum)"）：[S socket] = 16
// 只接受白金徽章，[M socket] = 65519 是彩色孔（Multicolored，接受除白金外的
// 四色徽章），与美服文档一致。
const avatarPlatinumSocket uint16 = 16

// DefaultAvatarSockets 返回一件时装的默认徽章孔扩展
// （30 字节 = 5 槽 × (u16 socket 类型 + u32 emblem)，与打孔器/嵌徽章同格式）。
//
// 默认孔**直接读 PVF 时装定义**，不硬编码：
//   - 光环等带 [emblem socket default] 段：按段内声明的 socket 逐个写入
//   - 其余时装读 [avatar type select] 尾部 "N [socket1] [socket2]…" 声明
//   - 定义里没有默认孔（如部分武器装扮、Look 纯外观时装）则返回 0 孔
//
// 时装不在 selection 的 Rows（仅 3174 条基本装备）里，所以优先走 Full 全量
// 目录（bootstrap 已挂载）：Full.Definition 直接取源脚本原始字段，不做
// "[import script] 链补齐"——那条链只补 [rarity]/[equipment type]/[durability]
// 等少数键，会把 [avatar type select] / [emblem socket default] 丢掉，补孔
// 就会变成 0 孔。只有没有 Full 时才回退 selection 自身，读不到即返回 0 孔。
func (c *EquipmentCatalog) DefaultAvatarSockets(id uint32) []byte {
	opts := make([]byte, avatarSocketOptionsSize)
	if c == nil {
		return opts
	}
	var d EquipmentDefinition
	if c.Full != nil {
		dd, err := c.Full.Definition(id)
		if err == nil {
			d = dd
		}
	} else {
		dd, err := c.definitionResolved(id, 0)
		if err == nil {
			d = dd
		}
	}
	if d.ID == 0 {
		return opts
	}
	for i, s := range defaultSocketsFromFields(d.Fields) {
		if i >= 5 {
			break
		}
		binary.LittleEndian.PutUint16(opts[i*6:], s)
	}
	return opts
}

// isSocketText 判断 token 是否文本型。PVF tokenizer 里普通字符串为
// Type 3、`[方括号]` 标记文本为 Type 6（实测 `[C socket]` 为 t6），
// 两者都带 Text。只有这两类才可能是 socket 名。
func isSocketText(t pvf.Token) bool { return t.Type == 3 || t.Type == 6 }

// defaultSocketsFromFields 从时装定义提取默认孔 socket 类型序列。
func defaultSocketsFromFields(fields map[string][]pvf.Token) []uint16 {
	var sockets []uint16
	// 1) [emblem socket default] 段（光环等专用字段）。
	if cells := fields["[emblem socket default]"]; len(cells) > 0 {
		for _, c := range cells {
			if isSocketText(c) {
				if m, ok := emblemSocketMask(c.Text); ok {
					sockets = append(sockets, m)
				}
			}
		}
	}
	if len(sockets) > 0 {
		return sockets
	}
	// 2) [avatar type select] 尾部 "N [socket1] [socket2]…"。
	// 实测（101500005 / 115500002 / acap_clonegg）布局：孔数 N 是倒数
	// 第 k+1 个 token（Type 0），其后紧跟 k 个文本 socket 名
	// （普通上衣 `2 [C socket] [C socket]`、115 时装 `3 [S socket] [C socket] [C socket]`）。
	cells := fields["[avatar type select]"]
	n := len(cells)
	k := 0
	for i := n - 1; i >= 0 && isSocketText(cells[i]); i-- {
		k++
	}
	if k > 0 {
		idx := n - k - 1
		if idx >= 0 && cells[idx].Type == 0 && int(cells[idx].Value) == k {
			for i := idx + 1; i < n; i++ {
				if m, ok := emblemSocketMask(cells[i].Text); ok {
					sockets = append(sockets, m)
				}
			}
		}
	}
	return sockets
}
