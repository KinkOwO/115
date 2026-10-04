package database

import (
	"context"
)

// HoldAdminGuard 持有游戏会话共享锁，与启动器管理写入协调启动时序。
func (s *Store) HoldAdminGuard(ctx context.Context) (func(), error) {
	// The guard is a shared advisory lock on PostgreSQL and a lease file on
	// SQLite, so it belongs to the engine (see engine.go).
	return s.engine.holdAdminGuard(ctx)
}
