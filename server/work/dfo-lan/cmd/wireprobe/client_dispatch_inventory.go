package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (client *gameConnection) dispatchCashshopAndBoxes(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.characters != nil && requestData.frame.ID == 63 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		payload, e := ceraQuery(ctx, client.gameStore, client.developmentAccount, requestData.plaintext)
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "cera_query_error", "error": e.Error()})
			return dispatchHandled
		}
		if e = client.output.send(0, 53, payload); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "cera_balance_response", "account_id": client.developmentAccount, "plain_hex": hex.EncodeToString(payload)})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && requestData.frame.ID == 64 {
		if client.shopPilot != nil && client.characters != nil && client.worldState != nil && client.selectedCharacterID != 0 && client.worldState.activeDungeon == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			receipt, applied, buyErr := client.purchaseSession.purchase(ctx, client.shopPilot, client.gameStore, client.developmentAccount, client.selectedCharacterID, requestData.plaintext, requestData.frame.Raw)
			cancel()
			if buyErr == nil {
				client.worldState.role.State = receipt.CharacterState
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				balance, readErr := client.gameStore.AccountCera(ctx, client.developmentAccount)
				cancel()
				if readErr != nil {
					client.event(map[string]any{"kind": "cera_committed_sync_error", "order": receipt.Order, "error": readErr.Error()})
					return dispatchClose
				}
				packets, encodeErr := shopPilotSpaces(client.shopPilot, receipt, balance, applied)
				if encodeErr != nil {
					client.event(map[string]any{"kind": "cera_committed_sync_error", "order": receipt.Order, "error": encodeErr.Error()})
					return dispatchClose
				}
				client.event(map[string]any{"kind": "cera_purchase_committed", "order": receipt.Order, "character_id": client.selectedCharacterID, "applied": applied, "charged": receipt.Charged, "gold_charged": receipt.GoldCharged, "before": receipt.Before, "after": receipt.After, "deliveries": receipt.Deliveries})
				if client.sendPlan(packets, client.logResponseBody) != nil {
					return dispatchClose
				}
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "cera_purchase_rejected", "error": buyErr.Error(), "charged": false})
		}
		items, e := protocol.DecodeCeraCart(requestData.plaintext)
		reason := "delivery_protocol_pending"
		if e != nil {
			reason = e.Error()
		}
		// No ledger mutation occurs on this path. An unsupported buy
		// must finish its native pending state instead of hanging.
		payload := protocol.CeraPurchaseCancelled()
		if e = client.output.send(1, 64, payload); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "cera_purchase_cancelled", "reason": reason, "items": items, "character_id": client.selectedCharacterID, "charged": false, "plain_hex": hex.EncodeToString(payload)})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.worldState != nil && client.itemService != nil && client.itemService.Boxes != nil &&
		((requestData.frame.ID == 2036 && protocol.IsCeraShopDeviceAction(requestData.plaintext)) ||
			(requestData.frame.ID == 495 && protocol.IsCeraShopDeviceRefresh(requestData.plaintext))) {
		blob, stateErr := radiantDeviceWindowState(client.itemService, client.worldState.role)
		if stateErr != nil {
			client.event(map[string]any{"kind": "radiant_device_state_refused", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "reason": stateErr.Error()})
			return dispatchHandled
		}
		if e := client.output.send(1, 2036, blob); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "radiant_device_state_sent", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(blob)})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && requestData.frame.ID == 681 && client.worldState != nil && client.itemService != nil && client.itemService.Boxes != nil {
		request, decodeErr := protocol.DecodeRadiantBoxOpen(requestData.plaintext)
		if decodeErr != nil {
			client.event(map[string]any{"kind": "event_request_ignored", "id": requestData.frame.ID, "reason": decodeErr.Error()})
			// 681 是通用事件请求帧：光辉之环宝箱(4202) 与 662 训练引导查询都用它。
			// 正文点名 662 时不能在这里吞掉，否则 commandDispatch 里的
			// dispatchBoostEvent 永远收不到（实机 2026-10-04 09:18:37 第三关 681
			// 只有 event_request_ignored、没有任何回包）。其余未知事件保持原样不回。
			if request.Event == boostup.EventID {
				return dispatchNext
			}
			return dispatchHandled
		}
		count, countErr := radiantBoxOpens(request.Mode)
		box, boxErr := radiantBoxHeld(client.itemService, client.worldState.role)
		if countErr != nil || boxErr != nil {
			reason := countErr
			if reason == nil {
				reason = boxErr
			}
			client.event(map[string]any{"kind": "event_request_refused", "character_id": client.selectedCharacterID, "reason": reason.Error()})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		packets, openErr := client.worldState.openRadiantBox(ctx, box, count)
		cancel()
		if openErr != nil {
			client.event(map[string]any{"kind": "event_request_refused", "character_id": client.selectedCharacterID, "box": box, "mode": request.Mode, "reason": openErr.Error()})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "radiant_box_opened", "character_id": client.selectedCharacterID, "box": box, "mode": request.Mode, "opened": count})
		if client.sendPlan(packets, client.logResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.characters != nil && client.worldState != nil && (requestData.frame.ID == 102 || requestData.frame.ID == 173) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		packets, e := client.worldState.hatchCreature(ctx, client.gameStore, requestData.frame.ID, requestData.plaintext, requestData.frame.Raw)
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "creature_hatch_error", "error": e.Error(), "character_id": client.selectedCharacterID})
			_ = client.output.send(1, requestData.frame.ID, []byte{0})
			return dispatchHandled
		}
		if client.sendPlan(packets, client.logResponseBody) != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "creature_hatch_success", "character_id": client.selectedCharacterID})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && (requestData.frame.ID == 160 || requestData.frame.ID == 41) && client.bootstrapped && requestData.verified && client.characters != nil && client.worldState != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var plan []outboundPacket
		var e error
		if requestData.frame.ID == 41 {
			pilotEnabled := odysseyTemporaryCreditsEnabled() && isOdysseyRewardRole(client.worldState.role) && client.worldState.activeDungeon != nil && client.worldState.activeDungeon.Definition.Odyssey
			plan, e = client.worldState.useCoinRevive(ctx, client.gameStore, requestData.plaintext, requestData.frame.Raw, pilotEnabled)
		} else if client.worldState.activeDungeon != nil || client.worldState.role.ID == 0 {
			e = fmt.Errorf("booster box use requires selected character in town")
		} else {
			plan, e = client.worldState.openBoosterItem(ctx, client.gameStore, client.wearService, client.lootService, client.boosterCatalog, client.odysseyChoices, requestData.plaintext, requestData.frame.Raw)
		}
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "booster_action_refused", "id": requestData.frame.ID, "reason": e.Error()})
			plan = []outboundPacket{{"booster_action_refused_ack", 1, requestData.frame.ID, boosterActionRefusal(requestData.frame.ID)}}
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 27 && client.bootstrapped && requestData.verified && client.characters != nil && client.worldState != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var plan []outboundPacket
		var e error
		if client.lotteryPools == nil || client.boosterCatalog == nil {
			e = fmt.Errorf("lottery item catalog unavailable")
		} else {
			plan, e = client.worldState.openLotteryItem(ctx, client.gameStore, client.lotteryPools, client.boosterCatalog.Items, requestData.plaintext, requestData.frame.Raw, client.wearService)
		}
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "lottery_item_refused", "character_id": client.selectedCharacterID, "reason": e.Error()})
			plan = []outboundPacket{{"lottery_item_refused_ack", 1, 27, protocol.Refusal(4)}}
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchSpecialEquipment(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && requestData.frame.ID == 451 && client.bootstrapped && requestData.verified && client.wearService != nil && client.wearService.Rules.Special {
		packets, err := avatarOption(client.wearService, client.worldState, requestData.plaintext, client.keys)
		if err != nil {
			client.event(map[string]any{"kind": "avatar_option_refused", "error": err.Error()})
			if err = client.output.send(1, 451, protocol.Refusal(4)); err != nil {
				return dispatchClose
			}
		} else if err = client.output.writePrepared(packets, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchAccountVault(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && (requestData.frame.ID == 305 || requestData.frame.ID == 306) && client.worldState != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		plan, err := client.worldState.upgradeAccountVault(ctx, requestData.frame.ID, requestData.plaintext, requestData.frame.Raw, client.keys, client.purchaseSession.prefix)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "账号金库操作被拒绝", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "reason": err.Error()})
			if err = client.output.send(1, requestData.frame.ID, accountVaultRefusal(err)); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, err := preparePackets(client.keys, plan)
		if err != nil {
			client.event(map[string]any{"kind": "账号金库回包编码失败", "reason": err.Error()})
			return dispatchClose
		}
		if err = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && (requestData.frame.ID == 307 || requestData.frame.ID == 308) && client.worldState != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		plan, err := client.worldState.changeAccountVaultGold(ctx, requestData.frame.ID, requestData.plaintext, requestData.frame.Raw, client.keys, client.purchaseSession.prefix)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "账号金库金币操作被拒", "id": requestData.frame.ID, "reason": err.Error(), "plain_hex": hex.EncodeToString(requestData.plaintext)})
			if client.output.send(1, requestData.frame.ID, accountVaultRefusal(err)) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, err := preparePackets(client.keys, plan)
		if err != nil {
			return dispatchClose
		}
		if client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 装备技能栏 / 冷却提醒 / 自定义按键（C2S 2254/2256/2257）。
	// 请求侧的 96B 形状由本机实机帧证实（67 个会话各 1 帧 id=2256，全部 96B、[13]=10）。
	// 应答沿用同一作者在 C2S2382 上的约定：先落库，成功后再回 <同一 op> 的 1 字节 ack。

	return dispatchNext
}

func (client *gameConnection) dispatchEquipmentSkillsAndMoves(requestData *clientRequest) dispatchAction {
	if equipmentSkillEnabled() && requestData.frame.Type == 1 &&
		(requestData.frame.ID == 2254 || requestData.frame.ID == 2256 || requestData.frame.ID == 2257) &&
		client.bootstrapped && requestData.verified && client.worldState != nil && client.worldState.role.ID == client.selectedCharacterID {
		eskCtx, eskCancel := context.WithTimeout(context.Background(), 5*time.Second)
		plan, eskErr := client.worldState.equipmentSkillPackets(eskCtx, requestData.frame.ID, requestData.plaintext)
		eskCancel()
		if eskErr != nil {
			client.event(map[string]any{"kind": "equipment_skill_rejected", "id": requestData.frame.ID,
				"character_id": client.selectedCharacterID, "reason": eskErr.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "id": packet.ID,
				"payload_bytes": len(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == protocol.PrimerTransformOpcode && client.bootstrapped && requestData.verified && client.worldState != nil && client.worldState.role.ID == client.selectedCharacterID {
		// 装备库「誓约 / 晶体变换」（CMD2381）。与 2259 同族：**永远回窗口应答**，
		// 拒因只写 events.jsonl（客户端在这条链上没有失败分支）。
		plan, primerErr := client.worldState.primerTransform(requestData.plaintext, client.event)
		if primerErr != nil {
			client.event(map[string]any{"kind": "primer_transform_rejected",
				"character_id": client.selectedCharacterID, "reason": primerErr.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "payload_bytes": len(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 2382 && client.bootstrapped && requestData.verified && client.worldState != nil && client.worldState.role.ID == client.selectedCharacterID {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		plan, oathErr := client.worldState.oathSelectionPackets(ctx, requestData.plaintext)
		cancel()
		if oathErr != nil {
			client.event(map[string]any{"kind": "oath_selection_rejected", "character_id": client.selectedCharacterID, "reason": oathErr.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 649 && client.bootstrapped && requestData.verified {
		var oldShield uint32
		if client.worldState != nil {
			if b, err := inventory.ReadBag(client.worldState.role.State); err == nil {
				oldShield = b.KnightDeck()[0]
			}
		}
		plan, deckErr := client.equipmentState.handleKnightDeck(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw)
		client.event(knightShieldObservation(client.worldState, 649, requestData.plaintext, oldShield, deckErr))
		if sendErr := client.output.send(1, 649, protocol.KnightDeckAck()); sendErr != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "knight_deck_acknowledged", "type": 1, "id": 649, "payload_bytes": 3})
		if deckErr == nil {
			if client.sendPlan(plan, func(packet outboundPacket) {
				client.event(map[string]any{"kind": packet.Name, "type": packet.Kind, "id": packet.ID, "payload_bytes": len(packet.Payload), "plain_hex": hex.EncodeToString(packet.Payload)})
			}) != nil {
				return dispatchClose
			}
		}
		return dispatchHandled
	}
	if requestData.frame.ID == 19 && client.bootstrapped && requestData.verified && client.wearService != nil {
		shieldRequest, shieldDecodeErr := protocol.DecodeItemMove(requestData.plaintext)
		shieldMove := shieldDecodeErr == nil && inventory.IsKnightShieldMove(shieldRequest)
		var oldShield uint32
		if shieldMove && client.worldState != nil {
			if b, err := inventory.ReadBag(client.worldState.role.State); err == nil {
				oldShield = b.KnightDeck()[0]
			}
		}
		plan, e := client.equipmentState.handle(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw)
		if shieldMove {
			client.event(knightShieldObservation(client.worldState, 19, requestData.plaintext, oldShield, e))
		}
		if e != nil {
			client.event(map[string]any{"kind": "equipment_move_refused", "reason": e.Error()})
			r, _ := protocol.DecodeItemMove(requestData.plaintext)
			if e = client.output.send(1, 19, protocol.ItemMoveRefused(r, inventory.MoveRefusalCode(e))); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		r, decodeErr := protocol.DecodeItemMove(requestData.plaintext)
		var cloneRefresh []outboundPacket
		var cloneRefreshed bool
		if decodeErr == nil && len(plan) > 0 {
			cloneRefresh, cloneRefreshed, e = dungeonCloneEquipmentRefresh(client.worldState, r)
			if e != nil {
				client.event(map[string]any{"kind": "equipment_dungeon_clone_refresh_error", "error": e.Error()})
				cloneRefreshed = false
			}
		}
		// Mode-0 actor rebuilds can strand the next dungeon room request.
		// Only an actual creature-list move may use this separate refresh.
		if decodeErr == nil && client.characters != nil && moveNeedsCreatureActorAppearance(r) {
			var visual []byte
			visual, e = client.characters.EntryBasicProbe(client.worldState.role, [2]byte{})
			if e == nil {
				plan = append(plan, outboundPacket{"creature_actor_appearance_updated", 0, 2, visual})
			}
		}
		if decodeErr == nil && client.characters != nil && !cloneRefreshed && cloneAvatarRemoval(r, client.wearService.Catalog) {
			// The existing mode-1 repair restores the Avatar association but
			// clears omitted slots across the actor's 48-slot table. Construct
			// its non-avatar restore before appending either packet, and send
			// that restore last, as the dungeon Clone paths already do.
			addition, additionErr := client.characters.EntryAddition(client.worldState.role)
			var restore []byte
			if additionErr == nil {
				restore, additionErr = inventory.NonAvatarWornSpaceUpdate(client.worldState.role.State)
			}
			if additionErr != nil {
				client.event(map[string]any{"kind": "equipment_avatar_addition_error", "error": additionErr.Error()})
			} else {
				plan = append(plan, outboundPacket{"equipment_avatar_addition_refreshed", 0, 2, addition})
				if len(restore) > 0 {
					plan = append(plan, outboundPacket{"equipment_nonavatar_worn_restored", 0, 14, restore})
				}
			}
		}
		if decodeErr == nil && (r.SourceList == 1 || r.DestinationList == 1) {
			sources, sourceErr := cloneAvatarSourcePackets(client.worldState.role.State)
			if sourceErr != nil {
				client.event(map[string]any{"kind": "clone_avatar_source_sync_error", "error": sourceErr.Error()})
				return dispatchClose
			}
			plan = append(plan, sources...)
		}
		plan = client.worldState.appendFameUpdate(plan, client.event)
		if decodeErr == nil && client.characters != nil && client.worldState != nil &&
			((r.SourceList == 3 && r.SourceSlot == 47) || (r.DestinationList == 3 && r.DestinationSlot == 47)) {
			oathCtx, oathCancel := context.WithTimeout(context.Background(), 5*time.Second)
			selection, oathErr := client.gameStore.EquippedOathSelection(oathCtx, client.developmentAccount, client.worldState.role.ID)
			oathCancel()
			if oathErr != nil {
				client.event(map[string]any{"kind": "oath_selection_refresh_error", "character_id": client.worldState.role.ID, "reason": oathErr.Error()})
			} else if info, infoErr := protocol.OathSystemInfo(selection.Level, selection.Option); infoErr == nil {
				plan = append(plan, outboundPacket{"oath_system_info_after_wear", 0, 2839, info})
			}
		}
		if cloneRefreshed {
			plan = append(plan, cloneRefresh...)
		}
		if decodeErr == nil && len(plan) > 0 && moveTouchesWorn(r) {
			plan, e = appendDungeonWornRandomOptions(plan, client.worldState)
			if e != nil {
				client.event(map[string]any{"kind": "equipment_random_option_restore_error", "error": e.Error()})
				return dispatchClose
			}
		}
		prepared, e := preparePackets(client.keys, plan)
		if e != nil {
			client.event(map[string]any{"kind": "equipment_encode_error", "error": e.Error()})
			return dispatchClose
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "character_id": client.worldState.role.ID, "type": p.Kind, "id": p.ID, "payload_bytes": len(p.Payload), "plain_hex": hex.EncodeToString(p.Payload)})
		}); e != nil {
			client.event(map[string]any{"kind": "equipment_write_error", "error": e.Error()})
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.ID == 20 && client.bootstrapped && requestData.verified && client.worldState != nil && len(requestData.plaintext) > 0 {
		// CMD20 SORT_ITEM: the client has already arranged the bag and
		// asks the server to adopt it. Answering is also what clears the
		// client's "inventory in use" latch.
		var plan []outboundPacket
		var e error
		switch requestData.plaintext[0] {
		case 0:
			plan, e = client.sortState.handle(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw)
		case 2, 45:
			plan, e = client.worldState.sortVaultSpace(requestData.plaintext[0])
		case 12:
			plan, e = client.worldState.sortAccountVaultCmd()
		default:
			client.event(map[string]any{"kind": "sort_unsupported_container", "space": requestData.plaintext[0]})
			return dispatchHandled
		}
		if e != nil {
			client.event(map[string]any{"kind": "item_sort_refused", "reason": e.Error()})
			if requestData.plaintext[0] != 0 {
				return dispatchHandled
			}
			if e = client.output.send(1, 20, protocol.Refusal(4)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, e := preparePackets(client.keys, plan)
		if e != nil {
			client.event(map[string]any{"kind": "item_sort_encode_error", "error": e.Error()})
			return dispatchClose
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "character_id": client.worldState.role.ID, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); e != nil {
			client.event(map[string]any{"kind": "item_sort_write_error", "error": e.Error()})
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchCosmeticsAndGold(requestData *clientRequest) dispatchAction {
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 18 {
		if !requestData.verified {
			return dispatchHandled
		}
		// 技能材料原因 2 和晶体契约原因 5 共用真实扣除与幂等事务。
		// 城镇丢弃、非材料物品等请求继续由通用删除路径处理。
		plan, e := client.worldState.deleteSkillMaterial(requestData.plaintext, requestData.frame.Raw)
		if e == nil {
			if client.sendPlan(plan, func(packet outboundPacket) {
				client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
			}) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "skill_material_refused", "reason": e.Error(), "character_id": client.worldState.role.ID})
		plan, e = client.worldState.deleteItems(requestData.plaintext, requestData.frame.Raw)
		if e != nil {
			client.event(map[string]any{"kind": "item_delete_refused", "reason": e.Error(), "character_id": client.worldState.role.ID})
			rows, _ := protocol.DecodeMaterialDelete(requestData.plaintext)
			reply := protocol.MaterialDeleteReply(rows, false)
			if contractRows, decodeErr := protocol.DecodeCubeContractDelete(requestData.plaintext); decodeErr == nil {
				reply = protocol.CubeContractDeleteReply(contractRows, false)
			}
			if e = client.output.send(1, 18, reply); e != nil {
				return dispatchClose
			}
			client.event(map[string]any{"kind": "item_delete_refusal_sent", "character_id": client.worldState.role.ID, "plain_hex": hex.EncodeToString(reply)})
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 507 {
		if !requestData.verified {
			return dispatchHandled
		}
		if len(requestData.plaintext) >= 11 && binary.LittleEndian.Uint32(requestData.plaintext[7:11]) == protocol.SeasonCapsuleAction {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			packets, err := client.worldState.useSeasonCapsule(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
			cancel()
			if err != nil {
				client.event(map[string]any{"kind": "season_capsule_refused", "character_id": client.selectedCharacterID, "reason": err.Error()})
				if slot, decodeErr := protocol.DecodeSeasonCapsule(requestData.plaintext); decodeErr == nil {
					if err = client.output.send(1, 507, protocol.SeasonCapsuleReply(slot, false)); err != nil {
						return dispatchClose
					}
				}
				return dispatchHandled
			}
			if client.sendPlan(packets, client.logCharacterResponse) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if len(requestData.plaintext) >= 11 && binary.LittleEndian.Uint32(requestData.plaintext[7:11]) == boostCapsuleAction {
			// Starter Boost 662 直升胶囊（[action type] 337，S-0904 实测）。
			// 被拒时沿用本帧既有动作分支的口径：记事件、不凭猜测补造回执。
			plan, e := client.worldState.useBoostCapsule(requestData.plaintext, client.event)
			if e != nil {
				client.event(map[string]any{"kind": "boost_capsule_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
				return dispatchHandled
			}
			// 取证口径：这一串帧要证明的就是「等级到底写没写进客户端」，
			// 只记 kind 不够，按 body 档记 id + plain_hex（纯日志，不改任何帧内容）。
			if client.sendPlan(plan, client.logWorldResponseBody) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		// CMD507 is the shared "use stackable" frame. Split it by action so
		// the fatigue potion (54) and `[add skin storage]` (169, damage font)
		// paths never collide; the fatigue path keeps its exact prior shape.
		_, action, actionErr := protocol.DecodeStackableAction(requestData.plaintext)
		if len(requestData.plaintext) >= 11 && binary.LittleEndian.Uint32(requestData.plaintext[7:11]) == protocol.AvatarInventoryExpansionAction {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var plan []outboundPacket
			var e error
			if client.characters == nil || client.gameStore == nil || client.purchaseSession == nil {
				e = fmt.Errorf("avatar inventory expansion storage unavailable")
			} else {
				plan, e = client.worldState.useAvatarInventoryExpansion(ctx, client.gameStore, client.shopPilot, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix, client.event)
			}
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "avatar_inventory_expansion_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
				if client.output.send(1, 507, protocol.AvatarInventoryExpansionRefused(binary.LittleEndian.Uint16(requestData.plaintext), strings.Contains(e.Error(), "fully expanded"))) != nil {
					return dispatchClose
				}
				return dispatchHandled
			}
			if client.sendPlan(plan, client.logWorldResponseBody) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if actionErr == nil && action == protocol.RosterBackgroundTicketAction {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			packets, e := client.worldState.useRosterBackgroundTicket(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix, client.event)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "背景券使用被拒绝", "character_id": client.worldState.role.ID, "reason": e.Error()})
				return dispatchHandled
			}
			if client.sendPlan(packets, client.logWorldResponseBody) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if actionErr == nil && action == protocol.AddSkinStorageAction {
			plan, e := client.worldState.useAddSkinStorage(requestData.plaintext, client.event)
			if e != nil {
				client.event(map[string]any{"kind": "add_skin_storage_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
				return dispatchHandled
			}
			if client.sendPlan(plan, client.logWorldResponse) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if len(requestData.plaintext) >= 11 && binary.LittleEndian.Uint32(requestData.plaintext[7:11]) == 206 {
			plan, e := client.worldState.useQuestAirshipItem(requestData.plaintext, client.event)
			if e != nil {
				client.event(map[string]any{"kind": "quest_item_action_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
				return dispatchHandled
			}
			if client.sendPlan(plan, client.logWorldAction) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if actionErr == nil && (action == protocol.ActionOpenAuraSkinSlot || action == protocol.ActionOpenCreatureSkinSlot) {
			// 幻化栏扩展券：光环 action 101、宠物 action 197。同一个 CMD507 上复用多种
			// 动作，这里只接这两路，54/169/206 保持各自原有的入口形状。
			plan, e := client.worldState.stackableAction(requestData.plaintext)
			if e != nil {
				client.event(map[string]any{"kind": "skin_slot_expand_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
				return dispatchHandled
			}
			if client.sendPlan(plan, client.logWorldAction) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.fatigueService == nil {
			client.event(map[string]any{"kind": "fatigue_potion_refused", "character_id": client.worldState.role.ID, "reason": "fatigue service unavailable"})
			return dispatchHandled
		}
		plan, e := client.worldState.recoverFatiguePotion(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "fatigue_potion_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldAction) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// CMD857 = ENUM_CMDPACKET_OPEN_AURA_SKIN_SLOT：幻化栏窗口弹「Unlock the Aura
	// Skin slot?」时点 OK 发的就是它。以前没有这一分支，服务端既不置位也不回包，客户
	// 端拿不到成功标志就永远不推进 —— 玩家看到的就是「点了没反应」。
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 857 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "open_skin_slot_rejected", "reason": "幻化栏开启请求校验失败"})
			return dispatchHandled
		}
		plan, e := client.worldState.openSkinSlot(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "open_skin_slot_refused", "character_id": client.worldState.role.ID,
				"reason": e.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 增幅摧毁装备后客户端会把金币显示清 0（存档是对的）。
	// 挂在会话上的延后补发在这里出队 —— 放在主循环里串行发送，避免并发写 socket。
	if client.worldState != nil {
		if body := client.equipmentState.takePendingGold(time.Now()); len(body) > 0 {
			if err := client.output.send(0, 14, body); err != nil {
				return dispatchClose
			}
			client.event(map[string]any{"kind": "amplify_gold_resynced", "character_id": client.worldState.role.ID})
		}
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 80 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "reinforcement_rejected", "reason": "强化请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.equipmentState.reinforce(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			// 客户端在 CMD80 的错误分支只认错误码（u16）去取 dstr 文案，不认原因字符串。
			// 以前一律发 22，而 22 恰好映射到「材料不足」，于是任何拒绝都被玩家看成材料不够。
			code := reinforcementRefusalCode(err)
			client.event(map[string]any{
				"kind":         "reinforcement_refused",
				"character_id": client.worldState.role.ID,
				"reason":       err.Error(),
				"error_code":   code,
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			// 14529B2F0 的失败分支只使用分发器读取的错误码，并清除等待态。
			if err = client.output.send(1, 80, protocol.Refusal(code)); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// CMD2258 = ENUM_CMDPACKET_EQUIPMENT_AWAKENING：装备调适。
	// 规则全部来自直读的 etc/115lvability/equipmentawakeningoptionsystem.cos；
	// 回包体 = u8 状态（0 = 成功）+ u16 结果码（客户端 sub_140B899B0）。

	return dispatchNext
}

func (client *gameConnection) dispatchEnhancementAndConsumables(requestData *clientRequest) dispatchAction {
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == protocol.EquipmentAwakeningOpcode {
		if !requestData.verified {
			client.event(map[string]any{"kind": "equipment_awakening_rejected", "reason": "装备调适请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.equipmentState.awakenEquipment(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{"kind": "equipment_awakening_refused", "character_id": client.worldState.role.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
			if err = client.output.send(1, protocol.EquipmentAwakeningOpcode, protocol.EquipmentAwakeningFailure()); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == protocol.SoleQualityOpcode {
		// CMD2288 = ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY：秘宝精度提升。
		// 规则全部来自直读的 etc/115lvability/soleequipmentsystem.cos；精度落在
		// 装备实例行 +172（internal/character/fame.go 消费的同一格）。
		// 回包 = kind 1、体 = u8 1 + u8 容器 + u16 槽位（**必须发**，否则客户端
		// 精度窗口卡在等待态；见 cmd/wireprobe/sole_flow.go 的文件头注释）。
		if !requestData.verified {
			client.event(map[string]any{"kind": "sole_quality_rejected", "reason": "秘宝精度请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.equipmentState.raiseSoleQuality(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{"kind": "sole_quality_refused", "character_id": client.worldState.role.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
			// 失败也把 ack 发掉（计划里已经带了），否则客户端窗口会卡住。
			if client.sendPlan(plan, nil) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == protocol.SoleCreateOpcode {
		// CMD2289 = ENUM_CMDPACKET_SOLE_EQUIPMENT_CREATE：秘宝制作（把半成品做成成品）。
		// 内容真源 = 直读 etc/115lvability/soleequipmentsystem.cos 的 `[create need materials]`
		// 段（与精度提升的 `[quality need materials]` 是两套独立表）；成品落进背包装备槽。
		// 回包 = kind 1、体 = u8 1 + u32 成品模板（**必须发**，否则制作窗口卡在等待态；
		// 见 cmd/wireprobe/sole_flow.go 的 raiseSoleCreate 文件头注释）。
		if !requestData.verified {
			client.event(map[string]any{"kind": "sole_create_rejected", "reason": "秘宝制作请求校验失败"})
			return dispatchHandled
		}
		plan, followDelay, err := client.equipmentState.raiseSoleCreate(client.wearService, client.worldState, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{"kind": "sole_create_refused", "character_id": client.worldState.role.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
			// 失败也把 ack 发掉（计划里已经带了），否则客户端窗口会卡住。
			if client.sendPlan(plan, nil) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		// 成功：**ack（plan[0]）必须立即发**（否则客户端清不掉等待态），其余结果刷新包
		// 延后 followDelay —— 见 sole_flow.go 的「结果刷新包的时机实验」。
		// 这里刻意阻塞：客户端正在播制作演出，这一小段时间不会要别的包。
		if len(plan) > 0 {
			if client.sendPlan(plan[:1], nil) != nil {
				return dispatchClose
			}
			plan = plan[1:]
			if followDelay > 0 {
				time.Sleep(followDelay)
			}
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 205 {
		// CMD205 = ENUM_CMDPACKET_INVEST_ITEM_AMPLIFY_OPTION：用增幅书（红字书）
		// 给装备打次元属性。
		//
		// ★ 2026-09-28：这条分派以前**整条缺失**。服务层的 ApplyAmplifyGrimoire 与
		// 流程层的 applyAmplifyGrimoire 都在、清单也装载了，但没有任何地方调用它们
		// —— 客户端发 205 得到的是「既不改状态也不回包」，玩家看到的就是
		// 「增幅书打了没效果」（白银书与黄金书都受影响）。
		if !requestData.verified {
			client.event(map[string]any{"kind": "amplify_grimoire_rejected", "reason": "打红字请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.worldState.applyAmplifyGrimoire(client.wearService, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{
				"kind":         "amplify_grimoire_refused",
				"character_id": client.worldState.role.ID,
				"reason":       err.Error(),
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			if err = client.output.send(1, 205, amplifyGrimoireRefusal()); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 430 {
		// CMD430 = 锻造（Refine，NPC Kiri）：仅武器、上限 +8、失败等级不变不碎。
		if !requestData.verified {
			client.event(map[string]any{"kind": "refine_rejected", "reason": "锻造请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.worldState.refine(client.wearService, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{
				"kind": "refine_refused", "character_id": client.worldState.role.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
			})
			// 锻造与其它升级命令共用同一张错误码表（唯一差别是 17 那格：
			// 锻造是 35076 "The equipment cannot be refined."，CMD80 是 1652）。
			code := refineRefusalCode(err)
			if err = client.output.send(1, 430, protocol.Refusal(code)); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 272 {
		// CMD272 = 附魔宝珠（ENCHANT_BY_BEAD）：扣 1 颗宝珠、把卡的附魔写进装备行。
		if !requestData.verified {
			client.event(map[string]any{"kind": "enchant_rejected", "reason": "附魔请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.worldState.enchantByBead(client.wearService, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{
				"kind": "enchant_refused", "character_id": client.worldState.role.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext),
			})
			if err = client.output.send(1, 272, protocol.Refusal(enchantRefusalCode(err))); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// CMD1722 = 装备继承：把材料件的强化 / 增幅 / 锻造 / 附魔转移到基础件，
	// 材料件清零保留。一次请求可带多条记录（多对装备同时轮换继承）。
	//
	// ★ 2026-09-28：这条分派以前**整条缺失**。客户端按下确认后直发 1722，
	// 服务端既不改状态也不回包，客户端就一直停在等待态 —— 玩家看到的就是
	// 「按下继承毫无效果」。与 CMD205（增幅书）那次是同一类缺陷。
	//
	// ⚠️⚠️ **任何方向、任何形态的 1722 出站包都禁止**。客户端的 opcode 表是
	// 两套独立命名空间：kind=1 → CMD 表，1722 在那一侧是客户端自己发出去的
	// 命令、没有接收 handler，回了会被当成「自己发的继承命令」解析、格式不符；
	// kind=0 → NOTI 表，1722 在那一侧的 handler 是**小游戏道具使用计数通知**
	// （sub_143348A90），与继承结果无关。所以成功 / 失败 / 拒绝都
	// **只落库 + 发 id14 行刷新，不回任何 1722 包**。取证见
	// internal/game/protocol/inherit.go 末尾的注释块。
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 1722 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "inherit_rejected", "reason": "继承请求校验失败"})
			return dispatchHandled
		}
		plan, err := client.worldState.inherit(client.wearService, requestData.plaintext, requestData.frame.Raw, client.event)
		if err != nil {
			client.event(map[string]any{
				"kind":         "inherit_refused",
				"character_id": client.worldState.role.ID,
				"reason":       err.Error(),
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			// 拒绝只记日志：1722 没有任何合法的回包通道（见上方注释），
			// 存档也没变所以无需刷新包。
			return dispatchHandled
		}
		if client.sendPlan(plan, nil) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 1565 {
		if !requestData.verified {
			return dispatchHandled
		}
		// CMD1565 is the skin cargo's 应用 click. The client sends it and
		// waits: nothing on screen changes until the selection frame comes
		// back, which is why an applied damage font used to look inert.
		plan, e := client.worldState.selectSkin(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "skin_selection_failed", "character_id": client.worldState.role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID, "id": packet.ID,
				"plain_hex": hex.EncodeToString(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 武器幻化复制确认（CMD1592）：窗口点确认后服务端扣武器本体 + 一枚模具，
	// 并把皮肤登记进幻化仓库。包体只有八字节，模板要靠 Index 自己解析。
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 1592 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "make_skin_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.makeSkin(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "make_skin_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 武器幻化应用（CMD1565）：幻化仓库窗口的容器同步。开页签与按 Apply 都发这
	// 一条，而 Apply 处理器硬编码 subtype=4（武器外观页），所以 subtype 4 且带皮
	// 肤 id 的帧就是「把这个外观应用到我的武器上」。落库后立刻用 opcode 2 的
	// mode0 用户信息块重建角色——装备外观块是驱动世界模型的唯一通道。
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 1565 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "skin_cargo_sync_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.syncSkin(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "skin_cargo_sync_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 44 && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "item_use_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.useStackable(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "item_use_refused", "character_id": client.worldState.role.ID, "reason": e.Error()})
			if r, decodeErr := protocol.DecodeUseStackable(requestData.plaintext); decodeErr == nil {
				if e = client.output.send(1, 44, protocol.UseStackableRefused(r)); e != nil {
					return dispatchClose
				}
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 表情快捷键（CMD1551）：体是 `u32 表情皮肤 id, u32 0`。实机 2026-09-28 三格三个值
	// （10179 / 10176 / 10175），都是 configs/skin-storage-items.json 里 instant emoticon 的
	// 皮肤 id（模板 10325568 / 10325577 等）⇒ 体首就是玩家按下的那个表情。
	//
	// 这里**只留观测，不回包**：attempt 1/3 试过回 CMD 2039（体一个 01 状态字节，客户端
	// sub_1444E8CC0 状态非 0 只发界面事件 2345），四格各按一次、服务端六条 emote_use_
	// acknowledged，实机**没有气泡**⇒ 已否证，帧撤掉。另有一条独立理由：2039 的处理器一个
	// 体字节都不读，带不了「哪个角色放哪个表情」，而气泡必须点名角色。1551 仍登记进
	// request_scope（见那里的注释）：不登记就只解密前八次，第八条之后连这条日志都没有。

	return dispatchNext
}

func (client *gameConnection) dispatchEquipmentTransactions(requestData *clientRequest) dispatchAction {
	if client.worldState != nil && client.bootstrapped && requestData.verified && requestData.frame.ID == 1551 {
		if len(requestData.plaintext) < 4 {
			client.event(map[string]any{"kind": "emote_use_rejected", "reason": "short body",
				"bytes": len(requestData.plaintext)})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "emote_use_observed", "character_id": client.worldState.role.ID,
			"skin_id": binary.LittleEndian.Uint32(requestData.plaintext)})
		return dispatchHandled
	}
	// 装备库（装备图鉴）「制作 / 变换」：CMD2259。
	// **阶段一：只回 6 字节应答（"打开哪个制作窗口"），不碰存档。**
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 2259 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "equipment_craft_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.equipmentCraft(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "equipment_craft_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID,
				"id": packet.ID, "bytes": len(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 装备库（装备图鉴）收藏：CMD2264。
	// 回包顺序 = 先提交账本 → 回 2264（**非空**正文）→ 补发 2610 权威快照。
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 2264 && client.lootService != nil && client.journalRules != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "equipment_journal_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.equipmentFavorite(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "equipment_journal_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(19)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID, "id": packet.ID, "bytes": len(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 795 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "avatar_recast_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.recastAvatar(client.wearService, requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "avatar_recast_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			// Native CMD795 displays a refusal popup for error 127.
			if client.output.send(1, requestData.frame.ID, protocol.Refusal(127)) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 201 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "avatar_emblem_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.useEmblems(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "avatar_emblem_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(17)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 206 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "avatar_socket_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.addAvatarSocket(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "avatar_socket_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(19)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 256 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "emblem_compound_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.compoundEmblems(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "emblem_compound_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			refusalCode := uint16(19)
			if strings.Contains(e.Error(), "bag category is full") {
				refusalCode = 4
			}
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(refusalCode)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 202 && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "avatar_disjoint_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.disjointAvatar(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "avatar_disjoint_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			refusalCode := uint16(19)
			if strings.Contains(e.Error(), "bag category is full") {
				refusalCode = 4
			}
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(refusalCode)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 26 && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "disjoint_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.disjointItem(requestData.plaintext, client.event)
		if e != nil {
			client.event(map[string]any{"kind": "disjoint_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			refusalCode := uint16(19)
			if strings.Contains(e.Error(), "material inventory is full") {
				refusalCode = 4
			}
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(refusalCode)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 393 && client.unsealService != nil && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "unseal_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, request, e := client.worldState.unsealRandomOption(client.unsealService, requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "unseal_refused", "id": requestData.frame.ID, "character_id": client.worldState.role.ID, "reason": e.Error()})
			if e = client.output.send(1, requestData.frame.ID, protocol.UnsealRefused(unsealRefusalCode(e))); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "character_id": client.worldState.role.ID, "id": packet.ID, "slot": request.TargetSlot})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 21 && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "shop_buy_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "shop_buy_request", "character_id": client.worldState.role.ID, "id": 21, "plain_hex": hex.EncodeToString(requestData.plaintext)})
		plan, e := client.worldState.buyItem(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "shop_buy_refused", "character_id": client.worldState.role.ID, "reason": e.Error(), "plain_hex": hex.EncodeToString(requestData.plaintext)})
			if e = client.output.send(1, 21, protocol.Refusal(4)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 22 && client.lootService != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "shop_sell_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "shop_sell_request", "character_id": client.worldState.role.ID, "id": 22, "plain_hex": hex.EncodeToString(requestData.plaintext)})
		plan, e := client.worldState.sellItem(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "shop_sell_refused", "character_id": client.worldState.role.ID, "reason": e.Error(), "plain_hex": hex.EncodeToString(requestData.plaintext)})
			if e = client.output.send(1, 22, protocol.Refusal(4)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}
