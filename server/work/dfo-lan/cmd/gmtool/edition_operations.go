package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type gmCommand struct {
	Account   int64                      `json:"account"`
	Character int64                      `json:"character"`
	GrantID   string                     `json:"grant_id"`
	Reason    string                     `json:"reason"`
	Action    string                     `json:"action"`
	Revision  string                     `json:"revision,omitempty"`
	Value     uint32                     `json:"value,omitempty"`
	Secondary uint32                     `json:"secondary,omitempty"`
	Template  uint32                     `json:"template,omitempty"`
	Amount    uint32                     `json:"amount,omitempty"`
	Container string                     `json:"container,omitempty"`
	Slot      uint16                     `json:"slot,omitempty"`
	Equipment *inventory.GMEquipment     `json:"equipment,omitempty"`
	IDs       []uint16                   `json:"ids,omitempty"`
	MailIDs   []int64                    `json:"mail_ids,omitempty"`
	Message   string                     `json:"message,omitempty"`
	Mail      bool                       `json:"mail,omitempty"`
	VP        *character.GMVariationEdit `json:"vp,omitempty"`
}

func stateRevision(raw json.RawMessage) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func patchField(raw json.RawMessage, key string, value any) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("存档不是对象")
	}
	v, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	doc[key] = v
	return json.Marshal(doc)
}
func patchInventory(raw json.RawMessage, key string, value any) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	inv, err := patchField(doc["inventory"], key, value)
	if err != nil {
		return nil, err
	}
	doc["inventory"] = inv
	return json.Marshal(doc)
}
func (s *server) gmRole(r *http.Request) (database.Character, error) {
	a, _ := strconv.ParseInt(r.URL.Query().Get("account"), 10, 64)
	id, _ := strconv.ParseInt(r.URL.Query().Get("character"), 10, 64)
	c, err := s.admin.Store.AdminCharacter(r.Context(), id)
	if err == nil && (a <= 0 || c.AccountID != a) {
		err = fmt.Errorf("账号与角色不匹配")
	}
	return c, err
}
func (s *server) registerGMOperations(mux *http.ServeMux) {
	mux.HandleFunc("/api/gm/snapshot", s.auth(s.handleGMSnapshot))
	mux.HandleFunc("/api/gm/quests", s.auth(s.handleGMQuests))
	mux.HandleFunc("/api/gm/mailbox", s.auth(s.handleGMMailbox))
	mux.HandleFunc("/api/gm/materials", s.auth(s.handleGMMaterials))
	mux.HandleFunc("/api/gm/vp", s.auth(s.handleGMVP))
	mux.HandleFunc("/api/gm/command", s.auth(s.handleGMCommand))
}

type gmInventoryRow struct {
	Container  string                 `json:"container"`
	Slot       uint16                 `json:"slot"`
	Template   uint32                 `json:"template"`
	Name       string                 `json:"name"`
	Amount     uint32                 `json:"amount"`
	Equipment  *inventory.GMEquipment `json:"equipment,omitempty"`
	Durability uint16                 `json:"durability,omitempty"`
}

func (s *server) itemName(id uint32) string {
	if it, ok := s.index.Get(id); ok {
		return it.Name
	}
	return fmt.Sprint("ID ", id)
}
func (s *server) handleGMSnapshot(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	c, err := s.gmRole(r)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	b, err := inventory.ReadBag(c.State)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	rows := []gmInventoryRow{}
	stack := func(container string, items []inventory.BagItem) {
		for _, it := range items {
			rows = append(rows, gmInventoryRow{Container: container, Slot: it.Slot, Template: it.Template, Name: s.itemName(it.Template), Amount: it.Amount})
		}
	}
	gear := func(container string, items []inventory.BagEquipment) {
		for _, it := range items {
			v := inventory.GMEquipmentValues(it)
			rows = append(rows, gmInventoryRow{Container: container, Slot: it.Slot, Template: it.Template, Name: s.itemName(it.Template), Amount: 1, Equipment: &v, Durability: it.Durability})
		}
	}
	stack("items", b.Items)
	stack("pet_items", b.PetItems)
	gear("equipment", b.Equipment)
	gear("worn", b.Worn)
	spaces := []int{}
	for space := range b.Special {
		spaces = append(spaces, int(space))
	}
	sort.Ints(spaces)
	for _, space := range spaces {
		gear(fmt.Sprintf("special:%d", space), b.Special[byte(space)])
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(c.State, &fields)
	cera, err := s.admin.Store.AccountCera(r.Context(), c.AccountID)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	writeJSON(w, map[string]any{"revision": stateRevision(c.State), "items": rows, "gold": b.Gold, "coin": b.Coin, "cera": cera, "skill_points": fields["skill_points"], "source": c.ConfigVersion})
}
func (s *server) handleGMVP(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	c, err := s.gmRole(r)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	if s.edition.skills == nil {
		writeErr(w, 503, "115技能源尚未准备")
		return
	}
	v, err := s.edition.skills.GMVariationView(character.Character{ID: c.ID, AccountID: c.AccountID, Profession: c.Profession, ConfigVersion: c.ConfigVersion, State: c.State})
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	writeJSON(w, v)
}
func (s *server) handleGMQuests(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	c, err := s.gmRole(r)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	saved, err := s.admin.Store.Quests(r.Context(), c.AccountID, c.ID)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	states := map[uint32]database.QuestState{}
	for _, v := range saved {
		states[uint32(v.ID)] = v
	}
	ids := []int{}
	for id := range s.edition.quests.Quests {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	rows := []map[string]any{}
	q := strings.ToLower(r.URL.Query().Get("q"))
	filter := r.URL.Query().Get("status")
	for _, id := range ids {
		d := s.edition.quests.Quests[uint32(id)]
		name := questName(d)
		state := states[uint32(id)]
		status := "available"
		if state.Status != "" {
			status = state.Status
		}
		if status == "accepted" && state.Progress == 0 {
			status = "ready"
		}
		if filter != "" && filter != status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strconv.Itoa(id), q) {
			continue
		}
		rows = append(rows, map[string]any{"id": id, "name": name, "status": status, "progress": state.Progress, "model": state.ProgressModel, "level": d.MinimumLevel, "maximum_level": d.MaximumLevel, "kind": d.Kind, "prerequisites": d.Prerequisites, "pending": d.Pending})
		if len(rows) >= 500 {
			break
		}
	}
	writeJSON(w, map[string]any{"quests": rows, "catalog_count": len(ids), "returned": len(rows)})
}
func questName(d catalog.QuestDefinition) string {
	for i, t := range d.Script.Cells {
		if t.Text == "[name]" && i+1 < len(d.Script.Cells) {
			return d.Script.Cells[i+1].Text
		}
	}
	return fmt.Sprintf("任务 %d", d.ID)
}
func (s *server) handleGMMailbox(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	c, err := s.gmRole(r)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	mail, err := s.admin.Store.Mailbox(r.Context(), c.AccountID, c.ID)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	rows := []map[string]any{}
	for _, m := range mail {
		names := []string{}
		for _, a := range m.Assets {
			if a.Gold != 0 {
				names = append(names, fmt.Sprintf("金币 %d", a.Gold))
			}
			if len(a.Item) > 0 {
				var item inventory.MailItem
				if json.Unmarshal(a.Item, &item) == nil {
					if item.Stack != nil {
						names = append(names, fmt.Sprintf("%s ×%d", s.itemName(item.Stack.Template), item.Stack.Amount))
					}
					if item.Equipment != nil {
						names = append(names, s.itemName(item.Equipment.Template))
					}
				}
			}
		}
		rows = append(rows, map[string]any{"id": m.ID, "sender": m.SenderName, "text": m.Text, "status": m.Status, "attachments": strings.Join(names, "；"), "assets": m.Assets, "expires_at": m.ExpiresAt})
	}
	writeJSON(w, map[string]any{"mail": rows})
}
func (s *server) handleGMMaterials(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	c, err := s.gmRole(r)
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	raw, err := s.admin.Store.AccountMaterials(r.Context(), c.AccountID)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	m, err := inventory.ReadAccountMaterials(raw)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	rows := []map[string]any{}
	for _, it := range s.index.items {
		if space, slot, ok := inventory.AccountMaterialTarget(it.ID); ok {
			rows = append(rows, map[string]any{"template": it.ID, "name": it.Name, "space": space, "slot": slot, "amount": m.Count(it.ID)})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["template"].(uint32) < rows[j]["template"].(uint32) })
	writeJSON(w, map[string]any{"materials": rows})
}
func (s *server) handleGMCommand(w http.ResponseWriter, r *http.Request) {
	var req gmCommand
	if !dashboardRequest(w, r, http.MethodPost, &req) {
		return
	}
	if req.Account <= 0 || req.Character <= 0 || len(req.GrantID) < 8 || len(req.GrantID) > 128 || strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 500 {
		writeErr(w, 400, "请选择账号和角色，并填写操作原因")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	details, _ := json.Marshal(req)
	g := database.Grant{ID: req.GrantID, AccountID: req.Account, Character: req.Character, Reason: req.Reason, Operator: "gm-" + s.edition.kind, Details: details}
	if req.Action == "cera_set" {
		balance, err := s.admin.Store.AccountCera(ctx, req.Account)
		if err != nil || balance > uint64(^uint32(0)) {
			writeErr(w, 400, "点券余额无法编码，请检查账号")
			return
		}
		g.Cera = int64(req.Value) - int64(balance)
		g.MaxCera = uint64(^uint32(0))
	}
	result, err := s.admin.Store.ApplyGrantTx(ctx, g, func(tx *database.Tx, c database.Character) (json.RawMessage, json.RawMessage, error) {
		if req.Revision != "" && req.Revision != stateRevision(c.State) {
			return nil, nil, fmt.Errorf("角色存档已改变，请刷新后重试")
		}
		state := c.State
		var err error
		receipt := map[string]any{"action": req.Action, "request": req}
		switch req.Action {
		case "gold_set", "coin_set":
			key := "gold"
			if req.Action == "coin_set" {
				key = "coin"
			}
			state, err = patchInventory(state, key, req.Value)
		case "cera_set":
		case "sp_set":
			if req.Value > 65535 || req.Secondary > 65535 {
				return nil, nil, fmt.Errorf("技能点超出客户端范围")
			}
			state, err = patchField(state, "skill_points", [2]uint16{uint16(req.Value), uint16(req.Secondary)})
		case "vp_set":
			if req.VP == nil || s.edition.skills == nil {
				return nil, nil, fmt.Errorf("缺少115 VP配置或技能源")
			}
			state, err = s.edition.skills.GMEditVariations(character.Character{ID: c.ID, AccountID: c.AccountID, Profession: c.Profession, ConfigVersion: c.ConfigVersion, State: c.State}, *req.VP)
		case "item_delete", "item_edit":
			state, err = s.editGMItem(state, req)
		case "item_grant":
			var assets []database.MailAsset
			state, assets, err = s.grantGMItem(state, req)
			if err == nil && req.Mail {
				var mid int64
				mid, err = tx.GMSystemMail(ctx, req.Message, assets)
				receipt["mail_id"] = mid
			}
		case "mail_send":
			assets := []database.MailAsset{}
			if req.Value > 0 {
				assets = append(assets, database.MailAsset{Gold: req.Value})
			}
			if req.Template > 0 {
				copyReq := req
				copyReq.Mail = true
				var itemAssets []database.MailAsset
				_, itemAssets, err = s.grantGMItem(state, copyReq)
				assets = append(assets, itemAssets...)
			}
			if err == nil {
				var mid int64
				mid, err = tx.GMSystemMail(ctx, req.Message, assets)
				receipt["mail_id"] = mid
			}
		case "mail_delete":
			err = tx.GMDeleteMail(ctx, req.MailIDs)
		case "material_set":
			if _, _, ok := inventory.AccountMaterialTarget(req.Template); !ok {
				return nil, nil, fmt.Errorf("物品不属于115晶体仓库")
			}
			if _, ok := s.index.Get(req.Template); !ok {
				return nil, nil, fmt.Errorf("当前PVF没有该物品")
			}
			err = tx.GMMaterials(ctx, func(raw json.RawMessage) (json.RawMessage, error) {
				m, e := inventory.ReadAccountMaterials(raw)
				if e != nil {
					return nil, e
				}
				slot, _ := inventory.AccountMaterialSlot(req.Template)
				if m.Counts == nil {
					m.Counts = map[uint16]uint32{}
				}
				m.Counts[slot] = req.Value
				return patchField(rawOrObject(raw), "counts", m.Counts)
			})
		case "quest_ready", "quest_complete", "quest_abandon":
			if c.ConfigVersion != s.edition.quests.Source.SaveIdentity() {
				return nil, nil, fmt.Errorf("任务与角色来源不一致")
			}
			seen := map[uint16]bool{}
			for _, id := range req.IDs {
				if id == 0 || id == 65535 || seen[id] {
					return nil, nil, fmt.Errorf("任务编号无效或重复")
				}
				seen[id] = true
				d, ok := s.edition.quests.Quests[uint32(id)]
				if !ok {
					return nil, nil, fmt.Errorf("当前PVF没有任务%d", id)
				}
				if req.Action == "quest_ready" {
					if _, _, e := quest.InitialProgress(d); e != nil {
						return nil, nil, fmt.Errorf("任务%d目标模型未确认，不能标为可交：%v", id, e)
					}
				}
			}
			err = tx.GMQuests(ctx, req.IDs, req.Action, c.ConfigVersion)
		default:
			return nil, nil, fmt.Errorf("未知GM操作")
		}
		if err != nil {
			return nil, nil, err
		}
		rec, err := json.Marshal(receipt)
		return state, rec, err
	})
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "applied": result.Applied, "receipt": result.Receipt, "grant_id": req.GrantID})
}
func rawOrObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{"version":"account-materials-v1"}`)
	}
	return raw
}

// Edit only the selected row as RawMessages; unknown stack/item fields survive.
func (s *server) editGMItem(state json.RawMessage, r gmCommand) (json.RawMessage, error) {
	var doc, inv map[string]json.RawMessage
	if err := json.Unmarshal(state, &doc); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(doc["inventory"], &inv); err != nil {
		return nil, err
	}
	key := r.Container
	special := map[string]json.RawMessage{}
	sp := ""
	if strings.HasPrefix(key, "special:") {
		sp = strings.TrimPrefix(key, "special:")
		if sp != "1" && sp != "7" {
			return nil, fmt.Errorf("不支持该装备容器")
		}
		if err := json.Unmarshal(inv["special_equipment"], &special); err != nil {
			return nil, err
		}
		key = "special_equipment"
	}
	if key != "items" && key != "pet_items" && key != "equipment" && key != "worn" && key != "special_equipment" {
		return nil, fmt.Errorf("无效背包容器")
	}
	raw := inv[key]
	if sp != "" {
		raw = special[sp]
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	found := false
	for i, row := range rows {
		var identity struct {
			Slot     uint16 `json:"slot"`
			Template uint32
		}
		if err := json.Unmarshal(row, &identity); err != nil {
			return nil, err
		}
		if identity.Slot != r.Slot {
			continue
		}
		if identity.Template != r.Template {
			return nil, fmt.Errorf("物品槽位已经改变")
		}
		if found {
			return nil, fmt.Errorf("重复槽位")
		}
		found = true
		if r.Action == "item_delete" {
			if key == "worn" {
				return nil, fmt.Errorf("请先在游戏内卸下装备，再删除")
			}
			rows = append(rows[:i], rows[i+1:]...)
			break
		}
		if key == "items" || key == "pet_items" {
			if r.Amount == 0 {
				return nil, fmt.Errorf("数量必须大于0")
			}
			def, ok := s.admin.Awarder.Catalog.Items[r.Template]
			if !ok {
				return nil, fmt.Errorf("当前PVF缺少物品")
			}
			limit := def.StackLimit
			if limit == 0 {
				limit = s.admin.Awarder.Rules.MissingStackLimit
			}
			if r.Amount > limit {
				return nil, fmt.Errorf("数量超过源堆叠上限%d", limit)
			}
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(row, &fields)
			amountKey := "Amount"
			for k := range fields {
				if strings.EqualFold(k, "amount") {
					amountKey = k
				}
			}
			v, _ := json.Marshal(r.Amount)
			fields[amountKey] = v
			rows[i], _ = json.Marshal(fields)
		} else {
			if r.Equipment == nil {
				return nil, fmt.Errorf("缺少装备打造参数")
			}
			var gear inventory.BagEquipment
			if err := json.Unmarshal(row, &gear); err != nil {
				return nil, err
			}
			updated, err := inventory.EditGMEquipment(s.admin.Awarder.Equipment, gear, *r.Equipment)
			if err != nil {
				return nil, err
			}
			rows[i], err = json.Marshal(updated)
			if err != nil {
				return nil, err
			}
		}
		break
	}
	if !found {
		return nil, fmt.Errorf("物品槽位不存在，请刷新")
	}
	encoded, _ := json.Marshal(rows)
	if sp != "" {
		special[sp] = encoded
		encoded, _ = json.Marshal(special)
	}
	inv[key] = encoded
	doc["inventory"], _ = json.Marshal(inv)
	return json.Marshal(doc)
}
func (s *server) grantGMItem(state json.RawMessage, r gmCommand) (json.RawMessage, []database.MailAsset, error) {
	fail := func(err error) (json.RawMessage, []database.MailAsset, error) { return nil, nil, err }
	if r.Template < 2 || r.Amount == 0 {
		return fail(fmt.Errorf("需要真实物品和数量"))
	}
	entry, ok := s.index.Get(r.Template)
	if !ok {
		return fail(fmt.Errorf("当前PVF没有该物品"))
	}
	a := s.admin.Awarder
	if a == nil {
		return fail(fmt.Errorf("物品来源未准备"))
	}
	original := state
	if r.Mail {
		state = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[],"equipment":[]}}`)
	}
	assets := []database.MailAsset{}
	if entry.Kind == "stackable" {
		if r.Equipment != nil {
			return fail(fmt.Errorf("堆叠物品不能附带装备打造参数"))
		}
		if r.Mail {
			def, ok := a.Catalog.Items[r.Template]
			if !ok {
				return fail(fmt.Errorf("物品定义缺失"))
			}
			limit := def.StackLimit
			if limit == 0 {
				limit = a.Rules.MissingStackLimit
			}
			if r.Amount > limit {
				return fail(fmt.Errorf("邮件数量超过堆叠上限%d", limit))
			}
			it := inventory.BagItem{Template: r.Template, Amount: r.Amount, ExpireTime: inventory.GrantExpireTime}
			m := inventory.MailItem{Stack: &it}
			if _, err := m.Row(); err != nil {
				return fail(err)
			}
			raw, _ := json.Marshal(m)
			return original, []database.MailAsset{{Item: raw}}, nil
		}
		updated, _, err := a.Grant(state, r.Template, r.Amount)
		if err == nil {
			updated, err = preserveGMStackFields(state, updated)
		}
		return updated, nil, err
	}
	if r.Amount > 9 {
		return fail(fmt.Errorf("装备每批最多9件"))
	}
	kind, err := a.Equipment.EquipmentKind(r.Template)
	if err != nil {
		return fail(err)
	}
	avatar := inventory.EquipmentBagSpace(kind) == 1
	if avatar {
		if r.Equipment != nil {
			return fail(fmt.Errorf("时装不能使用普通装备打造参数"))
		}
		// Avatar shop delivery already stores template+period in list 1. Reuse the
		// same mail claim placement instead of inventing a list-0 equipment row.
		for n := uint32(0); n < r.Amount; n++ {
			item := inventory.MailItem{Space: 1, Equipment: &inventory.BagEquipment{Template: r.Template, Period: inventory.GrantExpireTime}}
			if _, err = item.Row(); err != nil {
				return fail(err)
			}
			if r.Mail {
				raw, _ := json.Marshal(item)
				assets = append(assets, database.MailAsset{Item: raw})
			} else {
				b, e := inventory.ReadBag(state)
				if e != nil {
					return fail(e)
				}
				b, e = b.AddMailItem(a.Catalog, a.Rules, a.Equipment, item)
				if e != nil {
					return fail(e)
				}
				state, e = inventory.SaveBag(state, b)
				if e != nil {
					return fail(e)
				}
			}
		}
	} else {
		updated, receipt, e := a.Grant(state, r.Template, r.Amount)
		if e != nil {
			return fail(e)
		}
		b, e := inventory.ReadBag(updated)
		if e != nil {
			return fail(e)
		}
		wanted := map[uint16]bool{}
		for _, slot := range receipt.Slots {
			wanted[slot] = true
		}
		rows := b.Equipment
		if inventory.IsPetGear(kind) {
			rows = b.Special[7]
		}
		for i, it := range rows {
			if !wanted[it.Slot] || it.Template != r.Template {
				continue
			}
			if r.Equipment != nil {
				it, e = inventory.EditGMEquipment(a.Equipment, it, *r.Equipment)
				if e != nil {
					return fail(e)
				}
				rows[i] = it
			}
			if r.Mail {
				m := inventory.MailItem{Equipment: &it}
				if _, e = m.Row(); e != nil {
					return fail(e)
				}
				raw, _ := json.Marshal(m)
				assets = append(assets, database.MailAsset{Item: raw})
			}
		}
		if inventory.IsPetGear(kind) {
			b.Special[7] = rows
		} else {
			b.Equipment = rows
		}
		state, e = inventory.SaveBag(updated, b)
		if e != nil {
			return fail(e)
		}
	}
	if r.Mail {
		return original, assets, nil
	}
	state, err = preserveGMStackFields(original, state)
	return state, nil, err
}

// Keep future stack-instance fields through the ordinary awarder's typed bag
// roundtrip. Identity is slot+template; metadata never follows a replaced item.
func preserveGMStackFields(before, after json.RawMessage) (json.RawMessage, error) {
	var oldDoc, newDoc map[string]json.RawMessage
	if err := json.Unmarshal(before, &oldDoc); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(after, &newDoc); err != nil {
		return nil, err
	}
	var oldInv, newInv map[string]json.RawMessage
	if err := json.Unmarshal(oldDoc["inventory"], &oldInv); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newDoc["inventory"], &newInv); err != nil {
		return nil, err
	}
	known := map[string]bool{}
	typ := reflect.TypeOf(inventory.BagItem{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		key := strings.Split(f.Tag.Get("json"), ",")[0]
		if key == "" {
			key = f.Name
		}
		known[strings.ToLower(key)] = true
	}
	for _, key := range []string{"items", "pet_items"} {
		var oldRows, newRows []json.RawMessage
		if raw := oldInv[key]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &oldRows); err != nil {
				return nil, err
			}
		}
		if raw := newInv[key]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &newRows); err != nil {
				return nil, err
			}
		}
		oldByID := map[[2]uint32]json.RawMessage{}
		for _, raw := range oldRows {
			var row inventory.BagItem
			if err := json.Unmarshal(raw, &row); err != nil {
				return nil, err
			}
			oldByID[[2]uint32{uint32(row.Slot), row.Template}] = raw
		}
		for i, raw := range newRows {
			var row inventory.BagItem
			if err := json.Unmarshal(raw, &row); err != nil {
				return nil, err
			}
			old, ok := oldByID[[2]uint32{uint32(row.Slot), row.Template}]
			if !ok {
				continue
			}
			var previous, current map[string]json.RawMessage
			_ = json.Unmarshal(old, &previous)
			_ = json.Unmarshal(raw, &current)
			for k, v := range previous {
				if !known[strings.ToLower(k)] {
					current[k] = v
				}
			}
			var err error
			newRows[i], err = json.Marshal(current)
			if err != nil {
				return nil, err
			}
		}
		if len(newRows) > 0 {
			var err error
			newInv[key], err = json.Marshal(newRows)
			if err != nil {
				return nil, err
			}
		}
	}
	var err error
	newDoc["inventory"], err = json.Marshal(newInv)
	if err != nil {
		return nil, err
	}
	return json.Marshal(newDoc)
}
