package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// CashOrder is constructed from a server-side catalog, never client prices.
type CashOrder struct {
	Key          string          `json:"key"`
	Account      int64           `json:"account"`
	Character    int64           `json:"character"`
	Source       string          `json:"source"`
	Lines        []CashOrderLine `json:"lines"`
	DeliveryMode string          `json:"delivery_mode,omitempty"`
	// 零值沿用已发布的金库 1 订单摘要；45 为第二金库，12 为账号金库。
	VaultSpace byte `json:"vault_space,omitempty"`
}
type CashOrderLine struct {
	Product   uint32 `json:"product"`
	Template  uint32 `json:"template"`
	Quantity  uint32 `json:"quantity"`
	Units     uint32 `json:"units"`
	UnitPrice uint32 `json:"unit_price"`
}
type CashDelivery struct {
	ID       int64  `json:"id"`
	Product  uint32 `json:"product"`
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
	Quantity uint32 `json:"quantity,omitempty"`
}
type CashPremium struct {
	Type            uint8 `json:"type"`
	EndTime         int64 `json:"end_time"`
	RemainingSecond int64 `json:"remaining_seconds"`
}
type CashReceipt struct {
	Order          string          `json:"order"`
	Before         uint64          `json:"before"`
	After          uint64          `json:"after"`
	Charged        uint64          `json:"charged"`
	Deliveries     []CashDelivery  `json:"deliveries"`
	Premiums       []CashPremium   `json:"premiums,omitempty"`
	CharacterState json.RawMessage `json:"character_state,omitempty"`
	Vault          *VaultState     `json:"vault,omitempty"`
	VaultSpace     byte            `json:"vault_space,omitempty"`
	VaultGold      uint32          `json:"vault_gold,omitempty"`
}

func (o CashOrder) total() (uint64, error) {
	source, e := hex.DecodeString(o.Source)
	if e != nil || len(source) != 32 || o.Account <= 0 || o.Character <= 0 || len(o.Key) < 16 || len(o.Key) > 128 || len(o.Lines) == 0 || len(o.Lines) > 32 {
		return 0, fmt.Errorf("invalid cash order")
	}
	var total uint64
	for _, l := range o.Lines {
		if l.Product == 0 || l.Template == 0 || l.Quantity == 0 || l.Quantity > 1000 || l.Units == 0 || l.UnitPrice == 0 {
			return 0, fmt.Errorf("invalid cash order line")
		}
		if uint64(l.Quantity)*uint64(l.Units) > math.MaxUint32 {
			return 0, fmt.Errorf("cash delivery amount overflow")
		}
		total += uint64(l.Quantity) * uint64(l.UnitPrice)
		if total > math.MaxInt32 {
			return 0, fmt.Errorf("cash order price overflow")
		}
	}
	return total, nil
}

func (s *Store) MigrateCashShop(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS cash_orders(
 account_id bigint NOT NULL REFERENCES accounts(id), order_key text NOT NULL,
 character_id bigint NOT NULL REFERENCES characters(id), digest text NOT NULL,
 request jsonb NOT NULL, receipt jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,order_key));
 CREATE TABLE IF NOT EXISTS cash_inventory(
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id), character_id bigint NOT NULL REFERENCES characters(id),
 order_key text NOT NULL, line_index integer NOT NULL CHECK(line_index>=0),
 product bigint NOT NULL CHECK(product>0), template bigint NOT NULL CHECK(template>0),
 amount bigint NOT NULL CHECK(amount>0 AND amount<=4294967295),
 claimed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,order_key,line_index),
 FOREIGN KEY(account_id,order_key) REFERENCES cash_orders(account_id,order_key));
 CREATE TABLE IF NOT EXISTS account_premiums(
 account_id bigint NOT NULL REFERENCES accounts(id),
 premium_type smallint NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time bigint NOT NULL CHECK(end_time>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,premium_type));
 CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);`)
	if e != nil {
		return e
	}
	if err := s.migratePackagePlaceholders(ctx); err != nil {
		return err
	}
	return s.migrateCoinItems(ctx)
}

func (s *Store) migrateCoinItems(ctx context.Context) error {
	rows, err := s.DB.Query(ctx, `SELECT id, state FROM characters WHERE state->'inventory'->'items' @> '[{"Template": 1}]'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type charUpdate struct {
		id    int64
		state json.RawMessage
	}
	var updates []charUpdate

	for rows.Next() {
		var id int64
		var stateRaw json.RawMessage
		if err := rows.Scan(&id, &stateRaw); err != nil {
			return err
		}

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

		newInvRaw, err := json.Marshal(b)
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
		if _, err := s.DB.Exec(ctx, `UPDATE characters SET state = $1 WHERE id = $2`, u.state, u.id); err != nil {
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
	rows, err := s.DB.Query(ctx, `SELECT id, state FROM characters WHERE state->'inventory'->'items' @> '[{"Template": 590722921}]' OR state->'inventory'->'items' @> '[{"Template": 590722922}]'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type charUpdate struct {
		id    int64
		state json.RawMessage
	}
	var updates []charUpdate

	for rows.Next() {
		var id int64
		var stateRaw json.RawMessage
		if err := rows.Scan(&id, &stateRaw); err != nil {
			return err
		}

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
		newInvRaw, err := json.Marshal(b)
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
		if _, err := s.DB.Exec(ctx, `UPDATE characters SET state = $1 WHERE id = $2`, u.state, u.id); err != nil {
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
	var space byte
	err := s.DB.QueryRow(ctx, `SELECT coalesce((request->>'vault_space')::integer,0) FROM cash_orders WHERE account_id=$1 AND character_id=$2 AND order_key=$3`, account, character, key).Scan(&space)
	if err == nil {
		return space, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	var slots uint16
	err = s.DB.QueryRow(ctx, `SELECT v.slots FROM character_vaults v JOIN characters c ON c.id=v.character_id WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, character).Scan(&slots)
	if err != nil {
		return 0, err
	}
	if slots >= 200 {
		return 45, nil
	}
	return 0, nil
}

type CashPremiumActivation struct {
	Type           uint8
	DurationSecond int64
}

// purchaseCash keys activations by order-line index: a contract line never
// reaches cash_inventory, an ordinary line never activates a contract.
func (s *Store) purchaseCash(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), premiums map[int]CashPremiumActivation, upgrades ...func(VaultState) (VaultState, error)) (CashReceipt, bool, error) {
	var receipt CashReceipt
	cost, e := o.total()
	if e != nil {
		return receipt, false, e
	}
	raw, e := json.Marshal(o)
	if e != nil {
		return receipt, false, e
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return receipt, false, e
	}
	defer tx.Rollback(ctx)
	// Currency before character matches the GM and existing grant lock order.
	if _, e = tx.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,0) ON CONFLICT DO NOTHING`, o.Account); e != nil {
		return receipt, false, e
	}
	var balance int64
	if e = tx.QueryRow(ctx, `SELECT cera FROM account_currency WHERE account_id=$1 FOR UPDATE`, o.Account).Scan(&balance); e != nil {
		return receipt, false, e
	}
	var version string
	var state json.RawMessage
	if e = tx.QueryRow(ctx, `SELECT config_version,state FROM characters WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL FOR UPDATE`, o.Character, o.Account).Scan(&version, &state); e != nil {
		return receipt, false, e
	}
	var vault VaultState
	var vaultGold uint32
	vaultTable := "character_vaults"
	if len(upgrades) > 0 {
		if o.VaultSpace == 45 {
			// 0x1469DC950：金库 1 的零基档位必须大于 11，即至少 200 格。
			var primarySlots uint16
			if e = tx.QueryRow(ctx, `SELECT slots FROM character_vaults WHERE character_id=$1 FOR UPDATE`, o.Character).Scan(&primarySlots); e != nil {
				return receipt, false, e
			}
			if primarySlots < 200 {
				return receipt, false, fmt.Errorf("金库 1 须达到 200 格才能升级金库 2")
			}
			vaultTable = "character_secondary_vaults"
		}
		if o.VaultSpace == 12 {
			// 按账号锁定，与材料/金币升级及物品存取共用 account_vaults。
			// 未开通时不创建免费容量，读不到存档即拒绝且不扣点券。
			e = tx.QueryRow(ctx, `SELECT slots,items,gold FROM account_vaults WHERE account_id=$1 FOR UPDATE`, o.Account).Scan(&vault.Slots, &vault.Items, &vaultGold)
			vault.ConfigVersion = version
		} else {
			e = tx.QueryRow(ctx, `SELECT slots,items,config_version FROM `+vaultTable+` WHERE character_id=$1 FOR UPDATE`, o.Character).Scan(&vault.Slots, &vault.Items, &vault.ConfigVersion)
		}
		if e != nil {
			return receipt, false, e
		}
	}
	var prior string
	var saved json.RawMessage
	e = tx.QueryRow(ctx, `SELECT digest,receipt FROM cash_orders WHERE account_id=$1 AND order_key=$2`, o.Account, o.Key).Scan(&prior, &saved)
	if e == nil {
		if prior != digest {
			return receipt, false, fmt.Errorf("cash order key conflicts with existing request")
		}
		if e = json.Unmarshal(saved, &receipt); e != nil {
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
	receipt = CashReceipt{Order: o.Key, Before: uint64(balance), After: uint64(balance) - cost, Charged: cost}
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
			var oldEnd int64
			if e = tx.QueryRow(ctx, `SELECT end_time FROM account_premiums WHERE account_id=$1 AND premium_type=$2 FOR UPDATE`, o.Account, premium.Type).Scan(&oldEnd); e != nil && !errors.Is(e, pgx.ErrNoRows) {
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
			if _, e = tx.Exec(ctx, `INSERT INTO account_premiums(account_id,premium_type,end_time,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT(account_id,premium_type) DO UPDATE SET end_time=EXCLUDED.end_time,updated_at=now()`, o.Account, premium.Type, end); e != nil {
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
	if _, e = tx.Exec(ctx, `INSERT INTO cash_orders(account_id,order_key,character_id,digest,request,receipt) VALUES($1,$2,$3,$4,$5,'{}')`, o.Account, o.Key, o.Character, digest, raw); e != nil {
		return receipt, false, e
	}
	if _, e = tx.Exec(ctx, `UPDATE account_currency SET cera=$2,updated_at=now() WHERE account_id=$1`, o.Account, receipt.After); e != nil {
		return receipt, false, e
	}
	if deliver != nil {
		if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, o.Character, receipt.CharacterState); e != nil {
			return CashReceipt{}, false, e
		}
	}
	if receipt.Vault != nil {
		if o.VaultSpace == 12 {
			_, e = tx.Exec(ctx, `UPDATE account_vaults SET slots=$2,updated_at=now() WHERE account_id=$1`, o.Account, receipt.Vault.Slots)
		} else {
			_, e = tx.Exec(ctx, `UPDATE `+vaultTable+` SET slots=$2,updated_at=now() WHERE character_id=$1`, o.Character, receipt.Vault.Slots)
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
		if e = tx.QueryRow(ctx, `INSERT INTO cash_inventory(account_id,character_id,order_key,line_index,product,template,amount) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, o.Account, o.Character, o.Key, i, l.Product, l.Template, d.Amount).Scan(&d.ID); e != nil {
			return CashReceipt{}, false, e
		}
		receipt.Deliveries = append(receipt.Deliveries, d)
	}
	if deliver != nil || len(upgrades) > 0 {
		if _, e = tx.Exec(ctx, `UPDATE cash_inventory SET claimed_at=now() WHERE account_id=$1 AND order_key=$2`, o.Account, o.Key); e != nil {
			return CashReceipt{}, false, e
		}
	}
	saved, e = json.Marshal(receipt)
	if e != nil {
		return CashReceipt{}, false, e
	}
	if _, e = tx.Exec(ctx, `UPDATE cash_orders SET receipt=$3 WHERE account_id=$1 AND order_key=$2`, o.Account, o.Key, saved); e != nil {
		return CashReceipt{}, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return CashReceipt{}, false, e
	}
	if deliver != nil {
		s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, o.Account))
	}
	return receipt, true, nil
}

func (s *Store) CashInventory(ctx context.Context, account, character int64) ([]CashDelivery, error) {
	rows, e := s.DB.Query(ctx, `SELECT i.id,i.product,i.template,i.amount FROM cash_inventory i JOIN characters c ON c.id=i.character_id WHERE i.account_id=$1 AND i.character_id=$2 AND c.account_id=$1 AND c.deleted_at IS NULL AND i.claimed_at IS NULL ORDER BY i.id`, account, character)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []CashDelivery{}
	for rows.Next() {
		var d CashDelivery
		if e = rows.Scan(&d.ID, &d.Product, &d.Template, &d.Amount); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
