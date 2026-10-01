package inventory

// 增幅券（Amplifying Ticket）：把装备**直接**增幅到券上写死的那个等级。
//
// 与「增幅升级」的区别（两者都是 CMD80 mode=1，别混淆）：
//   - 增幅升级（amplify_upgrade.go）：在 NPC Klonter 处消耗矛盾结晶体 3242 + 金币，每级 +1，
//     成功率失败还会降级甚至摧毁；
//   - 增幅券（本文件）：背包里的道具，窗口「券位」放的就是这张券，
//     **跳级**到券的目标等级，失败只消耗券、等级不变（与强化券同一套路）。
//
// PVF 段是 [equipment amplify reinforcement ticket]，四个 token：
//
//	<目标等级> <成功率百分比> 'fixed' -1
//
// 例如 10_amplifying_ticket_probability90 = [10, 90, 'fixed', -1]，
// 即「把装备增幅到 +10，成功率 90%」。段值格式与 [equipment reinforcement ticket]
// （普通强化券）完全同构，所以两条路径的校验也基本对称。
//
// 前置条件：装备必须**已经有次元属性**（record[19] != 0，即先用增幅书打过红字）。
// 没有次元属性时等级字节会被客户端渲染成「强化 +N」而不是「增幅 +N」，
// 增幅券对它没有意义 —— 这与增幅升级（applyAmplifyUpgrade）保持一致的判定。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
)

const (
	amplifyTicketModel   = "amplify-ticket-v1"
	amplifyTicketSection = "[equipment amplify reinforcement ticket]"
)

var amplifyTickets map[uint32]reinforcementTicket

// IsAmplifyTicket 报告模板是否是一张增幅券（只看有没有那个段，不校验等级是否可用）。
func IsAmplifyTicket(template uint32) bool {
	_, ok := amplifyTickets[template]
	return ok
}

// LoadAmplifyTickets 读取增幅券规则（scripts/export_amplify_tickets.py 的导出产物）。
// 文件缺失时该功能整体不可用（不影响普通强化券、增幅升级与打红字）。
func LoadAmplifyTickets(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var c struct {
		Version      int                            `json:"version"`
		ClientSHA256 string                         `json:"client_sha256"`
		Items        map[uint32]reinforcementTicket `json:"items"`
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return err
	}
	if c.Version != 1 || len(c.ClientSHA256) != 64 || len(c.Items) == 0 {
		return fmt.Errorf("增幅券源目录无效")
	}
	// 这里**只校验结构**，不校验目标等级范围：库里存在等级写到 16..20 的异常券
	// （数量很少），若在这里判等级就会把 1629 张券整体拒绝掉，一张都用不了。
	// 越界的券改由 amplifyTicketSpec 在使用时返回 ok=false 单独拒绝。
	for id, item := range c.Items {
		if id < 2 || item.Path == "" || len(item.Fields[amplifyTicketSection]) != 4 {
			return fmt.Errorf("增幅券 %d 源定义不完整", id)
		}
	}
	amplifyTickets = c.Items
	return nil
}

// amplifyTicketSpec 读出券的目标等级与成功率。
// ok=false 表示这张券不被支持（越界等级、不是 fixed 券、格式异常）。
func amplifyTicketSpec(template uint32) (level byte, percent int, ok bool) {
	rule, exists := amplifyTickets[template]
	if !exists {
		return 0, 0, false
	}
	v := rule.Fields[amplifyTicketSection]
	if len(v) != 4 || v[0].Type != 0 || v[1].Type != 0 || v[2].Type != 6 ||
		v[2].Text != "fixed" || v[3].Type != 0 || v[3].Value != -1 {
		return 0, 0, false
	}
	// 等级上限与强化券同为 1..15：一个等级字节只有低五位，且客户端实机
	// 收到更高的值会出现 ADD_HACKTYPE_CNT 并锁死窗口。
	if v[0].Value < 1 || v[0].Value > 15 {
		return 0, 0, false
	}
	if v[1].Value < 0 || v[1].Value > 100 {
		return 0, 0, false
	}
	return byte(v[0].Value), int(v[1].Value), true
}

// AmplifyTicketReceipt 是一次增幅券使用的结果。
type AmplifyTicketReceipt struct {
	Request   protocol.ReinforcementRequest `json:"request"`
	Ticket    uint32                        `json:"ticket"`
	Remaining uint32                        `json:"remaining"`
	// Old / Level 是增幅前后的等级；Result 0=成功 1=失败（等级不变）。
	Old, Level, Result byte
	AmplifyType        byte         `json:"amplify_type"`
	SuccessPercent     int          `json:"success_percent"`
	Equipment          BagEquipment `json:"equipment"`
	EquipmentSpace     byte         `json:"equipment_space"`
}

// ApplyAmplifyTicket 处理 CMD80 mode=1 且窗口里放的是增幅券的情况。
func (s *WearService) ApplyAmplifyTicket(ctx context.Context, role storage.Character, key string, r protocol.ReinforcementRequest) (storage.Character, AmplifyTicketReceipt, error) {
	var out AmplifyTicketReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("增幅券需要有效装备目录及角色存档")
	}
	if r.Mode != 1 {
		return role, out, fmt.Errorf("增幅券请求的 mode 必须是 1，收到 %d", r.Mode)
	}
	if len(amplifyTickets) == 0 {
		return role, out, fmt.Errorf("增幅券规则未装载")
	}
	saved, _, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, amplifyTicketModel, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		next, receipt, e := s.applyAmplifyTicket(current, r)
		if e != nil {
			return nil, nil, e
		}
		encoded, e := json.Marshal(receipt)
		if e != nil {
			return nil, nil, e
		}
		return next, encoded, nil
	})
	if err != nil {
		return role, out, err
	}
	receipt, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err == nil {
		err = json.Unmarshal(receipt, &out)
	}
	saved.WireID = role.WireID
	return saved, out, err
}

func (s *WearService) applyAmplifyTicket(role storage.Character, r protocol.ReinforcementRequest) (json.RawMessage, AmplifyTicketReceipt, error) {
	var out AmplifyTicketReceipt
	fail := func(reason string) (json.RawMessage, AmplifyTicketReceipt, error) {
		return nil, out, fmt.Errorf("%s", reason)
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	// 1) 券：窗口「券位」放的就是这张增幅券。
	ticketIndex := -1
	for i, item := range bag.Items {
		if item.Slot == r.TicketSlot {
			ticketIndex = i
			break
		}
	}
	if ticketIndex < 0 || bag.Items[ticketIndex].Amount == 0 {
		return fail("增幅券不在所属角色背包")
	}
	item := bag.Items[ticketIndex]
	target, percent, ok := amplifyTicketSpec(item.Template)
	if !ok {
		return fail(fmt.Sprintf("窗口里放的不是可用的增幅券（模板 %d，目标等级需在 1..15 内）", item.Template))
	}
	rule, hasRule := amplifyTickets[item.Template]
	if !hasRule {
		return fail("未找到这张物品的增幅券规则")
	}
	// 尚未核实的专用券限制不能静默忽略；拒绝时事务不扣除任何物品。
	for _, tag := range []string{"[need material]", "[used trade type]", "[mod attr available level]", "[advanced enchant item by ticket window type]", "[action usable place]"} {
		if len(rule.Fields[tag]) > 0 {
			return fail("该专用增幅券包含尚未支持的限制：" + tag)
		}
	}
	if item.ExpireTime >= 946684800 && item.ExpireTime != 2147483647 && protocol.StoredItemExpired(item.ExpireTime, time.Now().Unix()) {
		if _, usable := rule.Fields["[usable expired item]"]; !usable {
			return fail("增幅券已经过期")
		}
	}
	var state struct {
		Level byte `json:"level"`
	}
	if err = json.Unmarshal(role.State, &state); err != nil {
		return nil, out, err
	}
	if min := rule.Fields["[minimum level]"]; len(min) > 0 && (len(min) != 1 || min[0].Type != 0 || int32(state.Level) < min[0].Value) {
		return fail("角色等级不符合增幅券要求")
	}
	// 没有 [usable job] 段视为不限制职业（强化券那边缺段会误拒，这里不照抄）。
	if jobs := rule.Fields["[usable job]"]; len(jobs) > 0 {
		job, jobKnown := s.Professions.Professions[role.Profession]
		allowedJob := false
		for _, j := range jobs {
			if j.Type == 6 && (j.Text == "[all]" || jobKnown && j.Text == job.Job) {
				allowedJob = true
			}
		}
		if !allowedJob {
			return fail("角色职业不符合增幅券要求")
		}
	}

	// 2) 目标装备：先按请求里的空间找（0 背包 / 3 已穿戴），找不到再退回另一侧。
	space := r.EquipmentSpace
	items := bag.Equipment
	if space == 3 {
		items = bag.Worn
	}
	gearIndex := -1
	for i, gear := range items {
		if gear.Slot == r.EquipmentSlot && gear.Template == r.EquipmentTemplate {
			gearIndex = i
			break
		}
	}
	if gearIndex < 0 {
		space = 0
		items = bag.Equipment
		if r.EquipmentSpace != 3 {
			space = 3
			items = bag.Worn
		}
		for i, gear := range items {
			if gear.Slot == r.EquipmentSlot && gear.Template == r.EquipmentTemplate {
				gearIndex = i
				break
			}
		}
	}
	if gearIndex < 0 {
		return fail("目标装备不在背包或已穿戴槽位里")
	}
	gear := items[gearIndex]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return fail("目标装备类型无效")
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return fail("此类物品不能使用增幅券")
	}
	if ids := rule.Fields["[reinforcement usable item list]"]; len(ids) > 0 {
		matched := false
		for _, id := range ids {
			if id.Type == 0 && uint32(id.Value) == gear.Template {
				matched = true
			}
		}
		if !matched {
			return fail("目标装备不在增幅券允许列表")
		}
	}

	row := EquipmentRow(gear)
	// 3) 前置：必须有次元属性（打过红字），否则等级会被渲染成「强化 +N」。
	ampType := row[amplifyTypeOffset]
	if ampType == 0 {
		return fail("该装备没有次元属性，不能增幅（先用增幅书打红字）")
	}
	// 已封装的装备不给用（offset 13 是 isSealed）。注意不能套用强化券那条
	// 「row[19]/row[20] 必须为 0」的检查 —— 那两位正是增幅必需的次元属性类型与数值。
	if row[13] != 0 {
		return fail("已封装的装备不能使用增幅券")
	}
	old := byte(amplifyLevel(row[:]))
	if int(old) >= int(target) {
		return fail(fmt.Sprintf("当前增幅等级 +%d 已达到或超过券的目标等级 +%d", old, target))
	}

	// 4) 判定：固定券失败只消耗券，不降级也不摧毁。
	roll, err := amplifyUpgradeRandomInt(100)
	if err != nil {
		return nil, out, err
	}
	level, result := old, byte(1)
	if roll < percent {
		level, result = target, 0
		setAmplifyLevel(row[:], level)
	}
	gear.Record = append([]byte(nil), row[:]...)
	out = AmplifyTicketReceipt{
		Request: r, Ticket: item.Template, Remaining: item.Amount - 1,
		Old: old, Level: level, Result: result,
		AmplifyType: ampType, SuccessPercent: percent,
		Equipment: gear, EquipmentSpace: space,
	}

	// 5) 扣券、写回装备行。
	remaining := item.Amount - 1
	if remaining == 0 {
		bag.Items = append(bag.Items[:ticketIndex:ticketIndex], bag.Items[ticketIndex+1:]...)
	} else {
		bag.Items[ticketIndex].Amount = remaining
	}
	items = append([]BagEquipment(nil), items...)
	items[gearIndex] = gear
	if space == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}
	next, err := SaveBag(role.State, bag)
	if err != nil {
		return nil, out, err
	}
	return next, out, nil
}
