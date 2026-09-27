package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// 征兆（omen）的**角色存档**。
//
// 征兆不是道具：把内层 PVF 的两张本地化文本表翻遍，`Stackable.uv.str` 里跟
// 「Omen」有关的只有 `Omen of Order Reward (CS)` 那几个**奖励盒**，`equipment.uv.str`
// 里一个都没有；2836 记录里那 4 个 u32 也不是「拿在手里的东西」，而是拿去做
// 奖励预览的模板号。所以官服的「携带征兆」= 角色存档里的一个占位标记，
// 客户端从 noti 2836 读它点亮 UI，服务端从这张表读它决定下一次通关怎么推进。
//
// 为什么必须落库而不是内存账本：征兆是跨场累积的玩家状态（官方：「通关时随机
// 累积」），重启归零会让玩家永远攒不满四档；而且它绑定**角色**，不是绑定服务端
// 会话（同一个 gateway 会承载多个角色/多个副本实例）。见
// docs/protocol/endkeeper-of-order-primer-20260926.md §38/§39。

// OmenState 是一个角色在某副本上的征兆存档快照。
type OmenState struct {
	// Held 是当前持有的征兆档数（0..4）。0 = 这一轮还没有征兆。
	Held int
	// OrthairePending 为真表示「上一场刚满档结算过，下一场该召唤隐藏 BOSS」。
	//
	// 它和 Held 分成两列，是因为两者归零的**时刻不同**：Held 在结算那一刻就
	// 归零（官方「结算征兆并重置」），而隐藏 BOSS 的机会要到**通关确认之后**才
	// 兑现 —— 掉线或退出不该吞掉已经攒到的那一次奥尔泰尔（与 oath_progress.go
	// 同一条教训）。
	OrthaierPending bool
}

// MigrateOmenState 建征兆的角色存档表。
//
// 表按 (角色, 副本) 记：征兆阶段表本来就是按副本挂的（[coupon drop table] 在
// 副本自己的 .dgn 里），两个深渊各攒各的账。
func (s *Store) MigrateOmenState(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_omen_state (
 character_id bigint NOT NULL REFERENCES characters(id), dungeon_id bigint NOT NULL CHECK(dungeon_id>0),
 held integer NOT NULL DEFAULT 0 CHECK(held>=0),
 orthaire_pending boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,dungeon_id));`)
	return err
}

// OmenState 读存档。没有记录就是「没有征兆、也没有待出的隐藏 BOSS」。
func (s *Store) OmenState(ctx context.Context, characterID, dungeonID int64) (OmenState, error) {
	if characterID <= 0 || dungeonID <= 0 {
		return OmenState{}, errors.New("invalid omen state key")
	}
	var out OmenState
	err := s.DB.QueryRow(ctx,
		`SELECT held, orthaire_pending FROM character_omen_state WHERE character_id=$1 AND dungeon_id=$2`,
		characterID, dungeonID).Scan(&out.Held, &out.OrthaierPending)
	if errors.Is(err, pgx.ErrNoRows) {
		return OmenState{}, nil
	}
	if err != nil {
		return OmenState{}, err
	}
	return out, nil
}

// SaveOmenHeld 只写持有档数，不动隐藏 BOSS 标记。
//
// 拆成两个**单列** upsert 而不是「读整行 → 改一列 → 整行回写」：这两列由不同
// 时刻推进（结算那一刻写 held，通关那一刻清 pending），整行回写会把对方刚写的
// 内容盖掉，而且中间没有事务隔离。
func (s *Store) SaveOmenHeld(ctx context.Context, characterID, dungeonID int64, held int) error {
	if characterID <= 0 || dungeonID <= 0 || held < 0 {
		return errors.New("invalid omen held")
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO character_omen_state(character_id,dungeon_id,held,updated_at) VALUES($1,$2,$3,now())
 ON CONFLICT(character_id,dungeon_id) DO UPDATE SET held=EXCLUDED.held, updated_at=now()`,
		characterID, dungeonID, held)
	return err
}

// SetOmenOrthaierPending 置 / 清「下一场该出隐藏 BOSS」。同样只动这一列。
func (s *Store) SetOmenOrthaierPending(ctx context.Context, characterID, dungeonID int64, pending bool) error {
	if characterID <= 0 || dungeonID <= 0 {
		return errors.New("invalid omen pending key")
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO character_omen_state(character_id,dungeon_id,orthaire_pending,updated_at) VALUES($1,$2,$3,now())
 ON CONFLICT(character_id,dungeon_id) DO UPDATE SET orthaire_pending=EXCLUDED.orthaire_pending, updated_at=now()`,
		characterID, dungeonID, pending)
	return err
}
