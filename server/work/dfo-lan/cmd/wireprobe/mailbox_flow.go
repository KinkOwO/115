package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"
)

var errMailMissing = errors.New("邮件不存在、已过期或不属于当前角色")
var errMailAttached = errors.New("请先领取附件和金币，再保存或删除邮件")

func mailboxRequest(id uint16) bool {
	switch id {
	case 94, 95, 96, 134, 315, 324, 781:
		return true
	}
	return false
}

// 错误码来自原生 0x145FDD430、0x145FDD210、0x145FC0DF0。
// 未核实的错误不伪装成功；0 会解除等待，详细原因写入服务端日志。
func mailboxFailure(id uint16, err error) []byte {
	var code uint16
	switch id {
	case 94, 315:
		switch {
		case errors.Is(err, storage.ErrMailRecipient):
			code = 3
		case errors.Is(err, storage.ErrMailSelf):
			code = 7
		case errors.Is(err, inventory.ErrMailGold):
			code = 10
		case errors.Is(err, inventory.ErrMailUntradeable):
			code = 23
		}
	case 95:
		switch {
		case errors.Is(err, errMailMissing):
			code = 21
		case errors.Is(err, inventory.ErrMailGold):
			code = 10
		case errors.Is(err, inventory.ErrMailBagFull):
			code = 4
		}
	case 324:
		code = 2
		if errors.Is(err, storage.ErrMailRecipient) {
			code = 21
		} else if errors.Is(err, storage.ErrMailSelf) {
			code = 7
		}
	}
	return protocol.Refusal(code)
}

func mailRemaining(m storage.MailMessage, now time.Time) uint32 {
	if m.Status == 3 {
		return 0
	}
	return uint32(max(0, min(int64(m.ExpiresAt.Sub(now)/time.Second), math.MaxUint32)))
}

// 正文编号同时作为附件组编号；金币先列出，原生组构造器由首行建立金币槽。
func mailboxSnapshot(messages []storage.MailMessage) ([]outboundPacket, error) {
	var attachments []protocol.MailAttachmentView
	var letters []protocol.MailTextView
	var total uint16
	now := time.Now()
	for _, m := range messages {
		if m.Deleted || (m.Status != 3 && !m.ExpiresAt.After(now)) {
			continue
		}
		remaining := mailRemaining(m, now)
		letter := protocol.MailTextView{ID: uint64(m.ID), Sender: m.SenderName, Text: m.Text, Remaining: remaining, Status: m.Status}
		if _, err := protocol.MailboxList(nil, []protocol.MailTextView{letter}, 1); err != nil {
			log.Printf("mail %d skipped: %v", m.ID, err)
			continue
		}
		total++
		appendAttachment := func(v protocol.MailAttachmentView, assetID int64) {
			if _, err := protocol.MailboxList([]protocol.MailAttachmentView{v}, nil, 0); err != nil {
				log.Printf("mail %d asset %d skipped: %v", m.ID, assetID, err)
				return
			}
			attachments = append(attachments, v)
		}
		for _, a := range m.Assets {
			if a.Claimed {
				continue
			}
			v := protocol.MailAttachmentView{ID: uint64(a.ID), MessageID: uint64(m.ID), Sender: m.SenderName, Gold: a.Gold, Remaining: remaining}
			if len(a.Item) != 0 {
				var item inventory.MailItem
				if err := json.Unmarshal(a.Item, &item); err != nil {
					log.Printf("mail %d asset %d skipped: %v", m.ID, a.ID, err)
					continue
				}
				if item.Space == 0 && item.Equipment == nil && item.Stack != nil && item.Stack.Template == 1 && item.Stack.Amount > 0 {
					if uint64(v.Gold)+uint64(item.Stack.Amount) > math.MaxUint32 {
						log.Printf("mail %d asset %d skipped: gold overflow", m.ID, a.ID)
						continue
					}
					v.Gold += item.Stack.Amount
					appendAttachment(v, a.ID)
					continue
				}
				var err error
				v.Record, err = item.Row()
				if err != nil {
					log.Printf("mail %d asset %d skipped: %v", m.ID, a.ID, err)
					continue
				}
				if item.Space == 1 {
					v.Avatar = true
					v.AvatarOptions, v.AvatarSockets, v.AvatarPeriod = item.Equipment.AvatarOptions, item.Equipment.AvatarSockets, item.Equipment.Period
				}
			}
			appendAttachment(v, a.ID)
		}
		letters = append(letters, letter)
	}
	body, err := protocol.MailboxList(attachments, letters, total)
	if err != nil {
		return nil, err
	}
	// 打开响应不能夹带 NOTI99：0x14530A200 在邮箱窗口存在时，连未读
	// 数为 0 都会经 0x145FDEBB0 排队再次发送 CMD96，导致无限刷新。
	// 未读提醒由登录及真实邮件状态变化路径单独发送。
	return []outboundPacket{{"mailbox_list_restored", 0, 97, body}}, nil
}

func mailBagPacket(state json.RawMessage) (outboundPacket, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return outboundPacket{}, err
	}
	body, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	return outboundPacket{"mailbox_inventory_restored", 0, 13, body}, err
}

// 领取成功先同步普通背包及时装栏，再通知客户端移除已领附件。
func mailClaimBagPackets(state json.RawMessage) ([]outboundPacket, error) {
	main, err := mailBagPacket(state)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{main}
	avatar, err := inventory.SpecialEquipmentPayload(state, 1)
	if err != nil {
		return nil, err
	}
	if len(avatar) > 0 {
		packets = append(packets, outboundPacket{"mailbox_avatar_inventory_updated", 0, 14, avatar})
	}
	return packets, nil
}

func (w *worldSession) mailboxAlarm(ctx context.Context) ([]byte, int64, error) {
	latest, n, err := w.characters.Store.MailboxDeliveryState(ctx, w.account, w.role.ID)
	return protocol.MailboxAlarm(n), latest, err
}

func (w *worldSession) handleMailbox(ctx context.Context, selected int64, id uint16, p, raw, keys []byte, prefix string) ([]outboundPacket, int64, error) {
	if w == nil || w.characters == nil || w.characters.Store == nil || selected == 0 || w.role.ID != selected || w.role.AccountID != w.account {
		return nil, 0, fmt.Errorf("邮件请求缺少当前账号所属的已选角色")
	}
	store := w.characters.Store
	key := fmt.Sprintf("mail:%s:%d:%x", prefix, id, sha256.Sum256(raw))
	switch id {
	case 781:
		server, err := protocol.DecodeMailServerCharacters(p)
		if err != nil {
			return nil, 0, err
		}
		if w.serverID > math.MaxUint8 {
			return nil, 0, fmt.Errorf("当前服务器编号超出邮件列表协议范围")
		}
		var rows []protocol.MailServerCharacter
		if uint32(server) == w.serverID {
			roles, err := store.Characters(ctx, w.account)
			if err != nil {
				return nil, 0, err
			}
			for _, role := range roles {
				// 复用账号隔离及未删除过滤；当前角色不能给自己发送邮件。
				if role.ID == selected {
					continue
				}
				var state character.State
				if err = json.Unmarshal(role.State, &state); err != nil {
					return nil, 0, err
				}
				rows = append(rows, protocol.MailServerCharacter{Name: role.Name, Level: uint16(state.Level)})
			}
		}
		payload, err := protocol.MailServerCharacters(server, rows)
		return []outboundPacket{{"mailbox_server_characters", 0, 705, payload}}, 0, err
	case 96:
		kind, err := protocol.DecodeMailboxOpen(p)
		if err != nil {
			return nil, 0, err
		}
		if kind != 0 {
			return nil, 0, fmt.Errorf("不支持的邮箱类型：%d", kind)
		}
		messages, err := store.Mailbox(ctx, w.account, selected)
		if err != nil {
			return nil, 0, err
		}
		packets, err := mailboxSnapshot(messages)
		packets = append(packets, outboundPacket{"mailbox_open_ready", 1, 96, protocol.MailboxOpenReady(uint16(len(messages)))})
		return packets, 0, err
	case 324:
		name, server, err := protocol.DecodeMailRecipient(p)
		if err != nil {
			return nil, 0, err
		}
		role, err := store.MailRecipient(ctx, name)
		if err != nil {
			return nil, 0, err
		}
		if role.ID == selected {
			return nil, 0, storage.ErrMailSelf
		}
		var state character.State
		if err = json.Unmarshal(role.State, &state); err != nil {
			return nil, 0, err
		}
		growth, err := state.WireAdvancement()
		if err != nil {
			return nil, 0, err
		}
		return []outboundPacket{{"mailbox_recipient_found", 1, 324, protocol.MailRecipientReply(role.Name, server, uint16(state.Level), role.Profession, growth)}}, 0, nil
	case 94, 315:
		return w.sendMail(ctx, id, p, keys, key)
	case 95:
		return w.claimMail(ctx, p, keys, key)
	case 134:
		return w.changeMailStatus(ctx, p, keys, key)
	}
	return nil, 0, fmt.Errorf("邮件命令无效")
}

func (w *worldSession) sendMail(ctx context.Context, id uint16, p, keys []byte, key string) ([]outboundPacket, int64, error) {
	r, err := protocol.DecodeMailSend(id, p)
	if err != nil {
		return nil, 0, err
	}
	if (len(r.Items) != 0 && w.loot == nil) || r.Special != 0 {
		return nil, 0, fmt.Errorf("邮件物品目录不可用或请求了特殊付费发送模式")
	}
	saved, receipt, _, err := w.characters.Store.SendMail(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, r.Recipient, r.Text,
		func(current storage.Character) (json.RawMessage, []storage.MailAsset, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, err
			}
			cost := uint64(r.Gold) + inventory.MailPostage(r.Gold, len(r.Items))
			if uint64(bag.Gold) < cost {
				return nil, nil, inventory.ErrMailGold
			}
			bag.Gold -= uint32(cost)
			var assets []storage.MailAsset
			if r.Gold != 0 {
				assets = append(assets, storage.MailAsset{ID: 1, Gold: r.Gold})
			}
			for _, row := range r.Items {
				var item inventory.MailItem
				bag, item, err = bag.TakeMailItem(w.loot.Catalog, w.loot.Equipment, row)
				if err != nil {
					return nil, nil, err
				}
				// 发出前确认此原始实例存在合法入包路径；收件人实际空位
				// 在领取事务中检查，不能先扣除再发现模板或堆叠规则不支持。
				if _, err = (inventory.Bag{Version: bag.Version}).AddMailItem(w.loot.Catalog, w.loot.BagRules, w.loot.Equipment, item); err != nil {
					return nil, nil, err
				}
				encoded, err := json.Marshal(item)
				if err != nil {
					return nil, nil, err
				}
				assets = append(assets, storage.MailAsset{ID: int64(len(assets) + 1), Item: encoded})
			}
			state, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, err
			}
			update, err := mailBagPacket(state)
			if err != nil {
				return nil, nil, err
			}
			// 提交前验证收件列表和背包都能编码；真实编号由存储事务分配。
			preview, err := mailboxSnapshot([]storage.MailMessage{{ID: 1, SenderName: current.Name, Text: r.Text, Status: 1, Assets: assets, ExpiresAt: time.Now().Add(15 * 24 * time.Hour)}})
			if err == nil && (len(preview) != 1 || len(preview[0].Payload) < 4 || int(preview[0].Payload[0]) != len(assets) || binary.LittleEndian.Uint16(preview[0].Payload[2:4]) != 1) {
				err = fmt.Errorf("发信预览含不可编码附件")
			}
			if err == nil {
				_, err = preparePackets(keys, append(preview, update, outboundPacket{"mailbox_sent", 1, id, []byte{1}}))
			}
			return state, assets, err
		})
	if err != nil {
		return nil, 0, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	update, err := mailBagPacket(saved.State)
	return []outboundPacket{update, {"mailbox_sent", 1, id, []byte{1}}}, receipt.RecipientID, err
}

func (w *worldSession) claimMail(ctx context.Context, p, keys []byte, key string) ([]outboundPacket, int64, error) {
	r, err := protocol.DecodeMailClaim(p)
	if err != nil {
		return nil, 0, err
	}
	if r.Kind != 0 {
		return nil, 0, fmt.Errorf("邮件领取类型无效")
	}
	saved, receipt, _, err := w.characters.Store.MutateMailbox(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "mail-claim-v1", nil,
		func(current storage.Character, messages []storage.MailMessage) (json.RawMessage, []storage.MailMessage, json.RawMessage, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, nil, err
			}
			type position struct{ message, asset int }
			index := map[uint64]position{}
			for i, m := range messages {
				for j, a := range m.Assets {
					index[uint64(a.ID)] = position{i, j}
				}
			}
			var results []protocol.MailClaimResult
			touched := map[int]bool{}
			for _, id := range r.IDs {
				pos, ok := index[id]
				if !ok {
					return nil, nil, nil, errMailMissing
				}
				m := &messages[pos.message]
				a := &m.Assets[pos.asset]
				if !a.Claimed {
					if uint64(bag.Gold)+uint64(a.Gold) > math.MaxUint32 {
						return nil, nil, nil, inventory.ErrMailGold
					}
					bag.Gold += a.Gold
					if len(a.Item) != 0 {
						if w.loot == nil {
							return nil, nil, nil, fmt.Errorf("邮件物品目录不可用")
						}
						var item inventory.MailItem
						if err = json.Unmarshal(a.Item, &item); err != nil {
							return nil, nil, nil, err
						}
						if item.Space == 0 && item.Equipment == nil && item.Stack != nil && item.Stack.Template == 1 && item.Stack.Amount > 0 {
							if uint64(bag.Gold)+uint64(item.Stack.Amount) > math.MaxUint32 {
								return nil, nil, nil, inventory.ErrMailGold
							}
							bag.Gold += item.Stack.Amount
						} else {
							bag, err = bag.AddMailItem(w.loot.Catalog, w.loot.BagRules, w.loot.Equipment, item)
							if err != nil {
								return nil, nil, nil, err
							}
						}
					}
					a.Claimed = true
				}
				touched[pos.message] = true
				results = append(results, protocol.MailClaimResult{MessageID: uint64(m.ID), AttachmentID: id})
			}
			var changed []storage.MailMessage
			for i := range messages {
				if !touched[i] {
					continue
				}
				m := &messages[i]
				m.Status = 2
				remaining := false
				for _, a := range m.Assets {
					remaining = remaining || !a.Claimed
				}
				// DSTR 5496：没有正文的邮件，附件领完自动消失。
				m.Deleted = m.Text == "" && !remaining
				changed = append(changed, *m)
			}
			state, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, nil, err
			}
			updates, err := mailClaimBagPackets(state)
			if err == nil {
				_, err = preparePackets(keys, append(updates, outboundPacket{"mailbox_claimed", 1, 95, protocol.MailClaimReply(r.Kind, results)}))
			}
			if err != nil {
				return nil, nil, nil, err
			}
			outcome, err := json.Marshal(results)
			return state, changed, outcome, err
		})
	if err != nil {
		return nil, 0, err
	}
	saved.WireID = w.role.WireID
	var results []protocol.MailClaimResult
	if err = json.Unmarshal(receipt, &results); err != nil {
		return nil, 0, err
	}
	var updates []outboundPacket
	if w.loot != nil {
		var materials inventory.AccountMaterials
		var swept storage.Character
		swept, materials, err = sweepAccountMaterials(ctx, w.characters.Store, saved)
		if err == nil {
			saved = swept
			updates, err = accountMaterialRefreshPackets(materials, saved)
		} else {
			log.Printf("mail claim account material sweep deferred for character %d: %v", saved.ID, err)
		}
	}
	if updates == nil {
		updates, err = mailClaimBagPackets(saved.State)
	}
	if err == nil {
		bag, readErr := inventory.ReadBag(saved.State)
		if readErr != nil {
			return nil, 0, readErr
		}
		if len(bag.PetItems)+len(bag.Special[7]) > 0 {
			petBody, petErr := inventory.PetContainerBody(bag, true)
			if petErr != nil {
				return nil, 0, petErr
			}
			updates = append(updates, outboundPacket{"mailbox_pet_container_restored", 0, 13, petBody})
		}
	}
	w.role = saved
	// 原生领取回调要查旧邮件对象，必须先更新背包，再应答，不提前清空列表。
	return append(updates, outboundPacket{"mailbox_claimed", 1, 95, protocol.MailClaimReply(r.Kind, results)}), 0, err
}

func (w *worldSession) changeMailStatus(ctx context.Context, p, keys []byte, key string) ([]outboundPacket, int64, error) {
	r, err := protocol.DecodeMailStatus(p)
	if err != nil {
		return nil, 0, err
	}
	response := protocol.MailStatusReply(r.IDs, r.Status)
	packets := []outboundPacket{{"mailbox_status_saved", 1, 134, response}}
	if _, err = preparePackets(keys, packets); err != nil {
		return nil, 0, err
	}
	messageIDs := make([]int64, len(r.IDs))
	for i, id := range r.IDs {
		messageIDs[i] = int64(id)
	}
	saved, _, _, err := w.characters.Store.MutateMailbox(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "mail-status-v1", messageIDs,
		func(current storage.Character, messages []storage.MailMessage) (json.RawMessage, []storage.MailMessage, json.RawMessage, error) {
			index := map[uint64]storage.MailMessage{}
			for _, m := range messages {
				index[uint64(m.ID)] = m
			}
			var changed []storage.MailMessage
			for _, id := range r.IDs {
				m, ok := index[id]
				if !ok {
					return nil, nil, nil, errMailMissing
				}
				if m.Deleted {
					if r.Status != 0 {
						return nil, nil, nil, errMailMissing
					}
					continue
				}
				if m.Status != 3 && !m.ExpiresAt.After(time.Now()) {
					if r.Status != 0 {
						return nil, nil, nil, errMailMissing
					}
					m.Deleted = true
					changed = append(changed, m)
					continue
				}
				if r.Status == 0 || r.Status == 3 {
					for _, a := range m.Assets {
						if !a.Claimed {
							return nil, nil, nil, errMailAttached
						}
					}
				}
				if r.Status == 0 {
					m.Deleted = true
				} else if m.Status != 3 || r.Status == 3 {
					m.Status = r.Status
				}
				changed = append(changed, m)
			}
			outcome, err := json.Marshal(r)
			return current.State, changed, outcome, err
		})
	if err != nil {
		return nil, 0, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return packets, 0, nil
}
