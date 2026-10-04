package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// Preserve the user's explicit unlimited daily-gift policy. Source-defined
// costs, eligibility, point ranges and thresholds remain archive facts.
const favorNoDailyLimit = true

// giveFavor handles CMD806。请求 p[6] 编码礼物所在账号材料栏槽位-256
// （0x6f=无色367，0x6c=白色364），据此扣对应材料并按 [favor level point up]
// 结算点数；此前硬编码 3037×100 导致选白色也扣无色。
func (w *worldSession) giveFavor(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil || w.store == nil {
		return nil, fmt.Errorf("favor gift before selection")
	}
	req, e := protocol.DecodeFavorGift(p)
	if e != nil {
		return nil, e
	}
	// 染色(p[0]=1)：剧情角色染色操作，不扣材料、不动好感度。逆向客户端
	// CMD806 响应 parser(0x14528f570) 定论：byte0=result(1)，byte1=op，
	// op=1 走染色分支（npcID u32 + count u8 + count 组 slot/color），本地
	// 写角色颜色后直接提交，不弹任何好感度窗；op=0 才走送礼分支。
	// 故应答为 0x01 + 染色请求体原样回显。此前回 16B 送礼格式
	// FavorGiftAckV5（byte1=0）会被当成送礼，误弹"好感度增加了X"。
	if req.IsDye() {
		ack := protocol.FavorDyeAck(p)
		return []outboundPacket{{"npc_favor_dye_ack", 1, 806, ack}}, nil
	}
	template, ok := inventory.StorageRowTemplate(req.GiftSlot())
	if !ok {
		return nil, fmt.Errorf("favor gift slot %d is not an account material", req.GiftSlot())
	}
	if w.service == nil || w.service.Catalog.Favor == nil {
		return nil, fmt.Errorf("native favor rules unavailable")
	}
	rules := w.service.Catalog.Favor
	minPoint, maxPoint := rules.PointRange(template)
	if minPoint == 0 {
		return nil, fmt.Errorf("template %d is not a valid favor gift", template)
	}
	var state character.State
	if e = json.Unmarshal(w.role.State, &state); e != nil {
		return nil, e
	}
	if state.Level < rules.OpenLevel {
		return nil, fmt.Errorf("favor requires level %d", rules.OpenLevel)
	}
	// 每次送礼的点数在礼物区间 [min,max] 内随机（PVF 语义）。
	delta := minPoint + rand.Int63n(maxPoint-minPoint+1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	giftTemplate := template
	dailyLimit := rules.DailyLimit
	if favorNoDailyLimit {
		dailyLimit = 99999999
	}
	saved, fs, rawMaterials, e := w.store.GiveFavor(ctx, w.account, w.role.ID, w.role.ConfigVersion, req.NPCID, database.FavorGift{
		Day:      time.Now().Format("2006-01-02"),
		Limit:    dailyLimit,
		Levels:   rules.Levels,
		MaxPoint: rules.MaxPoint(),
		Delta:    delta,
		Now:      time.Now(),
	}, func(current database.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		materials, e := inventory.ReadAccountMaterials(raw)
		if e != nil {
			return nil, nil, e
		}
		materials, _, e = materials.Spend(giftTemplate, rules.GiftCount)
		if e != nil {
			return nil, nil, e
		}
		updated, e := materials.Save()
		if e != nil {
			return nil, nil, e
		}
		return current.State, updated, nil
	})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	materials, e := inventory.ReadAccountMaterials(rawMaterials)
	if e != nil {
		return nil, e
	}
	refresh, e := accountMaterialRefreshPackets(materials, saved)
	if e != nil {
		return nil, e
	}
	// 806 回包：48B 安全长度，[0]=1 成功，[4]=好感度总点数。
	// noti194：8B（npcID + 总点数），v5 弹窗版语义。
	// noti195 NPC_MOOD：8B（npcID + 总点数），试探——194 无法更新好感度窗口
	// 百分比，195 是剩余唯一未尝试的好感度通知，可能负责窗口显示。
	// noti195 实测不更新好感度窗口；进城镇全量同步走 NOTI733
	// （FavorPointInfo，见 main.go 入场延迟推送），送礼链路不再发 733。
	point := uint32(fs.Point)
	// 194 携带本次增量 delta（弹窗"好感度增加了{delta}"）
	changed := protocol.FavorChangedLegacy(req.NPCID, uint32(delta))
	// 195 携带总点数——试探是否驱动好感度窗口百分比
	mood := protocol.FavorMood(req.NPCID, point)
	// ack: npcID@2, delta@6（弹窗数值）, point@8（总点数，可能用于百分比）
	ack := protocol.FavorGiftAckV5(req.NPCID, point, uint32(delta))
	packets := []outboundPacket{
		{"npc_favor_mood", 0, 195, mood},
		{"npc_favor_changed", 0, 194, changed},
		{"npc_favor_ack", 1, 806, ack},
	}
	packets = append(packets, refresh...)
	return packets, nil
}
