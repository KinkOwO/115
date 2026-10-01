package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// favorPointUp mirrors [favor level point up]，键是**礼物物品**而非 NPC
// （3033-3037/3262 是小晶块模板 ID，账号材料栏 363-368；[total favor leader
// npc] 里根本没有这几个 ID）。白色实测（2026-09-27）：p[6]=0x6c→槽364→
// 3034 白色小晶块。两个数值是每次送礼的随机点数区间 [min,max]（[extra
// favor gift] 大量 "物品 1 5000 7000" 条目证实 min/max 语义）。未知礼物拒绝。
func favorPointUp(template uint32) (minPoint, maxPoint int64) {
	switch template {
	case 3037: // 无色小晶块
		return 100, 300
	case 3033, 3034, 3035, 3036: // 黑/白/红/蓝
		return 200, 600
	case 3262: // 金色
		return 400, 900
	}
	return 0, 0
}

const favorDailyLimit = 5         // [favor gift limit]（用户要求体验满好感，实际不生效）
const favorOpenLevel = 20         // [favor condition level]
const favorGiftCount uint32 = 100 // [favor gift item count]
const favorNoDailyLimit = true    // 用户要求：去掉每日送礼次数限制，体验从头送到满

// [favor level point down] 0 21 500 / 1 14 1000 / 2 7 1500：好感度三级累计
// 门槛（不送礼按 21/14/7 点每日衰减），1500 为满——好感度名称颜色表也是
// 按 10%~100% 十档设计。
var favorLevels = []int64{500, 1000, 1500}

const favorMaxPoint int64 = 1500

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
	minPoint, maxPoint := favorPointUp(template)
	if minPoint == 0 {
		return nil, fmt.Errorf("template %d is not a valid favor gift", template)
	}
	var state character.State
	if e = json.Unmarshal(w.role.State, &state); e != nil {
		return nil, e
	}
	if int(state.Level) < favorOpenLevel {
		return nil, fmt.Errorf("favor requires level %d", favorOpenLevel)
	}
	// 每次送礼的点数在礼物区间 [min,max] 内随机（PVF 语义）。
	delta := minPoint + rand.Int63n(maxPoint-minPoint+1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	giftTemplate := template
	dailyLimit := favorDailyLimit
	if favorNoDailyLimit {
		dailyLimit = 99999999
	}
	saved, fs, rawMaterials, e := w.store.GiveFavor(ctx, w.account, w.role.ID, w.role.ConfigVersion, req.NPCID, storage.FavorGift{
		Day:      time.Now().Format("2006-01-02"),
		Limit:    dailyLimit,
		Levels:   favorLevels,
		MaxPoint: favorMaxPoint,
		Delta:    delta,
		Now:      time.Now(),
	}, func(current storage.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		materials, e := inventory.ReadAccountMaterials(raw)
		if e != nil {
			return nil, nil, e
		}
		materials, _, e = materials.Spend(giftTemplate, favorGiftCount)
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
