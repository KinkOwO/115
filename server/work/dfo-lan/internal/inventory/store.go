package inventory

// Store 是 inventory 领域声明的持久化能力接口（契约 §7.2 E12 收敛目标）。
// 当前 inventory 仍 import storage 是因为 Character 类型归 storage（model 包已撤销）：
// 彻底删除 inventory→storage 边需等 Character 迁出 storage（依赖 character 倒置 E11）。
// 在那之前本接口先就位：inventory 各服务的 Store 字段由 *storage.Store 改为 inventory.Store，
// 仓库事务与回执读写由工作流层（internal/workflow）持有并调用，本接口是契约面。
//
// *storage.Store 在方法签名上自然满足本接口（Character 同一类型、回调一致）。
import (
	"context"
	"dfolan/internal/db"
	"dfolan/internal/storage"
	"encoding/json"
	"time"
)

type Store interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
	CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error)
	CommitCharacterEventTx(ctx context.Context, account, id int64, version, key, model string, apply func(db.Tx, storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
	CommitAccountMaterialEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (storage.Character, json.RawMessage, bool, error)
	CommitAccountMaterialEventTx(ctx context.Context, account, id int64, version, key, model string, apply func(db.Tx, storage.Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (storage.Character, json.RawMessage, bool, error)
	CommitVaultTransfer(ctx context.Context, account, id int64, source, vaultVersion, key string, request []byte, apply func(storage.Character, VaultState) (json.RawMessage, json.RawMessage, error)) (storage.Character, VaultState, bool, error)
	LoadVault(ctx context.Context, account, id int64, initial uint16, version string, space ...byte) (VaultState, error)
	HasActivePremium(ctx context.Context, account int64, premiumType uint8, now time.Time) (bool, error)
	CountShopPurchases(ctx context.Context, scope ShopPurchaseScope, accountID, characterID int64, npcID, template uint32, start time.Time) (int, error)
	RecordShopPurchase(ctx context.Context, tx db.Tx, accountID, characterID int64, npcID, template uint32) error
}
