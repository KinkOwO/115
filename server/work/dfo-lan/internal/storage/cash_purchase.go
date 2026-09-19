package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"

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
type CashReceipt struct {
	Order          string          `json:"order"`
	Before         uint64          `json:"before"`
	After          uint64          `json:"after"`
	Charged        uint64          `json:"charged"`
	Deliveries     []CashDelivery  `json:"deliveries"`
	CharacterState json.RawMessage `json:"character_state,omitempty"`
	Vault          *VaultState     `json:"vault,omitempty"`
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
 FOREIGN KEY(account_id,order_key) REFERENCES cash_orders(account_id,order_key));`)
	if e != nil {
		return e
	}
	return s.migratePackagePlaceholders(ctx)
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
	return s.purchaseCash(ctx, o, nil)
}

// PurchaseCashToBag executes a pure, source-backed inventory mutation under
// the same lock and transaction as the debit and audit. On replay it returns
// the current character state, never an old inventory snapshot to overwrite it.
func (s *Store) PurchaseCashToBag(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (CashReceipt, bool, error) {
	if deliver == nil || o.DeliveryMode != "" {
		return CashReceipt{}, false, fmt.Errorf("missing or invalid bag delivery")
	}
	o.DeliveryMode = "bag-v1"
	return s.purchaseCash(ctx, o, deliver)
}

func (s *Store) PurchaseCashVault(ctx context.Context, o CashOrder, upgrade func(VaultState) (VaultState, error)) (CashReceipt, bool, error) {
	if upgrade == nil || o.DeliveryMode != "" {
		return CashReceipt{}, false, fmt.Errorf("invalid vault purchase")
	}
	o.DeliveryMode = "vault-upgrade-v1"
	return s.purchaseCash(ctx, o, nil, upgrade)
}

func (s *Store) purchaseCash(ctx context.Context, o CashOrder, deliver func(json.RawMessage) (json.RawMessage, error), upgrades ...func(VaultState) (VaultState, error)) (CashReceipt, bool, error) {
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
	if len(upgrades) > 0 {
		if e = tx.QueryRow(ctx, `SELECT slots,items,config_version FROM character_vaults WHERE character_id=$1 FOR UPDATE`, o.Character).Scan(&vault.Slots, &vault.Items, &vault.ConfigVersion); e != nil {
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
		if deliver != nil {
			receipt.CharacterState = state
		}
		if len(upgrades) > 0 {
			receipt.CharacterState = state
			receipt.Vault = &vault
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
		if _, e = tx.Exec(ctx, `UPDATE character_vaults SET slots=$2,updated_at=now() WHERE character_id=$1`, o.Character, receipt.Vault.Slots); e != nil {
			return CashReceipt{}, false, e
		}
	}
	for i, l := range o.Lines {
		d := CashDelivery{Product: l.Product, Template: l.Template, Amount: l.Quantity * l.Units, Quantity: l.Quantity}
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
