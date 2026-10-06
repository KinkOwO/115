package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func (client *gameConnection) dispatchCharacterEntry(requestData *clientRequest) dispatchAction {
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 4 && client.selectProbe != nil {
		if !requestData.verified {
			client.event(map[string]any{"kind": "select_rejected", "error": "checksum or cipher rejected"})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		role, e := client.characters.Select(ctx, client.developmentAccount, requestData.plaintext)
		if e == nil {
			e = prepareRolePVFDetails(ctx, client.characters, client.questService, client.lootService, role)
		}
		if e == nil && client.wearService != nil {
			var migrated bool
			role, migrated, e = client.wearService.MigrateCloneAvatars(ctx, role)
			if migrated {
				client.event(map[string]any{"kind": "clone_avatar_save_migrated", "character_id": role.ID})
			}
		}
		cancel()
		// Legacy third-awakened saves predate the 5-point VP grant: the
		// panel may show 5 points while the ledger still reads zero, and
		// one ordinary Learn response then clears it. Reconcile on
		// selection; the repair is a no-op once the ledger matches.
		if client.characters != nil && e == nil {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			reconciled, backfilled, reconcileErr := client.characters.ReconcileTechniquePoints(ctx, role)
			cancel()
			if reconcileErr != nil {
				client.event(map[string]any{"kind": "technique_points_reconcile_pending", "character_id": role.ID, "reason": reconcileErr.Error()})
			} else {
				role = reconciled
				if backfilled {
					client.event(map[string]any{"kind": "technique_points_reconciled", "character_id": role.ID})
				}
			}
		}
		if e != nil {
			client.event(map[string]any{"kind": "select_rejected", "error": e.Error()})
			return dispatchHandled
		}
		var creation *catalog.OdysseyCreateRewards
		if client.progressionService != nil && client.progressionService.Odyssey != nil {
			creation = client.progressionService.Odyssey.Creation
		}
		if odysseyRewardsEnabled() {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			updated, applied, rewardErr := grantOdysseyArmor(ctx, client.gameStore, client.wearService, role, creation)
			cancel()
			if rewardErr != nil {
				client.event(map[string]any{"kind": "odyssey_armor_pending", "character_id": role.ID, "reason": rewardErr.Error()})
			} else {
				role = updated
				if applied {
					client.event(map[string]any{"kind": "odyssey_armor_granted", "character_id": role.ID, "templates": creation.ArmorTemplates()})
				}
			}
		}
		if odysseyRewardsEnabled() {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			updated, applied, rewardErr := grantOdysseyWeaponBox(ctx, client.gameStore, role, creation)
			cancel()
			if rewardErr != nil {
				client.event(map[string]any{"kind": "odyssey_weapon_box_pending", "character_id": role.ID, "reason": rewardErr.Error()})
			} else {
				role = updated
				if applied {
					client.event(map[string]any{"kind": "odyssey_weapon_box_granted", "character_id": role.ID, "template": creation.Weapon.Template})
				}
			}
		}
		// 创建奖励 10417791 的 [stackable] 块第三行（10418028 x30）。
		// 独立事件键，与上面两项互不干扰；满包/目录未就绪时记 pending，
		// 下次登录自动重试。
		if odysseyRewardsEnabled() {
			if client.lootService == nil {
				client.event(map[string]any{"kind": "odyssey_create_potion_pending", "character_id": role.ID, "reason": "loot catalog unavailable"})
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				updated, applied, rewardErr := grantOdysseyCreatePotion(ctx, client.gameStore, client.lootService.Catalog, client.lootService.BagRules, role, creation)
				cancel()
				if rewardErr != nil {
					client.event(map[string]any{"kind": "odyssey_create_potion_pending", "character_id": role.ID, "reason": rewardErr.Error()})
				} else {
					role = updated
					if applied {
						client.event(map[string]any{"kind": "odyssey_create_potion_granted", "character_id": role.ID, "template": creation.Supplies[0].Template, "quantity": creation.Supplies[0].Count})
					}
				}
			}
		}
		if odysseyTemporaryCreditsEnabled() {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			updated, applied, creditErr := grantOdysseyCredits(ctx, client.gameStore, role)
			cancel()
			if creditErr != nil {
				client.event(map[string]any{"kind": "odyssey_test_credits_pending", "reason": creditErr.Error()})
			} else {
				role = updated
				if applied {
					client.event(map[string]any{"kind": "odyssey_test_credits_granted", "character_id": role.ID, "credits": 10})
				}
			}
		}
		if client.progressionService != nil && client.progressionService.Odyssey != nil {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			caught, repaired, catchErr := client.progressionService.OdysseyCatchup(ctx, role)
			if catchErr != nil {
				client.event(map[string]any{"kind": "odyssey_growth_catchup_pending", "character_id": role.ID, "reason": catchErr.Error()})
			} else {
				role = caught
			}
			if repaired {
				client.event(map[string]any{"kind": "odyssey_growth_catchup_committed", "character_id": role.ID})
			}
			updated, applied, pending := client.progressionService.OdysseyGifts(ctx, role)
			cancel()
			role = updated
			if applied {
				client.event(map[string]any{"kind": "odyssey_milestone_gifts_granted", "character_id": role.ID})
			}
			for _, err := range pending {
				client.event(map[string]any{"kind": "odyssey_milestone_gift_pending", "character_id": role.ID, "reason": err.Error()})
			}
			// 七章奖励：按服务端自有通关成绩补发，逐行独立收据；满包留欠，
			// 下次登录/通关重试。
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			rewarded, chapterApplied, chapterPending := client.progressionService.OdysseyChapterRewards(ctx, role)
			cancel()
			role = rewarded
			if chapterApplied {
				client.event(map[string]any{"kind": "odyssey_chapter_rewards_granted", "character_id": role.ID})
			}
			for _, err := range chapterPending {
				client.event(map[string]any{"kind": "odyssey_chapter_reward_pending", "character_id": role.ID, "reason": err.Error()})
			}
			// Graduation and earlier-level quests share one durable receipt.
			// Honour rewards use an independent idempotent mailbox delivery.
			if client.questService != nil {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				graduated, applied, gradErr := client.questService.GraduateOdyssey(ctx, role)
				cancel()
				if gradErr != nil {
					client.event(map[string]any{"kind": "odyssey_graduation_error", "character_id": role.ID, "reason": gradErr.Error()})
					return dispatchHandled
				}
				role = graduated
				if applied {
					client.event(map[string]any{"kind": "odyssey_graduated", "character_id": role.ID, "model": database.OdysseyGraduationEvent})
				}
			}
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			mailed, mailApplied, mailErr := client.progressionService.OdysseyHonorMail(ctx, role)
			cancel()
			role = mailed
			if mailErr != nil {
				client.event(map[string]any{"kind": "odyssey_honor_mail_pending", "character_id": role.ID, "reason": mailErr.Error()})
			} else if mailApplied {
				client.event(map[string]any{"kind": "odyssey_honor_mail_committed", "character_id": role.ID})
			}
		}
		// 源 `etc/titlebook.etc` `[maxlevel reward]`：满级礼盒属于任何模式的满级角色，
		// 不在奥德赛分支内。收据是事件键，所以已到 115 的老角色在这里补发一次，
		// 之后每次登录都是 no-op。
		if client.progressionService != nil && client.progressionService.MaxLevelReward != nil {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			updated, rewardApplied, rewardErr := client.progressionService.MaxLevelRewardMail(ctx, role)
			cancel()
			role = updated
			if rewardErr != nil {
				client.event(map[string]any{"kind": "max_level_reward_pending", "character_id": role.ID, "reason": rewardErr.Error()})
			} else if rewardApplied {
				client.event(map[string]any{"kind": "max_level_reward_mail_committed", "character_id": role.ID, "template": client.progressionService.MaxLevelReward.Template})
			}
		}
		profile := *client.selectProbe
		if client.itemService != nil && client.itemService.Boxes != nil {
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			updated, applied, repairErr := (&workflow.ItemService{Store: client.gameStore, Items: client.itemService}).RepairBoxRewards(ctx, role)
			cancel()
			if repairErr != nil {
				client.event(map[string]any{"kind": "box_reward_repair_error", "character_id": role.ID, "error": repairErr.Error()})
				return dispatchHandled
			}
			role = updated
			if applied {
				client.event(map[string]any{"kind": "box_rewards_repaired", "character_id": role.ID})
			}
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		profile.TutorialCompleted, e = client.gameStore.TutorialFlags(ctx, client.developmentAccount, role.ID)
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "tutorial_restore_error", "error": e.Error()})
			return dispatchHandled
		}
		// Experiment: native tutorial-progress global dword_14F5AF24 is
		// clamped to >= 0x1E (30) by sub_146CCE300. The leading selectProbe
		// byte (TutorialFlag) initializes it. Sending 1 left it < 30 and the
		// opening recap replayed; sending 30 should clear the gate.
		profile.TutorialFlag = 30
		client.event(map[string]any{"kind": "tutorial_flags_restored", "character_id": role.ID, "tutorial_flag": profile.TutorialFlag, "completed": profile.TutorialCompleted})
		profile.CreatedTime = uint32(role.CreatedAt.Unix())
		// Cera is an account balance the client reads from this
		// response. Without this it stayed at the configured zero,
		// so an operator top-up could never be seen in game.
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		cera, ce := client.gameStore.AccountCera(ctx, client.developmentAccount)
		cancel()
		if ce != nil {
			client.event(map[string]any{"kind": "cera_restore_error", "error": ce.Error()})
			return dispatchHandled
		}
		if cera > 0xffffffff {
			cera = 0xffffffff
		}
		profile.Cash = uint32(cera)
		client.event(map[string]any{"kind": "cera_restored", "account": client.developmentAccount, "cera": profile.Cash})
		// This loopback probe has one development account. A shared
		// multiplayer world will allocate its own unique actor IDs.
		profile.ActorServerID = role.WireID
		var fatiguePayload []byte
		if client.fatigueService != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			fp, err := client.fatigueService.State(ctx, client.developmentAccount, role.ID, time.Now())
			cancel()
			if err != nil {
				client.event(map[string]any{"kind": "fatigue_restore_error", "error": err.Error()})
				return dispatchHandled
			}
			profile.Fatigue = [3]uint16{fp.Used, fp.Limit, fp.UsedMax}
			fatiguePayload, err = protocol.Fatigue(fp.Used, fp.Limit, fp.UsedMax)
			if err != nil {
				client.event(map[string]any{"kind": "fatigue_restore_error", "error": err.Error()})
				return dispatchHandled
			}
			client.event(map[string]any{"kind": "fatigue_restored", "character_id": role.ID, "state": fp})
		}
		if client.questService != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			profile.ActiveQuests, e = client.questService.Active(ctx, role)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "quest_restore_rejected", "error": e.Error()})
				return dispatchHandled
			}
		}
		var basic []byte
		var addition []byte
		var vaultPayload []byte
		var secondaryVaultPayload []byte
		var accountVaultPayload []byte
		if client.vaultService != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			vaultPayload, e = client.vaultService.Bootstrap(ctx, role)
			if e == nil {
				secondaryVaultPayload, e = client.vaultService.BootstrapSpace(ctx, role, 45)
			}
			if e == nil && client.vaultService.Rules.Account != nil {
				var accountVault database.AccountVaultState
				accountVault, e = client.gameStore.LoadAccountVault(ctx, role.AccountID, role.ID)
				if e == nil {
					accountVaultPayload, e = inventory.AccountVaultPayload(accountVault, *client.vaultService.Rules.Account)
				}
			}
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "vault_entry_rejected", "error": e.Error()})
				return dispatchHandled
			}
		}
		var areaPayload []byte
		if client.config.TownEntryProbe != "" || contractPurchaseCrashFixEnabled() {
			premiumCtx, premiumCancel := context.WithTimeout(context.Background(), 5*time.Second)
			premiums, pe := client.gameStore.ActivePremiums(premiumCtx, client.developmentAccount, time.Now())
			premiumCancel()
			if pe != nil {
				client.event(map[string]any{"kind": "premium_restore_error", "error": pe.Error()})
				return dispatchHandled
			}
			for _, premium := range premiums {
				profile.Premiums = append(profile.Premiums, protocol.PremiumEntry{Type: premium.Type, RemainingSecond: premium.RemainingSecond})
			}
			client.event(map[string]any{"kind": "premiums_restored", "account": client.developmentAccount, "count": len(profile.Premiums)})
		}
		if client.config.TownEntryProbe != "" {
			var state character.State
			if e = json.Unmarshal(role.State, &state); e != nil || !client.townCatalog.Allows(state.Level, client.townPolicy.X, client.townPolicy.Y) {
				client.event(map[string]any{"kind": "town_entry_rejected", "error": "character level or spawn policy incompatible with source area"})
				return dispatchHandled
			}
			areaPayload, e = protocol.AreaUsers(client.townCatalog.TownID, client.townCatalog.AreaID, []protocol.AreaUser{{ActorServerID: role.WireID, X: client.townPolicy.X, Y: client.townPolicy.Y, Flags: client.townPolicy.Flags}})
			if e != nil {
				client.event(map[string]any{"kind": "town_entry_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		if client.config.EntryBasicProbe {
			basic, e = client.characters.EntryBasicProbe(role, [2]byte{})
			if e != nil {
				client.event(map[string]any{"kind": "entry_basic_error", "error": e.Error()})
				return dispatchHandled
			}
			if len(basic) >= 13 {
				client.event(map[string]any{"kind": "character_mode_projection", "character_id": role.ID, "name": role.Name, "odyssey_pilot": client.characters.Rules.OdysseyPilot, "entry_mode_byte": basic[len(basic)-13]})
			}
		}
		if client.config.EntryAdditionProbe {
			addition, e = client.characters.EntryAddition(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_addition_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		if client.worldState != nil {
			// 特殊频道有**自己的城镇**（源 clientchannelinfo 的 [seriaRoomTown]）：
			// 征讨/军团这类频道里角色只能在门口的专属城镇活动，落点取该城镇地图里
			// 第一个可行走矩形的中心；普通频道保持原有城镇与落点。
			spawn := database.WorldPosition{Town: client.townCatalog.TownID, Area: client.townCatalog.AreaID, X: client.townPolicy.X, Y: client.townPolicy.Y}
			if t, ok := client.gatewayRuntime.channelTowns[client.channelTypes[client.channel]]; ok {
				x, y := t.Spawn()
				spawn = database.WorldPosition{Town: t.TownID, Area: t.AreaID, X: x, Y: y}
				client.event(map[string]any{"kind": "channel_town_spawn", "channel": client.channel, "channel_type": client.channelTypes[client.channel], "town": t.TownID, "area": t.AreaID, "x": x, "y": y, "spawn_rects": len(t.Walkable)})
			}
			e = client.worldState.enter(role, spawn)
			if e != nil {
				client.event(map[string]any{"kind": "world_entry_error", "error": e.Error()})
				return dispatchHandled
			}
			// Publish this actor before the area list is serialized, so the list already
			// carries the other players standing in the same place.
			if client.hub != nil && len(basic) > 0 {
				client.worldState.peer = &lanPeer{roleID: role.ID, actorID: role.WireID, channel: client.channel, info: basic, addition: addition, send: client.output.send, mailChanged: client.mailChanges}
				client.hub.add(client.worldState.peer)
				client.worldState.enterArea()
			}
			areaPayload, e = client.worldState.areaPayload()
			if e != nil {
				client.event(map[string]any{"kind": "world_entry_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		payload, e := protocol.SelectProbeSuccess(profile)
		if e != nil {
			client.event(map[string]any{"kind": "entry_prepare_error", "error": e.Error()})
			return dispatchHandled
		}
		var userArea []byte
		if client.worldState != nil && len(areaPayload) > 0 {
			userArea, e = client.worldState.userAreaPayload()
			if e != nil {
				client.event(map[string]any{"kind": "entry_prepare_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		accountOptions := append([]byte(nil), client.accountOptionsPayload...)
		if client.characters != nil {
			optCtx, optCancel := context.WithTimeout(context.Background(), 5*time.Second)
			overrides, optErr := client.gameStore.AccountUnifiedOptions(optCtx, client.developmentAccount)
			optCancel()
			if optErr != nil {
				client.event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
			} else if len(overrides) > 0 {
				if accountOptions, optErr = protocol.AccountOptions(overrides); optErr != nil {
					client.event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
					accountOptions = append([]byte(nil), client.accountOptionsPayload...)
				}
			}
			hkCtx, hkCancel := context.WithTimeout(context.Background(), 5*time.Second)
			accHkA, errA := client.gameStore.AccountHotkeys(hkCtx, client.developmentAccount, protocol.UnifiedOptionHotkeys)
			accHkB, errB := client.gameStore.AccountHotkeys(hkCtx, client.developmentAccount, protocol.UnifiedOptionHotkeysExt)
			hkCancel()
			if errA != nil || errB != nil {
				client.event(map[string]any{"kind": "account_hotkeys_restore_error", "error_a": fmt.Sprint(errA), "error_b": fmt.Sprint(errB)})
			} else if len(accHkA) > 0 || len(accHkB) > 0 {
				if accountOptions == nil {
					var tmplErr error
					accountOptions, tmplErr = protocol.AccountOptions(nil)
					if tmplErr != nil {
						client.event(map[string]any{"kind": "account_options_template_error", "error": tmplErr.Error()})
					}
				}
				if accountOptions != nil {
					if fe := protocol.FillAccountHotkeys(accountOptions, accHkA, accHkB); fe != nil {
						client.event(map[string]any{"kind": "account_hotkeys_restore_error", "error": fe.Error()})
					} else {
						client.event(map[string]any{"kind": "account_hotkeys_restored", "count_a": len(accHkA), "count_b": len(accHkB)})
					}
				}
			}
		}
		if client.characters != nil {
			favCtx, favCancel := context.WithTimeout(context.Background(), 5*time.Second)
			favorites, favErr := client.gameStore.AccountWarpFavorites(favCtx, client.developmentAccount)
			favCancel()
			if favErr != nil {
				client.event(map[string]any{"kind": "warp_favorites_restore_error", "account_id": client.developmentAccount, "error": favErr.Error()})
				return dispatchHandled
			}
			if len(favorites) > 0 {
				if len(accountOptions) == 0 {
					accountOptions, favErr = protocol.AccountOptions(nil)
				}
				if favErr == nil {
					favErr = protocol.FillAccountWarpFavorites(accountOptions, favorites)
				}
				if favErr != nil {
					client.event(map[string]any{"kind": "warp_favorites_restore_error", "account_id": client.developmentAccount, "error": favErr.Error()})
					return dispatchHandled
				}
				client.event(map[string]any{"kind": "warp_favorites_restore_prepared", "account_id": client.developmentAccount, "stored_slots": len(favorites), "notification": 2826})
			}
		}
		plan := entryPayloads{Select: payload, Basic: basic, Addition: addition, Vault: vaultPayload, UserArea: userArea, Area: areaPayload, Fatigue: fatiguePayload, AccountOptions: accountOptions}
		// Starter Boost 662 进城恢复：可领礼物集合（2265）与训练进度（2638）都按
		// 本角色状态现算。读失败只丢这一帧并记事件——进城不该被活动状态卡住。
		if client.worldState != nil && client.worldState.boostup != nil {
			boost := client.worldState.boostup
			if gifts, boostErr := boostGiftAvailability(boost, role); boostErr != nil {
				client.event(map[string]any{"kind": "boost_gift_restore_error", "character_id": role.ID, "error": boostErr.Error()})
			} else {
				plan.BoostGifts = gifts
			}
			if status, boostErr := boostTrainingRestore(boost, role); boostErr != nil {
				client.event(map[string]any{"kind": "boost_training_restore_error", "character_id": role.ID, "error": boostErr.Error()})
			} else {
				plan.BoostTraining = status
			}
		}
		if client.characters != nil {
			oathCtx, oathCancel := context.WithTimeout(context.Background(), 5*time.Second)
			selection, oathErr := client.gameStore.EquippedOathSelection(oathCtx, client.developmentAccount, role.ID)
			oathCancel()
			if oathErr != nil {
				client.event(map[string]any{"kind": "oath_selection_restore_error", "character_id": role.ID, "reason": oathErr.Error()})
				return dispatchHandled
			}
			plan.OathSystemInfo, oathErr = protocol.OathSystemInfo(selection.Level, selection.Option)
			if oathErr != nil {
				client.event(map[string]any{"kind": "oath_selection_restore_error", "character_id": role.ID, "reason": oathErr.Error()})
				return dispatchHandled
			}
			// NOTI2634：服务端算出的「每角色一对 Set/Oath Point」。客户端不为誓约/晶体
			// 算总分，它只把服务端给的值写进角色实体 ⇒ 不发就恒 0。
			// packets() 会把这一帧排在**所有帧之后**（要在 actor 重建完实体之后写）。
			// ⚠️ 装备库→誓约 页签的「已添加的 誓约积分」/`?/750次` **不由这一对值驱动**：
			// 实机两种字段顺序下它都仍为 0，该页统计的是"登记进装备库的誓约装备"
			// （客户端文案 101039328 `… registered in the Armory`）；见
			// docs/protocol/oath-set-points-20261004.md §7.6。
			plan.OathPartSetPoints = append(plan.OathPartSetPoints,
				client.worldState.oathPointPackets()...)
		}
		// 装备技能栏/冷却提醒/自定义按键：两组快照（S2C2609）。恒发，
		// 没设过的角色得到全零载荷（等于客户端默认）。
		if client.characters != nil && equipmentSkillEnabled() {
			eskCtx, eskCancel := context.WithTimeout(context.Background(), 5*time.Second)
			eskSkills, eskCommands, eskErr := client.gameStore.EquipmentSkillSnapshots(eskCtx, client.developmentAccount, role.ID)
			eskCancel()
			if eskErr != nil {
				client.event(map[string]any{"kind": "equipment_skill_restore_error", "character_id": role.ID, "reason": eskErr.Error()})
			} else if eskInfo, eskInfoErr := protocol.EquipmentSkillInfo(eskSkills, eskCommands); eskInfoErr != nil {
				client.event(map[string]any{"kind": "equipment_skill_restore_error", "character_id": role.ID, "reason": eskInfoErr.Error()})
			} else {
				plan.EquipmentSkill = eskInfo
			}
		}
		plan.SecondaryVault = secondaryVaultPayload
		plan.CloneSources, e = cloneAvatarSourcePackets(role.State)
		if e != nil {
			client.event(map[string]any{"kind": "clone_avatar_source_restore_error", "error": e.Error()})
			return dispatchHandled
		}
		collectionCtx, collectionCancel := context.WithTimeout(context.Background(), 5*time.Second)
		collectionEquipment, collectionErr := client.gameStore.AdventureCollectionEquipment(collectionCtx, role.AccountID, role.ID)
		collectionCancel()
		if collectionErr != nil {
			client.event(map[string]any{"kind": "冒险图鉴登录读取失败", "character_id": role.ID, "error": collectionErr.Error()})
			return dispatchHandled
		}
		plan.AdventureCollection = protocol.AdventureCollectionGuide(collectionEquipment)
		// 装备图鉴从已提交的角色状态恢复，不覆盖冒险团及快捷键快照。
		if body, jErr := equipmentJournalEntryPayload(role, client.journalRules); jErr != nil {
			client.event(map[string]any{"kind": "equipment_journal_restore_error", "character_id": role.ID, "error": jErr.Error()})
		} else if len(body) > 0 {
			plan.Journal = body
		}
		plan.AccountVault = accountVaultPayload
		if client.characters != nil {
			gpCtx, gpCancel := context.WithTimeout(context.Background(), 5*time.Second)
			gpPayload, gpErr := client.gameStore.ResolveGamepadPayload(gpCtx, client.developmentAccount, role.ID)
			gpCancel()
			if gpErr != nil {
				client.event(map[string]any{"kind": "gamepad_options_restore_error", "character_id": role.ID, "error": gpErr.Error()})
			} else if len(gpPayload) > 0 {
				plan.GamepadOptions = gpPayload
			}
		}
		// Restore the persisted category-0 skin state; without owned +
		// selection frames the inventory CharBG keeps its default NEW
		// animation. A failed restore aborts this entry rather than
		// sending a fabricated success state.
		if client.characters != nil {
			skinCtx, skinCancel := context.WithTimeout(context.Background(), 5*time.Second)
			skinState, skinErr := client.gameStore.RestoreProfileSkins(skinCtx, client.developmentAccount, role.ID)
			skinCancel()
			if skinErr == nil {
				plan.ProfileSkinCargo, plan.ProfileSkinSelection, skinErr = character.RestoreProfileSkin(skinState)
			}
			if skinErr != nil {
				client.event(map[string]any{"kind": "profile_skin_restore_error", "character_id": role.ID, "error": skinErr.Error()})
				return dispatchHandled
			}
		}
		// Feed the account's damage-font cargo so the panel grid has
		// something to enumerate. A read failure only drops this frame:
		// entry must not depend on the skin storage the way the profile
		// decoration state does.
		if client.characters != nil && client.skinCatalog != nil {
			cargoCtx, cargoCancel := context.WithTimeout(context.Background(), 5*time.Second)
			cargo, cargoErr := damageFontCargo(cargoCtx, client.gameStore, client.developmentAccount, client.skinCatalog)
			if cargoErr == nil {
				plan.SkinCargoDamageFont = cargo
				// The chosen fonts are per character and per panel tab, and
				// only these frames put them back on the damage numbers.
				plan.SkinSelectionDamageFontNormal, cargoErr = restoreDamageFontSelection(cargoCtx,
					client.gameStore, role.ID, client.developmentAccount, client.skinCatalog,
					protocol.SkinSelectionDamageFontNormal)
				if cargoErr == nil {
					plan.SkinSelectionDamageFontCumulative, cargoErr = restoreDamageFontSelection(cargoCtx,
						client.gameStore, role.ID, client.developmentAccount, client.skinCatalog,
						protocol.SkinSelectionDamageFontCumulative)
				}
			}
			cargoCancel()
			if cargoErr != nil {
				client.event(map[string]any{"kind": "skin_cargo_damage_font_restore_error", "character_id": role.ID, "reason": cargoErr.Error()})
			}
		}
		// The 边框 and 觉醒插图 pages are filled for the same entry group. Their
		// packets go after the profile pair because a NOTI1545 frame rebuilds the
		// page it names and the last frame for page 0 is the state the client keeps;
		// these payloads carry that feature's built-in rows too, and a selection
		// frame is only sent when this character actually stores one.
		if client.characters != nil && client.skinCatalog != nil {
			familyCtx, familyCancel := context.WithTimeout(context.Background(), 5*time.Second)
			plan.restoreSkinFamilies(familyCtx, client.gameStore, client.developmentAccount, role.ID,
				client.skinCatalog, client.event)
			familyCancel()
		}
		// 幻化仓库（武器外观页签）容器只在复制时被推过一次，客户端重登即空；
		// 这里按存档重推 NOTI1545。读不出状态只记事件照常进场，仓库空一次
		// 比卡在角色选择界面好。
		//
		// 列表按本职业戴不戴得上过一遍：客户端那一页不做职业判断（见
		// weaponSkinPageIDs），修好复制校验之前存下的条目就一直摆在那里
		// （实机 2026-09-28）。nil 谓词 = 没有装备目录 = 原样发。
		weaponSkinUsable := client.characters.WeaponSkinUsableFor(role)
		plan.SkinCargo, e = skinCargoRestore(role.State, weaponSkinUsable)
		if e != nil {
			client.event(map[string]any{"kind": "skin_cargo_restore_error", "error": e.Error()})
			plan.SkinCargo = nil
			e = nil
		}
		// 幻化仓库里正在佩戴那一行的高亮：客户端只在 Apply 时写窗口本地格，
		// 重开窗口就丢。入场补一次 NOTI1546，重登后第一次打开就能看到边框。
		plan.SkinSelection, e = skinSelectionRestore(role.State)
		if e != nil {
			client.event(map[string]any{"kind": "skin_selection_restore_error", "error": e.Error()})
			plan.SkinSelection = nil
			e = nil
		}
		// 「最近获得」那五行格只由 NOTI1547 喂，而这帧是整表重建，所以入场必须
		// 把账号注册过的皮肤连同本角色复制出的武器外观一次发全。读失败只记事件
		// 不发帧：仓库少一栏条不能把进城卡住。
		if client.characters != nil && client.skinCatalog != nil {
			recentCtx, recentCancel := context.WithTimeout(context.Background(), 5*time.Second)
			recent, re := skinRecentRestore(recentCtx, client.gameStore, client.developmentAccount,
				role.State, client.skinCatalog, weaponSkinUsable)
			recentCancel()
			if re != nil {
				client.event(map[string]any{"kind": "skin_recent_restore_error",
					"character_id": role.ID, "reason": re.Error()})
			} else {
				plan.SkinRecent = recent
			}
		}
		plan.CubeContract, e = cubeContractRestore(role.State)
		if e != nil {
			client.event(map[string]any{"kind": "cube_contract_restore_error", "character_id": role.ID, "reason": e.Error()})
			return dispatchHandled
		}
		// Read-notice ledger for NOTI402 (tree 1) and NOTI426 (tree 2).
		// Without it the client re-pops the third-awakening teaching
		// frame on every login; a read failure leaves the frames unset,
		// matching the pre-fix behavior.
		if client.characters != nil {
			noticeCtx, noticeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			noticeTree1, t1Err := client.gameStore.CharacterNoticeSeen(noticeCtx, client.developmentAccount, role.ID, 1)
			noticeTree2, t2Err := client.gameStore.CharacterNoticeSeen(noticeCtx, client.developmentAccount, role.ID, 2)
			noticeCancel()
			if t1Err == nil {
				plan.InformNotice = protocol.InformNoticeSeen(noticeTree1)
			} else {
				client.event(map[string]any{"kind": "notice_seen_restore_error", "character_id": role.ID, "tree": 1, "reason": t1Err.Error()})
			}
			if t2Err == nil {
				plan.InformNotice2nd = protocol.InformNoticeSeen(noticeTree2)
			} else {
				client.event(map[string]any{"kind": "notice_seen_restore_error", "character_id": role.ID, "tree": 2, "reason": t2Err.Error()})
			}
		}
		// Introduce the players already standing here before the area list that
		// places them: the client only places actors it already knows.
		if client.worldState != nil {
			for _, o := range client.worldState.joinedPeers {
				plan.Peers = append(plan.Peers, client.worldState.hub.basicInfo(o))
				if len(o.addition) > 0 {
					plan.Peers = append(plan.Peers, o.addition)
				}
			}
		}
		plan.CinematicSkips, e = cinematicRestore(role.State)
		if e == nil {
			// Same success guard as cinematicRestore: an unparseable
			// state must drop both frames together, never emit the
			// digest without the skip bitmap.
			plan.StoryDigest, e = storyDigestRestore(role.State)
		}
		if e == nil {
			plan.SynopsisRead, e = synopsisRestore(role.State)
		}
		if e == nil && client.characters != nil {
			plan.SkillVariations, e = client.characters.VariationRestore(role)
		}
		if e != nil {
			client.event(map[string]any{"kind": "cinematic_restore_error", "error": e.Error()})
			return dispatchHandled
		}
		// Locked skills ride in the character option block (NOTI2827),
		// which entryPayloads.packets() puts last: this client crashes
		// about 0.3~1s after town entry when 2827 arrives early.
		lockCtx, lockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		locks, lockErr := client.gameStore.SkillLocks(lockCtx, role.ID)
		lockCancel()
		if lockErr != nil {
			client.event(map[string]any{"kind": "entry_skill_lock_error", "error": lockErr.Error()})
			return dispatchHandled
		}
		plan.SkillLocks, e = unifiedCharacPayload(client.unifiedCharacTemplate, locks, client.config.SkillLockOffset)
		if e != nil {
			client.event(map[string]any{"kind": "entry_skill_lock_error", "error": e.Error()})
			return dispatchHandled
		}
		// Restore per-character system settings (CMD2377 subtype 0x05)
		// onto the fresh NOTI2827 block so toggles survive relog/char switch.
		{
			restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 3*time.Second)
			settings, sErr := client.gameStore.CharacterUnifiedOptions(restoreCtx, role.ID)
			restoreCancel()
			if sErr != nil {
				client.event(map[string]any{"kind": "charac_settings_restore_error", "error": sErr.Error()})
			} else if len(settings) > 0 {
				if fe := protocol.FillCharacSettings(plan.SkillLocks, settings); fe != nil {
					client.event(map[string]any{"kind": "charac_settings_restore_error", "error": fe.Error()})
				} else {
					client.event(map[string]any{"kind": "charac_settings_restored", "character_id": role.ID, "count": len(settings)})
				}
			}
		}
		// Restore the six character effect settings carried by CMD2377
		// subtype 0x12 into their own object in NOTI2827.
		{
			effectCtx, effectCancel := context.WithTimeout(context.Background(), 3*time.Second)
			effects, effectErr := client.gameStore.CharacterUnifiedOptionGroup(effectCtx, role.ID, protocol.UnifiedOptionCharacterEffects)
			effectCancel()
			if effectErr != nil {
				client.event(map[string]any{"kind": "charac_effect_options_restore_error", "character_id": role.ID, "error": effectErr.Error()})
			} else if len(effects) > 0 {
				if fe := protocol.FillCharacEffects(plan.SkillLocks, effects); fe != nil {
					client.event(map[string]any{"kind": "charac_effect_options_restore_error", "character_id": role.ID, "error": fe.Error()})
				} else {
					client.event(map[string]any{"kind": "charac_effect_options_restored", "character_id": role.ID, "count": len(effects), "options": effects})
				}
			}
		}
		// Restore per-character hotkeys (CMD2377 subtype 0x03 / 0x04)
		// onto the fresh NOTI2827 block.
		if client.characters != nil && len(plan.SkillLocks) == protocol.UnifiedCharacOptionSize {
			chkCtx, chkCancel := context.WithTimeout(context.Background(), 3*time.Second)
			charHkA, errA := client.gameStore.CharacterHotkeys(chkCtx, role.ID, protocol.UnifiedOptionHotkeys)
			charHkB, errB := client.gameStore.CharacterHotkeys(chkCtx, role.ID, protocol.UnifiedOptionHotkeysExt)
			chkCancel()
			if errA != nil || errB != nil {
				client.event(map[string]any{"kind": "charac_hotkeys_restore_error", "character_id": role.ID, "error_a": fmt.Sprint(errA), "error_b": fmt.Sprint(errB)})
			} else if len(charHkA) > 0 || len(charHkB) > 0 {
				if fe := protocol.FillCharacHotkeys(plan.SkillLocks, charHkA, charHkB); fe != nil {
					client.event(map[string]any{"kind": "charac_hotkeys_restore_error", "character_id": role.ID, "error": fe.Error()})
				} else {
					client.event(map[string]any{"kind": "charac_hotkeys_restored", "character_id": role.ID, "count_a": len(charHkA), "count_b": len(charHkB)})
				}
			}
		}
		client.event(map[string]any{"kind": "entry_skill_lock_prepared", "character_id": role.ID, "count": len(locks), "bytes": len(plan.SkillLocks)})
		if client.lootService != nil {
			// Relocate old stackables before the list-0 inventory snapshot.
			// A failed relocation rolls back and does not prevent entry.
			sweepCtx, sweepCancel := context.WithTimeout(context.Background(), 5*time.Second)
			var applied bool
			var sweepErr error
			var sweptRole database.Character
			sweptRole, applied, sweepErr = client.gameStore.CommitCharacterEvent(sweepCtx, role.AccountID, role.ID, role.ConfigVersion,
				"stack-slot-resweep", "stack-slot-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					bag, err := inventory.ReadBag(current.State)
					if err != nil {
						return nil, nil, err
					}
					fixed, moved, err := inventory.SweepStackSlots(bag, client.lootService.Catalog, client.lootService.BagRules)
					if err != nil {
						return nil, nil, err
					}
					state, err := inventory.SaveBag(current.State, fixed)
					if err != nil {
						return nil, nil, err
					}
					outcome, err := json.Marshal(map[string]bool{"moved": moved})
					return state, outcome, err
				})
			sweepCancel()
			if sweepErr != nil {
				client.event(map[string]any{"kind": "entry_stack_slot_error", "character_id": role.ID, "error": sweepErr.Error()})
			} else {
				role = sweptRole
				if applied {
					client.event(map[string]any{"kind": "entry_stack_slot_swept", "character_id": role.ID})
				}
			}
			petCtx, petCancel := context.WithTimeout(context.Background(), 5*time.Second)
			petRole, _, petErr := client.gameStore.CommitCharacterEvent(petCtx, role.AccountID, role.ID, role.ConfigVersion,
				"pet-container-resweep", "pet-container-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					bag, err := inventory.ReadBag(current.State)
					if err != nil {
						return nil, nil, err
					}
					fixed, _, err := inventory.SweepPetConsumables(bag, client.lootService.Catalog, client.lootService.BagRules)
					if err != nil {
						return nil, nil, err
					}
					state, err := inventory.SaveBag(current.State, fixed)
					return state, json.RawMessage(`{}`), err
				})
			petCancel()
			if petErr != nil {
				client.event(map[string]any{"kind": "entry_pet_container_error", "character_id": role.ID, "error": petErr.Error()})
			} else {
				role = petRole
			}
		}
		if client.wearService != nil && client.wearService.Catalog != nil {
			gearCtx, gearCancel := context.WithTimeout(context.Background(), 5*time.Second)
			gearRole, applied, gearErr := client.gameStore.CommitCharacterEvent(gearCtx, role.AccountID, role.ID, role.ConfigVersion,
				"pet-gear-resweep", "pet-gear-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					bag, err := inventory.ReadBag(current.State)
					if err != nil {
						return nil, nil, err
					}
					fixed, _, err := inventory.SweepPetGear(bag, client.wearService.Catalog)
					if err != nil {
						return nil, nil, err
					}
					state, err := inventory.SaveBag(current.State, fixed)
					return state, json.RawMessage(`{}`), err
				})
			gearCancel()
			if gearErr != nil {
				client.event(map[string]any{"kind": "entry_pet_gear_error", "character_id": role.ID, "error": gearErr.Error()})
			} else {
				role = gearRole
				if applied {
					client.event(map[string]any{"kind": "entry_pet_gear_swept", "character_id": role.ID})
				}
			}
		}
		loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		loyaltyRole, _, loyaltyErr := client.gameStore.CommitCharacterEvent(loyaltyCtx, role.AccountID, role.ID, role.ConfigVersion,
			fmt.Sprintf("creature-loyalty-login:%d", time.Now().UnixNano()), "creature-loyalty-login-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				state, err := inventory.BeginCreatureLoyaltySession(current.State, time.Now().Unix())
				return state, json.RawMessage(`{}`), err
			})
		loyaltyCancel()
		if loyaltyErr != nil {
			client.event(map[string]any{"kind": "entry_creature_loyalty_error", "character_id": role.ID, "error": loyaltyErr.Error()})
		} else {
			role = loyaltyRole
		}
		if client.wearService != nil {
			plan.KnightDeck, e = client.wearService.KnightDeckPayload(workflow.InventoryRole(role))
			if e != nil {
				client.event(map[string]any{"kind": "entry_knight_deck_error", "character_id": role.ID, "error": e.Error()})
				plan.KnightDeck = nil
			}
			plan.Worn, e = inventory.WornPayload(role.State)
			if e == nil {
				plan.WornUpdate, e = inventory.WornSpaceUpdate(role.State)
			}
			if e == nil {
				// Full worn set through the id-14 slot-update channel,
				// mirroring the equipment-move heal frames (see the
				// third-pass note in entry_flow.go packets()).
				var bag inventory.Bag
				bag, e = inventory.ReadBag(role.State)
				if e == nil && len(bag.Worn) > 0 {
					// Coexisting clone/look avatars share a body slot; the
					// id-14 slot channel carries one row per slot.
					plan.WornSlots, e = inventory.EquipmentPayload(3, bag.WornBaseItems(), false)
				}
			}
			if e == nil {
				plan.WeaponEquipped = inventory.HasWornWeapon(role.State)
				if plan.WeaponEquipped {
					plan.WeaponAppearance, e = client.characters.AppearanceProbe(role, [2]byte{})
				}
			}
			if e == nil {
				plan.AvatarReady, e = inventory.SpecialEquipmentRestorePayload(role.State, 1)
			}
			if e == nil {
				plan.Avatars, e = inventory.EquipmentPayload(1, nil, true)
			}
			if e == nil {
				plan.Creatures, e = inventory.PetContainerRestorePayload(role.State)
			}
			if e == nil {
				plan.CreatureList, _ = inventory.CreatureListPayload(role.State)
				plan.CreatureGrowth, _ = inventory.CreatureGrowthPayload(role.State)
			}
			if e != nil {
				client.event(map[string]any{"kind": "entry_worn_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		if client.lootService != nil {
			rewardCtx, rewardCancel := context.WithTimeout(context.Background(), 5*time.Second)
			recovered, rewardErr := (&workflow.LootService{Store: client.gameStore, Loot: client.lootService}).RecoverBlackPurgatoryCards(rewardCtx, role)
			rewardCancel()
			role = recovered
			if rewardErr != nil {
				client.event(map[string]any{"kind": "黑鸦未领翻牌或领主奖励保留", "character_id": role.ID, "error": rewardErr.Error()})
			}
			// Sweep the seventeen account-shared materials out of the bag
			// into the account storage before the snapshots are built, then
			// deliver the list35 storage snapshot ahead of list0 so the
			// client harvest (sub_145ADC2A0) adopts the fixed slots.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var materials inventory.AccountMaterials
			role, materials, e = sweepAccountMaterials(ctx, client.gameStore, role)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "entry_account_materials_error", "error": e.Error()})
				materials = inventory.NewAccountMaterials()
				fallbackCtx, fallbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if raw, readErr := client.gameStore.AccountMaterials(fallbackCtx, role.AccountID); readErr == nil {
					if savedMaterials, parseErr := inventory.ReadAccountMaterials(raw); parseErr == nil {
						materials = savedMaterials
					}
				}
				fallbackCancel()
				e = nil
			}
			plan.AccountMaterials, e = accountMaterialSnapshot(materials)
			if e == nil {
				plan.RadiantSouls, e = radiantSoulSnapshot(materials)
			}
			if e == nil {
				plan.Inventory, e = client.itemService.Bootstrap(workflow.InventoryRole(role))
			}
			if e != nil {
				client.event(map[string]any{"kind": "entry_inventory_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		if client.progressionService != nil {
			plan.OdysseyProgress, e = client.progressionService.OdysseyProgressPayload(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_odyssey_progress_error", "error": e.Error()})
				return dispatchHandled
			}
			plan.Experience, e = character.ExperiencePayload(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_experience_error", "error": e.Error()})
				return dispatchHandled
			}
			if client.questService != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				ids, qe := client.questService.Completed(ctx, role)
				if qe == nil {
					plan.CompletedQuests, qe = protocol.CompletedQuests(ids)
				}
				if qe == nil {
					available, ae := client.questService.Available(ctx, role)
					if ae != nil {
						qe = ae
					} else {
						plan.AvailableQuests, qe = protocol.AvailableQuests(plan.Experience[0], available)
					}
				}
				cancel()
				if qe != nil {
					client.event(map[string]any{"kind": "entry_completed_quests_error", "error": qe.Error()})
					return dispatchHandled
				}
			}
		}
		if len(addition) > 0 {
			plan.Skills, e = client.characters.EntrySkills(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_skills_error", "error": e.Error()})
				return dispatchHandled
			}
			plan.ComboSkillInfo, e = client.characters.ComboSkillInfoNotify(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_combo_skill_info_error", "error": e.Error()})
				return dispatchHandled
			}
			plan.SkillPreset, e = client.characters.SkillPresetInfo(role)
			if e != nil {
				client.event(map[string]any{"kind": "entry_skill_preset_error", "error": e.Error()})
				return dispatchHandled
			}
		}
		if len(addition) > 0 && len(areaPayload) > 0 {
			plan.Complete = protocol.EnterGameworldComplete()
		}
		if client.config.BoosterGageHide {
			// NOTI398 displayValue=0 collapses the top-left Liberation Trace
			// panel; preparePackets skips empty payloads, so the flag-off path
			// equals the pre-fix behavior.
			plan.BoosterGage = protocol.BoosterGage(0)
		}
		plan.BuffEnhancement, e = client.characters.BuffEnhancementRestore(role)
		if e != nil {
			client.event(map[string]any{"kind": "entry_buff_enhancement_error", "character_id": role.ID, "error": e.Error()})
			// A damaged optional registration must not prevent entry or
			// rewrite the player's saved data. Clear only the client cache.
			plan.BuffEnhancement, _ = protocol.BuffEnhancementAllData(0, nil)
		}
		prepared, e := preparePackets(client.keys, plan.packets())
		if e != nil {
			client.event(map[string]any{"kind": "entry_encode_error", "character_id": role.ID, "error": e.Error(), "frames_sent": 0})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "entry_preflight_passed", "character_id": role.ID, "frame_count": len(prepared)})
		e = client.output.writePrepared(prepared, func(p preparedPacket) {
			entry := map[string]any{"kind": p.Name, "character_id": role.ID, "actor_server_id": role.WireID, "type": p.Kind, "id": p.ID, "plain_bytes": len(p.Payload), "client_acceptance": "pending"}
			if p.ID == 4 {
				entry["name"] = role.Name
			}
			if p.ID == 23 || p.ID == 24 {
				if client.worldState != nil {
					entry["position"] = client.worldState.state.Position
				} else {
					entry["town_id"], entry["area_id"] = client.townCatalog.TownID, client.townCatalog.AreaID
				}
			}
			if p.ID == 13 || p.ID == 36 || p.ID == 2425 || (p.Kind == 0 && p.ID == 433) {
				entry["plain_hex"] = hex.EncodeToString(p.Payload)
			}
			client.event(entry)
		})
		if e != nil {
			client.event(map[string]any{"kind": "entry_write_error", "character_id": role.ID, "error": e.Error()})
			return dispatchClose
		}
		client.comboState.lastNotify = nil
		client.selectedCharacterID = role.ID
		if client.worldState != nil {
			client.worldState.fameInitialized = false
			if client.sendPlan(client.worldState.appendFameUpdate(nil, client.event), nil) != nil {
				return dispatchClose
			}
		}
		client.selectedBasic, client.selectedAddition = basic, addition
		client.mailAlarmRole, client.mailDeliveryID = 0, 0
		select {
		case client.mailChanges <- struct{}{}:
		default:
		}
		if client.worldState != nil {
			if e = client.worldState.announceSelf(client.event); e != nil {
				client.event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
			}
		}
		// next79 挂接（ispins_wiring.go）：N1719 保活 goroutine 与登录期
		// N2254 分支（伊斯频道挂 pending、普通频道 1.1s 后推 N2254+781+782）。
		client.ispinsPostSelection(role.ID)
		// 进城镇好感度全量同步：NOTI733(NPC_FAVOR_POINT_INFO) 是客户端
		// 唯一的无弹窗全量装载入口（handler 0x1452db190：先清空 favor
		// map 再逐条装入并刷新，不派发任何 UI 事件）；806 ack 虽也写
		// 缓存但必弹好感度窗。NOTI124 刚完成时好感度子系统尚未就绪，
		// 早发会被丢弃，沿用 900ms 延迟（2026-09-29 定案时序）。
		// **伊斯频道（Type 81）待机区分支不发**：官服待机区抓包全程
		// 无 N733（待机区无 NPC，好感度子系统不适用），且该推送同样
		// 会撞进场景装载期（2026-10-03 闪退会话 seq 66 实证）。
		if client.worldState == nil || client.worldState.channelType != 81 {
			go func(characterID int64) {
				time.Sleep(900 * time.Millisecond)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				points, listErr := client.gameStore.ListFavor(ctx, characterID)
				cancel()
				if listErr != nil {
					client.event(map[string]any{"kind": "npc_favor_point_info_skipped", "character_id": characterID, "error": listErr.Error()})
					return
				}
				if len(points) == 0 {
					return
				}
				records := make([]protocol.FavorPointInfoRecord, 0, len(points))
				for _, fp := range points {
					records = append(records, protocol.FavorPointInfoRecord{NPCID: fp.NPCID, Point: uint32(fp.Point)})
				}
				payload := protocol.FavorPointInfo(records)
				if err := client.output.send(0, 733, payload); err != nil {
					client.event(map[string]any{"kind": "npc_favor_point_info_failed", "character_id": characterID, "error": err.Error()})
					return
				}
				client.event(map[string]any{"kind": "npc_favor_point_info_sent", "character_id": characterID, "npc_count": len(records), "plain_bytes": len(payload)})
			}(role.ID)
		}
		// 蔚蓝号（Azure Main，channelType 102）进城时补一次奖励计数快照。
		// 原因见 azure_main_flow.go 的 azureRewardSnapshot：缺它客户端会在
		// 「创建攻坚队」时报「奖励已领完」。只对 102 生效。
		if client.worldState != nil && client.worldState.channelType == azureMainChannelType {
			azureRewards, azureErr := client.worldState.azureRewardSnapshot()
			if azureErr != nil {
				client.event(map[string]any{"kind": "azure_main_rewards_error", "error": azureErr.Error()})
			} else {
				for _, azurePacket := range azureRewards {
					if sendErr := client.output.send(azurePacket.Kind, azurePacket.ID, azurePacket.Payload); sendErr != nil {
						client.event(map[string]any{"kind": "azure_main_rewards_send_error", "id": azurePacket.ID, "error": sendErr.Error()})
						break
					}
					client.event(map[string]any{"kind": azurePacket.Name, "id": azurePacket.ID})
				}
			}
			// 再补一发**延迟重发**：本仓已有两处先例说明这个客户端在进场早期会丢包
			// （N733 好感度用 900ms；N2254 改成等场景就绪再发，否则撞进装载期）。
			// 计数快照是纯计数器，重发无副作用。
			go func() {
				time.Sleep(900 * time.Millisecond)
				if snap, snapErr := client.worldState.azureRewardSnapshot(); snapErr == nil {
					for _, azurePacket := range snap {
						if sendErr := client.output.send(azurePacket.Kind, azurePacket.ID, azurePacket.Payload); sendErr != nil {
							return
						}
						client.event(map[string]any{"kind": azurePacket.Name + "_delayed", "id": azurePacket.ID})
					}
				}
			}()
		}
		return dispatchHandled
	}

	return dispatchNext
}
