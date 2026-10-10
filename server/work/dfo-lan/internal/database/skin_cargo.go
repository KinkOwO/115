package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"math"
	"time"
)

// AccountSkin is one skin a player registered by using an
// `[action type] [add skin storage]` stackable (CMD507 action 169).
type AccountSkin struct {
	SourceTemplate uint32    `json:"source_template"`
	SkinKey        uint32    `json:"skin_key"`
	UnlockedAt     time.Time `json:"unlocked_at"`
}

// MigrateSkinCargo creates the account-shared skin cargo, upgrading the shape an
// earlier version of this table left behind. The upgrade only touches this table,
// so existing character and item saves are untouched.
func (s *Store) MigrateSkinCargo(ctx context.Context) error {
	return s.execMigration(ctx, "0015_skin_cargo.sql")
}

// UnlockSkin registers one skin for the account. The insert is idempotent on
// (account, template), so a replayed hotkey press cannot register twice, and a
// use whose registration failed after the item was already spent still lands on
// the next press.
func (s *Store) UnlockSkin(ctx context.Context, account int64, template, skinKey uint32) error {
	if account == 0 || template == 0 || skinKey == 0 {
		return fmt.Errorf("invalid skin unlock")
	}
	return s.queries.UnlockSkin(ctx, sqlcgen.UnlockSkinParams{AccountID: account, SourceTemplate: int64(template), SkinKey: int64(skinKey)})
}

// UnlockSkins 批量解锁皮肤仓库条目（账号级）。既有 UnlockSkin 是 ON CONFLICT(account_id,source_template)
// DO NOTHING ⇒ 天然幂等；批量版只是把同一批插入放进**一个事务**里（全解锁量级约 1.7k 行，
// 逐条隐式事务在 SQLite 上会明显拖慢建号路径），并返回本次真实新增的行数。
//
// 背景：唯一既有调用点是 CMD507 action 169（用 [add skin storage] 消耗品，见
// cmd/wireprobe/skin_storage_flow.go:65）。奖励脚本要"新角色直接全解锁"，需要一条账号级批量口。
//
// 为什么必须批量：SQLite 上一条裸 INSERT 就是一次隐式事务 = 一次 WAL 提交，1.7k 条会把建号路径
// 拖到几十秒；放进一个显式事务后整批只有一次提交，写出来的东西与逐条 UnlockSkin 完全相同。
//
// 为什么幂等安全：写语句就是既有 UnlockSkin 的那一条，冲突目标仍是主键 (account_id, source_template)，
// DO NOTHING 让重放、脚本重跑、同一批里的重复条目既不报错也不覆盖已经存下的 skin_key
// （先到先得）；两张表结构、协议与存档形态都没动。
//
// 新增行数怎么来：既有 UnlockSkin 是 sqlc 的 :exec，适配层只回传 error，拿不到 RowsAffected，
// 而补一条 :execrows 查询要改 sqlcgen/queryset/生成的 SQL（本任务不允许）。所以这里用
// 「先读已有集合、再统计本次新增」：begin 走 DSN 的 _txlock=immediate（见 internal/database/engine.go:71），
// 事务一开就持有写锁，同一事务内先读后写的这两步不会被别的写者插队，读到的 pre-image 与插入基线一致。
//
// 因为主键是 (account_id, source_template)，"新增行数"按模板算：数据库里已有该模板 ⇒ 本轮不写、不计数；
// 同一批里第二次出现的模板同理（两者都与 DO NOTHING 等价，只是省掉一次空写）。
// 于是 added 恰好等于真实插入行数，重跑一次全解锁会返回 0 而不是再插 1.7k 条。
func (s *Store) UnlockSkins(ctx context.Context, account int64, skins []AccountSkin) (int, error) {
	// 空批次（含 nil）：不改盘，也不开事务——某个职业/等级没有可解锁条目时不该碰存储。
	if len(skins) == 0 {
		return 0, nil
	}
	if account == 0 {
		return 0, fmt.Errorf("invalid skin unlock: account id is 0")
	}
	// 先整批校验、再动盘：批里只要有一条模板号或皮肤号为 0，整批拒绝，绝不写一半
	// （半批写入会让调用方无法判断"重跑一次"够不够，而 0 号模板本身也不是一条皮肤）。
	// 报错带上出错的模板号与下标，方便奖励脚本直接定位是哪条清单条目坏了。
	for i, skin := range skins {
		if skin.SourceTemplate == 0 || skin.SkinKey == 0 {
			return 0, fmt.Errorf("invalid skin unlock at index %d: template=%d skin=%d", i, skin.SourceTemplate, skin.SkinKey)
		}
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.rollback(ctx)
	rows, err := tx.queries().ListSkins(ctx, account)
	if err != nil {
		return 0, err
	}
	stored, err := listSkinsRows(rows)
	if err != nil {
		return 0, err
	}
	// pre-image 按模板去重：ListSkins 已滤掉 skin_key=0，而上面已保证入参皮肤号非 0，两边口径一致。
	existing := make(map[uint32]struct{}, len(stored))
	for _, row := range stored {
		existing[row.SourceTemplate] = struct{}{}
	}
	added := 0
	for _, skin := range skins {
		if _, ok := existing[skin.SourceTemplate]; ok {
			continue
		}
		existing[skin.SourceTemplate] = struct{}{}
		if err := tx.queries().UnlockSkin(ctx, sqlcgen.UnlockSkinParams{
			AccountID:      account,
			SourceTemplate: int64(skin.SourceTemplate),
			SkinKey:        int64(skin.SkinKey),
		}); err != nil {
			return 0, err
		}
		added++
	}
	if err = tx.commit(ctx); err != nil {
		return 0, err
	}
	return added, nil
}

// ListSkins returns every skin registered to the account, oldest unlock first. A
// row whose key is still unknown is left out rather than sent: skin id 0 is not a
// skin, and the page frame rebuilds the whole cargo from the ids it carries.
func (s *Store) ListSkins(ctx context.Context, account int64) ([]AccountSkin, error) {
	rows, err := s.queries.ListSkins(ctx, account)
	if err != nil {
		return nil, err
	}
	return listSkinsRows(rows)
}

// listSkinsRows 把生成层的行收成领域条目，并挡掉超出 uint32 的脏值。
// ListSkins 与 UnlockSkins 共用它，同一张表的宽度校验只保留一处。
func listSkinsRows(rows []sqlcgen.ListSkinsRow) ([]AccountSkin, error) {
	var out []AccountSkin
	for _, row := range rows {
		if row.SourceTemplate < 0 || row.SourceTemplate > math.MaxUint32 || row.SkinKey < 0 || row.SkinKey > math.MaxUint32 {
			return nil, fmt.Errorf("stored skin cargo id out of uint32 range")
		}
		out = append(out, AccountSkin{SourceTemplate: uint32(row.SourceTemplate), SkinKey: uint32(row.SkinKey), UnlockedAt: row.UnlockedAt})
	}
	return out, nil
}
