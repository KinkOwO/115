package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"fmt"
	"time"
)

func (client *gameConnection) dispatchAccountQueries(requestData *clientRequest) dispatchAction {
	if client.bootstrapped && requestData.frame.ID == 1960 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "server_time_request_rejected", "reason": "服务器时间请求校验失败"})
			return dispatchHandled
		}
		if err := protocol.DecodeServerTimeRequest(requestData.plaintext); err != nil {
			client.event(map[string]any{"kind": "server_time_request_rejected", "reason": err.Error()})
			return dispatchHandled
		}
		if err := client.sendServerTime("客户端请求"); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.bootstrapped && requestData.frame.ID == 1395 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "adventure_request_rejected", "reason": "冒险团请求校验失败"})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		payload, err := client.worldState.handleAdventure(ctx, client.selectedCharacterID, requestData.plaintext)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "adventure_request_rejected", "character_id": client.selectedCharacterID, "reason": err.Error()})
			// 原生处理器不检查成功参数，不能给它发送仅三字节的
			// 通用拒绝体，否则它仍会读取完整详情并越界。
			return dispatchHandled
		}
		if err := client.output.send(1, requestData.frame.ID, payload); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "adventure_info_sent", "character_id": client.selectedCharacterID, "id": requestData.frame.ID, "attempt": "4（CMD217与原生包尾读取链已核实）", "plain_hex": hex.EncodeToString(payload)})
		return dispatchHandled
	}
	if client.bootstrapped && client.selectedCharacterID != 0 && (requestData.frame.ID == 1406 || requestData.frame.ID == 2331 || requestData.frame.ID == 1719 || requestData.frame.ID == 1811 || requestData.frame.ID == 2419 || requestData.frame.ID == 2405 || requestData.frame.ID == 2139) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "adventure_request_rejected", "id": requestData.frame.ID, "reason": "冒险团命令校验失败"})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var packets []outboundPacket
		var err error
		switch requestData.frame.ID {
		case 2139:
			client.event(map[string]any{"kind": "图鉴引导登记请求", "attempt": "2/3（按原生CMD33更新单任务进度）", "character_id": client.selectedCharacterID})
			packets, err = client.worldState.registerAdventureCollection(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
		case 2419:
			packets, err = client.worldState.claimSeasonReward(ctx, requestData.plaintext)
		case 2405:
			packets, err = client.worldState.acquireSeasonOath(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
		case 2331:
			packets, err = client.worldState.setAdventureBestHonor(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
		case 1719:
			packets, err = client.worldState.setAdventureElite(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
		case 1811:
			packets, err = client.worldState.loadAdventureElite(ctx, requestData.plaintext)
		default:
			packets, err = client.worldState.buyAdventureItem(ctx, requestData.plaintext, requestData.frame.Raw, client.purchaseSession.prefix)
		}
		cancel()
		if requestData.frame.ID == 1719 || requestData.frame.ID == 1811 {
			client.event(adventureEliteDiagnostic(client.worldState, requestData.frame.ID, packets, err))
		}
		if err != nil {
			client.event(map[string]any{"kind": "adventure_request_rejected", "id": requestData.frame.ID, "reason": err.Error()})
			if err = client.output.send(1, requestData.frame.ID, adventureFailure(requestData.frame.ID, err)); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(packets, client.logCharacterResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.bootstrapped && mailboxRequest(requestData.frame.ID) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "mailbox_request_rejected", "id": requestData.frame.ID, "reason": "邮箱请求校验失败"})
			return dispatchHandled
		}
		mailCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		packets, recipient, err := client.worldState.handleMailbox(mailCtx, client.selectedCharacterID, requestData.frame.ID, requestData.plaintext, requestData.frame.Raw, client.keys, client.purchaseSession.prefix)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "mailbox_request_rejected", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "reason": err.Error()})
			// CMD781 的原生完成路径是 NOTI705；失败只记录，不猜测命令应答。
			if requestData.frame.ID == 781 {
				return dispatchHandled
			}
			if err := client.output.send(1, requestData.frame.ID, mailboxFailure(requestData.frame.ID, err)); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		// 投递已提交，即使发件人连接随后断开，收件人仍应收到通知。
		if recipient != 0 && client.hub != nil {
			client.hub.notifyMailbox(recipient)
		}
		if client.sendPlan(packets, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		// CMD95/134 原生回调会更新附件与已读/删除状态，不再发送 NOTI99。
		// NOTI99 会重新请求 CMD96；NOTI97 全量恢复经 0x145FCF970 销毁旧
		// 邮件对象，而详情领取路径 0x145FD6A50 仍会访问已选对象。
		// 保留登录和真正新投递的提醒，避免读信后刷新造成悬空引用。
		return dispatchHandled
	}
	if client.bootstrapped && requestData.frame.ID == 2261 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "special_warp_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, err := client.worldState.prepareSpecialWarp(requestData.plaintext)
		if err != nil {
			client.event(map[string]any{"kind": "special_warp_rejected", "reason": err.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	if client.bootstrapped && requestData.frame.ID == 2285 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "exit_dialog_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		if len(requestData.plaintext) != 0 {
			client.event(map[string]any{"kind": "exit_dialog_rejected", "reason": "unexpected request body", "bytes": len(requestData.plaintext)})
			return dispatchHandled
		}
		// CMD2285 is CONTENT_BRIEFING, issued by the in-game menu before
		// its local Exit path. Its callback at 145250c80 consumes a
		// mandatory 0x80-byte structure after the common success header.
		// A header-only reply triggers CMD217 (CMDPACKET_OVERFLOW_INFO).
		if err := client.output.send(1, requestData.frame.ID, protocol.ExitDialogReady()); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "exit_dialog_ready", "id": requestData.frame.ID, "character_id": client.selectedCharacterID})
		return dispatchHandled
	}
	if client.bootstrapped && requestData.frame.ID == 682 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "exit_shutdown_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		fastExit, e := protocol.DecodeExitShutdownSignal(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "exit_shutdown_rejected", "reason": e.Error(), "bytes": len(requestData.plaintext)})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "exit_shutdown_signal", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "fast": fastExit})
		client.selectedCharacterID = 0
		client.selectedBasic, client.selectedAddition = nil, nil
		if client.worldState != nil {
			client.worldState.departArea()
		}
		clearSelectedWorld(client.worldState)
		client.bootstrapped = false
		client.event(map[string]any{"kind": "exit_session_closed", "peer": client.peer})
		return dispatchClose
	}

	return dispatchNext
}

func (client *gameConnection) dispatchUnifiedOptions(requestData *clientRequest) dispatchAction {
	if client.bootstrapped && requestData.frame.ID == 2377 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "unified_option_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		// CMD2377 SET_UNIFIED_OPTION carries one option block per frame:
		// subtype 0x13 is the skill lock, 0x12 character effects, 0x05
		// ordinary settings, 0x0b warp favorite deltas, and 0x01 the
		// account settings. Account options restore through NOTI2826.
		opt, e := protocol.DecodeUnifiedOption(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "unified_option_rejected", "reason": e.Error(), "bytes": len(requestData.plaintext)})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "unified_option_accepted", "character_id": client.selectedCharacterID, "scope": opt.Scope, "subtype": opt.Subtype, "entries": len(opt.Entries) + len(opt.WarpFavorites)})
		switch opt.Subtype {
		case protocol.UnifiedOptionWarpFavorites:
			if client.gameStore == nil || client.characters == nil || client.worldState == nil || client.worldState.role.ID == 0 || client.worldState.role.ID != client.selectedCharacterID || client.worldState.role.AccountID != client.developmentAccount {
				client.event(map[string]any{"kind": "warp_favorites_rejected", "reason": "requires the owned selected character", "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = client.gameStore.SaveAccountWarpFavorites(ctx, client.worldState.role.AccountID, opt.WarpFavorites)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "warp_favorites_rejected", "reason": e.Error(), "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			// This means the transaction committed, not that the client displayed it.
			client.event(map[string]any{"kind": "warp_favorites_saved", "account_id": client.worldState.role.AccountID, "character_id": client.selectedCharacterID, "changed_slots": len(opt.WarpFavorites)})
		case protocol.UnifiedOptionSkillLock:
			if client.characters == nil || client.worldState == nil || client.worldState.role.ID == 0 || client.worldState.role.ID != client.selectedCharacterID {
				client.event(map[string]any{"kind": "skill_lock_rejected", "reason": "skill lock requires the owned selected character", "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			sum := sha256.Sum256(requestData.plaintext)
			key := fmt.Sprintf("skill-lock-v1:%x", sum)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			locks, applied, e := client.characters.SaveSkillLocks(ctx, client.worldState.role, key, opt)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "skill_lock_rejected", "reason": e.Error(), "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "skill_lock_saved", "character_id": client.selectedCharacterID, "applied": applied, "count": len(locks), "locks": locks})
		case protocol.UnifiedOptionSettings:
			if client.characters == nil || client.worldState == nil || client.worldState.role.ID == 0 || client.worldState.role.ID != client.selectedCharacterID {
				client.event(map[string]any{"kind": "character_settings_rejected", "reason": "requires the owned selected character", "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = client.gameStore.SaveCharacterUnifiedOptions(ctx, client.worldState.role.AccountID, client.worldState.role.ID, unifiedEntries(opt.Entries))
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "character_settings_rejected", "reason": e.Error(), "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "character_settings_saved", "character_id": client.selectedCharacterID, "entries": len(opt.Entries)})
		case protocol.UnifiedOptionCharacterEffects:
			if opt.Scope != protocol.UnifiedOptionScopeCharac || client.characters == nil || client.worldState == nil || client.worldState.role.ID == 0 || client.worldState.role.ID != client.selectedCharacterID {
				client.event(map[string]any{"kind": "character_effect_options_rejected", "reason": "requires the owned selected character", "character_id": client.selectedCharacterID, "scope": opt.Scope})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = client.gameStore.SaveCharacterUnifiedOptionGroup(ctx, client.worldState.role.AccountID, client.worldState.role.ID, opt.Subtype, unifiedEntries(opt.Entries))
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "character_effect_options_rejected", "reason": e.Error(), "character_id": client.selectedCharacterID})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "character_effect_options_saved", "character_id": client.selectedCharacterID, "entries": len(opt.Entries), "options": opt.Entries})
		case protocol.UnifiedOptionAccount:
			if client.characters == nil || opt.Scope != protocol.UnifiedOptionScopeAccount {
				client.event(map[string]any{"kind": "account_settings_rejected", "reason": "账号设置存储不可用或作用域无效"})
				return dispatchHandled
			}
			accountID := client.developmentAccount
			ownedRole := client.worldState != nil && client.worldState.role.ID != 0 && client.worldState.role.ID == client.selectedCharacterID
			if ownedRole {
				accountID = client.worldState.role.AccountID
			}
			var effectFlags byte
			for _, entry := range opt.Entries {
				if entry.Position == protocol.GrowthEffectOption && entry.Value != 65535 {
					effectFlags, e = protocol.GrowthEffectFlags(entry.Value)
					if e != nil {
						break
					}
				}
			}
			if e != nil {
				client.event(map[string]any{"kind": "account_settings_rejected", "reason": e.Error()})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = client.gameStore.SaveAccountUnifiedOptions(ctx, accountID, unifiedEntries(opt.Entries))
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "account_settings_rejected", "reason": e.Error()})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "account_settings_saved", "entries": len(opt.Entries)})
			if effectFlags != 0 && ownedRole {
				// attempt 1/3：按1452FA194注册及1452C9940读取发送NOTI343。
				// 只更新显示阶段；不发送会重建装备、技能的全量USERINFO。
				effect := protocol.CharacterGrowthEffect(client.worldState.role.WireID, effectFlags)
				if e = client.output.send(0, 343, effect); e != nil {
					return dispatchClose
				}
				basic, refreshErr := client.characters.EntryBasicProbe(client.worldState.role, [2]byte{})
				if refreshErr != nil {
					client.event(map[string]any{"kind": "growth_effect_cache_error", "reason": refreshErr.Error()})
				} else {
					client.selectedBasic = basic
					client.worldState.hub.updateGrowthEffect(client.worldState.peer, basic, effect)
				}
				client.event(map[string]any{"kind": "growth_effect_updated", "character_id": client.selectedCharacterID, "flags": effectFlags})
			}
		case protocol.UnifiedOptionHotkeys, protocol.UnifiedOptionHotkeysExt:
			if client.characters == nil {
				client.event(map[string]any{"kind": "hotkeys_rejected", "reason": "storage unavailable"})
				return dispatchHandled
			}
			charID := client.selectedCharacterID
			if charID == 0 && client.worldState != nil && client.worldState.role.ID != 0 {
				charID = client.worldState.role.ID
			}
			accountID := client.developmentAccount
			if client.worldState != nil && client.worldState.role.AccountID != 0 {
				accountID = client.worldState.role.AccountID
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if opt.Scope == protocol.UnifiedOptionScopeAccount {
				if charID != 0 {
					_ = client.gameStore.PromoteCharacterHotkeysToAccount(ctx, accountID, charID, opt.Subtype)
				}
				if len(opt.Entries) > 0 {
					e = client.gameStore.SaveAccountHotkeys(ctx, accountID, opt.Subtype, unifiedEntries(opt.Entries))
				}
				if e == nil {
					_ = client.gameStore.ClearAccountCharacterHotkeys(ctx, accountID, opt.Subtype)
				}
				cancel()
				if e != nil {
					client.event(map[string]any{"kind": "account_hotkeys_rejected", "reason": e.Error(), "subtype": opt.Subtype})
					return dispatchHandled
				}
				client.event(map[string]any{"kind": "account_hotkeys_saved", "subtype": opt.Subtype, "entries": len(opt.Entries), "character_id": charID})
			} else {
				if charID == 0 {
					cancel()
					client.event(map[string]any{"kind": "character_hotkeys_rejected", "reason": "requires the owned selected character", "character_id": client.selectedCharacterID})
					return dispatchHandled
				}
				_ = client.gameStore.CopyAccountHotkeysToCharacter(ctx, accountID, charID, opt.Subtype)
				e = client.gameStore.SaveCharacterHotkeys(ctx, accountID, charID, opt.Subtype, unifiedEntries(opt.Entries))
				cancel()
				if e != nil {
					client.event(map[string]any{"kind": "character_hotkeys_rejected", "reason": e.Error(), "character_id": charID, "subtype": opt.Subtype})
					return dispatchHandled
				}
				client.event(map[string]any{"kind": "character_hotkeys_saved", "character_id": charID, "subtype": opt.Subtype, "entries": len(opt.Entries)})
			}
		case protocol.UnifiedOptionHotkeyUI:
			client.event(map[string]any{"kind": "hotkey_ui_event", "character_id": client.selectedCharacterID, "scope": opt.Scope})
		}
		return dispatchHandled
	}

	if client.bootstrapped && (requestData.frame.ID == 1950 || requestData.frame.ID == 1951) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "gamepad_settings_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		if client.characters == nil {
			client.event(map[string]any{"kind": "gamepad_settings_rejected", "id": requestData.frame.ID, "reason": "storage unavailable"})
			return dispatchHandled
		}
		charID := client.selectedCharacterID
		if charID == 0 && client.worldState != nil && client.worldState.role.ID != 0 {
			charID = client.worldState.role.ID
		}
		if requestData.frame.ID == 1950 {
			if len(requestData.plaintext) < 14 {
				client.event(map[string]any{"kind": "gamepad_keys_rejected", "reason": "payload too short", "bytes": len(requestData.plaintext)})
				return dispatchHandled
			}
			scope := requestData.plaintext[13]
			tsv := requestData.plaintext[14:]
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var err error
			var hotPayload []byte
			if scope == 1 {
				err = client.gameStore.SaveAccountGamepadKeys(ctx, client.developmentAccount, tsv)
				if err == nil {
					_ = client.gameStore.ClearAccountCharacterGamepadSettings(ctx, client.developmentAccount)
					hotPayload, _ = client.gameStore.AccountGamepadPayload(ctx, client.developmentAccount)
				}
			} else {
				if charID == 0 {
					cancel()
					client.event(map[string]any{"kind": "gamepad_keys_rejected", "reason": "requires selected character", "bytes": len(requestData.plaintext)})
					return dispatchHandled
				}
				err = client.gameStore.SaveCharacterGamepadKeys(ctx, client.developmentAccount, charID, tsv)
				if err == nil {
					hotPayload, _ = client.gameStore.ResolveGamepadPayload(ctx, client.developmentAccount, charID)
				}
			}
			cancel()
			if err != nil {
				client.event(map[string]any{"kind": "gamepad_keys_save_error", "scope": scope, "error": err.Error()})
				return dispatchHandled
			}
			// 回复 ACK: Kind=1, ID=1950, Payload=[0]
			if err := client.output.send(1, 1950, []byte{0}); err != nil {
				client.event(map[string]any{"kind": "gamepad_keys_ack_error", "error": err.Error()})
				return dispatchHandled
			}
			// 即时热生效：主动向客户端发送最新的 NOTI 2128
			if len(hotPayload) > 0 {
				if err := client.output.send(0, 2128, hotPayload); err != nil {
					client.event(map[string]any{"kind": "gamepad_hot_reload_error", "error": err.Error()})
				} else {
					client.event(map[string]any{"kind": "gamepad_hot_reloaded", "account_id": client.developmentAccount, "character_id": charID, "scope": scope, "bytes": len(hotPayload)})
				}
			}
			client.event(map[string]any{"kind": "gamepad_keys_saved", "account_id": client.developmentAccount, "character_id": charID, "scope": scope, "bytes": len(tsv)})
		} else if requestData.frame.ID == 1951 {
			if len(requestData.plaintext) < 24 {
				client.event(map[string]any{"kind": "gamepad_options_rejected", "reason": "payload too short", "bytes": len(requestData.plaintext)})
				return dispatchHandled
			}
			scope := requestData.plaintext[13]
			opts := requestData.plaintext[14:24]
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var err error
			if scope == 1 {
				err = client.gameStore.SaveAccountGamepadOptions(ctx, client.developmentAccount, opts)
				if err == nil {
					_ = client.gameStore.ClearAccountCharacterGamepadSettings(ctx, client.developmentAccount)
				}
			} else {
				if charID == 0 {
					cancel()
					client.event(map[string]any{"kind": "gamepad_options_rejected", "reason": "requires selected character", "bytes": len(requestData.plaintext)})
					return dispatchHandled
				}
				err = client.gameStore.SaveCharacterGamepadOptions(ctx, client.developmentAccount, charID, opts)
			}
			cancel()
			if err != nil {
				client.event(map[string]any{"kind": "gamepad_options_save_error", "scope": scope, "error": err.Error()})
				return dispatchHandled
			}
			// 回复 ACK: Kind=1, ID=1951, Payload=[0]
			if err := client.output.send(1, 1951, []byte{0}); err != nil {
				client.event(map[string]any{"kind": "gamepad_options_ack_error", "error": err.Error()})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "gamepad_options_saved", "account_id": client.developmentAccount, "character_id": charID, "scope": scope})
		}
		return dispatchHandled
	}

	return dispatchNext
}

// unifiedEntries maps a decoded CMD2377 block to its durable storage shape.
func unifiedEntries(entries []protocol.UnifiedOptionEntry) []database.UnifiedOptionEntry {
	out := make([]database.UnifiedOptionEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, database.UnifiedOptionEntry{Position: e.Position, Value: e.Value})
	}
	return out
}
