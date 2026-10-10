# S1 input: extracting domain `Store` interfaces (PostgreSQL + SQLite dual-engine)

> 状态：**S1 取证输入（2026-10-04）**——只读取证产物，**不是已完成工作的记录**，实施尚未开始。
> 上游方案见 [sqlite-dual-engine-design.md](sqlite-dual-engine-design.md)（其 §2.4 与附录 A 引用本文）。
> 本文为英文逐项明细；方案正文为中文。取证时仓库未改动，文中数字均为当时的实测值。

Read-only reconnaissance. Repo: `server/work/dfo-lan`, module `dfolan`.
Anchor: `internal/database/store.go:32 type Store struct` with unexported fields
`db *pgxpool.Pool` (`:33`), `queries *sqlcgen.Queries` (`:34`), `adventureEnabled bool` (`:35`).

All `file:line` below are the `func` line. Method counts come from a case-sensitive
`^func \(s \*Store\) [A-Z]` scan over the 61 non-test `internal/database/*.go` files:
**202 exported methods** in 58 of those files, plus 8 unexported `*Store` methods:
`commitAccountMaterialEvent` (`account_material_event.go:43`),
`commitAdventureExperience` (`adventure.go:268`), `migrateCoinItems`
(`cash_purchase.go:35`), `migratePackagePlaceholders` (`cash_purchase.go:122`),
`purchaseCash` (`cash_purchase.go:328`), `commitCharacterEvent` (`character_event.go:65`),
`execMigration` (`migrations.go:48`), `migrateWarpFavorites` (`warp_favorites.go:14`).
(Beware: PowerShell `Select-String` is case-insensitive by default, so `[A-Z]` there also
matches those 8 and reports 210 — use ripgrep or `-cmatch`.) Paths in §1 are relative to
`internal/database/`.
"Call sites" = out-of-package matches of `\.Name\s*\(` with receiver filtering for generic
names; see the caveat at the end.

---

## 1. Method inventory by domain

| # | Proposed interface | Methods | Source file(s) | Call sites |
| --- | --- | --- | --- | --- |
| 1.1 | *(concrete bootstrap surface — no domain interface)* | `Close`, `Migrate`, `InitializeGame`, `HoldAdminGuard`, `DatabaseName` | `store.go:77,78,183`, `migrations.go:99`, `admin_guard.go:11` | ~18, 7, 1, 1, 1 |
| 1.2 | `AccountCharacterStore` | `NameExists`, `CreateCharacter`, `Characters`, `DeleteCharacter`, `ChangeCharacterSlots`, `DevelopmentAccount`, `DevelopmentCharacterAccount`, `Accounts`, `AdminCharacters`, `AdminCharacter`, `EquipmentSkillSnapshots`, `SaveEquipmentSkillSnapshot`, `ClearEquipmentSkill` | `store.go:74,89,125,171,189,201,206`, `character_delete.go:12`, `character_slot.go:90`, `equipment_skill.go:31,64,86` | 1,27,33,5,1,34,2,1,1,2,1,1,1 |
| 1.3 | `CharacterEventStore` | `CommitCharacterEvent`, `CommitCharacterEventTx`, `CommitCharacterPremiumEvent`, `CharacterEventReceipt`, `MarkCharacterNotice`, `CharacterNoticeSeen` | `character_event.go:32,45,53,22`, `character_notice_store.go:22,42` | 87,2,7,32,3,2 |
| 1.4 | `VaultStore` | `LoadVault`, `CommitVaultMove`, `CommitVaultCrossMove`, `CommitVaultTransfer`, `LoadAccountVault`, `CommitAccountVault`, `CommitAccountVaultCrossMove`, `CommitAccountVaultSort` | `vault.go:57,87,143`, `vault_transfer.go:17`, `account_vault.go:25,77,152,33` | 6,3,1,1,2,6,1,1 |
| 1.5 | `CashPurchaseStore` | `PurchaseCash`, `PurchaseCashPremium`, `PurchaseCashMixed`, `PurchaseCashToBag`, `PurchaseCashVault`, `VaultPurchaseSpace`, `CashInventory` | `cash_purchase.go:248,257,267,284,292,303,531` | 1,0,2,5,2,1,2 |
| 1.5 | `PremiumStore` | `ActivePremiums`, `HasActivePremium`, `ActivePremiumSet`, `ActivatePremium`, `HasGrowthPremium`, `HasTacticianPremium` | `premium.go:30,50,58,71,109,114` | 1,4,0,0,4,2 |
| 1.6 | `MailStore` | `MailRecipient`, `MailboxDeliveryState`, `Mailbox`, `SendMail`, `CommitSystemMail`, `MutateMailbox` | `mailbox.go:32,49,60,78,213,233` | 1,1,1,2,1,2 |
| 1.6 | `GMMailStore` | `SendGMMail`, `GMMails`, `RevokeGMMail` | `gm_mail.go:29,61,73` | 1,1,1 |
| 1.7 | `QuestStore` | `Quests`, `AcceptQuest`, `AcceptQuestGroups`, `AbandonQuest`, `MarkMeetNPCQuest`, `ClearQuests`, `ClearActQuests`, `CompletedQuestIDs`, `RepairLegacyQuest`, `CompleteQuestObjective`, `CompleteQuestUseObjective`, `RecordQuestMapClear`, `CommitQuestReward` | `quest_read.go:11,31,53`, `quest.go:21,31,98,112,128,153`, `quest_objective.go:18,32,45`, `quest_reward.go:20` | 23,5,3,5,1,1,2,2,4,7,1,2,1 |
| 1.8 | `ProgressionReadStore` | `RunMonsterExperience`, `RunFatigueLedger` | `progression_read.go:10,22` | 1,1 |
| 1.9 | `AdventureStore` | `CharacterSeason`, `AdventureLevel`, `CommitAdventure`, `LoadAdventure`, `PrepareAdventure`, `AdventureCollectionEquipment`, `AdventureEquipmentRegistered` | `adventure.go:18,37,47,131,163`, `adventure_collection.go:10,21` | 1,1,4,1,7,1,2 |
| 1.10 | `FatigueStore` | `ConsumeRoomFatigue`, `RunPaidFatigue`, `LoadFatigue`, `RecoverFatigue` | `fatigue.go:21,83,89`, `fatigue_recovery.go:15` | 12,1,5,1 |
| 1.11 | `WorldStore` | `LoadWorld`, `SaveWorld`, `ScrubPollutedWorldPositions` | `world.go:28,69,96` | 5,6,1 |
| 1.12 | `UnifiedOptionStore` | `SaveAccountUnifiedOptions`, `SaveCharacterUnifiedOptions`, `AccountUnifiedOptions`, `CharacterUnifiedOptions`, `SaveCharacterUnifiedOptionGroup`, `CharacterUnifiedOptionGroup`, `SaveAccountHotkeys`, `SaveCharacterHotkeys`, `AccountHotkeys`, `CharacterHotkeys`, `PromoteCharacterHotkeysToAccount`, `ClearAccountCharacterHotkeys`, `CopyAccountHotkeysToCharacter` | `unified_option.go:39,65,93,110,127,158,177,205,233,249,267,280,293` | 1,1,3,2,1,1,1,1,2,2,1,1,1 |
| 1.12 | `GamepadStore` | `SaveAccountGamepadKeys`, `SaveAccountGamepadOptions`, `AccountGamepadPayload`, `SaveCharacterGamepadKeys`, `SaveCharacterGamepadOptions`, `ClearCharacterGamepadSettings`, `ClearAccountCharacterGamepadSettings`, `CharacterGamepadPayload`, `ResolveGamepadPayload` | `gamepad.go:51,60,69,88,97,105,114,123,144` | 1,1,1,1,1,0,2,0,2 |
| 1.12 | `WarpFavoriteStore` | `SaveAccountWarpFavorites`, `AccountWarpFavorites` | `warp_favorites.go:18,44` | 1,1 |
| 1.12 | `ProfileSkinStore` | `RestoreProfileSkins` | `profile_skin.go:17` | 2 |
| 1.13 | `SkinStore` | `UnlockSkin`, `ListSkins`, `SelectSkin`, `SelectedSkin`, `SkinFavorites`, `SetSkinFavorite`, `SetSkinSelectionList`, `SkinSelectionList`, `SetSkinSelectionSlots`, `SkinSelectionSlots` | `skin_cargo.go:30,40`, `skin_selection.go:32,41`, `skin_favorite.go:30,65`, `skin_selection_list.go:36,61,90,117` | 1,8,3,1,3,1,1,1,1,1 |
| 1.14 | `TowerStore` | `ReadTowerProgress`, `ReserveTowerEntry`, `AdvanceTowerFloor`, `TowerGriefProgress`, `AdvanceTowerGrief` | `tower_progress.go:56,70,86`, `tower_grief.go:36,103` | 1,1,1,1,0 |
| 1.15 | `OathStore` | `EquippedOathSelection`, `SelectEquippedOathOption`, `OathOption`, `SaveOathOption`, `OathProgressClears`, `BumpOathProgress` | `oath_options.go:53,90,138,155`, `oath_progress.go:23,41` | 6,2,0,0,1,1 |
| 1.16 | `OdysseyStore` | `CommitOdysseyGraduation`, `CommitOdysseyHonorMail` | `odyssey_graduation.go:17`, `odyssey_honor.go:14` | 1,1 |
| 1.17 | `BleedingMineStore` | `UpdateBleedingMineRewards`, `BleedingMineTeams`, `SaveBleedingMineTeam` | `bleeding_mine.go:18,79,96` | 9,2,2 |
| 1.17 | `BlackPurgatoryStore` | `FreezeBlackPurgatoryReward`, `BlackPurgatoryQuota`, `ReadPendingBlackPurgatoryRewards` | `black_purgatory.go:21,77`, `black_purgatory_read.go:14` | 1,7,1 |
| 1.17 | `OmenStore` | `OmenState`, `SaveOmenHeld`, `SetOmenOrthaierPending` | `omen_state.go:46,65,73` | 1,3,1 |
| 1.17 | `MoonRewardStore` | `PendingMoonRewardRuns` | `moon_rewards_read.go:9` | 1 |
| 1.18 | `GrantStore` | `GrantHistory`, `ApplyGrant`, `AccountCera` | `grant.go:28,90,74` | 3,3,17 |
| 1.19 | `ShopPurchaseStore` | `CountShopPurchases` | `shop_purchase.go:62` | 1 |
| 1.20 | `RosterBackgroundStore` | `RosterBackgrounds`, `SelectRosterBackground` | `roster_background.go:20,66` | 4,1 |
| 1.21 | `AccountMaterialStore` | `AccountMaterials`, `CommitAccountMaterialSweep`, `CommitAccountMaterialEvent`, `CommitAccountMaterialEventTx` | `account_materials.go:21,30`, `account_material_event.go:39,31` | 10,1,5,1 |
| 1.22 | `FavorStore` | `ListFavor`, `GiveFavor` | `favor.go:34,49` | 1,4 |
| 1.23 | `StoryDigestStore` | `AdvanceStoryDigest` | `story_digest.go:12` | **0 — dead export** |
| 1.24 | `BirthStore` | `BirthStage`, `StartBirth`, `AdvanceBirth` | `birth.go:34,50,61` | 8,4,6 |
| 1.24 | `TutorialStore` | `SaveTutorialFlag`, `TutorialFlags` | `tutorial.go:12,24` | 5,4 |
| 1.25 | `SkillLockStore` | `SkillLocks`, `CommitSkillLocks` | `skill_lock.go:26,34` | 1,1 |
| 1.25 | `FameStore` | `RecordCharacterFame` | `fame.go:12` | 1 |
| 1.26 | `IspinsStore` | `IspinsWeeklyUsed`, `RecordIspinsWeeklyClear` | `ispins_weekly.go:23,28` | 5,1 |
| 1.27 | *(raw-SQL escape hatch — must NOT enter a domain interface)* | `DiagnosticQuery`, `DiagnosticExec` | `diagnostic.go:14,54` | 1,6 |

Sum of §1.1–1.27 = **165**. The remaining **37** are the migration/DDL surface in §1.28.

### 1.28 Migration / DDL surface (37 methods — bootstrap and `charactercheck` only)

`Migrate` (`store.go:78`) plus, with `file:line`:

| Method | file:line | Method | file:line |
| --- | --- | --- | --- |
| `MigrateAccountMaterials` | `account_materials.go:15` | `MigrateOathOptions` | `oath_options.go:134` |
| `MigrateAccountVault` | `account_vault.go:21` | `MigrateOathProgress` | `oath_progress.go:17` |
| `MigrateAdventure` | `adventure.go:118` | `MigrateOmenState` | `omen_state.go:41` |
| `MigrateBirth` | `birth.go:22` | `MigratePremiums` | `premium.go:24` |
| `MigrateBleedingMine` | `bleeding_mine.go:12` | `MigrateProfileSkins` | `profile_skin.go:10` |
| `MigrateCashShop` | `cash_purchase.go:24` | `MigrateQuests` | `quest.go:18` |
| `MigrateCharacterEvents` | `character_event.go:18` | `MigrateQuestObjectives` | `quest_objective.go:10` |
| `MigrateCharacterNotices` | `character_notice_store.go:15` | `MigrateQuestRewards` | `quest_reward.go:10` |
| `MigrateEquipmentSkill` | `equipment_skill.go:23` | `MigrateRosterBackgrounds` | `roster_background.go:15` |
| `MigrateFatigue` | `fatigue.go:13` | `MigrateSaveIdentity` | `source_rebaseline.go:53` |
| `MigrateGamepad` | `gamepad.go:21` | `MigrateShopPurchases` | `shop_purchase.go:23` |
| `MigrateGMMail` | `gm_mail.go:25` | `MigrateSkillLocks` | `skill_lock.go:22` |
| `MigrateGrants` | `grant.go:69` | `MigrateSkinCargo` | `skin_cargo.go:22` |
| `MigrateMailbox` | `mailbox.go:28` | `MigrateSkinSelection` | `skin_selection.go:24` |
| `MigrateTowerProgress` | `tower_progress.go:45` | `MigrateSkinSelectionList` | `skin_selection_list.go:28` |
| `MigrateTowerGriefProgress` | `tower_grief.go:24` | `MigrateTutorial` | `tutorial.go:9` |
| `MigrateUnifiedOptions` | `unified_option.go:21` | `MigrateVault` | `vault.go:13` |
| `MigrateWorld` | `world.go:19` | `UpgradeSecondaryVaultCapacity` | `vault.go:20` |

7 have zero out-of-package callers; 22 are called only from `cmd/wireprobe/bootstrap.go`
(22 sites, `:574–:1209`) and/or `internal/toolcmd/charactercheck/*`. `MigrateSaveIdentity`
(`bootstrap.go:1163`) returns a rebaseline count and is on the boot path.

### 1.29 Exported methods with zero out-of-package callers (21)

`ActivatePremium`, `ActivePremiumSet`, `AdvanceStoryDigest`, `AdvanceTowerGrief`,
`CharacterGamepadPayload`, `ClearCharacterGamepadSettings`, `MigrateAdventure`,
`MigrateBleedingMine`, `MigrateEquipmentSkill`, `MigrateGamepad`, `MigrateMailbox`,
`MigrateOathOptions`, `MigrateOathProgress`, `MigrateOmenState`, `MigrateProfileSkins`,
`MigrateRosterBackgrounds`, `MigrateShopPurchases`, `MigrateTowerGriefProgress`,
`OathOption`, `PurchaseCashPremium`, `SaveOathOption`.

Note: `CountShopPurchases` exists on **both** `*Store` (`shop_purchase.go:62`) and `*Tx`
(`shop_purchase.go:69`) with different signatures.

---

## 2. Methods whose signatures leak driver / generated types

**Result: 0 exported `*Store` methods leak `pgx.`, `pgtype.`, `sqlcgen.` or `pgxpool.`
in their signature.** Every exported signature uses domain types, `[]byte`,
`json.RawMessage`, `context.Context`, `time.Time` or primitives. `pgtype.` appears only
inside function bodies (`grant.go:118`, `mailbox.go:160`); the driver boundary is entirely
inside unexported helpers:

| file:line | Signature |
| --- | --- |
| `tx.go:19` | `func newTx(tx pgx.Tx, account, character int64) *Tx` |
| `store.go:161` | `func lockCharacter(ctx context.Context, tx pgx.Tx, account, id int64) (Character, error)` |
| `adventure.go:233` | `func lockAdventure(ctx context.Context, tx pgx.Tx, role Character) (AccountAdventure, error)` |
| `adventure.go:255` | `func saveAdventure(ctx context.Context, tx pgx.Tx, account int64, p AccountAdventure) error` |
| `adventure.go:268` | `func (s *Store) commitAdventureExperience(ctx context.Context, tx pgx.Tx, role Character, state json.RawMessage) error` — **the only `pgx.` in a method signature, and it is unexported** |
| `account_vault.go:16` | `func lockSharedVault(ctx context.Context, q *sqlcgen.Queries, account int64) (AccountVaultState, error)` |
| `roster_background.go:33` | `func readRosterBackgrounds(ctx context.Context, queries *sqlcgen.Queries, account int64) (character.RosterBackgroundState, error)` |
| `vault.go:37` | `func lockPersonalVault(ctx context.Context, q *sqlcgen.Queries, id int64, secondary bool) (VaultState, error)` |
| `vault.go:49` | `func savePersonalVaultItems(ctx context.Context, q *sqlcgen.Queries, id int64, items json.RawMessage, secondary bool) error` |
| `shop_purchase.go:73` | `func countShopPurchases(ctx context.Context, queries *sqlcgen.Queries, scope ShopPurchaseScope, ...)` |
| `skill_lock.go:103` | `func skillLocksQuery(ctx context.Context, queries *sqlcgen.Queries, id int64) ([]uint16, error)` |
| `store.go:150` | `func storedCharacter(row sqlcgen.CharactersRow) Character` |
| `mailbox.go:40` | `func storedMail(row sqlcgen.MailboxRow) (MailMessage, error)` |
| `tower_progress.go:52` | `func storedTowerProgress(row sqlcgen.ReadTowerProgressRow) TowerProgress` |

### 2.1 The real blocker: `func(*Tx, ...)` callback parameters (2 methods)

| file:line | Method | Callback signature |
| --- | --- | --- |
| `character_event.go:45` | `CommitCharacterEventTx` | `apply func(*Tx, Character) (json.RawMessage, json.RawMessage, error)` |
| `account_material_event.go:31` | `CommitAccountMaterialEventTx` | `apply func(*Tx, Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)` |

`database.Tx` is exported, so these *can* sit in an interface — but they are the only
reason `Tx` is public, and they already force `*database.Tx` into three other packages:

| file:line | Why |
| --- | --- |
| `cmd/wireprobe/roster_background_flow.go:65` | literal `func(tx *database.Tx, current database.Character) ...` passed to `CommitCharacterEventTx` |
| `internal/workflow/shop.go:63` | `func (s *ShopService) checkShopLimit(ctx context.Context, tx *database.Tx, shopID, template uint32) error` |
| `internal/workflow/shop.go:118` | literal `func(tx *database.Tx, current database.Character, rawCounts json.RawMessage) ...` |
| `internal/workflow/shop.go:136` | literal `func(tx *database.Tx, current database.Character) ...` |

`newTx` is constructed only in-package (`account_material_event.go:91`,
`character_event.go:131`), so `Tx`'s fields stay private.

### 2.2 Methods needing signature narrowing before interface extraction

Counts: **30 exported** methods take a `func(...)` callback; including the 3 unexported
siblings (`account_material_event.go:43`, `cash_purchase.go:328`, `character_event.go:65`)
the total callback-bearing `*Store` methods is **33**. Methods literally named `Commit*`
with an `apply`/`consume`/`deliver`/`mutate` callback: **21**. The full list — every one of
these pins a distinct closure type into the interface contract, and `CommitCharacterEvent`
alone has 87 call sites:

| file:line | Method | Callback parameter(s) |
| --- | --- | --- |
| `account_materials.go:30` | `CommitAccountMaterialSweep` | `apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)` |
| `account_material_event.go:31` | `CommitAccountMaterialEventTx` | `apply func(*Tx, Character, json.RawMessage) (...)` |
| `account_material_event.go:39` | `CommitAccountMaterialEvent` | `apply func(Character, json.RawMessage) (...)` |
| `account_vault.go:33` | `CommitAccountVaultSort` | `sortItems func(AccountVaultState) (json.RawMessage, error)` |
| `account_vault.go:77` | `CommitAccountVault` | `apply func(Character, json.RawMessage, AccountVaultState) (...)` |
| `account_vault.go:152` | `CommitAccountVaultCrossMove` | `apply func(Character, AccountVaultState, VaultState) (...)` |
| `admin_guard.go:11` | `HoldAdminGuard` | returns release closure `func()` |
| `adventure.go:47` | `CommitAdventure` | `apply func(Character, *AccountAdventure) (...)` |
| `black_purgatory.go:21` | `FreezeBlackPurgatoryReward` | `create func() (json.RawMessage, error)` |
| `black_purgatory_read.go:14` | `ReadPendingBlackPurgatoryRewards` | `consume func(json.RawMessage) error` |
| `bleeding_mine.go:18` | `UpdateBleedingMineRewards` | `apply func(Character, json.RawMessage) (..., []MailAsset, error)` |
| `cash_purchase.go:267` | `PurchaseCashMixed` | `deliver func(json.RawMessage) (json.RawMessage, error)` |
| `cash_purchase.go:284` | `PurchaseCashToBag` | `deliver func(json.RawMessage) (json.RawMessage, error)` |
| `cash_purchase.go:292` | `PurchaseCashVault` | `upgrade func(VaultState) (VaultState, error)` |
| `character_event.go:32` | `CommitCharacterEvent` | `apply func(Character) (json.RawMessage, json.RawMessage, error)` |
| `character_event.go:45` | `CommitCharacterEventTx` | `apply func(*Tx, Character) (...)` |
| `character_event.go:53` | `CommitCharacterPremiumEvent` | `apply func(Character) (..., []CashPremiumActivation, error)` |
| `diagnostic.go:14` | `DiagnosticQuery` | `consume func([]string, []any) error` |
| `fatigue_recovery.go:15` | `RecoverFatigue` | `consume func(Character) (json.RawMessage, error)` |
| `favor.go:49` | `GiveFavor` | `consume func(Character, json.RawMessage) (...)` |
| `grant.go:90` | `ApplyGrant` | `mutate func(Character) (json.RawMessage, json.RawMessage, error)` |
| `mailbox.go:78` | `SendMail` | `prepare func(Character) (json.RawMessage, []MailAsset, error)` |
| `mailbox.go:233` | `MutateMailbox` | `apply func(Character, []MailMessage) (...)` |
| `odyssey_graduation.go:17` | `CommitOdysseyGraduation` | `apply func(Character, bool) (..., []uint16, error)` |
| `odyssey_honor.go:14` | `CommitOdysseyHonorMail` | `apply func(Character, bool) (..., json.RawMessage, error)` |
| `quest_reward.go:20` | `CommitQuestReward` | `apply func(Character) (json.RawMessage, json.RawMessage, error)` |
| `skill_lock.go:34` | `CommitSkillLocks` | `apply func(current []uint16) ([]uint16, error)` |
| `vault.go:87` | `CommitVaultMove` | `apply func(role Character, vault VaultState) (newRoleState, newVaultItems json.RawMessage, err error)` + variadic `space ...byte` |
| `vault.go:143` | `CommitVaultCrossMove` | `apply func(Character, VaultState, VaultState) (...)` |
| `vault_transfer.go:17` | `CommitVaultTransfer` | `apply func(Character, VaultState) (json.RawMessage, json.RawMessage, error)` |

---

## 3. Existing narrow interfaces already satisfied by `*database.Store`

This is the repo's established convention: the **consumer** package declares a small
interface; `cmd/wireprobe/bootstrap.go` injects the concrete `*database.Store` into it.

| Interface | Package (file) | Methods | Evidence | Other implementers |
| --- | --- | --- | --- | --- |
| `EventStore` | `internal/character` (`store.go:11`) | `CommitCharacterEvent`, `HasGrowthPremium`, `HasTacticianPremium` | embedded by `character.Store`; assertion `internal/database/character_contract_test.go:9` | — |
| `Store` | `internal/character` (`store.go:18`) | `EventStore` + `NameExists`, `Characters`, `CreateCharacter`, `CompletedQuestIDs`, `AccountUnifiedOptions`, `CharacterUnifiedOptions`, `AdventureLevel`, `ChangeCharacterSlots`, `CommitSkillLocks` (12) | **compile-time assertion** `internal/database/character_contract_test.go:9`; field `internal/character/service.go:70 Store Store` | `internal/character/store_test.go:13 var concrete *database.Store` (interface-only test) |
| `ProgressionStore` | `internal/character` (`store.go:32`) | `EventStore` + `CharacterEventReceipt`, `RunMonsterExperience`, `RunFatigueLedger` (6) | **assertion** `character_contract_test.go:10`; field `internal/character/progression.go:22`; injected `bootstrap.go:790` | — |
| `FatigueStore` | `internal/character` (`store.go:40`) | `HasGrowthPremium`, `LoadFatigue`, `ConsumeRoomFatigue`, `RunPaidFatigue`, `RecoverFatigue` (5) | **assertion** `character_contract_test.go:11`; field `internal/character/fatigue.go:20` | — |
| `odysseyHonorStore` *(unexported)* | `internal/character` (`odyssey_honor.go:11`) | `CommitOdysseyHonorMail` | runtime assertion `internal/character/odyssey_honor.go:68 s.Store.(odysseyHonorStore)` | — |
| `Store` | `internal/quest` (`store.go:26`) | `Quests`, `AcceptQuestGroups`, `AdventureEquipmentRegistered`, `CompleteQuestObjective`, `MarkMeetNPCQuest`, `RecordQuestMapClear`, `CompleteQuestUseObjective`, `CommitOdysseyGraduation` (8) | field `internal/quest/service.go:14`; injected `bootstrap.go:1035` | — |
| `Store` | `internal/world` (`service.go:19`) | `LoadWorld` | field `internal/world/service.go:24`; injected `bootstrap.go:674` | `fakeStore` (`internal/world/store_interface_test.go:13`) |
| `VaultLedger` | `internal/workflow` (`cash_vault.go:12`) | `PurchaseCashVault` | runtime assertion `cmd/wireprobe/shop_pilot.go:165 store.(workflow.VaultLedger)` | — |
| `equipmentEventStore` *(unexported)* | `internal/workflow` (`equipment_event.go:9`) | `CommitCharacterEvent`, `CharacterEventReceipt` | generic helper `commitEquipmentEvent[T]` (`:19`) called from `item_equipment.go:125`, `loot_rewards.go` (7 sites), `shop.go:177`, `unseal.go:90`, `wear.go:22,175`, `quest_seeking.go:38`, `avatar_recast.go:36`, `bag_operations.go:56,100`, `item_avatar_emblem.go` (4), `loot_pickup.go:42` | — |
| `boosterEventStore` *(unexported)* | `cmd/wireprobe` (`booster_flow.go:150`) | `CommitCharacterEvent`, `CharacterEventReceipt` | param type `booster_flow.go:203` | — |
| *(inline anonymous)* | `cmd/wireprobe` (`booster_flow.go:173`) | `CommitCharacterPremiumEvent` | runtime assertion `booster_flow.go:173` | — |
| *(inline anonymous)* | `internal/workflow` (`cash_vault.go:35`) | `VaultPurchaseSpace` | runtime assertion on `ledger` (`cash_vault.go:35`) | — |
| `PremiumStore` | `internal/inventory` (`wear.go:63`) | `HasConquerorPremium` | adapter `internal/workflow/inventory_role.go:16 type PremiumReader struct{ Store *database.Store }` implements it (`:18`); injected `bootstrap.go:1060` | `PremiumReader` (`inventory_role.go:16`) |

Not satisfied by `*database.Store`: `inventory.EquipmentDefinitioner`
(`internal/inventory/equipment_journal.go:204`) is satisfied by catalog types.

---

## 4. Concrete-`Store`-only call sites

### 4.1 Type assertion to `*database.Store` (1 — hard blocker)

| file:line | Why |
| --- | --- |
| `cmd/wireprobe/booster_flow.go:246` | `if realStore, ok := store.(*database.Store); ok` — downcasts the `boosterEventStore` interface param purely to call `selectOdysseyWeapon`, which takes `*database.Store` (`odyssey_weapon_box.go:83`). If the value stops being `*Store`, the branch silently degrades to the generic error at `booster_flow.go:195`. |

### 4.2 Raw-SQL / DDL / lifecycle calls with no interface substitute

| file:line | Call | Why concrete-only |
| --- | --- | --- |
| `internal/toolcmd/dbq/main.go:28` | `s.DiagnosticQuery(ctx, *sql, func(fields []string, vals []any) error …)` | raw SQL passthrough; `pgx` row/field machinery |
| `cmd/wireprobe/primer_transform_flow_test.go:37,41,245,249,358,362` | `admin.DiagnosticExec(ctx, "CREATE SCHEMA "+schema)` / `DROP SCHEMA … CASCADE` | PostgreSQL-only DDL in the test harness |
| `cmd/wireprobe/bootstrap.go:506` | `gameStore.ScrubPollutedWorldPositions(scrubCtx, towns)` | one-off data repair via `*sqlcgen` bulk update |
| `cmd/wireprobe/bootstrap.go:513` | `s.HoldAdminGuard(ctx)` → `releaseAdminGuard` | `admin_guard.go:23 sqlcgen.TrySharedAdminGuard`; process-lifetime lock, returns a `func()` resource handle |
| `cmd/wireprobe/bootstrap.go:518` | `s.InitializeGame(ctx)` | DDL + game bootstrap (`migrations.go:99`) |
| `cmd/wireprobe/bootstrap.go:1163` | `gameStore.MigrateSaveIdentity(ctx, identity)` | schema/identity migration, returns rebaseline count |
| `cmd/wireprobe/bootstrap.go:1203` | `gameStore.UpgradeSecondaryVaultCapacity(ctx)` | one-off data migration (`vault.go:20`) |
| `cmd/wireprobe/bootstrap.go:574,576,578,601,612,615,618,649,676,808,810,813,816,845,1143,1145,1148,1201,1206,1209` | 22 `Migrate*` calls | DDL — see §1.28 |
| `internal/toolcmd/charactercheck/main.go:46,49,52,56,165` + `*_check.go` (15 calls) | `Migrate`, `Migrate*` | storage conformance tool drives DDL directly |
| `internal/toolcmd/initialrepair/main.go:98,101`; `internal/toolcmd/questrepair/main.go:55`; `internal/toolcmd/storagecheck/main.go:25`; `cmd/admin/main.go:120`; `cmd/gmtool/main.go:278` | `MigrateCharacterEvents`, `MigrateCharacterNotices`, `MigrateQuests`, `Migrate`, `MigrateGrants`, `MigrateGMMail` | tool/bootstrap entry points |

### 4.3 `Open` / `Close` / `LoadConfig` lifecycle (outside the domain surface)

Concrete `*Store.Close()` sites: `cmd/admin/main.go:119`, `cmd/admin/main_test.go:30`,
`cmd/gmtool/main.go:277`, `cmd/gmtool/pvf.go:25`, `cmd/wireprobe/catalog_runtime_test.go:109`,
`cmd/wireprobe/primer_transform_flow_test.go:35,49,243,257,356,370`,
`internal/toolcmd/charactercheck/main.go:42,153`, `internal/toolcmd/dbq/main.go:27`,
`internal/toolcmd/initialrepair/main.go:97`, `internal/toolcmd/questrepair/main.go:54`,
`internal/toolcmd/storagecheck/main.go:24`, plus `cmd/wireprobe/bootstrap.go:493`
(registered as a resource cleanup) — **18 sites**.
`Open`/`LoadConfig` sites: `cmd/wireprobe/bootstrap.go:485,489`; `cmd/gmtool/main.go:253,270`;
`cmd/admin/main.go:111,115`; `internal/toolcmd/dbq/main.go:17,23`;
`internal/toolcmd/initialrepair/main.go:89,93`; `internal/toolcmd/questrepair/main.go:46,50`;
`internal/toolcmd/storagecheck/main.go:14,20`; and
`cmd/wireprobe/primer_transform_flow_test.go:31,45,239,253,352,366`.

### 4.4 Fields / params typed exactly `*database.Store` (the holders to convert)

Production field declarations (13) and embedded/aliased shapes:

| file:line | Declaration |
| --- | --- |
| `internal/admin/grant.go:20` | `Store    *database.Store` (GrantService) |
| `internal/workflow/wear.go:15` | `Store *database.Store` (WearService) |
| `internal/workflow/vault.go:15` | `Store *database.Store` (VaultService) |
| `internal/workflow/unseal.go:27` | `Store         *database.Store` (UnsealService) |
| `internal/workflow/shop.go:19` | `Store *database.Store` (ShopService) |
| `internal/workflow/item_equipment.go:149` | `Store *database.Store` (ItemService) |
| `internal/workflow/loot_rewards.go:15` | `Store *database.Store` (LootService) |
| `internal/workflow/quest_finish.go:19` | `Store *database.Store` (QuestService) |
| `internal/workflow/inventory_role.go:16` | `type PremiumReader struct{ Store *database.Store }` (adapter to `inventory.PremiumStore`) |
| `cmd/gmtool/main.go:144` | `store    *database.Store` |
| `cmd/wireprobe/bootstrap.go:48` | `gameStore             *database.Store` (bootstrap result struct) |
| `cmd/wireprobe/bootstrap.go:450` | `var gameStore *database.Store` (assigned `:494`) |
| `cmd/wireprobe/world_flow.go:37` | `store             *database.Store` (`worldSession.store`) |

Free-function parameters typed `*database.Store`: `internal/workflow/bag_operations.go:36,96`;
`cmd/wireprobe/account_materials_flow.go:22`, `cinematic_flow.go:71,99`,
`moon_solo_resources.go:12`, `odyssey_revive.go:65`, `odyssey_rewards.go:60,103,143`,
`odyssey_weapon_box.go:83`, `omen_state.go:37`, `reward_flow.go:19,39,57,84`,
`roster_background_flow.go:18`, `skin_family_flow.go:128,363,470,517,534,581,612`,
`skin_flow.go:421`, `skin_selection_flow.go:97,208`, `skin_storage_flow.go:136`,
`synopsis_flow.go:55`; and `reopened *database.Store` in 14
`internal/toolcmd/charactercheck/*_check.go` files
(`wear,tutorial,quest_reward,quest_objective,progression,clear_reward,card,module,birth,loot,learning,grant,fatigue,delete`).

### 4.5 Test-only construction that bypasses storage

| file:line | Why it blocks |
| --- | --- |
| `cmd/wireprobe/favor_flow_test.go:143` | `store: &database.Store{}` — zero-value Store with `db == nil`; reached via `w.store.GiveFavor(...)` (`favor_flow.go:68`), so a fake is needed once `store` is an interface |
| `cmd/wireprobe/oath_direct_entry_test.go:24` | `characters: &character.Service{Store: &database.Store{}}` |
| `internal/character/store_test.go:13` | `var concrete *database.Store` — interface-only test still names the concrete type |

### 4.6 Test fixture coupling (largest structural blocker)

```
fixture.go:24   type fixtureStore = Store          // alias — promotes ALL 202 methods
fixture.go:26   type TestFixture struct {
fixture.go:27       *fixtureStore                  // embedded concrete pointer
fixture.go:29       admin  *Store
fixture.go:30       schema string
fixture.go:31   }
fixture.go:48   ... admin.db.Exec(ctx, "CREATE SCHEMA "+quoted)   // direct .db
fixture.go:57   ... f.queries.FixtureSchema(ctx)                   // direct .queries
```

`database.TestFixture` is used out-of-package as `fixture *database.TestFixture` at
`cmd/wireprobe/fatigue_items_test.go:20` and `cmd/wireprobe/shop_gold_test.go:14`, and as
`s *database.TestFixture` in all 14 `internal/toolcmd/charactercheck/*_check.go` files.
Making `Store` an interface turns `*fixtureStore` into an interface embed (promotion still
works, but `f.fixtureStore` can no longer be assigned from `Open(...)`) and breaks
`admin.db.Exec` / `f.queries.*`, which need a separate concrete handle.

### 4.7 Direct `s.db` / `s.queries` access

`Store.db` and `Store.queries` are unexported and there is **zero** `.db` access outside
`internal/database`. Inside the package the driver handle is touched directly in **41
non-test files / 67 sites** — the sites that must become driver-agnostic. Densest:
`unified_option.go` (5), `premium.go` (4), `account_vault.go` / `diagnostic.go` /
`fixture.go` / `oath_options.go` / `store.go` (3 each). Hand-written `pgx` pool APIs:
`s.db.Begin`/`BeginTx`/`pgx.BeginFunc` (see §5) and, in tests, `admin.db.Exec`
(`fixture.go:48,81`).

### 4.8 What blocks replacing `*database.Store` with an interface

1. One type assertion to the concrete type (`booster_flow.go:246`), reachable only because
   `selectOdysseyWeapon` and the `grantOdyssey*` helpers take `*database.Store`.
2. `DiagnosticQuery`/`DiagnosticExec` (7 out-of-package sites) — raw SQL, `pgx`-only.
3. The 37 `Migrate*` + `InitializeGame`/`HoldAdminGuard`/`UpgradeSecondaryVaultCapacity`/
   `ScrubPollutedWorldPositions`/`Close`/`Open`/`LoadConfig` bootstrap-and-tool surface
   (~50 out-of-package sites, concentrated in `cmd/wireprobe/bootstrap.go` and
   `internal/toolcmd/*`).
4. 13 field declarations + ~40 free-function parameters typed `*database.Store`
   (mechanical but wide).
5. `database.TestFixture` (`fixtureStore = Store` embeds the concrete pointer;
   `admin.db.Exec`), plus the two zero-value `&database.Store{}` test constructions.

---

## 5. Transaction callback API surface

### 5.1 `type Tx` (`internal/database/tx.go`)

```
tx.go:12   type Tx struct {
tx.go:13       tx          pgx.Tx
tx.go:14       queries     *sqlcgen.Queries
tx.go:15       accountID   int64
tx.go:16       characterID int64
tx.go:17   }
tx.go:19   func newTx(tx pgx.Tx, account, character int64) *Tx
```

All fields unexported; `newTx` unexported. Callers: `account_material_event.go:91`,
`character_event.go:131`.

### 5.2 Exported methods on `*Tx` (3)

| file:line | Signature |
| --- | --- |
| `shop_purchase.go:69` | `func (tx *Tx) CountShopPurchases(ctx context.Context, scope ShopPurchaseScope, npcID, template uint32, start time.Time) (int, error)` |
| `shop_purchase.go:94` | `func (tx *Tx) RecordShopPurchase(ctx context.Context, npcID, template uint32) error` |
| `roster_background.go:98` | `func (tx *Tx) UnlockRosterBackground(ctx context.Context, grant character.RosterBackgroundUnlock, now time.Time) error` |

No other `*Tx` receiver exists in `internal/database`.

### 5.3 `Commit*`-family methods on `*Store` with a callback (26; 25 open their own tx)

| file:line | Method | Callback | Opens tx? |
| --- | --- | --- | --- |
| `character_event.go:32` | `CommitCharacterEvent` | `apply func(Character) (...)` | yes — `:77 s.db.Begin` via `commitCharacterEvent:65` |
| `character_event.go:45` | `CommitCharacterEventTx` | `apply func(*Tx, Character) (...)` | yes (hands `*Tx` to caller) |
| `character_event.go:53` | `CommitCharacterPremiumEvent` | `apply func(Character) (..., []CashPremiumActivation, error)` | yes (plus `premium.go:78`) |
| `account_material_event.go:31` | `CommitAccountMaterialEventTx` | `apply func(*Tx, Character, json.RawMessage) (...)` | yes — `:51 s.db.Begin` |
| `account_material_event.go:39` | `CommitAccountMaterialEvent` | `apply func(Character, json.RawMessage) (...)` | yes |
| `account_materials.go:30` | `CommitAccountMaterialSweep` | `apply func(Character, json.RawMessage) (...)` | yes — `:36` |
| `account_vault.go:33` | `CommitAccountVaultSort` | `sortItems func(AccountVaultState) (json.RawMessage, error)` | yes — `:38` |
| `account_vault.go:77` | `CommitAccountVault` | `apply func(Character, json.RawMessage, AccountVaultState) (...)` | yes — `:86` |
| `account_vault.go:152` | `CommitAccountVaultCrossMove` | `apply func(Character, AccountVaultState, VaultState) (...)` | yes — `:165` |
| `adventure.go:47` | `CommitAdventure` | `apply func(Character, *AccountAdventure) (...)` | yes — `:54` |
| `black_purgatory.go:21` | `FreezeBlackPurgatoryReward` | `create func() (json.RawMessage, error)` | yes — `:28` |
| `bleeding_mine.go:18` | `UpdateBleedingMineRewards` | `apply func(Character, json.RawMessage) (..., []MailAsset, error)` | yes — `:25` |
| `cash_purchase.go:267` | `PurchaseCashMixed` | `deliver func(json.RawMessage) (json.RawMessage, error)` + `premiums map[int]CashPremiumActivation` | yes — `:342` |
| `cash_purchase.go:284` | `PurchaseCashToBag` | `deliver func(json.RawMessage) (json.RawMessage, error)` | yes — `purchaseCash:328` |
| `cash_purchase.go:292` | `PurchaseCashVault` | `upgrade func(VaultState) (VaultState, error)` | yes — same path (variadic `upgrades ...func(VaultState) (VaultState, error)`) |
| `fatigue_recovery.go:15` | `RecoverFatigue` | `consume func(Character) (json.RawMessage, error)` | yes — `:21` |
| `favor.go:49` | `GiveFavor` | `consume func(Character, json.RawMessage) (...)` | yes — `:55` |
| `grant.go:90` | `ApplyGrant` | `mutate func(Character) (json.RawMessage, json.RawMessage, error)` | yes — `:101` |
| `mailbox.go:78` | `SendMail` | `prepare func(Character) (json.RawMessage, []MailAsset, error)` | yes — `:86` |
| `mailbox.go:233` | `MutateMailbox` | `apply func(Character, []MailMessage) (...)` | yes — `:239` |
| `odyssey_graduation.go:17` | `CommitOdysseyGraduation` | `apply func(Character, bool) (..., []uint16, error)` | yes — `:23` |
| `odyssey_honor.go:14` | `CommitOdysseyHonorMail` | `apply func(Character, bool) (..., json.RawMessage, error)` | **no** — runs inside the `*Tx` from `CommitCharacterEventTx` (`odyssey_honor.go:20 tx.queries…`) |
| `quest_reward.go:20` | `CommitQuestReward` | `apply func(Character) (json.RawMessage, json.RawMessage, error)` | yes — `:25` |
| `skill_lock.go:34` | `CommitSkillLocks` | `apply func(current []uint16) ([]uint16, error)` | yes — `:38` |
| `vault.go:87` | `CommitVaultMove` | `apply func(role Character, vault VaultState) (newRoleState, newVaultItems json.RawMessage, err error)` | yes — `:98` |
| `vault.go:143` | `CommitVaultCrossMove` | `apply func(Character, VaultState, VaultState) (...)` | yes — `:151` |
| `vault_transfer.go:17` | `CommitVaultTransfer` | `apply func(Character, VaultState) (json.RawMessage, json.RawMessage, error)` | yes — `:27` |

**26 methods; 25 open a transaction internally, 1 (`CommitOdysseyHonorMail`) does not.**
That single exception is the only reason `Tx` is exported at all.

### 5.4 Transaction-start inventory in `internal/database` (non-test): 54 sites

`pgx.BeginFunc`: `unified_option.go:49,72,139,187,212`, `warp_favorites.go:28` (6).
`s.db.BeginTx`: `store.go:93`, `roster_background.go:21` (RepeatableRead + ReadOnly) (2).
`s.db.Begin` (46): the 25 in §5.3 plus `character_delete.go:13`, `character_slot.go:91`,
`equipment_skill.go:41,68`, `fatigue.go:26`, `gm_mail.go:33`, `ispins_weekly.go:35`,
`migrations.go:55`, `oath_options.go:57,94,159`, `oath_progress.go:45`, `premium.go:78`,
`profile_skin.go:19`, `quest.go:36,160`, `quest_objective.go:50`, `quest_read.go:57`,
`roster_background.go:71`, `skin_favorite.go:37`, `skin_selection_list.go:40,94`,
`story_digest.go:14`, `black_purgatory.go:95`, `bleeding_mine.go:100`, `adventure.go:168`.

---

## 6. Counts summary

| Metric | Value |
| --- | --- |
| Total exported `*Store` methods | **202** (case-sensitive `-cmatch` / ripgrep; a case-insensitive PowerShell scan reports 210 by also matching 8 unexported helpers) |
| Non-test `.go` files in `internal/database` | **61** (102 total, 41 are `*_test.go`) |
| Files containing ≥1 exported `*Store` method | **58** |
| Exported methods on `*Tx` | **3** |
| `Commit*`-family callback methods on `*Store` | **26** (25 open their own transaction) |
| Exported methods with a `func(...)` callback parameter | **30** (33 including unexported siblings) |
| Transaction-start sites in `internal/database` (non-test) | **54** |
| Exported methods with zero out-of-package callers | **21** |
| `database.Store` textual occurrences, whole repo | **63** |
| … inside `internal/database` | 3 |
| … **outside `internal/database`** | **60 occurrences across 45 files** |
| Out-of-package type assertions to `*database.Store` | **1** (`cmd/wireprobe/booster_flow.go:246`) |
| **§2 count: exported methods leaking `pgx.`/`pgtype.`/`sqlcgen.`/`pgxpool.`** | **0** |
| `pgx.` in an unexported method signature | 1 (`adventure.go:268 commitAdventureExperience`) |
| Methods naming `*Tx` in a callback | **2** (`character_event.go:45`, `account_material_event.go:31`) |
| Out-of-package `*database.Tx` references | **4** (`roster_background_flow.go:65`, `workflow/shop.go:63,118,136`) |
| Out-of-package narrow interfaces already satisfied by `*Store` | **10 named** + **2 inline anonymous** + **1 adapter-satisfied** (`inventory.PremiumStore` via `workflow.PremiumReader`) |
| Out-of-package interface-consumption sites | compile-time 3 (`internal/database/character_contract_test.go:9,10,11`); runtime assertions at `internal/character/odyssey_honor.go:68`, `cmd/wireprobe/booster_flow.go:173,246`, `cmd/wireprobe/shop_pilot.go:41,165`, `internal/workflow/cash_vault.go:35` |
| Direct `.db` / `.queries` access inside the package | 67 sites across 41 non-test files |

### Counting caveats

* For generic method names the call-site counter filters by receiver
  (`store|s|db|reopened|admin|full|gameStore|w.store|.Store|f|fixture`). Residual
  over-counting remains for `Close` (`s.Close()` also matches catalog/source/archive
  handles); the authoritative `*Store.Close()` list is §4.3 (18 sites). Treat other §1
  numbers as close bounds rather than exact.
* `LoadWorld` shows 5 out-of-package matches with the receiver filter; the remaining ~25
  textual matches in `internal/world/service.go` and its tests go through the `world.Store`
  interface — the desired end state.
* `commitCharacterEvent` is the **unexported** helper at `character_event.go:65`; it is not
  part of the 202 exported methods, which is why an unrefined `\.\w+\(` count over-reports it.
* `internal/database/character_contract_test.go:9-11` counts as 3 of the 63 repo-wide
  `database.Store` occurrences (it is inside `internal/database`).
