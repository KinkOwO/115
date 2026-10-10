package protocol

import "fmt"

// —— Avatar Preset（装扮预设 / ENUM_NOTIPACKET_AVATAR_PRESET_LIST, noti 1585）——
//
// 读包序列由反汇编客户端 handler（DFO.exe 0x14335A230，地址取自
// analysis/dumps/skin-noti/df7-registrar-table.json）得到：
//
//	ReadU8  -> 当前页索引（1 起）    官方 01
//	ReadU8  -> 页数                  官方 01
//	每页 {
//	  ReadU8               -> 页状态，官方 01
//	  ReadString(max 0x100)-> 页名（u32 长度 + 原始字节），官方 "New Presets"
//	  ReadU8               -> 本页条目数，官方 12
//	  每条目 50 字节 {
//	    u8  序号            官方 0..11
//	    u32 u32 u16 u16 u32
//	    30 字节             （官方样本全 0，内容未逆清）
//	    u16
//	    u8
//	  }
//	}
//	ReadU8 -> 弹窗标志：非 0 时客户端弹 UI2875 并显示 dstr 0x605AE13（0x14335a6eb）
//
// ★ "栏位"是**页**，不是条目：
//   - 编辑窗 `ui/inventory/avatarpresetedit/main.xui` 里 `preset_btn_slot.xui` 实例
//     **恰好 10 个**、静态 ID `presetBtn_0..presetBtn_9`，另有 `ID="STR:tab"` 以及
//     `preset_name` / `preset_name_change_btn`（页能改名 ⇒ 页是有名字的容器）。
//   - handler 收尾把**第 1 字节**写进 UI 属性 0x0F，并拿它与页对象数组逐个比对
//     （0x14335a7c0 `cmp dword [r9], r10d`，步长 0x38）⇒ 第 1 字节 = 当前页索引。
//     ⇒ 官方那 12 条是"每页条目数"，与 10 个按钮对不上；**页签数量由第 2 字节决定**。
//
// 官方帧 = 2 + (1+4+11+1+12*50) + 5 = 624，见 avatar_preset_test.go 的金标准。
const (
	// AvatarPresetPageName 是官方默认页名。
	AvatarPresetPageName = "New Presets"

	// OfficialAvatarPresetPages 是官方默认帧的**页数**（页签数）。
	OfficialAvatarPresetPages byte = 1

	// OfficialAvatarPresetRecordsPerPage 是官方默认帧里每页的条目数（12）。
	// 它与编辑窗那 10 个按钮对不上，语义未逆清，按官方原样转发、不参与解锁。
	OfficialAvatarPresetRecordsPerPage byte = 12

	// MaxAvatarPresetPages 是服务端允许下发的页数上限。客户端真实上限未逆清，
	// 先收口到一个有限值，避免存档被改坏时发出越界包。
	MaxAvatarPresetPages byte = 20

	avatarPresetRecordSize  = 50
	avatarPresetListTrailer = 5
)

// DefaultAvatarPresetPages 是存档里没有 AvatarPresetSlots 时下发的页数（1）。
func DefaultAvatarPresetPages() byte { return OfficialAvatarPresetPages }

// AvatarPresetList 构造 noti1585 ENUM_NOTIPACKET_AVATAR_PRESET_LIST 的包体。
//
// pages 是**页签数**（1..MaxAvatarPresetPages）。每页都用官方页名与官方条目数，
// 第 1 字节固定为第 1 页 —— 不做"自动跳到新解锁页"，免得改变客户端的选中态。
func AvatarPresetList(pages byte) ([]byte, error) {
	if pages == 0 || pages > MaxAvatarPresetPages {
		return nil, fmt.Errorf("invalid avatar preset pages %d", pages)
	}
	name := AvatarPresetPageName
	page := make([]byte, 0, 1+4+len(name)+1+int(OfficialAvatarPresetRecordsPerPage)*avatarPresetRecordSize)
	page = append(page, 1) // 页状态，官方定值
	page = append(page, add32(nil, uint32(len(name)))...)
	page = append(page, name...)
	page = append(page, OfficialAvatarPresetRecordsPerPage)
	record := make([]byte, avatarPresetRecordSize)
	for i := byte(0); i < OfficialAvatarPresetRecordsPerPage; i++ {
		record[0] = i
		page = append(page, record...)
	}
	out := make([]byte, 0, 2+int(pages)*len(page)+avatarPresetListTrailer)
	out = append(out, 1, pages) // 当前页索引=1、页数
	for i := byte(0); i < pages; i++ {
		out = append(out, page...)
	}
	// 官方尾部 5 字节：弹窗标志 + 4 字节未消费字段，原样补 0。
	return append(out, make([]byte, avatarPresetListTrailer)...), nil
}
