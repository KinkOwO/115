package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
)

func MoveAccountVaultCross(account, personal Vault, limit uint32, r protocol.ItemMoveRequest, items catalog.LootCatalog, equipment *EquipmentCatalog) (Vault, Vault, uint32, error) {
	if account.Slots == 0 {
		return account, personal, 0, fmt.Errorf("账号金库尚未开通")
	}
	if !((r.SourceList == 2 || r.SourceList == 45) && r.DestinationList == 12) && !(r.SourceList == 12 && (r.DestinationList == 2 || r.DestinationList == 45)) {
		return account, personal, 0, fmt.Errorf("不是账号金库跨库移动")
	}
	if r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return account, personal, 0, fmt.Errorf("账号金库跨库请求无效")
	}
	if r.SourceList != 12 {
		from := personal.ItemAt(r.SourceSlot)
		if from == nil || from.Template != r.SourceItem {
			return account, personal, 0, fmt.Errorf("个人金库源物品不存在")
		}
		if err := accountVaultItemAllowed(*from, items, equipment); err != nil {
			return account, personal, 0, err
		}
	}
	move := r
	if move.SourceList == 12 {
		move.SourceList = 2
	} else {
		move.SourceList = 45
	}
	if move.DestinationList == 12 {
		move.DestinationList = 2
	} else {
		move.DestinationList = 45
	}
	return MoveVaultCross(account, personal, limit, move)
}

func ReadAccountVault(v storage.AccountVaultState) (Vault, error) {
	return ReadExtendedVault(storage.VaultState{Slots: v.Slots, Items: v.Items})
}
