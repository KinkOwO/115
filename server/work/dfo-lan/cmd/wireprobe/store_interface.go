package main

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"time"
)

// persistentStore is the persistence surface the gateway (composition root and
// dispatcher) consumes. It is satisfied by the concrete *database.Store.
//
// Why one shared interface instead of one per flow: the gateway threads a single
// store handle through worldSession.store, client.gameStore and ~15 free
// functions, and also assigns it into service interfaces declared by other
// packages (world.Store, character.Store/ProgressionStore/FatigueStore,
// quest.Store, workflow.wearStore/lootStore/questStore/itemStore). Go requires
// the source interface to cover every target, so per-flow interfaces would
// multiply into an unwieldy set of assignment constraints. The composition root
// legitimately sees the widest surface; everything below it names only what it
// needs.
//
// The method set is generated from the real *database.Store declarations: every
// method called on a store handle in this package, plus the method sets of the
// interfaces this package assigns into. Signatures are verbatim - do not retype
// them. store_contract_test.go asserts the concrete store still satisfies this,
// so drift breaks the build instead of failing at runtime.
//
// The DDL/lifecycle surface (Migrate*, InitializeGame, HoldAdminGuard, Close,
// DatabaseName, ScrubPollutedWorldPositions, UpgradeSecondaryVaultCapacity) is
// deliberately NOT here: bootstrap keeps those on the concrete local returned by
// database.Open. The concrete *database.Store is likewise still allowed in local
// variables at the construction point; engine selection and the
// PostgreSQL/SQLite split stay behind internal/database.
type persistentStore interface {
	AbandonQuest(ctx context.Context, account, characterID int64, qid uint16) error
	AcceptQuestGroups(ctx context.Context, account, characterID int64, qid uint16, version string, minLevel, maxLevel uint32, groups [][]uint32, initial uint32, model string) (database.QuestState, error)
	AccountCera(ctx context.Context, account int64) (uint64, error)
	AccountGamepadPayload(ctx context.Context, accountID int64) ([]byte, error)
	AccountHotkeys(ctx context.Context, account int64, subtype byte) (map[uint16]uint16, error)
	AccountMaterials(ctx context.Context, account int64) (json.RawMessage, error)
	AccountUnifiedOptions(ctx context.Context, account int64) (map[uint16]uint16, error)
	AccountWarpFavorites(ctx context.Context, account int64) ([]protocol.WarpFavoriteEntry, error)
	ActivePremiums(ctx context.Context, account int64, now time.Time) ([]database.CashPremium, error)
	AdvanceBirth(ctx context.Context, account, id int64, stage byte, dungeon uint32) (bool, error)
	AdvanceTowerFloor(ctx context.Context, account int64, policy database.TowerPolicy, floor uint16, run string) (database.TowerProgress, error)
	AdventureCollectionEquipment(ctx context.Context, account, character int64) (map[uint32]bool, error)
	AdventureEquipmentRegistered(ctx context.Context, account, character int64, template uint32) (bool, error)
	AdventureLevel(ctx context.Context, account, character int64) (uint32, error)
	ApplyGrant(ctx context.Context, g database.Grant, mutate func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.GrantResult, error)
	BirthStage(ctx context.Context, account, id int64) (byte, uint32, error)
	BlackPurgatoryQuota(ctx context.Context, account, id int64, run, action string, day, week time.Time) (database.BlackPurgatoryQuota, error)
	BleedingMineTeams(ctx context.Context, account int64) ([3][4]int64, error)
	BumpOathProgress(ctx context.Context, characterID, dungeonID int64, needed int) (int, int, error)
	ChangeCharacterSlots(ctx context.Context, account int64, r database.CharacterSlotChange, capacity int) error
	CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error)
	CharacterHotkeys(ctx context.Context, characterID int64, subtype byte) (map[uint16]uint16, error)
	CharacterNoticeSeen(ctx context.Context, account, character int64, tree byte) ([]uint16, error)
	Characters(ctx context.Context, account int64) ([]database.Character, error)
	CharacterSeason(ctx context.Context, account, id int64) (adventure.SeasonState, error)
	CharacterUnifiedOptionGroup(ctx context.Context, characterID int64, subtype byte) (map[uint16]uint16, error)
	CharacterUnifiedOptions(ctx context.Context, characterID int64) (map[uint16]uint16, error)
	ClearAccountCharacterGamepadSettings(ctx context.Context, accountID int64) error
	ClearAccountCharacterHotkeys(ctx context.Context, accountID int64, subtype byte) error
	ClearActQuests(ctx context.Context, account, characterID int64, version string, ids []uint16) (int, error)
	ClearEquipmentSkill(ctx context.Context, accountID, characterID int64) error
	ClearQuests(ctx context.Context, account, characterID int64, version string, ids []uint16) (int, error)
	CommitAccountMaterialEvent(ctx context.Context, account, id int64, version, key, model string, apply func(database.Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (database.Character, json.RawMessage, bool, error)
	CommitAccountMaterialSweep(ctx context.Context, account, id int64, version string, apply func(database.Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (database.Character, json.RawMessage, error)
	CommitAccountVault(ctx context.Context, account, character int64, version, key string, operation uint16, apply func(database.Character, json.RawMessage, database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error), ) (database.Character, json.RawMessage, database.AccountVaultState, bool, error)
	CommitAccountVaultCrossMove(ctx context.Context, account, character int64, version, key string, space byte, apply func(database.Character, database.AccountVaultState, database.VaultState) (json.RawMessage, database.AccountVaultState, json.RawMessage, error), ) (database.Character, database.AccountVaultState, database.VaultState, bool, error)
	CommitAccountVaultSort(ctx context.Context, account, character int64, sortItems func(database.AccountVaultState) (json.RawMessage, error)) (database.AccountVaultState, error)
	CommitAdventure(ctx context.Context, account, id int64, key string, apply func(database.Character, *database.AccountAdventure) (json.RawMessage, json.RawMessage, error)) (database.Character, database.AccountAdventure, json.RawMessage, error)
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error)
	CommitCharacterEventTx(ctx context.Context, account, id int64, version, key, model string, apply func(*database.Tx, database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error)
	CommitCharacterPremiumEvent(ctx context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error)) (database.Character, bool, error)
	CommitOdysseyGraduation(ctx context.Context, account, id int64, version string, apply func(database.Character, bool) (json.RawMessage, json.RawMessage, []uint16, error)) (database.Character, bool, error)
	CommitQuestReward(ctx context.Context, account, id int64, qid uint16, version, progressModel, rewardModel string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.QuestRewardCommit, error)
	CommitSkillLocks(ctx context.Context, account, id int64, key, model string, apply func(current []uint16) ([]uint16, error)) ([]uint16, bool, error)
	CommitSystemMail(ctx context.Context, account, id int64, version, key, model, senderName, body string, assets []database.MailAsset) (int64, bool, error)
	CommitVaultCrossMove(ctx context.Context, account, id int64, apply func(database.Character, database.VaultState, database.VaultState) (json.RawMessage, json.RawMessage, json.RawMessage, error), ) (database.Character, database.VaultState, database.VaultState, error)
	CommitVaultMove(ctx context.Context, account, id int64, apply func(role database.Character, vault database.VaultState) (newRoleState json.RawMessage, newVaultItems json.RawMessage, err error), space ...byte) (database.Character, database.VaultState, error)
	CompletedQuestIDs(ctx context.Context, account int64, version string, ids []uint16) (map[int64][]uint16, error)
	CompleteQuestObjective(ctx context.Context, account, characterID int64, qid uint16, version, model string) (bool, error)
	CompleteQuestUseObjective(ctx context.Context, account, characterID int64, qid uint16, version, model, eventKey string, template uint32) (bool, error)
	ConsumeRoomFatigue(ctx context.Context, account, id int64, day string, limit uint16, run string, room uint32, cost uint16) (database.FatigueState, bool, error)
	CopyAccountHotkeysToCharacter(ctx context.Context, account, characterID int64, subtype byte) error
	CreateCharacter(ctx context.Context, c database.Character, maxCharacters int) (database.Character, error)
	DeleteCharacter(ctx context.Context, account int64, slot uint16, name string) (int64, error)
	EquipmentSkillSnapshots(ctx context.Context, accountID, characterID int64) (skills, commands []byte, err error)
	EquippedOathSelection(ctx context.Context, accountID, characterID int64) (database.EquippedOathOption, error)
	FreezeBlackPurgatoryReward(ctx context.Context, account, id int64, version, run, model string, create func() (json.RawMessage, error), ) (json.RawMessage, error)
	GiveFavor(ctx context.Context, account, id int64, version string, npcID uint32, g database.FavorGift, consume func(database.Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (database.Character, database.FavorState, json.RawMessage, error)
	HasActivePremium(ctx context.Context, account int64, premiumType uint8, now time.Time) (bool, error)
	HasGrowthPremium(ctx context.Context, account int64, now time.Time) (bool, error)
	HasTacticianPremium(ctx context.Context, account int64, now time.Time) (bool, error)
	IspinsWeeklyUsed(ctx context.Context, account, id int64, now time.Time) (bool, error)
	ListFavor(ctx context.Context, characterID int64) ([]database.FavorPoint, error)
	ListSkins(ctx context.Context, account int64) ([]database.AccountSkin, error)
	LoadAccountVault(ctx context.Context, account, character int64) (database.AccountVaultState, error)
	LoadAdventure(ctx context.Context, account, character int64, defaultName string) (database.AccountAdventure, error)
	LoadFatigue(ctx context.Context, account, id int64, day string, limit uint16) (database.FatigueState, error)
	LoadWorld(ctx context.Context, account, characterID int64, channelType uint32, initial database.WorldPosition, version string) (database.WorldState, error)
	Mailbox(ctx context.Context, account, id int64) ([]database.MailMessage, error)
	MailboxDeliveryState(ctx context.Context, account, id int64) (int64, uint16, error)
	MailRecipient(ctx context.Context, name string) (database.Character, error)
	MarkCharacterNotice(ctx context.Context, account, character int64, tree byte, notice uint16, seen bool) error
	MarkMeetNPCQuest(ctx context.Context, account, characterID int64, qid uint16, version, model string) error
	MutateMailbox(ctx context.Context, account, id int64, version, key, model string, messageIDs []int64, apply func(database.Character, []database.MailMessage) (json.RawMessage, []database.MailMessage, json.RawMessage, error)) (database.Character, json.RawMessage, bool, error)
	NameExists(ctx context.Context, name string) (bool, error)
	OathProgressClears(ctx context.Context, characterID, dungeonID int64) (int, error)
	OmenState(ctx context.Context, characterID, dungeonID int64) (database.OmenState, error)
	PendingMoonRewardRuns(ctx context.Context, characterID int64, model string) ([]string, error)
	PrepareAdventure(ctx context.Context, role database.Character, day string) (database.AccountAdventure, error)
	PromoteCharacterHotkeysToAccount(ctx context.Context, account, characterID int64, subtype byte) error
	PurchaseCashToBag(ctx context.Context, o database.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (database.CashReceipt, bool, error)
	Quests(ctx context.Context, account, id int64) ([]database.QuestState, error)
	ReadPendingBlackPurgatoryRewards(ctx context.Context, account, character int64, model string, consume func(json.RawMessage) error) error
	ReadTowerProgress(ctx context.Context, account int64, policy database.TowerPolicy, legacyFloor uint16) (database.TowerProgress, error)
	RecordCharacterFame(ctx context.Context, account, characterID int64, current uint32) (uint32, error)
	RecordIspinsWeeklyClear(ctx context.Context, account, id int64, version, run string, now time.Time, limited bool) error
	RecordQuestMapClear(ctx context.Context, account, characterID int64, run string, mapID uint32, version, model string, matching []uint16) (bool, error)
	RecoverFatigue(ctx context.Context, account, id int64, version string, r database.FatigueRecovery, consume func(database.Character) (json.RawMessage, error)) (database.Character, database.FatigueState, error)
	ReserveTowerEntry(ctx context.Context, account int64, policy database.TowerPolicy, floor uint16, now time.Time) (database.TowerProgress, error)
	ResolveGamepadPayload(ctx context.Context, accountID, characterID int64) ([]byte, error)
	RestoreProfileSkins(ctx context.Context, account, character int64) (character.ProfileSkinState, error)
	RosterBackgrounds(ctx context.Context, account int64) (character.RosterBackgroundState, error)
	RunFatigueLedger(ctx context.Context, account, character int64, run string) (int64, int64, error)
	RunMonsterExperience(ctx context.Context, account, character int64, run string) (uint64, error)
	RunPaidFatigue(ctx context.Context, id int64, run string) (bool, error)
	SaveAccountGamepadKeys(ctx context.Context, accountID int64, tsv []byte) error
	SaveAccountGamepadOptions(ctx context.Context, accountID int64, opts []byte) error
	SaveAccountHotkeys(ctx context.Context, account int64, subtype byte, entries []database.UnifiedOptionEntry) error
	SaveAccountUnifiedOptions(ctx context.Context, account int64, entries []database.UnifiedOptionEntry) error
	SaveAccountWarpFavorites(ctx context.Context, account int64, entries []protocol.WarpFavoriteEntry) error
	SaveBleedingMineTeam(ctx context.Context, account, actor int64, group uint32, ids [4]int64) error
	SaveCharacterGamepadKeys(ctx context.Context, accountID, characterID int64, tsv []byte) error
	SaveCharacterGamepadOptions(ctx context.Context, accountID, characterID int64, opts []byte) error
	SaveCharacterHotkeys(ctx context.Context, account, characterID int64, subtype byte, entries []database.UnifiedOptionEntry) error
	SaveCharacterUnifiedOptionGroup(ctx context.Context, account, characterID int64, subtype byte, entries []database.UnifiedOptionEntry) error
	SaveCharacterUnifiedOptions(ctx context.Context, account, id int64, entries []database.UnifiedOptionEntry) error
	SaveEquipmentSkillSnapshot(ctx context.Context, accountID, characterID int64, which string, data []byte) error
	SaveOmenHeld(ctx context.Context, characterID, dungeonID int64, held int) error
	SaveTutorialFlag(ctx context.Context, account, id int64, index byte, completed bool) error
	SaveWorld(ctx context.Context, account, characterID int64, channelType uint32, old database.WorldState, next database.WorldPosition) (database.WorldState, error)
	SelectedSkin(ctx context.Context, character int64, category uint32) (uint32, error)
	SelectEquippedOathOption(ctx context.Context, accountID, characterID int64, itemID uint32, option int) (database.EquippedOathOption, error)
	SelectRosterBackground(ctx context.Context, account int64, page uint16, b character.RosterBackground) (character.RosterBackgroundState, error)
	SelectSkin(ctx context.Context, character int64, category, skinKey uint32) error
	SendMail(ctx context.Context, account, id int64, version, key, name, body string, prepare func(database.Character) (json.RawMessage, []database.MailAsset, error)) (database.Character, database.MailSendReceipt, bool, error)
	SetOmenOrthaierPending(ctx context.Context, characterID, dungeonID int64, pending bool) error
	SetSkinFavorite(ctx context.Context, character int64, page uint32, key uint32, starred bool, limit int) (bool, error)
	SetSkinSelectionList(ctx context.Context, character int64, category uint32, keys []uint32) error
	SetSkinSelectionSlots(ctx context.Context, character int64, category uint32, keys []uint32) error
	SkillLocks(ctx context.Context, id int64) ([]uint16, error)
	SkinFavorites(ctx context.Context, character int64, groups int) ([][]uint32, error)
	SkinSelectionList(ctx context.Context, character int64, category uint32) ([]uint32, error)
	SkinSelectionSlots(ctx context.Context, character int64, category uint32, width int) ([]uint32, error)
	StartBirth(ctx context.Context, account, id int64) error
	TowerGriefProgress(ctx context.Context, account int64, floors [101]uint32) (database.TowerGriefProgress, error)
	TutorialFlags(ctx context.Context, account, id int64) ([]byte, error)
	UnlockSkin(ctx context.Context, account int64, template, skinKey uint32) error
	UpdateBleedingMineRewards(ctx context.Context, account, actor int64, version string, apply func(database.Character, json.RawMessage) (json.RawMessage, json.RawMessage, []database.MailAsset, error), ) (database.Character, json.RawMessage, error)
}
