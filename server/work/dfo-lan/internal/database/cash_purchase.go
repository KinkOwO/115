package database

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type CashOrder = cashshop.CashOrder
type CashOrderLine = cashshop.CashOrderLine
type CashDelivery = cashshop.CashDelivery
type CashPremium = cashshop.CashPremium
type CashReceipt = cashshop.CashReceipt

func (s *Store) MigrateCashShop(ctx context.Context) error {
	e := s.execMigration(ctx, "0029_cash_shop.sql")
	if e != nil {
		return e
	}
	if err := s.migratePackagePlaceholders(ctx); err != nil {
		return err
	}
	return s.migrateCoinItems(ctx)
}

func (s *Store) migrateCoinItems(ctx context.Context) error {
	rows, err := s.queries.CoinItemMigrationCandidates(ctx)
	if err != nil {
		return err
	}

	type charUpdate struct {
		id    int64
		state json.RawMessage
	}
	var updates []charUpdate

	for _, row := range rows {
		id, stateRaw := row.ID, row.State

		var stateMap map[string]json.RawMessage
		if err := json.Unmarshal(stateRaw, &stateMap); err != nil {
			continue
		}
		invRaw, ok := stateMap["inventory"]
		if !ok {
			continue
		}

		type bagItem struct {
			Slot       uint16 `json:"slot"`
			Template   uint32 `json:"Template"`
			Amount     uint32 `json:"Amount"`
			ExpireTime uint32 `json:"expire_time,omitempty"`
		}
		type bagStruct struct {
			Expansion byte            `json:"expansion,omitempty"`
			Version   string          `json:"version"`
			Gold      uint32          `json:"gold"`
			Coin      uint32          `json:"coin,omitempty"`
			Items     []bagItem       `json:"items"`
			Equipment json.RawMessage `json:"equipment,omitempty"`
			Worn      json.RawMessage `json:"worn,omitempty"`
			Special   json.RawMessage `json:"special_equipment,omitempty"`
		}

		var b bagStruct
		if err := json.Unmarshal(invRaw, &b); err != nil {
			continue
		}

		var newItems []bagItem
		coinCount := uint32(0)
		for _, it := range b.Items {
			if it.Template == 1 {
				coinCount += it.Amount
			} else {
				newItems = append(newItems, it)
			}
		}

		if coinCount == 0 {
			continue
		}

		if uint64(b.Coin)+uint64(coinCount) > math.MaxUint32 {
			b.Coin = math.MaxUint32
		} else {
			b.Coin += coinCount
		}
		b.Items = newItems

		newInvRaw, err := mergeCashInventoryProjection(invRaw, b)
		if err != nil {
			return err
		}
		stateMap["inventory"] = newInvRaw
		newStateRaw, err := json.Marshal(stateMap)
		if err != nil {
			return err
		}
		updates = append(updates, charUpdate{id: id, state: newStateRaw})
	}

	for _, u := range updates {
		if err := s.queries.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: u.id, State: u.state}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migratePackagePlaceholders(ctx context.Context) error {
	pkgTemplate := uint32(590722921)
	subTemplates := []uint32{590722922, 590722923, 590722926, 590722927, 590722928, 590722929}
	const maxExpireTime = uint32(math.MaxInt32)

	// Also find characters with sub-boxes whose expire_time is not set
	rows, err := s.queries.PackagePlaceholderMigrationCandidates(ctx)
	if err != nil {
		return err
	}

	type charUpdate struct {
		id    int64
		state json.RawMessage
	}
	var updates []charUpdate

	for _, row := range rows {
		id, stateRaw := row.ID, row.State

		var stateMap map[string]json.RawMessage
		if err := json.Unmarshal(stateRaw, &stateMap); err != nil {
			continue
		}
		invRaw, ok := stateMap["inventory"]
		if !ok {
			continue
		}

		type bagItem struct {
			Slot       uint16 `json:"slot"`
			Template   uint32 `json:"Template"`
			Amount     uint32 `json:"Amount"`
			ExpireTime uint32 `json:"expire_time,omitempty"`
		}
		type bagStruct struct {
			Expansion byte            `json:"expansion,omitempty"`
			Version   string          `json:"version"`
			Gold      uint32          `json:"gold"`
			Items     []bagItem       `json:"items"`
			Equipment json.RawMessage `json:"equipment,omitempty"`
			Worn      json.RawMessage `json:"worn,omitempty"`
			Special   json.RawMessage `json:"special_equipment,omitempty"`
		}

		var b bagStruct
		if err := json.Unmarshal(invRaw, &b); err != nil {
			continue
		}

		occupied := map[uint16]bool{}
		pkgCount := uint32(0)
		var newItems []bagItem
		needsUpdate := false

		isSubBox := func(t uint32) bool {
			for _, st := range subTemplates {
				if st == t {
					return true
				}
			}
			return false
		}

		for _, it := range b.Items {
			if it.Template == pkgTemplate {
				pkgCount += it.Amount
				needsUpdate = true
			} else {
				if isSubBox(it.Template) && it.ExpireTime != maxExpireTime {
					it.ExpireTime = maxExpireTime
					needsUpdate = true
				}
				occupied[it.Slot] = true
				newItems = append(newItems, it)
			}
		}

		if pkgCount > 0 {
			nextSlot := uint16(65)
			for _, subTpl := range subTemplates {
				for nextSlot <= 120 && occupied[nextSlot] {
					nextSlot++
				}
				if nextSlot > 120 {
					return fmt.Errorf("character %d inventory full during package migration", id)
				}
				newItems = append(newItems, bagItem{
					Slot:       nextSlot,
					Template:   subTpl,
					Amount:     pkgCount,
					ExpireTime: maxExpireTime,
				})
				occupied[nextSlot] = true
				nextSlot++
			}
		}

		if !needsUpdate {
			continue
		}

		b.Items = newItems
		newInvRaw, err := mergeCashInventoryProjection(invRaw, b)
		if err != nil {
			return err
		}
		stateMap["inventory"] = newInvRaw
		newStateRaw, err := json.Marshal(stateMap)
		if err != nil {
			return err
		}
		updates = append(updates, charUpdate{id: id, state: newStateRaw})
	}

	for _, u := range updates {
		if err := s.queries.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: u.id, State: u.state}); err != nil {
			return err
		}
	}
	return nil
}

// PurchaseCash atomically debits the account and deposits complete, unopened
// items in its cash inventory. A failed insert rolls back the entire debit.
// Replay returns the original receipt, not a receipt recomputed at today's price.
func (s *Store) PurchaseCash(ctx context.Context, o CashOrder) (CashReceipt, bool, error) {
	if o.DeliveryMode != "" {
		return CashReceipt{}, false, fmt.Errorf("unexpected cash delivery mode")
	}
	return s.purchaseCash(ctx, o, nil, nil)
}

// PurchaseCashPremium activates an account contract atomically with the CERA
// debit. No contract wrapper or placeholder is inserted into cash_inventory.
func (s *Store) PurchaseCashPremium(ctx context.Context, o CashOrder, premiumType uint8, durationSecond int64) (CashReceipt, bool, error) {
	if o.DeliveryMode != "" || premiumType == 0 || durationSecond <= 0 {
		return CashReceipt{}, false, fmt.Errorf("invalid premium purchase")
	}
	return s.purchaseCash(ctx, o, nil, map[int]CashPremiumActivation{0: {Type: premiumType, DurationSecond: durationSecond}})
}

// PurchaseCashMixed charges one order inside a single transaction: every line
// named in premiums activates its account contract (no inventory row), the
// remaining lines deliver through the caller's bag mutation.
func (s *Store) PurchaseCashMixed(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), premiums map[int]CashPremiumActivation) (CashReceipt, bool, error) {
	if o.DeliveryMode != "" {
		return CashReceipt{}, false, fmt.Errorf("unexpected cash delivery mode")
	}
	if deliver == nil {
		return CashReceipt{}, false, fmt.Errorf("missing bag delivery")
	}
	if len(premiums) == 0 || len(premiums) > len(o.Lines) {
		return CashReceipt{}, false, fmt.Errorf("invalid contract cart")
	}
	o.DeliveryMode = "bag-v1"
	return s.purchaseCash(ctx, o, deliver, premiums)
}

// PurchaseCashToBag executes a pure, source-backed inventory mutation under
// the same lock and transaction as the debit and audit. On replay it returns
// the current character state, never an old inventory snapshot to overwrite it.
func (s *Store) PurchaseCashToBag(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (CashReceipt, bool, error) {
	if deliver == nil || o.DeliveryMode != "" {
		return CashReceipt{}, false, fmt.Errorf("missing or invalid bag delivery")
	}
	o.DeliveryMode = "bag-v1"
	return s.purchaseCash(ctx, o, deliver, nil)
}

func (s *Store) PurchaseCashVault(ctx context.Context, o CashOrder, upgrade func(VaultState) (VaultState, error)) (CashReceipt, bool, error) {
	if upgrade == nil || o.DeliveryMode != "" || (o.VaultSpace != 0 && o.VaultSpace != 45 && o.VaultSpace != 12) {
		return CashReceipt{}, false, fmt.Errorf("invalid vault purchase")
	}
	o.DeliveryMode = "vault-upgrade-v1"
	return s.purchaseCash(ctx, o, nil, nil, upgrade)
}

// 共用首档商品不携带金库编号。重放先沿用原订单目标；新请求依据
// 金库 1 的服务器存档区分：8 格可升金库 1，达到 200 格才可升金库 2。
// 真正扣款时仍在锁内重新校验档位，不能借此跳档或重复领取。
func (s *Store) VaultPurchaseSpace(ctx context.Context, account, character int64, key string) (byte, error) {
	space, err := s.queries.CashOrderVaultSpace(ctx, sqlcgen.CashOrderVaultSpaceParams{AccountID: account, CharacterID: character, OrderKey: key})
	if err == nil {
		if space < 0 || space > math.MaxUint8 {
			return 0, fmt.Errorf("invalid stored vault purchase space: %d", space)
		}
		return byte(space), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	vault, err := s.queries.OwnedPrimaryVault(ctx, sqlcgen.OwnedPrimaryVaultParams{AccountID: account, CharacterID: character})
	if err != nil {
		return 0, err
	}
	if vault.Slots >= 200 {
		return 45, nil
	}
	return 0, nil
}

type CashPremiumActivation = cashshop.CashPremiumActivation

// purchaseCash keys activations by order-line index: a contract line never
// reaches cash_inventory, an ordinary line never activates a contract.
func (s *Store) purchaseCash(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), premiums map[int]CashPremiumActivation, upgrades ...func(VaultState) (VaultState, error)) (CashReceipt, bool, error) {
	var receipt CashReceipt
	cost, goldCost, e := o.Totals()
	if e != nil {
		return receipt, false, e
	}
	if goldCost > 0 && (deliver == nil || len(upgrades) > 0) {
		return receipt, false, fmt.Errorf("Gold purchase requires atomic bag delivery")
	}
	raw, e := json.Marshal(o)
	if e != nil {
		return receipt, false, e
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return receipt, false, e
	}
	defer tx.Rollback(ctx)
	// Currency before character matches the GM and existing grant lock order.
	q := s.queries.WithTx(tx)
	if e = q.EnsureAccountCurrency(ctx, o.Account); e != nil {
		return receipt, false, e
	}
	balance, e := q.LockAccountCurrency(ctx, o.Account)
	if e != nil {
		return receipt, false, e
	}
	role, e := lockCharacter(ctx, tx, o.Account, o.Character)
	if e != nil {
		return receipt, false, e
	}
	version, state := role.ConfigVersion, role.State
	var vault VaultState
	var vaultGold uint32
	secondary := o.VaultSpace == 45
	if len(upgrades) > 0 {
		if o.VaultSpace == 45 {
			// 0x1469DC950：金库 1 的零基档位必须大于 11，即至少 200 格。
			primary, err := q.LockPrimaryVault(ctx, o.Character)
			if err != nil {
				return receipt, false, err
			}
			if primary.Slots < 200 {
				return receipt, false, fmt.Errorf("金库 1 须达到 200 格才能升级金库 2")
			}
		}
		if o.VaultSpace == 12 {
			// 按账号锁定，与材料/金币升级及物品存取共用 account_vaults。
			// 未开通时不创建免费容量，读不到存档即拒绝且不扣点券。
			shared, err := lockSharedVault(ctx, q, o.Account)
			e = err
			vault = VaultState{Slots: shared.Slots, Items: shared.Items, ConfigVersion: version}
			vaultGold = shared.Gold
		} else {
			vault, e = lockPersonalVault(ctx, q, o.Character, secondary)
		}
		if e != nil {
			return receipt, false, e
		}
	}
	prior, e := q.CashOrderReceipt(ctx, sqlcgen.CashOrderReceiptParams{AccountID: o.Account, OrderKey: o.Key})
	if e == nil {
		if prior.Digest != digest {
			return receipt, false, fmt.Errorf("cash order key conflicts with existing request")
		}
		if e = json.Unmarshal(prior.Receipt, &receipt); e != nil {
			return receipt, false, e
		}
		if deliver != nil || len(premiums) > 0 {
			receipt.CharacterState = state
		}
		if len(upgrades) > 0 {
			receipt.CharacterState = state
			receipt.Vault = &vault
			receipt.VaultSpace = o.VaultSpace
			receipt.VaultGold = vaultGold
		}
		return receipt, false, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return receipt, false, e
	}
	if version != o.Source {
		return receipt, false, fmt.Errorf("cash order catalog does not match character source")
	}
	if balance < 0 || uint64(balance) < cost {
		return receipt, false, fmt.Errorf("insufficient CERA")
	}
	if balance > math.MaxInt32 {
		return receipt, false, fmt.Errorf("CERA balance exceeds native range")
	}
	receipt = CashReceipt{Order: o.Key, Before: uint64(balance), After: uint64(balance) - cost, Charged: cost, GoldCharged: goldCost}
	if len(premiums) > 0 {
		now := time.Now().Unix()
		indexes := make([]int, 0, len(premiums))
		for i := range premiums {
			indexes = append(indexes, i)
		}
		sort.Ints(indexes)
		receipt.CharacterState = state
		receipt.Premiums = make([]CashPremium, 0, len(premiums))
		for _, i := range indexes {
			premium := premiums[i]
			if premium.DurationSecond <= 0 || premium.DurationSecond > math.MaxInt64 {
				return CashReceipt{}, false, fmt.Errorf("invalid premium duration")
			}
			oldEnd, err := q.LockPremiumExpiry(ctx, sqlcgen.LockPremiumExpiryParams{AccountID: o.Account, PremiumType: int16(premium.Type)})
			e = err
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return CashReceipt{}, false, e
			}
			base := now
			if oldEnd > base {
				base = oldEnd
			}
			end := base + premium.DurationSecond
			if end <= base {
				return CashReceipt{}, false, fmt.Errorf("premium expiry overflow")
			}
			if e = q.SavePremiumExpiry(ctx, sqlcgen.SavePremiumExpiryParams{AccountID: o.Account, PremiumType: int16(premium.Type), EndTime: end}); e != nil {
				return CashReceipt{}, false, e
			}
			receipt.Premiums = append(receipt.Premiums, CashPremium{Type: premium.Type, EndTime: end, RemainingSecond: end - now})
		}
	}
	if deliver != nil {
		receipt.CharacterState, e = deliver(state)
		if e != nil {
			return CashReceipt{}, false, e
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(receipt.CharacterState, &object) != nil || object == nil {
			return CashReceipt{}, false, fmt.Errorf("invalid bag delivery state")
		}
	}
	if len(upgrades) > 0 {
		input := vault
		input.Items = append(json.RawMessage(nil), vault.Items...)
		next, err := upgrades[0](input)
		if err != nil {
			return CashReceipt{}, false, err
		}
		if next.Slots <= vault.Slots || next.ConfigVersion != vault.ConfigVersion || string(next.Items) != string(vault.Items) {
			return CashReceipt{}, false, fmt.Errorf("invalid vault upgrade mutation")
		}
		receipt.CharacterState = state
		receipt.Vault = &next
		receipt.VaultSpace = o.VaultSpace
		receipt.VaultGold = vaultGold
	}
	if e = q.RecordCashOrder(ctx, sqlcgen.RecordCashOrderParams{AccountID: o.Account, OrderKey: o.Key, CharacterID: o.Character, Digest: digest, Request: raw}); e != nil {
		return receipt, false, e
	}
	if e = q.SaveAccountCurrency(ctx, sqlcgen.SaveAccountCurrencyParams{AccountID: o.Account, Cera: int64(receipt.After)}); e != nil {
		return receipt, false, e
	}
	if deliver != nil {
		if e = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: o.Character, State: receipt.CharacterState}); e != nil {
			return CashReceipt{}, false, e
		}
	}
	if receipt.Vault != nil {
		if o.VaultSpace == 12 {
			e = q.SaveAccountVaultSlots(ctx, sqlcgen.SaveAccountVaultSlotsParams{AccountID: o.Account, Slots: int32(receipt.Vault.Slots)})
		} else if secondary {
			e = q.SaveSecondaryVaultSlots(ctx, sqlcgen.SaveSecondaryVaultSlotsParams{CharacterID: o.Character, Slots: int32(receipt.Vault.Slots)})
		} else {
			e = q.SavePrimaryVaultSlots(ctx, sqlcgen.SavePrimaryVaultSlotsParams{CharacterID: o.Character, Slots: int32(receipt.Vault.Slots)})
		}
		if e != nil {
			return CashReceipt{}, false, e
		}
	}
	for i, l := range o.Lines {
		d := CashDelivery{Product: l.Product, Template: l.Template, Amount: l.Quantity * l.Units, Quantity: l.Quantity}
		if _, isContract := premiums[i]; isContract {
			receipt.Deliveries = append(receipt.Deliveries, d)
			continue
		}
		if d.ID, e = q.RecordCashInventory(ctx, sqlcgen.RecordCashInventoryParams{AccountID: o.Account, CharacterID: o.Character, OrderKey: o.Key, LineIndex: int32(i), Product: int64(l.Product), Template: int64(l.Template), Amount: int64(d.Amount)}); e != nil {
			return CashReceipt{}, false, e
		}
		receipt.Deliveries = append(receipt.Deliveries, d)
	}
	if deliver != nil || len(upgrades) > 0 {
		if e = q.MarkCashOrderDelivered(ctx, sqlcgen.MarkCashOrderDeliveredParams{AccountID: o.Account, OrderKey: o.Key}); e != nil {
			return CashReceipt{}, false, e
		}
	}
	saved, e := json.Marshal(receipt)
	if e != nil {
		return CashReceipt{}, false, e
	}
	if e = q.SaveCashOrderReceipt(ctx, sqlcgen.SaveCashOrderReceiptParams{AccountID: o.Account, OrderKey: o.Key, Receipt: saved}); e != nil {
		return CashReceipt{}, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return CashReceipt{}, false, e
	}
	return receipt, true, nil
}

func (s *Store) CashInventory(ctx context.Context, account, character int64) ([]CashDelivery, error) {
	rows, e := s.queries.CashInventory(ctx, sqlcgen.CashInventoryParams{AccountID: account, CharacterID: character})
	if e != nil {
		return nil, e
	}
	out := []CashDelivery{}
	for _, row := range rows {
		if row.Product < 0 || row.Product > math.MaxUint32 || row.Template < 0 || row.Template > math.MaxUint32 || row.Amount < 0 || row.Amount > math.MaxUint32 {
			return nil, fmt.Errorf("cash inventory exceeds client range: %d", row.ID)
		}
		out = append(out, CashDelivery{ID: row.ID, Product: uint32(row.Product), Template: uint32(row.Template), Amount: uint32(row.Amount)})
	}
	return out, nil
}
