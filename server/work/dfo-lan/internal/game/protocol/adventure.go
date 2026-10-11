package protocol

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// CMD1395：0x143C76780 写入目标角色 u16、查询类型 u32。
// 打开自己的信息页时，客户端依次请求类型 2、0；两份都必须应答。
type AdventureRequest struct {
	Target uint16
	Kind   uint32
}

func DecodeAdventureRequest(p []byte) (AdventureRequest, error) {
	var req AdventureRequest
	if len(p) < 6 {
		return req, fmt.Errorf("冒险团查询字段不完整")
	}
	req.Target = binary.LittleEndian.Uint16(p)
	req.Kind = binary.LittleEndian.Uint32(p[2:])
	if req.Target == 0 || req.Target == 0xffff || req.Kind > 2 {
		return req, fmt.Errorf("冒险团查询目标或类型无效")
	}
	return req, padding(p[6:], 16)
}

type AdventureCharacter struct {
	Profession, Advancement byte
	Level                   uint32
	CharacterID             uint32
	Name                    string
	Server                  byte
	Awakening               uint16
}

type AdventureInfo struct {
	Name                     string
	CreatedDate              uint32 // YYYYMMDD，0x143C7529B 按 10000/100 拆分。
	Level                    uint32
	Experience               uint64
	CharacterCount           uint16
	ConnectionDays           uint32
	RecommendedDungeonClears uint32
	BestHonor                *AdventureCharacter
	Characters               []AdventureCharacter
	Points                   [5]uint32
	PointExperience          [5]uint32
	Purchases                map[uint32]uint32
}

const (
	adventureCharacterSize = 0x34
	adventureCharacterMax  = 100
	adventureRosterSize    = adventureCharacterMax * (adventureCharacterSize + 3)
	adventureDetailSize    = 0x1418
)

// 角色代表名由 0x143C6A900 的 MultiByteToWideChar(CP_ACP) 消费，
// 团名由 0x146D78070 消费。均须限制编码后的字节数，不能截断汉字。
func adventureText(s string, maxBytes int) ([]byte, error) {
	if s == "" || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return nil, fmt.Errorf("冒险团名称无效")
	}
	b, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if err != nil || len(b) > maxBytes {
		return nil, fmt.Errorf("冒险团名称超出客户端编码或长度限制")
	}
	return b, nil
}

func adventureRoster(rows []AdventureCharacter) ([]byte, error) {
	if len(rows) > adventureCharacterMax {
		return nil, fmt.Errorf("冒险团职业代表超过客户端容量")
	}
	b := make([]byte, adventureRosterSize)
	seen := map[[2]byte]bool{}
	for i, row := range rows {
		key := [2]byte{row.Profession, row.Advancement}
		if row.Profession > 16 || row.Advancement == 0 || row.Advancement > 15 || row.Awakening > 3 || row.Level == 0 || row.CharacterID == 0 || seen[key] {
			return nil, fmt.Errorf("冒险团职业代表数据无效或重复")
		}
		seen[key] = true
		name, err := adventureText(row.Name, 29)
		if err != nil {
			return nil, err
		}
		r := b[i*adventureCharacterSize : (i+1)*adventureCharacterSize]
		r[0], r[1] = row.Profession, row.Advancement
		binary.LittleEndian.PutUint32(r[4:], row.Level)
		binary.LittleEndian.PutUint32(r[8:], row.CharacterID)
		copy(r[12:42], name)
		r[42] = row.Server
		binary.LittleEndian.PutUint16(r[44:], row.Awakening)
		// 尚未独立存档的历史收藏不冒充已领取；现有职业代表由上面的
		// 52 字节记录显示，后面的 100 个收藏三元组保持原生初始值。
	}
	return b, nil
}

// CMD1395 成功体由 0x143C6C960 消费；通用命令分发先取成功字节。
// 身份顺序为 u8 服务器、u32 频道（NOTI2435 的 0x1452C9140 可交叉核对）。
// 0x146D77F50 读取 u32 长度再原样复制字节，不是压缩块或逐字段读数。
func AdventureResponse(req AdventureRequest, server byte, channel uint32, info AdventureInfo) ([]byte, error) {
	if req.Target == 0 || req.Target == 0xffff || req.Kind > 2 || info.Level < 1 {
		return nil, fmt.Errorf("冒险团回复参数无效")
	}
	roster, err := adventureRoster(info.Characters)
	if err != nil {
		return nil, err
	}
	p := add32([]byte{1}, req.Kind)
	p = add16(add32(append(p, server), channel), req.Target)
	if req.Kind == 2 {
		p = append(add32(p, uint32(len(roster))), roster...)
		// 143C6CB6A 在读取职业列表后继续调用 14259A170 消费包尾，
		// 不是只刷新界面。自己的资料分支依次读取：两组 u32 记录数、
		// u32 状态、i16 状态、f32 状态；空记录也必须保留全部 18 字节。
		// 读取点：14259A5D3/A708/AD4D/AD86/ADC8；查看他人的分支
		// 14259A1D0/A2EB/A4D4/A50D/A551 使用同样的空记录布局。
		// 这些记录尚无存档来源，明确发送空集合和初始状态，不伪造完成记录。
		p = add32(p, 0)
		p = add32(p, 0)
		p = add32(p, 0)
		p = add16(p, 0)
		return add32(p, 0), nil
	}
	name, err := adventureText(info.Name, 16)
	if err != nil {
		return nil, err
	}
	// 0x143C74D90：日期、四个未知状态字节、完整详情块、团名、
	// u16 跨区记录数、u32/u16/u8 尾字段。未知玩法维持未开通状态。
	p = append(add32(p, info.CreatedDate), 0, 0, 0, 0)
	detail := make([]byte, adventureDetailSize+adventureRosterSize)
	binary.LittleEndian.PutUint32(detail, info.Level)
	binary.LittleEndian.PutUint64(detail[8:], info.Experience)
	// 143C754D3 将 +0x10..+0x47 的账号摘要复制到资料对象 +0xA0，
	// 其中 +0x14 为连续登录天数（14394BA2B 读取资料 +0xA4）。
	// 143C74FF6 从 +0x44 取 u16 角色总数，143C75487 写到资料 +0x58，
	// 14394B8B9 将它显示在总角色数控件；不能用去重后的职业代表数代替。
	// 143C754D3 将详情块+0x10复制到资料+0xA0；143947088读取此u32
	// 作为熟练度第0项（推荐地下城通关）的次数，不是连续登录天数。
	binary.LittleEndian.PutUint32(detail[0x10:], info.RecommendedDungeonClears)
	binary.LittleEndian.PutUint32(detail[0x14:], info.ConnectionDays)
	binary.LittleEndian.PutUint16(detail[0x44:], info.CharacterCount)
	if row := info.BestHonor; row != nil {
		if row.Profession > 16 || row.Advancement < 1 || row.Advancement > 15 || row.Awakening > 3 || row.Level == 0 {
			return nil, fmt.Errorf("冒险团主力角色数据无效")
		}
		// 14394D560 消费资料对象+0xC8；2331从同一职业代表的
		// UI对象+0/+4/+0x34/+8取得职业、转职、觉醒、显示等级。
		for _, offset := range []int{0x38, 0x1410} {
			// 143C75BBC/E34另将详情块+0x1410的8字节复制到资料+0x17C；
			// 设置窗口1411209D1从此处恢复已选项，遗漏它会再次显示未设置。
			detail[offset], detail[offset+1], detail[offset+2] = row.Profession, row.Advancement, byte(row.Awakening)
			binary.LittleEndian.PutUint32(detail[offset+4:], row.Level)
		}
	}
	// 143C75472/7A 按等级 +1 从客户端源表查下一级需求；只有 NOTI1337
	// 的独立 24 字节经验结构才在 +0x10 携带下一级经验，不能混用布局。
	// 0x14394916C/174 分别读取余额和兑换进度，不能当作单个 u64 积分。
	for i, point := range info.Points {
		binary.LittleEndian.PutUint32(detail[0x48+i*8:], point)
		binary.LittleEndian.PutUint32(detail[0x4c+i*8:], info.PointExperience[i])
	}
	if len(info.Purchases) > 512 {
		return nil, fmt.Errorf("冒险团限购记录超过客户端容量")
	}
	ids := make([]uint32, 0, len(info.Purchases))
	for id := range info.Purchases {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		binary.LittleEndian.PutUint32(detail[0x70+i*8:], id)
		binary.LittleEndian.PutUint32(detail[0x74+i*8:], info.Purchases[id])
	}
	// 0x143C6F510 的 100 个条目以 -1 表示未登记，不能全部填零。
	for i := 0; i < 100; i++ {
		binary.LittleEndian.PutUint32(detail[0x10ec+i*8:], 0xffffffff)
	}
	copy(detail[adventureDetailSize:], roster)
	p = append(add32(p, uint32(len(detail))), detail...)
	p = append(add32(p, uint32(len(name))), name...)
	p = add16(p, 0)
	p = add32(p, 0)
	p = add16(p, 0)
	return append(p, 0), nil
}

// CMD1406 的 0x143C77460：类别 u8、商品 u32、购买数量 u32。
func DecodeAdventurePurchase(p []byte) (category byte, item, count uint32, err error) {
	if len(p) < 9 {
		return 0, 0, 0, fmt.Errorf("冒险团购买字段不完整")
	}
	category, item, count = p[0], binary.LittleEndian.Uint32(p[1:]), binary.LittleEndian.Uint32(p[5:])
	if category >= 5 || item < 2 || count == 0 {
		return 0, 0, 0, fmt.Errorf("冒险团购买参数无效")
	}
	err = padding(p[9:], 16)
	return
}

// NOTI1337 / 0x143C74700：目标、24 字节等级/经验结构、四个未知状态字节。
func AdventureExperience(target uint16, level uint32, experience, next uint64) []byte {
	body := make([]byte, 24)
	binary.LittleEndian.PutUint32(body, level)
	binary.LittleEndian.PutUint64(body[8:], experience)
	binary.LittleEndian.PutUint64(body[16:], next)
	p := append(add32(add16(nil, target), 24), body...)
	return append(p, 0, 0, 0, 0)
}

// CMD2331：14111FD40发送12字节原生结构，14111DD40发送kind=2恢复自动选择。
// 第3字节是结构对齐空隙，不是业务字段；原生sender未初始化，不校验其值。
// 2026-09-28用户保存向量：000500000100000001000000，末尾4字节为传输填充。
func DecodeAdventureBestHonor(p []byte) (row AdventureCharacter, automatic bool, err error) {
	if len(p) < 12 {
		return row, false, fmt.Errorf("冒险团主力角色设置字段不完整")
	}
	if err = padding(p[12:], 16); err != nil {
		return
	}
	switch binary.LittleEndian.Uint32(p[8:]) {
	case 1:
		row.Profession, row.Advancement, row.Awakening = p[0], p[1], uint16(p[2])
		row.Level = binary.LittleEndian.Uint32(p[4:])
		if row.Profession > 16 || row.Advancement < 1 || row.Advancement > 15 || row.Awakening > 3 || row.Level == 0 {
			err = fmt.Errorf("冒险团主力角色设置无效")
		}
	case 2:
		automatic = true
	default:
		err = fmt.Errorf("冒险团主力角色设置操作无效")
	}
	return
}

// 精锐设置使用角色选择列表索引，不能当作角色数据库编号或场景 WireID。
// 142E63EF0发送531字节；142E5A4C0按同样大小读取NOTI1754的每条记录。
// 前12字节含原生未初始化值，不回放整段请求。
// +0x33起是4组各30个i32技能编号，不是装备快照；1879处理器
// 142E5B578按(精锐槽+1)*120+0x33取后三组，交给142E653A0应用技能开关。
type AdventureEliteSelection struct {
	Mode       uint16
	Slots      [3]int32
	APCIndices [3]uint32 // PVF special APC [index]; account-save decoder still rejects NPCs
	SkillUsage [3][30]int32
}

func DecodeAdventureEliteSelection(p []byte) (r AdventureEliteSelection, err error) {
	if len(p) < 531 {
		return r, fmt.Errorf("精锐角色设置字段不完整")
	}
	if err = padding(p[531:], 8); err != nil {
		return
	}
	r.Mode = binary.LittleEndian.Uint16(p[0x0d:])
	if r.Mode < 1 || r.Mode > 4 {
		return r, fmt.Errorf("精锐角色模式无效")
	}
	for i := range r.Slots {
		// 142E5C590只对特殊APC [index]做索引转换。本账号角色的标记和特殊索引均为零。
		if p[0x10+i] != 0 || binary.LittleEndian.Uint32(p[0x17+i*4:]) != 0 {
			return r, fmt.Errorf("精锐设置包含未接入的外部APC索引")
		}
		r.Slots[i] = int32(binary.LittleEndian.Uint32(p[0x27+i*4:]))
		if r.Slots[i] < -1 {
			return r, fmt.Errorf("精锐角色列表索引无效")
		}
		if r.Slots[i] >= 0 {
			for j := range r.SkillUsage[i] {
				r.SkillUsage[i][j] = int32(binary.LittleEndian.Uint32(p[0x33+(i+1)*120+j*4:]))
			}
		}
	}
	return
}

func AdventureEliteSelections(rows []AdventureEliteSelection) ([]byte, error) {
	if len(rows) > 4 {
		return nil, fmt.Errorf("精锐角色模式数量无效")
	}
	p := []byte{byte(len(rows))}
	seen := map[uint16]bool{}
	for _, row := range rows {
		if row.Mode < 1 || row.Mode > 4 || seen[row.Mode] {
			return nil, fmt.Errorf("精锐角色模式无效或重复")
		}
		seen[row.Mode] = true
		body := make([]byte, 531)
		binary.LittleEndian.PutUint16(body[0x0d:], row.Mode)
		// 原生默认结构+0x23起四个i32均为-1；后三个是精锐槽，首个不是随行角色。
		binary.LittleEndian.PutUint32(body[0x23:], ^uint32(0))
		for i, slot := range row.Slots {
			if slot < -1 {
				return nil, fmt.Errorf("精锐角色列表索引无效")
			}
			binary.LittleEndian.PutUint32(body[0x27+i*4:], uint32(slot))
			if index := row.APCIndices[i]; index != 0 {
				if slot != -1 {
					return nil, fmt.Errorf("特殊APC索引不能同时绑定账号角色槽位")
				}
				body[0x10+i] = 1
				binary.LittleEndian.PutUint32(body[0x17+i*4:], index)
			}
			if slot >= 0 {
				for j, skill := range row.SkillUsage[i] {
					binary.LittleEndian.PutUint32(body[0x33+(i+1)*120+j*4:], uint32(skill))
				}
			}
		}
		p = append(p, body...)
	}
	return p, nil
}

// CMD1811由NOTI1754恢复选择后自动发出，只携带u16模式。
func DecodeAdventureEliteLoad(p []byte) (uint16, error) {
	if len(p) < 2 {
		return 0, fmt.Errorf("精锐角色加载字段不完整")
	}
	mode := binary.LittleEndian.Uint16(p)
	if mode < 1 || mode > 4 {
		return 0, fmt.Errorf("精锐角色加载模式无效")
	}
	return mode, padding(p[2:], 8)
}

// 0x14052F19F写u32阶段到17字节原生请求的+13；前13字节不是奖励参数。
func DecodeSeasonReward(p []byte) (uint32, error) {
	if len(p) < 17 {
		return 0, fmt.Errorf("迷雾誓约领奖请求不完整")
	}
	if err := padding(p[17:], 8); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(p[13:17]), nil
}

// CMD507原生失败分支0x1459372E1：action357为迷雾经验道具。
const SeasonCapsuleAction uint32 = 357

func DecodeSeasonCapsule(p []byte) (uint16, error) {
	if len(p) != 59 && len(p) != 64 {
		return 0, fmt.Errorf("迷雾经验道具请求长度无效")
	}
	if p[2] != 0 || binary.LittleEndian.Uint32(p[7:11]) != SeasonCapsuleAction {
		return 0, fmt.Errorf("迷雾经验道具容器或操作无效")
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot < 2 {
		return 0, fmt.Errorf("迷雾经验道具槽位无效")
	}
	return slot, nil
}

// 0x14593512A与0x145936E3B：成功/失败均继续读u16槽、u8容器、u32动作。
func SeasonCapsuleReply(slot uint16, success bool) []byte {
	p := []byte{1}
	if !success {
		p = Refusal(3)
	}
	tail := make([]byte, 7)
	binary.LittleEndian.PutUint16(tail, slot)
	binary.LittleEndian.PutUint32(tail[3:], SeasonCapsuleAction)
	return append(p, tail...)
}

// 0x14028D1A0：u32保留值0和u8选择索引。
func DecodeSeasonOath(p []byte) (byte, error) {
	if len(p) < 5 || binary.LittleEndian.Uint32(p) != 0 {
		return 0, fmt.Errorf("誓约装备获取请求无效")
	}
	if err := padding(p[5:], 8); err != nil {
		return 0, err
	}
	return p[4], nil
}
