package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
)

type MailSendItem struct {
	List             byte
	Slot             uint16
	Template, Amount uint32
}

type MailSendRequest struct {
	Recipient, Text string
	Gold            uint32
	Items           []MailSendItem
	Special         uint32
	Server          byte
}

type MailClaimRequest struct {
	Kind byte
	IDs  []uint64
}

type MailStatusRequest struct {
	IDs    []uint64
	Status uint16
}

// 邮件字符串经 0x146D76080 编为 u32 长度和 UTF-8 字节；接收侧
// 0x146D78070 对名字和正文分别使用 30、513 字节缓冲区。
type mailReader struct {
	p   []byte
	off int
	err error
}

func (r *mailReader) take(n int) []byte {
	if r.err != nil || n < 0 || n > len(r.p)-r.off {
		r.err = fmt.Errorf("邮件请求字段被截断")
		return make([]byte, max(0, min(n, 8)))
	}
	b := r.p[r.off : r.off+n]
	r.off += n
	return b
}
func (r *mailReader) u8() byte    { return r.take(1)[0] }
func (r *mailReader) u16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *mailReader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *mailReader) u64() uint64 { return binary.LittleEndian.Uint64(r.take(8)) }
func (r *mailReader) text(limit int) string {
	n := r.u32()
	if r.err != nil || n > uint32(limit) || int(n) > len(r.p)-r.off {
		r.err = fmt.Errorf("邮件字符串长度无效")
		return ""
	}
	s := string(r.take(int(n)))
	if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		r.err = fmt.Errorf("邮件字符串编码无效")
	}
	return s
}
func (r *mailReader) ids() []uint64 {
	n := r.u32()
	if r.err != nil || n == 0 || n > 255 || uint64(n)*8 > uint64(len(r.p)-r.off) {
		r.err = fmt.Errorf("邮件编号数量无效")
		return nil
	}
	ids := make([]uint64, n)
	seen := map[uint64]bool{}
	for i := range ids {
		ids[i] = r.u64()
		if ids[i] == 0 || ids[i] > 0x7fffffffffffffff || seen[ids[i]] {
			r.err = fmt.Errorf("邮件编号无效或重复")
		}
		seen[ids[i]] = true
	}
	return ids
}
func (r *mailReader) finish() error {
	if r.err != nil {
		return r.err
	}
	return padding(r.p[r.off:], 16)
}

// 0x145FC97E0：94 固定一行（无附件时全零），315 多一个 u8 行数；
// 0x145FBEB70 构造的每行依次是容器、槽位、模板、数量。
func DecodeMailSend(id uint16, p []byte) (MailSendRequest, error) {
	r := mailReader{p: p}
	out := MailSendRequest{Recipient: r.text(19)}
	out.Gold = r.u32()
	n := 1
	if id == 315 {
		n = int(r.u8())
	} else if id != 94 {
		return out, fmt.Errorf("邮件发送命令无效")
	}
	if n > 10 || n == 0 {
		return out, fmt.Errorf("邮件附件数量超出客户端上限")
	}
	seen := map[[2]uint16]bool{}
	for i := 0; i < n; i++ {
		v := MailSendItem{r.u8(), r.u16(), r.u32(), r.u32()}
		if v == (MailSendItem{}) && id == 94 {
			continue
		}
		key := [2]uint16{uint16(v.List), v.Slot}
		if v.Template == 0 || v.Amount == 0 || seen[key] {
			return out, fmt.Errorf("邮件附件为空或槽位重复")
		}
		seen[key] = true
		out.Items = append(out.Items, v)
	}
	out.Text = r.text(512)
	out.Special = r.u32()
	out.Server = r.u8()
	if strings.TrimSpace(out.Recipient) == "" || out.Recipient != strings.TrimSpace(out.Recipient) {
		return out, fmt.Errorf("收件人名字无效")
	}
	if out.Gold == 0 && len(out.Items) == 0 && out.Text == "" {
		return out, fmt.Errorf("不能发送空邮件")
	}
	return out, r.finish()
}

// 0x145FD6A50/0x145FDEF80：类型 u8、数量 u32、附件编号 u64[]。
func DecodeMailClaim(p []byte) (MailClaimRequest, error) {
	r := mailReader{p: p}
	out := MailClaimRequest{Kind: r.u8()}
	out.IDs = r.ids()
	return out, r.finish()
}

// 0x145FD3C90/0x145FDEC60：数量 u32、编号 u64[]、状态 u16。
// 原生消费 0 为删除、2 为已读、3 为保存。
func DecodeMailStatus(p []byte) (MailStatusRequest, error) {
	r := mailReader{p: p}
	out := MailStatusRequest{IDs: r.ids()}
	out.Status = r.u16()
	if out.Status != 0 && out.Status != 2 && out.Status != 3 {
		return out, fmt.Errorf("邮件状态操作无效")
	}
	return out, r.finish()
}

func DecodeMailRecipient(p []byte) (string, byte, error) {
	r := mailReader{p: p}
	name := r.text(19)
	server := r.u8()
	if name == "" || strings.TrimSpace(name) != name {
		return name, server, fmt.Errorf("收件人名字无效")
	}
	return name, server, r.finish()
}

// CMD781：0x145FCB8A0 只写服务器 u8；实机请求为该字节加 7 字节零填充。
func DecodeMailServerCharacters(p []byte) (byte, error) {
	if len(p) != 8 {
		return 0, fmt.Errorf("邮件角色列表请求长度无效")
	}
	if err := padding(p[1:], 8); err != nil {
		return 0, fmt.Errorf("邮件角色列表请求填充无效：%w", err)
	}
	return p[0], nil
}

type MailServerCharacter struct {
	Name  string
	Level uint16
}

// NOTI705：0x145310B90 先读服务器和行数（均为 u8），每行依次读取
// UTF-8 名字、u8 状态、u16 等级、5 个 u8、u32、u8。
// 邮箱经 0x145FDFA60/0x145FCC790 填充菜单，只使用服务器、名字和首个状态。
// 普通角色的状态及此菜单不消费的扩展字段置零，不附加未经核实的 CMD781 应答。
func MailServerCharacters(server byte, roles []MailServerCharacter) ([]byte, error) {
	if len(roles) > 255 {
		return nil, fmt.Errorf("邮件角色列表超出协议容量")
	}
	p := []byte{server, byte(len(roles))}
	for _, role := range roles {
		if role.Name == "" || len(role.Name) > 29 || !utf8.ValidString(role.Name) || strings.ContainsRune(role.Name, 0) || strings.TrimSpace(role.Name) != role.Name || role.Level == 0 {
			return nil, fmt.Errorf("邮件角色列表数据无效")
		}
		p = addName(p, role.Name)
		p = add16(append(p, 0), role.Level)
		p = append(p, 0, 0, 0, 0, 0)
		p = append(add32(p, 0), 0)
	}
	return p, nil
}

type MailAttachmentView struct {
	ID, MessageID uint64
	Sender        string
	Gold          uint32
	Record        [CurrentItemRecordSize]byte
	Remaining     uint32
	State         byte
}
type MailTextView struct {
	ID           uint64
	SenderID     uint32
	Sender, Text string
	Remaining    uint32
	Status       uint16
	State        byte
}

// NOTI97 0x14530A3A0；普通装备直接复用 181 字节实例记录。
// 剩余时间单位为秒：0x145FCDDC0 减去 (当前毫秒时钟-接收时钟)/1000。
func MailboxList(items []MailAttachmentView, letters []MailTextView, total uint16) ([]byte, error) {
	if len(items) > 255 || len(letters) > 32767 || total > 32767 {
		return nil, fmt.Errorf("邮箱列表超出协议容量")
	}
	p := []byte{byte(len(items)), 0}
	for _, a := range items {
		if a.ID == 0 || len(a.Sender) > 29 || !utf8.ValidString(a.Sender) {
			return nil, fmt.Errorf("邮件附件快照无效")
		}
		if a.Gold != 0 && binary.LittleEndian.Uint32(a.Record[2:6]) == 0 {
			// 0x14530A7BF 将外层数量写回物品记录 +6，再创建图标对象；
			// 独立 Gold 字段只提供邮件金额。纯金币附件也必须填数量，
			// 否则标题/悬停金额正确，图标却显示 0。这里只改回包副本。
			binary.LittleEndian.PutUint32(a.Record[6:10], a.Gold)
		}
		p = binary.LittleEndian.AppendUint64(p, a.ID)
		p = addName(p, a.Sender)
		p = add32(p, a.Gold)
		p = add32(p, binary.LittleEndian.Uint32(a.Record[2:6]))
		p = append(p, 0)
		p = add32(p, binary.LittleEndian.Uint32(a.Record[6:10]))
		p = add16(p, binary.LittleEndian.Uint16(a.Record[11:13]))
		p = append(p, a.Record[13])
		p = add32(p, 0)
		p = append(p, 0, 0)
		p = add16(p, 0)
		p = append(p, a.Record[:]...)
		p = add32(p, a.Remaining)
		p = binary.LittleEndian.AppendUint64(p, a.MessageID)
		p = append(p, a.State)
	}
	p = add16(p, total)
	p = add16(p, uint16(len(letters)))
	for _, m := range letters {
		if m.ID == 0 || len(m.Sender) > 29 || len(m.Text) > 512 || !utf8.ValidString(m.Sender) || !utf8.ValidString(m.Text) {
			return nil, fmt.Errorf("邮件正文快照无效")
		}
		p = binary.LittleEndian.AppendUint64(p, m.ID)
		p = add32(p, m.SenderID)
		p = addName(p, m.Sender)
		p = addName(p, m.Text)
		p = add32(p, m.Remaining)
		p = add16(p, m.Status)
		p = append(p, m.State)
		p = add32(p, 0)
	}
	return p, nil
}

func MailboxOpenReady(total uint16) []byte { return add16([]byte{1, 0}, total) }
func MailboxAlarm(count uint16) []byte     { return add16(nil, count) }

type MailClaimResult struct {
	MessageID, AttachmentID uint64
	Code                    uint32
}

// 0x1452830D0：成功、类型、u32 结果数，每行两个 u64 和一个 u32。
func MailClaimReply(kind byte, rows []MailClaimResult) []byte {
	p := add32([]byte{1, kind}, uint32(len(rows)))
	for _, r := range rows {
		p = binary.LittleEndian.AppendUint64(p, r.MessageID)
		p = binary.LittleEndian.AppendUint64(p, r.AttachmentID)
		p = add32(p, r.Code)
	}
	return p
}
func MailStatusReply(ids []uint64, status uint16) []byte {
	p := add32([]byte{1}, uint32(len(ids)))
	for _, id := range ids {
		p = binary.LittleEndian.AppendUint64(p, id)
		p = add16(p, status)
	}
	return p
}

// 0x145287FB0：名字、服务器、等级、职业及标志，最后为服务器名称。
func MailRecipientReply(name string, server byte, level uint16, profession, advancement byte) []byte {
	p := addName([]byte{1}, name)
	p = append(p, server)
	p = add16(p, level)
	p = append(p, profession, advancement, 0, 0, 0, 0)
	return addName(p, "本地")
}

// DecodeMailboxOpen 解码 CMD96。当前客户端 0x145FDE830 经
// 0x146D75CC0 写入一个类型字节；实机请求补齐为 16 字节。
func DecodeMailboxOpen(p []byte) (byte, error) {
	if len(p) != 16 {
		return 0, fmt.Errorf("邮箱打开请求应为 1 字节类型及 16 字节对齐填充")
	}
	if err := padding(p[1:], 16); err != nil {
		return 0, fmt.Errorf("邮箱打开请求填充无效：%w", err)
	}
	return p[0], nil
}
