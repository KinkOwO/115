package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/localgm"
	"dfolan/internal/toolcmd/setlevel"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type onlineGMCall struct {
	ctx      context.Context
	request  localgm.Request
	response chan onlineGMResponse
}
type onlineGMResponse struct {
	matched bool
	result  localgm.Result
}
type onlineGMRegistry struct {
	mu       sync.Mutex
	clients  map[*gameConnection]bool
	database string
}

func (g *onlineGMRegistry) add(c *gameConnection) func() {
	g.mu.Lock()
	if g.clients == nil {
		g.clients = map[*gameConnection]bool{}
	}
	g.clients[c] = true
	g.mu.Unlock()
	return func() { g.mu.Lock(); delete(g.clients, c); g.mu.Unlock() }
}
func (g *onlineGMRegistry) execute(ctx context.Context, req localgm.Request) localgm.Result {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	g.mu.Lock()
	cs := make([]*gameConnection, 0, len(g.clients))
	for c := range g.clients {
		cs = append(cs, c)
	}
	g.mu.Unlock()
	for _, c := range cs {
		call := onlineGMCall{ctx, req, make(chan onlineGMResponse, 1)}
		select {
		case c.gmCalls <- call:
		case <-c.connection.done:
			continue
		case <-ctx.Done():
			return localgm.Failure(504, ctx.Err())
		}
		select {
		case reply := <-call.response:
			if reply.matched {
				return reply.result
			}
		case <-c.connection.done:
			return localgm.Failure(409, fmt.Errorf("角色连接已断开；请刷新后以相同操作编号重试"))
		case <-ctx.Done():
			return localgm.Failure(504, fmt.Errorf("操作结果待确认；请以相同操作编号重试"))
		}
	}
	return localgm.Failure(409, fmt.Errorf("请先在游戏中进入选中的角色，再执行在线GM操作"))
}

type onlineGMRequest struct {
	Account   int64                `json:"account"`
	Character int64                `json:"character"`
	GrantID   string               `json:"grant_id"`
	Reason    string               `json:"reason"`
	Gold      uint32               `json:"gold"`
	Cera      int64                `json:"cera"`
	Items     []database.GrantItem `json:"items"`
	Level     int                  `json:"level"`
	Action    string               `json:"action"`
	Value     uint32               `json:"value"`
}

func (c *gameConnection) handleOnlineGM(call onlineGMCall) {
	var req onlineGMRequest
	if e := json.Unmarshal(call.request.Body, &req); e != nil {
		call.response <- onlineGMResponse{true, localgm.Failure(400, e)}
		return
	}
	if c.worldState == nil || !c.bootstrapped || req.Character != c.selectedCharacterID || req.Account != c.worldState.role.AccountID {
		call.response <- onlineGMResponse{}
		return
	}
	result := c.applyOnlineGM(call.ctx, call.request, req)
	call.response <- onlineGMResponse{true, result}
}
func (c *gameConnection) applyOnlineGM(ctx context.Context, request localgm.Request, req onlineGMRequest) localgm.Result {
	fail := func(e error) localgm.Result { return localgm.Failure(400, e) }
	if req.Account <= 0 || req.Character <= 0 || len(req.GrantID) < 8 || len(req.GrantID) > 128 || strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 500 || len(req.Items) > 100 {
		return fail(fmt.Errorf("账号、角色、操作编号、原因或物品数量无效"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	switch request.Path {
	case "/api/grant":
		if req.Gold == 0 && req.Cera == 0 && len(req.Items) == 0 {
			return fail(fmt.Errorf("没有要发放的内容"))
		}
	case "/api/gm/level":
		if req.Level < 2 || req.Level > 255 || c.progressionService == nil {
			return fail(fmt.Errorf("等级无效"))
		}
	case "/api/gm/command":
		if req.Action != "gold_set" && req.Action != "cera_set" {
			return fail(fmt.Errorf("在线模式仅开放发物品、金币/点券和等级；其他修改请退出游戏后使用"))
		}
	default:
		return fail(fmt.Errorf("未开放的在线操作"))
	}
	if c.lootService == nil || c.gameStore == nil {
		return fail(fmt.Errorf("在线发放目录未就绪"))
	}
	details, _ := json.Marshal(request)
	grant := database.Grant{ID: req.GrantID, AccountID: req.Account, Character: req.Character, Reason: req.Reason, Operator: "gm-local-online", MaxCera: math.MaxInt32, Details: details}
	if request.Path == "/api/grant" {
		grant.Gold, grant.Cera, grant.Items = req.Gold, req.Cera, req.Items
	}
	if request.Path == "/api/gm/command" && req.Action == "cera_set" {
		if req.Value > math.MaxInt32 {
			return fail(fmt.Errorf("点券不能超过2147483647"))
		}
		balance, e := c.gameStore.AccountCera(ctx, req.Account)
		if e != nil {
			return fail(e)
		}
		grant.Cera = int64(req.Value) - int64(balance)
	}
	var threshold uint64
	if request.Path == "/api/gm/level" {
		var e error
		threshold, e = setlevel.LevelThreshold(c.progressionService.Catalog, byte(req.Level))
		if e != nil {
			return fail(e)
		}
	}
	backup, e := database.Backup(ctx, database.SQLiteTarget(c.onlineGM.database), filepath.Join(filepath.Dir(c.onlineGM.database), "gm-backups", "local"), nil)
	if e == nil {
		e = database.VerifyBackup(backup)
	}
	if e != nil {
		return fail(fmt.Errorf("自动备份失败，未修改：%w", e))
	}
	awarder := inventory.Awarder{Catalog: c.lootService.Catalog, Rules: c.lootService.BagRules, Equipment: c.lootService.Equipment}
	projection := c.characters
	if request.Path == "/api/gm/level" {
		projection, e = c.characters.WithEntryProjectionSnapshot(c.worldState.role)
		if e != nil {
			return fail(e)
		}
	}
	var plan []outboundPacket
	result, e := c.gameStore.ApplyGrantTx(ctx, grant, func(_ *database.Tx, role database.Character) (json.RawMessage, json.RawMessage, error) {
		role.WireID = c.worldState.role.WireID
		raw := role.State
		var e error
		switch request.Path {
		case "/api/grant":
			if req.Gold != 0 {
				raw, _, e = awarder.Grant(raw, 0, req.Gold)
				if e != nil {
					return nil, nil, e
				}
			}
			for _, it := range req.Items {
				if it.Template == 0 || it.Amount == 0 {
					return nil, nil, fmt.Errorf("物品编号或数量无效")
				}
				raw, _, e = awarder.Grant(raw, it.Template, it.Amount)
				if e != nil {
					return nil, nil, e
				}
			}
		case "/api/gm/level":
			raw, _, e = setlevel.ApplyLevel(role, byte(req.Level), threshold)
		case "/api/gm/command":
			if req.Action == "gold_set" {
				var bag inventory.Bag
				bag, e = inventory.ReadBag(raw)
				if e == nil {
					bag.Gold = req.Value
					raw, e = inventory.SaveBag(raw, bag)
				}
			}
		}
		if e != nil {
			return nil, nil, e
		}
		role.State = raw
		plan, e = c.onlineGMRefresh(role, request.Path == "/api/gm/level", projection)
		if e != nil {
			return nil, nil, e
		}
		if _, e = preparePackets(c.keys, plan); e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]any{"request": request, "online": true})
		return raw, receipt, nil
	})
	if e != nil {
		return fail(e)
	}
	// Read the latest state on replay, never restore the receipt's older snapshot.
	role, e := c.gameStore.AdminCharacter(ctx, req.Character)
	if e != nil {
		return localgm.Failure(500, fmt.Errorf("修改已提交，刷新待重试：%w", e))
	}
	role.WireID = c.worldState.role.WireID
	c.worldState.role = role
	c.equipmentState.pendingGoldBody = nil // A delayed destruction notice must not replay an older wallet.
	plan, e = c.onlineGMRefresh(role, request.Path == "/api/gm/level", projection)
	if e == nil {
		for _, p := range plan {
			if p.Name == "online_gm_actor" {
				c.selectedBasic = append([]byte(nil), p.Payload...)
			}
			if p.Name == "appearance_attributes_restored" {
				c.selectedAddition = append([]byte(nil), p.Payload...)
			}
		}
	}
	if e == nil {
		var balance uint64
		balance, e = c.gameStore.AccountCera(ctx, req.Account)
		if e == nil {
			var b []byte
			b, e = protocol.CeraBalance(balance)
			if e == nil {
				plan = append(plan, outboundPacket{"online_gm_cera", 0, 53, b})
			}
		}
	}
	if e == nil {
		e = c.sendPlan(plan, c.logCharacterResponse)
	}
	if e != nil {
		return localgm.Failure(500, fmt.Errorf("修改已保存，客户端刷新失败；以相同操作编号重试或重登：%w", e))
	}
	c.event(map[string]any{"kind": "online_gm_applied", "character_id": req.Character, "grant_id": req.GrantID, "applied": result.Applied})
	return localgm.JSON(200, map[string]any{"ok": true, "applied": result.Applied, "grant_id": req.GrantID, "receipt": result.Receipt, "online": true, "client_refreshed": true, "backup": backup})
}
func (c *gameConnection) onlineGMRefresh(role database.Character, level bool, projection *character.Service) ([]outboundPacket, error) {
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"online_gm_inventory", 0, 13, body}}
	for _, space := range []byte{1, 7} {
		p, e := inventory.EquipmentPayload(space, bag.Special[space], true)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"online_gm_special", 0, 13, p})
	}
	pets, e := inventory.PetContainerBody(bag, true)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"online_gm_pets", 0, 13, pets})
	if level {
		xp, e := character.ExperiencePayload(role)
		if e != nil {
			return nil, e
		}
		basic, e := projection.EntryBasicProbe(role, projection.ChannelContext)
		if e != nil {
			return nil, e
		}
		restore, e := appearanceRestore(projection, role)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"online_gm_experience", 0, 37, xp}, outboundPacket{"online_gm_actor", 0, 2, basic})
		plan = append(plan, restore...)
	}
	return plan, nil
}
