package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"time"
)

// HoldAdminGuard 持有游戏会话共享锁，与启动器管理写入协调启动时序。
func (s *Store) HoldAdminGuard(ctx context.Context) (func(), error) {
	c, err := s.db.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	release := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// 不把携带会话锁的连接归还连接池。
		_ = c.Conn().Close(closeCtx)
		c.Release()
	}
	ok, err := sqlcgen.New(c).TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		c.Release()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("已有 GM 写入正在进行，请稍后重试")
	}
	return release, nil
}
